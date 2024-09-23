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

	// Check if the key already exists
	if currentVersionedValue, ok := shard.items[key]; ok {
		// Abort if the provided version is lower or equal to the current version
		for v := range currentVersionedValue {
			if version <= v {
				return errors.New("abort: cannot overwrite with an older or same version")
			}
		}
	}

	// Store the new value with the version, removing any old version for the key
	shard.items[key] = map[int64]VersionedValue{
		version: {Value: value},
	}

	fmt.Println("SET: ", version, value)
	return nil
}
func (m ConcurrentMap) Get(key string, version int64) (interface{}, int64, bool) {
	shard := m.getShard(key)
	shard.RLock()
	defer shard.RUnlock()

	// Check if the key exists
	if currentVersionedValue, ok := shard.items[key]; ok {
		// There will be only one version for the key
		for currentVersion, value := range currentVersionedValue {
			// Return the value if the provided version is greater or equal to the stored version
			if version >= currentVersion {
				fmt.Println("GET: ", version, value.Value)
				return value.Value, currentVersion, true
			}
			if version < currentVersion {
				fmt.Println("GET: Requested version is lower than stored version")
				return nil, -2, false
			}
		}
	}
	return nil, -1, false // Return false if the requested version is lower than the stored version
}

func (m ConcurrentMap) Delete(key string) {
	shard := m.getShard(key)
	shard.Lock()
	defer shard.Unlock()
	delete(shard.items, key)
}
