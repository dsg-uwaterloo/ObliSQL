package main

import (
	"container/list"
	"sync"
)

type Cache struct {
	cacheMap   map[string]*list.Element
	accessList *list.List
	capacity   int
	mutex      sync.RWMutex
}

type cacheItem struct {
	key   string
	value string
}

func NewCache(keys, values []string, cacheCapacity int) *Cache {
	c := &Cache{
		cacheMap:   make(map[string]*list.Element),
		accessList: list.New(),
		capacity:   cacheCapacity,
	}

	if len(keys) == len(values) {
		for i := 0; i < len(keys); i++ {
			c.InsertIntoCache(keys[i], values[i])
		}
	}

	return c
}

func (c *Cache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return len(c.cacheMap)
}

func (c *Cache) CheckIfKeyExists(key string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	_, exists := c.cacheMap[key]
	return exists
}

func (c *Cache) GetValue(key string) string {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if elem, exists := c.cacheMap[key]; exists {
		c.accessList.MoveToFront(elem)
		return elem.Value.(*cacheItem).value
	}
	return ""
}

func (c *Cache) GetValueWithoutPositionChange(key string) string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if elem, exists := c.cacheMap[key]; exists {
		return elem.Value.(*cacheItem).value
	}
	return ""
}

func (c *Cache) InsertIntoCache(key, value string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if elem, exists := c.cacheMap[key]; exists {
		c.accessList.MoveToFront(elem)
		elem.Value.(*cacheItem).value = value
	} else {
		if len(c.cacheMap) >= c.capacity {
			oldest := c.accessList.Back()
			if oldest != nil {
				delete(c.cacheMap, oldest.Value.(*cacheItem).key)
				c.accessList.Remove(oldest)
			}
		}
		elem := c.accessList.PushFront(&cacheItem{key, value})
		c.cacheMap[key] = elem
	}
}

func (c *Cache) EvictLRElementFromCache() (string, string) {
	// c.mutex.Lock()
	// defer c.mutex.Unlock()

	if c.accessList.Len() == 0 {
		return "", ""
	}

	oldest := c.accessList.Back()
	item := oldest.Value.(*cacheItem)
	delete(c.cacheMap, item.key)
	c.accessList.Remove(oldest)

	return item.key, item.value
}

func (c *Cache) GetValueWithoutPositionChangeNew(key string) (string, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if elem, exists := c.cacheMap[key]; exists {
		return elem.Value.(*cacheItem).value, true
	}
	return "", false
}

func (c *Cache) GetMutex() *sync.RWMutex {
	return &c.mutex
}
