package oramexecutor

import (
	"errors"
	"fmt"
	"sort"
)

//  Core ORAM functions

var globalDummyBlock Block

//  Core ORAM functions

type ORAM struct {
	LogCapacity int // height of the tree or logarithm base 2 of capacity (i.e. capacity is 2 to the power of this value)
	Z           int // number of blocks in a bucket (typically, 3 to 7)
	RedisClient *RedisClient
	StashMap    map[string]Block
	StashSize   int // Maximum number of blocks the stash can hold
	keyMap      map[string]int
}

// Initializing ORAM and populating with key = -1
func (o *ORAM) initialize() {
	totalBuckets := (1 << (o.LogCapacity + 1)) - 1
	for i := 0; i < totalBuckets; i++ {
		bucket := Bucket{
			Blocks: make([]Block, o.Z),
		}
		for j := 0; j < o.Z; j++ {
			bucket.Blocks[j] = globalDummyBlock
		}
		o.RedisClient.WriteBucketToDb(i, bucket) // initialize dosen't use redis batching mechanism to write: NOTE: don't run this initialize formally for experiments
	}
}

// ClearStash clears the ORAM stash by resetting it to an empty state.
func (o *ORAM) ClearStash() {
	o.StashMap = make(map[string]Block) // Resets the stash (assuming stash is a map of block indices to data).
	fmt.Println("Stash has been cleared.")
}

// ClearKeymap clears the ORAM keymap by resetting it to an empty state.
func (o *ORAM) ClearKeymap() {
	o.keyMap = make(map[string]int) // Resets the stash (assuming stash is a map of block indices to data).
	fmt.Println("KeyMap has been cleared.")
}

// getDepth calculates the depth of a bucket in the tree
func (o *ORAM) GetDepth(bucketIndex int) int {
	depth := 0
	for (1<<depth)-1 <= bucketIndex {
		depth++
	}
	return depth - 1
}

// Calculate the bucket index for a given level and leaf
func (o *ORAM) bucketForLevelLeaf(level, leaf int) int {
	return ((leaf + (1 << (o.LogCapacity))) >> (o.LogCapacity - level)) - 1
}

// on this level, do paths of entryLeaf and leaf share the same bucket
func (o *ORAM) canInclude(entryLeaf, leaf, level int) bool {
	return entryLeaf>>(o.LogCapacity-level) == leaf>>(o.LogCapacity-level)
}

// ReadPath reads the paths from the root to the given leaves and optionally populates the stash.
func (o *ORAM) ReadPaths(leafs []int) {

	// Calculate the maximum valid leaf value
	maxLeaf := (1 << o.LogCapacity) - 1

	// Use a map to keep track of unique bucket indices
	uniqueBuckets := make(map[int]struct{})

	// Collect all unique bucket indices from root to each leaf
	for _, leaf := range leafs {
		if leaf < 0 || leaf > maxLeaf {
			fmt.Printf("invalid leaf value: %d, valid range is [0, %d]\n", leaf, maxLeaf)
		}
		for level := o.LogCapacity; level >= 0; level-- {
			bucket := o.bucketForLevelLeaf(level, leaf)
			_, exists := uniqueBuckets[bucket]
			// If the key exists
			if exists {
				break
			}
			uniqueBuckets[bucket] = struct{}{}
		}
	}

	bucketsData, _ := o.RedisClient.ReadBucketsFromDb(uniqueBuckets)
	// Read the blocks from all buckets in all retrived buckets
	for _, bucketData := range bucketsData {
		for _, block := range bucketData.Blocks {
			// Skip "empty" blocks
			keyStr := block.GetKey()
			if keyStr != "-1" {
				o.StashMap[keyStr] = block
			}
		}
	}

}

func (o *ORAM) WritePath(bucketInfo []int, requests *[]BucketRequest) {
	bucketId := bucketInfo[0]
	leaf := bucketInfo[1]
	level := bucketInfo[2]

	// Collect up to Z blocks that belong in this bucket
	toInsert := make([]Block, 0, o.Z)
	toDelete := make([]string, 0, o.Z)

	for key, blk := range o.StashMap {
		if len(toInsert) == o.Z {
			break // bucket is already full
		}
		currentBlockLeaf := o.keyMap[key]
		// If this block can be placed on this bucket
		if o.canInclude(currentBlockLeaf, leaf, level) {
			toInsert = append(toInsert, blk)
			toDelete = append(toDelete, key)
		}
	}

	// Build the final bucket
	bkt := Bucket{
		Blocks: make([]Block, o.Z),
	}
	copy(bkt.Blocks, toInsert)

	// Fill the rest with dummy blocks
	for i := len(toInsert); i < o.Z; i++ {
		bkt.Blocks[i] = globalDummyBlock
	}

	// Remove the inserted blocks from stash
	for _, key := range toDelete {
		delete(o.StashMap, key)
	}

	// Accumulate this bucket into the list of buckets to write
	*requests = append(*requests, BucketRequest{
		BucketId: bucketId,
		Bucket:   bkt,
	})
}

func (o *ORAM) WritePaths(previousPositionLeaves []int) {
	requests := make([]BucketRequest, 0)
	bucketsToFillMap := make(map[int][]int)

	// 1) Collect the unique buckets from all leaf paths
	for _, leaf := range previousPositionLeaves {
		for level := o.LogCapacity; level >= 0; level-- {
			bucketId := o.bucketForLevelLeaf(level, leaf)
			bucketsToFillMap[bucketId] = []int{leaf, level}
		}
	}

	// 2) Sort bucket IDs as before
	var bucketIds []int
	for bucketId := range bucketsToFillMap {
		bucketIds = append(bucketIds, bucketId)
	}
	sort.Slice(bucketIds, func(i, j int) bool {
		// Keep your chosen order (descending)
		return bucketIds[i] > bucketIds[j]
	})

	// 3) Write each bucket
	for _, bucketId := range bucketIds {
		leafLevel := bucketsToFillMap[bucketId]
		// bucketInfo = [bucketId, leaf, level]
		o.WritePath([]int{bucketId, leafLevel[0], leafLevel[1]}, &requests)
	}

	// 4) Execute the Redis pipeline/batch write
	if err := o.RedisClient.WriteBucketsToDb(requests); err != nil {
		fmt.Println("Error writing buckets:", err)
	}
}

// ORAM batching mechanism
/*
	write path 50 previous position leafs at once to redis - MGET vs pipeline - use MGET 180X faster than GET, pipeline is 12x faster than GET. pipeline allows batching, non blocking for other clients
	pipeline also allows different types of commands.

	for each batch, as we writepath, keep a track of all buckets already touched/altered
	if a bucket is already changed, we dont' write that bucket or any of the buckets above it
	becuase all buckets above have already been considered for all elements of the stash
	no point in redoing, this is an optmization. also, the other path may write -1s into
	already filled buckets
*/
func (o *ORAM) Batching(requests []Request, batchSize int) ([]string, error) {

	if len(requests) > batchSize {
		return nil, errors.New("batch size exceeded")
	}

	// fakeReadMap = {key: fakeRead?, key: fakeRead? , ...}
	fakeReadMap := make(map[string]bool)

	//determine keys from position map for all of them ; if something doesn't exist, add it to Keymap and stash as done in Access()
	previousPositionLeavesMap := make(map[int]struct{})

	// previousPositionLeaves: List of Unique leaves
	// - true leaves for first time keys in batch that already exist in system
	// - fake leaves for keys never seen before (GET or PUT)
	// - fake leaves for keys already seen in batch
	var previousPositionLeaves []int

	for _, req := range requests {
		_, keyReadBefore := fakeReadMap[req.Key]
		var previousPositionLeaf = -1

		if !keyReadBefore { // First occurrence in batch
			var exists bool
			// Use the original key to check Keymap
			previousPositionLeaf, exists = o.keyMap[req.Key]

			if !exists { // Key not in Keymap
				if req.Value == "" { // GET for new key: fake read
					previousPositionLeaf = GetRandomInt(1 << o.LogCapacity)
				} else { // PUT for new key: add to stash
					previousPositionLeaf = GetRandomInt(1 << o.LogCapacity)
					newBlock, err := NewBlock(req.Key, req.Value)
					if err != nil {
						return nil, fmt.Errorf("error creating block: %v", err)
					}
					// Use the trimmed key (req.Key) as the stash key
					o.StashMap[req.Key] = *newBlock
				}
			}
			fakeReadMap[req.Key] = true

		} else { // Key seen before: fake read
			previousPositionLeaf = GetRandomInt(1 << o.LogCapacity)
		}

		// Ensure unique leaves
		if _, exists := previousPositionLeavesMap[previousPositionLeaf]; !exists {
			previousPositionLeavesMap[previousPositionLeaf] = struct{}{}
			previousPositionLeaves = append(previousPositionLeaves, previousPositionLeaf)
		}

		// Update Keymap with the original key (trimmed)
		o.keyMap[req.Key] = GetRandomInt(1 << o.LogCapacity)
	}

	// perform read path, go through all paths at once and find non overlapping buckets, fetch all from redis at once - read path use redis MGET
	// adding all blocks to stash
	o.ReadPaths(previousPositionLeaves)

	// Retrieve values from stash map for all keys in requests and load them into an array
	values := make([]string, len(requests))

	// Craft reply to requests
	for i, req := range requests {
		if req.Value != "" { // PUT: update stash
			newBlock, err := NewBlock(req.Key, req.Value)
			if err != nil {
				return nil, fmt.Errorf("error creating block: %v", err)
			}
			o.StashMap[req.Key] = *newBlock
			values[i] = req.Value // Echo back the value
		} else if block, exists := o.StashMap[req.Key]; exists { // GET: existing key
			values[i] = block.GetValue()
		} else { // GET: non-existent key
			values[i] = "-1"
		}
	}

	// write path 50 previous position leafs at once to redis - MGET vs pipeline - use MGET 180X faster than GET, pipeline is 12x faster than GET. pipeline allows batching, non blocking for other clients
	// pipeline also allows different types of commands.

	// for each batch, as we writepath, keep a track of all buckets already touched/altered
	// if a bucket is already changed, we dont' write that bucket or any of the buckets above it
	// becuase all buckets above have already been considered for all elements of the stash
	// no point in redoing, this is an optmization. also, the other path may write -1s into
	// already filled buckets

	o.WritePaths(previousPositionLeaves)

	//fmt.Printf("Stash size after batch: %d\n", len(o.StashMap))

	// return the results to the batch in an array
	return values, nil
}
