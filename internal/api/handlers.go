package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"project/internal/cache"
	"project/internal/models"
	"project/internal/repository"
)

// Обработчик для HTTP-запросов к заказам
type OrderHandler struct {
	db    *storage.DB
	cache *cache.OrderCache
}

// NewOrderHandler создает новый экземпляр OrderHandler
func NewOrderHandler(db *storage.DB, cache *cache.OrderCache) *OrderHandler {
	return &OrderHandler{
		db:    db,
		cache: cache,
	}
}

// GetOrderByUID обрабатывает GET-запросы для получения заказа по UID
func (h *OrderHandler) GetOrderByUID(w http.ResponseWriter, r *http.Request) {
	orderUID := chi.URLParam(r, "order_uid")
	if orderUID == "" {
		http.Error(w, "Параметр 'order_uid' не указан", http.StatusBadRequest)
		return
	}

	// Поиск заказа в кэше
	order, found := h.cache.Get(orderUID)
	if found {
		log.Printf("Заказ %s найден в кэше", orderUID)
		sendJSONResponse(w, order)
		return
	}

	// Если заказа нет в кэше, ищем его в базе данных
	log.Printf("Заказ %s не найден в кэше, ищу в БД...", orderUID)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	order, err := h.db.GetOrder(ctx, orderUID)
	if err != nil {
		log.Printf("Ошибка при получении заказа из БД: %v", err)
		http.Error(w, "Заказ не найден", http.StatusNotFound)
		return
	}

	// Сохраняем найденный заказ в кэш
	h.cache.Set(orderUID, order)

	// Отправляем JSON-ответ
	sendJSONResponse(w, order)
}

// sendJSONResponse — вспомогательная функция для отправки JSON-ответа
func sendJSONResponse(w http.ResponseWriter, data models.Order) {
    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(data); err != nil {
        log.Printf("Ошибка при кодировании JSON: %v", err)
        http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
        return
    }
}
