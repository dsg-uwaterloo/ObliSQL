package main

import (
	"testing"
)

func TestCacheOperations(t *testing.T) {
	cache := NewCache([]string{"key1", "key2"}, []string{"value1", "value2"}, 3)

	if cache.Size() != 2 {
		t.Errorf("Expected size 2, got %d", cache.Size())
	}

	if !cache.CheckIfKeyExists("key1") {
		t.Error("key1 should exist")
	}

	if cache.CheckIfKeyExists("key3") {
		t.Error("key3 should not exist")
	}

	if v := cache.GetValue("key1"); v != "value1" {
		t.Errorf("Expected value1, got %s", v)
	}

	if v := cache.GetValueWithoutPositionChange("key2"); v != "value2" {
		t.Errorf("Expected value2, got %s", v)
	}

	cache.InsertIntoCache("key3", "value3")
	if cache.Size() != 3 {
		t.Errorf("Expected size 3, got %d", cache.Size())
	}

	if !cache.CheckIfKeyExists("key3") {
		t.Error("key3 should exist")
	}

	cache.InsertIntoCache("key4", "value4")
	if cache.Size() != 3 {
		t.Errorf("Expected size 3, got %d", cache.Size())
	}

	if cache.CheckIfKeyExists("key1") {
		t.Error("key1 should not exist")
	}

	evictedKey, evictedValue := cache.EvictLRElementFromCache()
	if evictedKey != "key2" || evictedValue != "value2" {
		t.Errorf("Expected evicted (key2, value2), got (%s, %s)", evictedKey, evictedValue)
	}

	if cache.Size() != 2 {
		t.Errorf("Expected size 2, got %d", cache.Size())
	}

	value, isPresent := cache.GetValueWithoutPositionChangeNew("key3")
	if !isPresent {
		t.Error("key3 should be present")
	}
	if value != "value3" {
		t.Errorf("Expected value3, got %s", value)
	}

	value, isPresent = cache.GetValueWithoutPositionChangeNew("key1")
	if isPresent {
		t.Error("key1 should not be present")
	}
	if value != "" {
		t.Errorf("Expected empty string, got %s", value)
	}
}
