package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"github.com/go-chi/chi/v5"

	"project/internal/api"
	"project/internal/cache"
	"project/internal/repository"
	"project/internal/kafka"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Настройка подключения к базе данных с использованием переменной окружения
	dbConnString := os.Getenv("DATABASE_URL")
	if dbConnString == "" {
		log.Fatalf("Переменная окружения DATABASE_URL не установлена")
	}
	db, err := storage.NewDB(ctx, dbConnString)
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}
	defer db.Close()

	orderCache := cache.NewOrderCache(1000)

	log.Println("Восстановление кэша из БД...")
	orders, err := db.GetAllOrders(ctx)
	if err != nil {
		log.Fatalf("Ошибка при восстановлении кэша из БД: %v", err)
	}
	for _, order := range orders {
		orderCache.Set(order.OrderUID, order)
	}
	log.Printf("Кэш успешно восстановлен. Загружено %d заказов.", len(orders))

	// Запуск Kafka-консьюмера с использованием переменной окружения
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		log.Fatalf("Переменная окружения KAFKA_BROKER не установлена")
	}
	kafkaConsumer := kafka.NewConsumer([]string{kafkaBroker}, "orders", "orders_group", db, orderCache)
	go kafkaConsumer.Run(ctx)
	defer kafkaConsumer.Close()

	handler := api.NewOrderHandler(db, orderCache)
	router := chi.NewRouter()
	router.Get("/order/{order_uid}", handler.GetOrderByUID)
	router.Mount("/", http.FileServer(http.Dir("./web")))

	// Использование переменной окружения для порта
	appPort := os.Getenv("APP_PORT")
	if appPort == "" {
		appPort = "8082" // Значение по умолчанию
	}
	httpServer := &http.Server{
		Addr:    ":" + appPort,
		Handler: router,
	}

	go func() {
		log.Printf("HTTP-сервер запущен на порту %s", appPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка HTTP-сервера: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Получен сигнал завершения, выключаю сервер...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Ошибка при выключении сервера: %v", err)
	}

	log.Println("Сервер успешно выключен.")
}