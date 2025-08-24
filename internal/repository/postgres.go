package storage

import (
	"context"
	"fmt"
	"log"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"project/internal/models"
)

// DB представляет структуру для работы с базой данных, используя pgxpool.
type DB struct {
	pool *pgxpool.Pool
}

// NewDB создает новый экземпляр DB и подключается к PostgreSQL.
func NewDB(ctx context.Context, connString string) (*DB, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к базе данных: %w", err)
	}

	err = pool.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка проверки соединения с БД: %w", err)
	}

	log.Println("Успешное подключение к базе данных PostgreSQL!")
	return &DB{pool: pool}, nil
}

// SaveOrder сохраняет или обновляет полный заказ в базу данных в рамках одной транзакции,
// используя ON CONFLICT DO UPDATE.
func (s *DB) SaveOrder(ctx context.Context, order models.Order) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("не удалось начать транзакцию: %w", err)
	}
	defer tx.Rollback(ctx)

	// Сохранение/обновление основной информации о заказе
	_, err = tx.Exec(ctx, `
		INSERT INTO orders (order_uid, track_number, entry, locale, internal_signature,
			customer_id, delivery_service, shard_key, sm_id, date_created, oof_shard)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (order_uid) DO UPDATE SET
			track_number = EXCLUDED.track_number,
			entry = EXCLUDED.entry,
			locale = EXCLUDED.locale,
			internal_signature = EXCLUDED.internal_signature,
			customer_id = EXCLUDED.customer_id,
			delivery_service = EXCLUDED.delivery_service,
			shard_key = EXCLUDED.shard_key,
			sm_id = EXCLUDED.sm_id,
			date_created = EXCLUDED.date_created,
			oof_shard = EXCLUDED.oof_shard
	`, order.OrderUID, order.TrackNumber, order.Entry, order.Locale, order.InternalSignature,
		order.CustomerID, order.DeliveryService, order.ShardKey, order.SmID, order.DateCreated, order.OofShard)
	if err != nil {
		return fmt.Errorf("не удалось сохранить/обновить основной заказ: %w", err)
	}

	// Сохранение/обновление информации о доставке
	_, err = tx.Exec(ctx, `
		INSERT INTO deliveries (order_uid, name, phone, zip, city, address, region, email)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (order_uid) DO UPDATE SET
			name = EXCLUDED.name,
			phone = EXCLUDED.phone,
			zip = EXCLUDED.zip,
			city = EXCLUDED.city,
			address = EXCLUDED.address,
			region = EXCLUDED.region,
			email = EXCLUDED.email
	`, order.OrderUID, order.Delivery.Name, order.Delivery.Phone, order.Delivery.Zip,
		order.Delivery.City, order.Delivery.Address, order.Delivery.Region, order.Delivery.Email)
	if err != nil {
		return fmt.Errorf("не удалось сохранить/обновить доставку: %w", err)
	}

	// Сохранение/обновление информации об оплате
	_, err = tx.Exec(ctx, `
		INSERT INTO payments (order_uid, transaction, request_id, currency, provider,
			amount, payment_dt, bank, delivery_cost, goods_total, custom_fee)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (order_uid) DO UPDATE SET
			transaction = EXCLUDED.transaction,
			request_id = EXCLUDED.request_id,
			currency = EXCLUDED.currency,
			provider = EXCLUDED.provider,
			amount = EXCLUDED.amount,
			payment_dt = EXCLUDED.payment_dt,
			bank = EXCLUDED.bank,
			delivery_cost = EXCLUDED.delivery_cost,
			goods_total = EXCLUDED.goods_total,
			custom_fee = EXCLUDED.custom_fee
	`, order.OrderUID, order.Payment.Transaction, order.Payment.RequestID, order.Payment.Currency,
		order.Payment.Provider, order.Payment.Amount, order.Payment.PaymentDT, order.Payment.Bank,
		order.Payment.DeliveryCost, order.Payment.GoodsTotal, order.Payment.CustomFee)
	if err != nil {
		return fmt.Errorf("не удалось сохранить/обновить оплату: %w", err)
	}

	// Удаление старых товаров перед вставкой новых
	_, err = tx.Exec(ctx, "DELETE FROM items WHERE order_uid = $1", order.OrderUID)
	if err != nil {
		return fmt.Errorf("не удалось удалить старые товары: %w", err)
	}

	// Сохранение информации о товарах
	for _, item := range order.Items {
		_, err = tx.Exec(ctx, `
			INSERT INTO items (order_uid, chrt_id, track_number, price, rid, name,
				sale, size, total_price, nm_id, brand, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, order.OrderUID, item.ChrtID, item.TrackNumber, item.Price, item.Rid, item.Name,
			item.Sale, item.Size, item.TotalPrice, item.NmID, item.Brand, item.Status)
		if err != nil {
			return fmt.Errorf("не удалось сохранить товар: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// GetOrder возвращает полный заказ из базы данных по его UID.
func (s *DB) GetOrder(ctx context.Context, uid string) (models.Order, error) {
	var order models.Order
	var delivery models.Delivery
	var payment models.Payment
	var items []models.Item

	// Запрос основной информации о заказе
	err := s.pool.QueryRow(ctx, `
		SELECT
			order_uid, track_number, entry, locale, internal_signature, customer_id,
			delivery_service, shard_key, sm_id, date_created, oof_shard
		FROM orders WHERE order_uid = $1
	`, uid).Scan(
		&order.OrderUID, &order.TrackNumber, &order.Entry, &order.Locale, &order.InternalSignature,
		&order.CustomerID, &order.DeliveryService, &order.ShardKey, &order.SmID, &order.DateCreated, &order.OofShard,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Order{}, nil // Возвращаем пустой объект, если заказ не найден
		}
		return models.Order{}, fmt.Errorf("не удалось получить заказ: %w", err)
	}

	// Запрос информации о доставке
	err = s.pool.QueryRow(ctx, `
		SELECT name, phone, zip, city, address, region, email
		FROM deliveries WHERE order_uid = $1
	`, uid).Scan(
		&delivery.Name, &delivery.Phone, &delivery.Zip, &delivery.City, &delivery.Address, &delivery.Region, &delivery.Email,
	)
	if err != nil {
		log.Printf("Не удалось получить доставку для заказа %s: %v", uid, err)
	}

	// Запрос информации об оплате
	err = s.pool.QueryRow(ctx, `
		SELECT
			transaction, request_id, currency, provider, amount, payment_dt, bank,
			delivery_cost, goods_total, custom_fee
		FROM payments WHERE order_uid = $1
	`, uid).Scan(
		&payment.Transaction, &payment.RequestID, &payment.Currency, &payment.Provider, &payment.Amount,
		&payment.PaymentDT, &payment.Bank, &payment.DeliveryCost, &payment.GoodsTotal, &payment.CustomFee,
	)
	if err != nil {
		log.Printf("Не удалось получить оплату для заказа %s: %v", uid, err)
	}

	// Запрос информации о товарах
	rows, err := s.pool.Query(ctx, `
		SELECT
			chrt_id, track_number, price, rid, name, sale, size,
			total_price, nm_id, brand, status
		FROM items WHERE order_uid = $1
	`, uid)
	if err != nil {
		log.Printf("Не удалось получить товары для заказа %s: %v", uid, err)
	}
	defer rows.Close()

	for rows.Next() {
		var item models.Item
		err := rows.Scan(
			&item.ChrtID, &item.TrackNumber, &item.Price, &item.Rid, &item.Name, &item.Sale,
			&item.Size, &item.TotalPrice, &item.NmID, &item.Brand, &item.Status,
		)
		if err != nil {
			log.Printf("Ошибка при сканировании товара для заказа %s: %v", uid, err)
			continue
		}
		items = append(items, item)
	}

	order.Delivery = delivery
	order.Payment = payment
	order.Items = items

	return order, nil
}

// GetAllOrders возвращает все заказы из базы данных.
func (s *DB) GetAllOrders(ctx context.Context) ([]models.Order, error) {
	var orders []models.Order
	
	// Используем транзакцию для получения всех заказов и их связанных данных
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("не удалось начать транзакцию: %w", err)
	}
	defer tx.Rollback(ctx)

	// Запрос всех order_uid для получения всех заказов
	rows, err := tx.Query(ctx, "SELECT order_uid FROM orders")
	if err != nil {
		return nil, fmt.Errorf("не удалось получить order_uids: %w", err)
	}
	defer rows.Close()

	var uids []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("не удалось сканировать order_uid: %w", err)
		}
		uids = append(uids, uid)
	}

	// Теперь получаем каждый заказ по его uid
	for _, uid := range uids {
		order, err := s.GetOrder(ctx, uid)
		if err != nil {
			return nil, fmt.Errorf("не удалось получить заказ %s: %w", uid, err)
		}
		orders = append(orders, order)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("не удалось подтвердить транзакцию: %w", err)
	}

	return orders, nil
}

// Close закрывает пул соединений к базе данных.
func (s *DB) Close() {
	s.pool.Close()
}