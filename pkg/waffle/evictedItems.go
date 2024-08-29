package main

import (
	"sync"
)

type EvictedItems struct {
	cacheMap map[string]string
	capacity int
	mutex    sync.Mutex
}

func NewEvictedItems() *EvictedItems {
	return &EvictedItems{
		cacheMap: make(map[string]string),
		capacity: 0,
	}
}

func NewEvictedItemsWithCapacity(capacity int) *EvictedItems {
	return &EvictedItems{
		cacheMap: make(map[string]string),
		capacity: capacity,
	}
}

func (e *EvictedItems) GetValue(key string) string {
	// e.mutex.Lock()
	// defer e.mutex.Unlock()
	if value, exists := e.cacheMap[key]; exists {
		return value
	}
	return ""
}

func (e *EvictedItems) Insert(key, value string) {
	e.cacheMap[key] = value
}

func (e *EvictedItems) Erase(key string) {
	// e.mutex.Lock()
	// defer e.mutex.Unlock()
	delete(e.cacheMap, key)
}

func (e *EvictedItems) Size() int {
	// e.mutex.Lock()
	// defer e.mutex.Unlock()
	return len(e.cacheMap)
}

func (e *EvictedItems) CheckIfKeyExists(key string) bool {
	// e.mutex.Lock()
	// defer e.mutex.Unlock()
	_, exists := e.cacheMap[key]
	return exists
}

func (e *EvictedItems) GetMutex() *sync.Mutex {
	return &e.mutex
}
