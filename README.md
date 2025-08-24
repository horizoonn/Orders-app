# Order Service 
Сервис обработки заказов - это демонстрационный сервис для управления и отображения данных о заказах. Сервис подключается к базе данных PostgreSQL, подписывается на Kafka для получения данных о заказах, кэширует данные в памяти и предоставляет HTTP API для получения информации о заказах по их идентификаторам. Проект организован по слоям и папкам согласно best practice: точка входа — один исполняемый модуль в `cmd/order-service`. 
---
## 🏗️ Архитектура
```
┌─────────────┐ ┌─────────────┐ ┌─────────────┐  
│    Kafka    │ │  PostgreSQL │ │    Cache    │  
│   (Message  │ │  (Database) │ │ (In-Memory) │  
│    Queue)   │ │             │ │             │  
└─────────────┘ └─────────────┘ └─────────────┘  
                     │ │ │  
                     │ │ │  
                     ▼ ▼ ▼  
┌─────────────────────────────────────────────────┐  
│                   Order Service                 │  
│ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ │  
│ │ Kafka       │ │     HTTP    │ │    Cache    │ │  
│ │ Consumer    │ │    Server   │ │   Manager   │ │  
│ └─────────────┘ └─────────────┘ └─────────────┘ │  
└─────────────────────────────────────────────────┘  
                       │  
                       ▼  
                ┌─────────────┐  
                │ Web UI      │  
                │ (HTML)      │  
                └─────────────┘
```

--- 
## ✨ Функциональность
- **Получение заказов из Kafka** — асинхронный consumer, валидация и логирование невалидных сообщений. 
- **Сохранение в PostgreSQL** — для каждого заказа транзакционно сохраняются order, delivery, payment, items.  
- **Кэширование в памяти** — LRU-кэш оптимизирует доступ к часто запрашиваемым заказам. 
- **HTTP API** — выдача заказа по `order_uid` через `GET /order/{order_uid}`. 
- **Веб-интерфейс** — простая HTML-страницы для поиска заказа по ID.  
- **Восстановление кэша** — при старте приложения кэш инициализируется из базы. 
- **Обработка ошибок** — устойчивость к невалидным сообщениям и сбоям БД, логирование. 
- **Graceful shutdown** — корректная остановка сервера и консьюмера по SIGINT/SIGTERM.
--- 
## 🎯 Особенности реализации 
- **Кэш**: потокобезопасный LRU, автопрогрев из БД при старте, обновление при успешной обработке сообщения. 
- **Ошибки**: все кейсы логируются, сбои БД не приводят к потере Kafka сообщений.
- **Использование Kafka-UI**
- **Производительность**: LRU-кэш, SQL-индексы, асинхронная обработка.
- **Надёжность**: транзакции в БД, повторные попытки сохранения, коммит offset в Kafka — только после успешного сохранения в БД и кэше. 
- **Восстановление**: кэш мгновенно восстанавливается из базы после рестарта.  
- **Graceful shutdown**: корректное закрытие HTTP-сервера, пула БД и консьюмера при SIGINT/SIGTERM. 
---
## 📁 Структура проекта
```
MYAPP/  
├── cmd/order-service/  
│ └── main.go # Точка входа    
├── db/  
│ └── schema.sql # SQL-схема БД  
├── internal/  
│ ├── api/handlers.go # HTTP-обработчики  
│ ├── cache/cache.go # In-memory LRU-кэш  
│ ├── kafka/consumer.go # Kafka consumer  
│ ├── models/order.go # Модели: заказ, доставка, оплата, товары  
│ ├── repository/postgres.go # Работа с PostgreSQL 
├── web/index.html # Веб-интерфейс (HTML)  
├── docker-compose.yml  
├── Dockerfile  
├── Makefile  
├── go.mod, go.sum  
└── README.md
```

## 🚀 Быстрый старт 
```
# 1. Запуск инфраструктуры (Поднимает PostgreSQL, Zookeeper, Kafka, Order Service.)
make docker-up

# 2. Отправка тестовых заказов(Отправит тестовый заказ с `order_uid: b563feb7b2b84b6test` в Kafka.)
make test-kafka

# 3. Проверка работы - Перейдите в браузере: [http://localhost:8082](http://localhost:8082) - Введите `order_uid` и получите данные заказа. 

# 4. Остановка сервисов
make docker-down

# 5. Просмотр доступных команд
make help

```
--- 
## ⚙️ Конфигурация - используются переменные окружения:
```
DATABASE_URL=postgres://postgres:password@db:5432/orders_db  
KAFKA_BROKER=kafka:9092  
APP_PORT=8082
```

## 📊 Модель данных

Сервис обрабатывает заказы со следующей структурой:
```
{  
"order_uid": "b563feb7b2b84b6test",  
"track_number": "WBILMTESTTRACK",  
"entry": "WBIL",  
"delivery": {  
"name": "Test Testov",  
"phone": "+9720000000",  
"zip": "2639809",  
"city": "Kiryat Mozkin",  
"address": "Ploshad Mira 15",  
"region": "Kraiot",  
"email": "[test@gmail.com](mailto:test@gmail.com)"  
},  
"payment": {  
"transaction": "b563feb7b2b84b6test",  
"request_id": "",  
"currency": "USD",  
"provider": "wbpay",  
"amount": 1817,  
"payment_dt": 1637907727,  
"bank": "alpha",  
"delivery_cost": 1500,  
"goods_total": 317,  
"custom_fee": 0  
},  
"items": [  
{  
"chrt_id": 9934930,  
"track_number": "WBILMTESTTRACK",  
"price": 453,  
"rid": "ab4219087a764ae0btest",  
"name": "Mascaras",  
"sale": 30,  
"size": "0",  
"total_price": 317,  
"nm_id": 2389212,  
"brand": "Vivienne Sabo",  
"status": 202  
}  
],  
"locale": "en",  
"internal_signature": "",  
"customer_id": "test",  
"delivery_service": "meest",  
"shard_key": "9",  
"sm_id": 99,  
"date_created": "2021-11-26T06:22:19Z",  
"oof_shard": "1"  
}
```

--- 
## 📋 Требования 
- Go 1.21+ 
- Docker & Docker Compose 
- PostgreSQL 15 (автоматически поднимается через compose) 
- Kafka 7.4.0 (через compose) #



