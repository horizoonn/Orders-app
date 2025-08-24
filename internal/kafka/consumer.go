package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"
	"errors"
	"fmt"

	"project/internal/cache"
	"project/internal/models"
	"project/internal/repository"

	"github.com/segmentio/kafka-go"
)

// Consumer представляет Kafka-консьюмер
type Consumer struct {
	reader *kafka.Reader
	db     *storage.DB
	cache  *cache.OrderCache
}

// NewConsumer создает новый экземпляр Kafka-консьюмера.
func NewConsumer(brokers []string, topic, groupID string, db *storage.DB, cache *cache.OrderCache) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})
	return &Consumer{
		reader: reader,
		db:     db,
		cache:  cache,
	}
}

// Run запускает основной цикл чтения сообщений из Kafka.
func (c *Consumer) Run(ctx context.Context) {
	log.Println("Запуск Kafka-консьюмера...")
	for {
		select {
		case <-ctx.Done():
			log.Println("Контекст завершен, выключаю Kafka-консьюмер.")
			return
		default:
			m, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
					log.Printf("Ошибка чтения сообщения: %v", err)
				}
				continue
			}

			log.Printf("Получено сообщение из Kafka: %s", string(m.Value))

			var order models.Order
			if err := json.Unmarshal(m.Value, &order); err != nil {
				log.Printf("Ошибка парсинга сообщения: %v. Сообщение: %s. Пропускаем.", err, string(m.Value))
				c.commitMessage(ctx, m)
				continue
			}

			if order.OrderUID == "" {
				log.Printf("Невалидное сообщение: отсутствует order_uid. Пропускаем.")
				c.commitMessage(ctx, m)
				continue
			}

			if err := c.saveOrderWithRetry(ctx, order); err != nil {
				log.Printf("КРИТИЧЕСКАЯ ОШИБКА: не удалось сохранить заказ %s после нескольких попыток: %v. Сообщение НЕ будет закоммичено.", order.OrderUID, err)
				continue
			}

			// Шаг 4: Если все успешно, коммитим сообщение
			log.Printf("Заказ %s успешно обработан и сохранен.", order.OrderUID)
			c.commitMessage(ctx, m)
		}
	}
}

// saveOrderWithRetry пытается сохранить заказ в БД с несколькими попытками и экспоненциальной задержкой
func (c *Consumer) saveOrderWithRetry(ctx context.Context, order models.Order) error {
	const maxAttempts = 5
	var delay = time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := c.db.SaveOrder(ctx, order)
		if err == nil {
			c.cache.Set(order.OrderUID, order)
			return nil
		}

		log.Printf("Попытка %d/%d: Ошибка сохранения заказа %s: %v", attempt, maxAttempts, order.OrderUID, err)

		if attempt == maxAttempts {
			break
		}

		select {
		case <-time.After(delay):
			delay *= 2 // Экспоненциальная задержка (1s, 2s, 4s, ...)
		case <-ctx.Done():
			log.Println("Отмена сохранения из-за завершения контекста.")
			return ctx.Err()
		}
	}
	return fmt.Errorf("превышено максимальное число попыток (%d)", maxAttempts)
}

// commitMessage — вспомогательная функция для коммита сообщения в Kafka
func (c *Consumer) commitMessage(ctx context.Context, m kafka.Message) {
	if err := c.reader.CommitMessages(ctx, m); err != nil {
		log.Printf("Ошибка коммита сообщения: %v", err)
	}
}

// Close закрывает Kafka-консьюмер.
func (c *Consumer) Close() error {
	return c.reader.Close()
}