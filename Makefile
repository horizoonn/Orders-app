.PHONY: help build run docker-up docker-down test-kafka clean

# Переменные
APP_NAME = order-service
DOCKER_COMPOSE = docker-compose

# Помощь - показывает все доступные команды
help:
	@echo " Order Service - Available commands:"
	@echo "  help         - Show this help"
	@echo "  build        - Build the application"
	@echo "  run          - Run the application locally"
	@echo "  docker-up    - Start all services in Docker"
	@echo "  docker-down  - Stop all services"
	@echo "  logs         - Show application logs"
	@echo "  clean        - Remove build files"

# Сборка Go приложения
build:
	@echo " Building the application..."
	go build -o bin/$(APP_NAME) ./cmd/order-service/main.go
	@echo " Done! Output file: bin/$(APP_NAME)"

# Запуск приложения локально (нужны запущенные DB и Kafka)
run: build
	@echo " Running the application..."
	DATABASE_URL="postgres://postgres:password@localhost:5433/orders_db" \
	KAFKA_BROKER="localhost:9093" \
	APP_PORT="8082" \
	./bin/$(APP_NAME)

# Запуск всех сервисов в Docker
docker-up:
	@echo " Starting services..."
	$(DOCKER_COMPOSE) up --build -d
	@echo " Services started:"
	@echo "   Web: http://localhost:8082"
	@echo "   DB: localhost:5433"
	@echo "   Kafka: localhost:9093"

# Остановка всех сервисов
docker-down:
	@echo " Stopping services..."
	$(DOCKER_COMPOSE) down

# Просмотр логов приложения
logs:
	@echo " Application logs:"
	$(DOCKER_COMPOSE) logs -f

logs-app:
	@echo " Application logs:"
	$(DOCKER_COMPOSE) logs -f app

# Очистка build файлов
clean:
	@echo " Cleaning up..."
	rm -rf bin/
	go clean
	@echo " Done!"
