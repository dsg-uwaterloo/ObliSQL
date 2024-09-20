package main

import (
	"errors"
	"fmt"
	"hash/fnv"
	"sync"
)

const shardCount = 32

type VersionedValue struct {
	Value interface{}
}

type ConcurrentMapShard struct {
	sync.RWMutex
	items map[string]map[int64]VersionedValue
}

type ConcurrentMap []*ConcurrentMapShard

func NewConcurrentMap() ConcurrentMap {
	m := make(ConcurrentMap, shardCount)
	for i := 0; i < shardCount; i++ {
		m[i] = &ConcurrentMapShard{
			items: make(map[string]map[int64]VersionedValue),
		}
	}
	return m
}

func (m ConcurrentMap) getShard(key string) *ConcurrentMapShard {
	hasher := fnv.New32()
	hasher.Write([]byte(key))
	return m[uint(hasher.Sum32())%uint(shardCount)]
}

func (m ConcurrentMap) Set(key string, value interface{}, version int64) error {
	shard := m.getShard(key)
	shard.Lock()
	defer shard.Unlock()

	if versions, ok := shard.items[key]; ok {
		for v := range versions {
			if v > version {
				return errors.New("abort: higher version exists")
			}
		}
	} else {
		shard.items[key] = make(map[int64]VersionedValue)
	}
	fmt.Println("SET: ", version, value)
	shard.items[key][version] = VersionedValue{Value: value}
	return nil
}

func (m ConcurrentMap) Get(key string, version int64) (interface{}, int64, bool) {
	shard := m.getShard(key)
	shard.RLock()
	defer shard.RUnlock()

	if versions, ok := shard.items[key]; ok {
		var highestLesserVersion int64 = -1
		var highestLesserValue interface{}
		for v, val := range versions {
			if v <= version && v > highestLesserVersion {
				highestLesserVersion = v
				highestLesserValue = val.Value
			}
		}
		if highestLesserVersion != -1 {
			fmt.Println("GET: ", version, highestLesserValue.(string))
			return highestLesserValue, highestLesserVersion, true
		}
	}
	return nil, -1, false
}

func (m ConcurrentMap) GetExactVersion(key string, version int64) (interface{}, bool) {
	shard := m.getShard(key)
	shard.RLock()
	defer shard.RUnlock()

	if versions, ok := shard.items[key]; ok {
		if val, exists := versions[version]; exists {
			return val.Value, true
		}
	}
	return nil, false
}

func (m ConcurrentMap) Delete(key string) {
	shard := m.getShard(key)
	shard.Lock()
	defer shard.Unlock()
	delete(shard.items, key)
}

func (m ConcurrentMap) DeleteVersion(key string, version int64) {
	shard := m.getShard(key)
	shard.Lock()
	defer shard.Unlock()
	if versions, ok := shard.items[key]; ok {
		delete(versions, version)
		if len(versions) == 0 {
			delete(shard.items, key)
		}
	}
}
