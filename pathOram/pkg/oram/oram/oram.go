package oram

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"

	"pathOram/pkg/oram/block"
	"pathOram/pkg/oram/bucket"
	"pathOram/pkg/oram/bucketRequest"
	"pathOram/pkg/oram/crypto"
	"pathOram/pkg/oram/redis"
	"pathOram/pkg/oram/request"

	"github.com/schollz/progressbar/v3"
)

type tempBlock struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

//  Core ORAM functions

var globalDummyBlock block.Block

type ORAM struct {
	LogCapacity int // height of the tree or logarithm base 2 of capacity (i.e. capacity is 2 to the power of this value)
	Z           int // number of blocks in a bucket (typically, 3 to 7)
	RedisClient *redis.RedisClient
	StashMap    map[string]block.Block
	StashSize   int // Maximum number of blocks the stash can hold
	Keymap      map[string]int
}

func NewORAM(LogCapacity, Z, StashSize int, redisAddr string, useSnapshot bool, key []byte) (*ORAM, error) {
	// If key is not provided (nil or empty), generate a random key
	if len(key) == 0 {
		var err error
		key, err = crypto.GenerateRandomKey()
		if err != nil {
			return nil, err
		}
	}

	// create dummy block
	dummyKey := [32]byte{}
	copy(dummyKey[:], "-1")
	dummyValue := [512]byte{}
	globalDummyBlock = block.Block{
		Key:   dummyKey,
		Value: dummyValue,
	}

	client, err := redis.NewRedisClient(redisAddr, key)
	if err != nil {
		return nil, err
	}

	oram := &ORAM{
		RedisClient: client,
		LogCapacity: LogCapacity,
		Z:           Z,
		StashSize:   StashSize,
		StashMap:    make(map[string]block.Block),
		Keymap:      make(map[string]int),
	}

	if useSnapshot {
		// Load the Stashmap and Keymap into memory
		// Allow redis to update state using dump.rdb
		oram.loadSnapshotMaps()
		fmt.Println("ORAM initialized with provided snapshot")
	} else {
		fmt.Println("Flushing Redis DB and initializing ORAM")
		// Clear the Redis database to ensure a fresh start
		if err := client.FlushDB(); err != nil {
			return nil, fmt.Errorf("failed to flush Redis database: %v", err)
		}
		oram.initialize()

		fmt.Println("ORAM DB is ready for opeartions!")
	}

	return oram, nil
}

// Load Keymap and Stashmap into memory
func (oram *ORAM) loadSnapshotMaps() {
	// Read from snapshot.json
	// Open the file for reading
	file, err := os.Open("/Users/nachiketrao/Desktop/URA/obliq-nachi/ObliSQL/pathOram/proxy_snapshot.json")
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return // No need to return anything here, just exit the function
	}
	defer file.Close()

	// Decode JSON data into a map
	var data map[string]interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		fmt.Printf("Error decoding JSON data: %v\n", err)
		return // No need to return anything here, just exit the function
	}

	// Load Keymap
	if keymap, ok := data["Keymap"].(map[string]interface{}); ok {
		// Convert map[string]interface{} to map[string]int for Keymap
		for key, value := range keymap {
			if val, ok := value.(float64); ok { // JSON numbers are decoded as float64
				oram.Keymap[key] = int(val) // Convert float64 to int
			}
		}
	} else {
		fmt.Println("Error: Keymap data is not of expected type")
	}

	// Load StashMap
	if stashmap, ok := data["StashMap"].(map[string]interface{}); ok {
		// Convert map[string]interface{} to map[string]block.Block for StashMap
		for key, value := range stashmap {
			// Assuming block.Block is a struct, and you need to decode the value into that type
			stashBlock, ok := value.(map[string]interface{}) // Assuming stash block is a map
			if !ok {
				fmt.Println("Error: StashMap block is not of expected type")
				continue
			}

			// Marshal and unmarshal to convert the stash block map to block.Block
			stashBlockData, err := json.Marshal(stashBlock)
			if err != nil {
				fmt.Printf("Error marshaling stash block data: %v\n", err)
				continue
			}
			var tb tempBlock
			err = json.Unmarshal(stashBlockData, &tb)

			var blockData block.Block
			copy(blockData.Key[:], tb.Key)
			copy(blockData.Value[:], tb.Value)
			oram.StashMap[tb.Key] = blockData
			if err != nil {
				fmt.Printf("Error unmarshaling stash block: %v\n", err)
				continue
			}
			// Add the block to StashMap
			oram.StashMap[key] = blockData
		}
	} else {
		fmt.Println("Error: StashMap data is not of expected type")
	}
}

// Tree height (real depth) is (o.LogCapacity + 1)
// Number of leaves is (2 ^ (o.LogCapacity))

// Initializing ORAM and populating with key = -1
func (o *ORAM) initialize() {

	totalBuckets := (1 << (o.LogCapacity + 1)) - 1
	bar := progressbar.Default(int64(totalBuckets), "Setting values...")

	for i := 0; i < totalBuckets; i++ {
		bucket := bucket.Bucket{
			Blocks: make([]block.Block, o.Z),
		}

		for j := 0; j < o.Z; j++ {
			bucket.Blocks[j] = globalDummyBlock
		}

		o.RedisClient.WriteBucketToDb(i, bucket) // initialize dosen't use redis batching mechanism to write: NOTE: don't run this initialize formally for experiments

		bar.Add(1)
	}

	bar.Finish()
}

// ClearStash clears the ORAM stash by resetting it to an empty state.
func (o *ORAM) ClearStash() {
	o.StashMap = make(map[string]block.Block) // Resets the stash (assuming stash is a map of block indices to data).
	fmt.Println("Stash has been cleared.")
}

// ClearKeymap clears the ORAM Keymap by resetting it to an empty state.
func (o *ORAM) ClearKeymap() {
	o.Keymap = make(map[string]int) // Resets the stash (assuming stash is a map of block indices to data).
	fmt.Println("Keymap has been cleared.")
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

func (o *ORAM) WritePath(bucketInfo []int, requests *[]bucketRequest.BucketRequest) {
	bucketId := bucketInfo[0]
	leaf := bucketInfo[1]
	level := bucketInfo[2]

	// Collect up to Z blocks that belong in this bucket
	toInsert := make([]block.Block, 0, o.Z)
	toDelete := make([]string, 0, o.Z)

	for key, blk := range o.StashMap {
		if len(toInsert) == o.Z {
			break // bucket is already full
		}
		currentBlockLeaf := o.Keymap[key]
		// If this block can be placed on this bucket
		if o.canInclude(currentBlockLeaf, leaf, level) {
			toInsert = append(toInsert, blk)
			toDelete = append(toDelete, key)
		}
	}

	// Build the final bucket
	bkt := bucket.Bucket{
		Blocks: make([]block.Block, o.Z),
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
	*requests = append(*requests, bucketRequest.BucketRequest{
		BucketId: bucketId,
		Bucket:   bkt,
	})
}

func (o *ORAM) WritePaths(previousPositionLeaves []int) {
	requests := make([]bucketRequest.BucketRequest, 0)
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

/*
initialize fully for now beforehand

create separate repo for base pathoram

batching optimization is kind of like:
run readpath on 50 - but maintain set and fetch every block only once, one redis request for all 50

write/read stashmap[key] for all 50

writepath to 50 previousPositionLeafs and try to clear out stash -  1 redis request

many batches come in; go routines?
*/

func (o *ORAM) Batching(requests []request.Request, batchSize int) ([]string, error) {

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
			previousPositionLeaf, exists = o.Keymap[req.Key]

			if !exists { // Key not in Keymap
				if req.Value == "" { // GET for new key: fake read
					previousPositionLeaf = crypto.GetRandomInt(1 << o.LogCapacity)
				} else { // PUT for new key: add to stash
					previousPositionLeaf = crypto.GetRandomInt(1 << o.LogCapacity)
					newBlock, err := block.NewBlock(req.Key, req.Value)
					if err != nil {
						return nil, fmt.Errorf("error creating block: %v", err)
					}
					// Use the trimmed key (req.Key) as the stash key
					o.StashMap[req.Key] = *newBlock
				}
			}
			fakeReadMap[req.Key] = true

		} else { // Key seen before: fake read
			previousPositionLeaf = crypto.GetRandomInt(1 << o.LogCapacity)
		}

		// Ensure unique leaves
		if _, exists := previousPositionLeavesMap[previousPositionLeaf]; !exists {
			previousPositionLeavesMap[previousPositionLeaf] = struct{}{}
			previousPositionLeaves = append(previousPositionLeaves, previousPositionLeaf)
		}

		// Update Keymap with the original key (trimmed)
		o.Keymap[req.Key] = crypto.GetRandomInt(1 << o.LogCapacity)
	}

	// perform read path, go through all paths at once and find non overlapping buckets, fetch all from redis at once - read path use redis MGET
	// adding all blocks to stash
	o.ReadPaths(previousPositionLeaves)

	// Retrieve values from stash map for all keys in requests and load them into an array
	values := make([]string, len(requests))

	// Craft reply to requests
	for i, req := range requests {
		if req.Value != "" { // PUT: update stash
			newBlock, err := block.NewBlock(req.Key, req.Value)
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
