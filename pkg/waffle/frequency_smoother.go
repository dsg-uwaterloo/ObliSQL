package main

import (
	"sort"
	"sync"
)

// KeyValuePair is a helper struct to hold key-value pairs.
type KeyValuePair struct {
	Key  string
	Freq int
}

// FrequencySmoother represents a structure that maintains frequency of keys.
type FrequencySmoother struct {
	accessTree  []KeyValuePair
	accessFreqs map[string]int
	mu          sync.Mutex
}

// NewFrequencySmoother initializes and returns a new FrequencySmoother.
func NewFrequencySmoother() *FrequencySmoother {
	return &FrequencySmoother{
		accessTree:  []KeyValuePair{},
		accessFreqs: make(map[string]int),
	}
}

// freqCmp is a comparator function for sorting key-value pairs by frequency and then by key.
func freqCmp(a, b KeyValuePair) bool {
	if a.Freq == b.Freq {
		return a.Key < b.Key
	}
	return a.Freq < b.Freq
}

// Insert adds a new key with an initial frequency of 0 if it does not exist.
func (fs *FrequencySmoother) Insert(key string) {
	// fs.mu.Lock()
	// defer fs.mu.Unlock()

	if _, exists := fs.accessFreqs[key]; exists {
		return
	}
	fs.accessFreqs[key] = 0
	fs.accessTree = append(fs.accessTree, KeyValuePair{Key: key, Freq: 0})
	fs.sortAccessTree()
}

// getMinFrequency returns the minimum frequency in the access tree.
func (fs *FrequencySmoother) GetMinFrequency() int {
	// fs.mu.Lock()
	// defer fs.mu.Unlock()

	if len(fs.accessTree) == 0 {
		return -1 // Or some error handling
	}
	return fs.accessTree[0].Freq
}

// GetKeyWithMinFrequency returns the key with the minimum frequency.
func (fs *FrequencySmoother) GetKeyWithMinFrequency() string {
	// fs.mu.Lock()
	// defer fs.mu.Unlock()

	if len(fs.accessTree) == 0 {
		return "" // Or some error handling
	}
	return fs.accessTree[0].Key
}

// IncrementFrequency increases the frequency of a given key by 1.
func (fs *FrequencySmoother) IncrementFrequency(key string) {
	// fs.mu.Lock()
	// defer fs.mu.Unlock()

	if _, exists := fs.accessFreqs[key]; !exists {
		return
	}
	// Find and remove the current pair from accessTree
	fs.removeFromAccessTree(key)
	fs.accessFreqs[key]++
	fs.accessTree = append(fs.accessTree, KeyValuePair{Key: key, Freq: fs.accessFreqs[key]})
	fs.sortAccessTree()
}

// SetFrequency sets the frequency of a given key to a specific value.
func (fs *FrequencySmoother) SetFrequency(key string, value int) {
	// fs.mu.Lock()
	// defer fs.mu.Unlock()

	if _, exists := fs.accessFreqs[key]; !exists {
		return
	}
	fs.removeFromAccessTree(key)
	fs.accessFreqs[key] = value
	fs.accessTree = append(fs.accessTree, KeyValuePair{Key: key, Freq: value})
	fs.sortAccessTree()
}

// RemoveKey removes a key from both the access frequency map and the access tree.
func (fs *FrequencySmoother) RemoveKey(key string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, exists := fs.accessFreqs[key]; !exists {
		return
	}
	fs.removeFromAccessTree(key)
	delete(fs.accessFreqs, key)
}

// AddKey re-adds a key with its current frequency.
func (fs *FrequencySmoother) AddKey(key string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, exists := fs.accessFreqs[key]; !exists {
		return
	}
	fs.accessTree = append(fs.accessTree, KeyValuePair{Key: key, Freq: fs.accessFreqs[key]})
	fs.sortAccessTree()
}

// GetFrequency returns the frequency of a given key.
func (fs *FrequencySmoother) GetFrequency(key string) int {
	// fs.mu.Lock()
	// defer fs.mu.Unlock()

	if freq, exists := fs.accessFreqs[key]; exists {
		return freq
	}
	return -1 // Or some error handling
}

// Size returns the number of keys in the access frequency map.
func (fs *FrequencySmoother) Size() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return len(fs.accessFreqs)
}

// sortAccessTree sorts the access tree based on frequency and then key.
func (fs *FrequencySmoother) sortAccessTree() {
	sort.SliceStable(fs.accessTree, func(i, j int) bool {
		return freqCmp(fs.accessTree[i], fs.accessTree[j])
	})
}

// removeFromAccessTree removes a key-value pair from the access tree.
func (fs *FrequencySmoother) removeFromAccessTree(key string) {
	for i, kv := range fs.accessTree {
		if kv.Key == key {
			fs.accessTree = append(fs.accessTree[:i], fs.accessTree[i+1:]...)
			break
		}
	}
}
