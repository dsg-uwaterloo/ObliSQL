package main

import (
	"sync"
)

// ThreadSafeUnorderedMap is a thread-safe map with string keys and values of type list of channels for promises.
type ThreadSafeUnorderedMap struct {
	internalMap map[string][]chan string
	mutex       sync.RWMutex
}

// NewThreadSafeUnorderedMap initializes and returns a new instance of ThreadSafeUnorderedMap.
func NewThreadSafeUnorderedMap() *ThreadSafeUnorderedMap {
	return &ThreadSafeUnorderedMap{
		internalMap: make(map[string][]chan string),
	}
}

// InsertIfNotPresent inserts a new key with an associated list of promise channels if the key doesn't already exist.
// Returns false if the key was not present and was inserted; returns true if the key was already present.
func (tsm *ThreadSafeUnorderedMap) InsertIfNotPresent(key string, promise chan string) bool {
	// tsm.mutex.Lock()
	// defer tsm.mutex.Unlock()

	_, exists := tsm.internalMap[key]
	if !exists {
		tsm.internalMap[key] = []chan string{promise}
		return false
	}

	tsm.internalMap[key] = append(tsm.internalMap[key], promise)
	return true
}

// IsPresent checks if a key exists in the map.
func (tsm *ThreadSafeUnorderedMap) IsPresent(key string) bool {
	// tsm.mutex.RLock()
	// defer tsm.mutex.RUnlock()

	_, exists := tsm.internalMap[key]
	return exists
}

// ClearPromises clears all promises associated with a key, sends the provided value to all of them, and removes the key from the map.
func (tsm *ThreadSafeUnorderedMap) ClearPromises(key string, value string) {
	// tsm.mutex.Lock()
	// defer tsm.mutex.Unlock()

	promises, exists := tsm.internalMap[key]
	if exists {
		for _, promise := range promises {
			if promise != nil {
				promise <- value
				close(promise)
			}
		}
		delete(tsm.internalMap, key)
	}
}

// GetMutex provides direct access to the mutex.
func (tsm *ThreadSafeUnorderedMap) GetMutex() *sync.RWMutex {
	return &tsm.mutex
}
