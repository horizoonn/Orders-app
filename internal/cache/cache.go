package cache

import (
	"container/list"
	"sync"
	"project/internal/models"
)

// cacheItem представляет элемент в кэше
type cacheItem struct {
	key   string
	value models.Order
}

// OrderCache представляет LRU-кэш для заказов.
type OrderCache struct {
	mtx        sync.RWMutex
	data      map[string]*list.Element
	eviction  *list.List
	capacity  int
}

// NewOrderCache создает и возвращает новый экземпляр кэша с заданным лимитом.
func NewOrderCache(capacity int) *OrderCache {
	return &OrderCache{
		data:      make(map[string]*list.Element),
		eviction:  list.New(),
		capacity:  capacity,
	}
}

// Set сохраняет или обновляет заказ в кэше.
func (c *OrderCache) Set(orderUID string, order models.Order) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	// Если элемент уже существует, обновляем его и перемещаем в начало списка
	if ele, ok := c.data[orderUID]; ok {
		c.eviction.MoveToFront(ele)
		ele.Value.(*cacheItem).value = order
		return
	}

	// Если кэш полон, удаляем самый старый элемент
	if c.eviction.Len() >= c.capacity {
		c.removeOldest()
	}

	// Добавляем новый элемент в начало списка и в map
	item := &cacheItem{
		key:   orderUID,
		value: order,
	}
	ele := c.eviction.PushFront(item)
	c.data[orderUID] = ele
}

// Get извлекает заказ из кэша и обновляет его позицию.
func (c *OrderCache) Get(orderUID string) (models.Order, bool) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	if ele, ok := c.data[orderUID]; ok {
		c.eviction.MoveToFront(ele)
		return ele.Value.(*cacheItem).value, true
	}
	return models.Order{}, false
}

// GetAll возвращает все заказы из кэша.
func (c *OrderCache) GetAll() map[string]models.Order {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	orders := make(map[string]models.Order, c.eviction.Len())
	for _, ele := range c.data {
		item := ele.Value.(*cacheItem)
		orders[item.key] = item.value
	}
	return orders
}

// removeOldest удаляет самый старый элемент из кэша.
func (c *OrderCache) removeOldest() {
	if c.eviction.Len() == 0 {
		return
	}
	ele := c.eviction.Back()
	item := c.eviction.Remove(ele).(*cacheItem)
	delete(c.data, item.key)
}