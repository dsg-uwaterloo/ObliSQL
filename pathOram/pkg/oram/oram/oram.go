package oram

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"pathOram/pkg/oram/block"
	"pathOram/pkg/oram/bucket"
	"pathOram/pkg/oram/bucketRequest"
	"pathOram/pkg/oram/crypto"
	"pathOram/pkg/oram/redis"
	"pathOram/pkg/oram/request"

	"github.com/schollz/progressbar/v3"
)

//  Core ORAM functions

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
			var blockData block.Block
			err = json.Unmarshal(stashBlockData, &blockData)
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
		for j := range bucket.Blocks {
			bucket.Blocks[j].Key = "-1"
			bucket.Blocks[j].Value = ""
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
func (o *ORAM) ReadPaths(leafs []int) map[int]struct{} {

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
			if block.Key != "-1" {
				// fmt.Println("Read block from Tree and put in stash with key: ", block.Key)
				o.StashMap[block.Key] = block
			}
		}
	}

	return uniqueBuckets

}

func (o *ORAM) WritePaths(oldLeaves map[string]int, bucketIndices map[int]struct{}) {
	newBuckets := make(map[int]bucket.Bucket)

	// Initialize all buckets we need to write
	for idx := range bucketIndices {
		newBuckets[idx] = bucket.Bucket{
			Blocks:         make([]block.Block, 0, o.Z),
			RealBlockCount: 0,
		}
	}

	toDelete := make([]string, 0)

	for key, block := range o.StashMap {
		newLeafID := o.Keymap[key]
		oldLeafID, hasOldPath := oldLeaves[key]

		// Iterate through each level (from root to leaf)
		for level := o.LogCapacity; level >= 0; level-- {
			bucketIDNew := o.bucketForLevelLeaf(level, newLeafID)

			// If the block has an old path, check if it intersects with the new path
			if hasOldPath {
				bucketIDOld := o.bucketForLevelLeaf(level, oldLeafID)
				if bucketIDOld != bucketIDNew {
					continue // Skip if paths don't intersect at this level
				}
			}

			// Check if this bucket is in the set we're writing to
			if _, exists := bucketIndices[bucketIDNew]; !exists {
				continue
			}

			// If bucket has space, place the block
			if newBuckets[bucketIDNew].RealBlockCount < o.Z {
				newBucket := newBuckets[bucketIDNew]
				newBucket.Blocks = append(newBucket.Blocks, block)
				newBucket.RealBlockCount++
				newBuckets[bucketIDNew] = newBucket
				toDelete = append(toDelete, key)
				break // Block placed, move to next block
			}
		}
	}

	// Remove placed blocks from stash
	for _, key := range toDelete {
		delete(o.StashMap, key)
	}

	// Pad buckets with dummy blocks and prepare Redis requests
	requests := make([]bucketRequest.BucketRequest, 0)
	for idx := range bucketIndices {
		bucket := newBuckets[idx]
		for len(bucket.Blocks) < o.Z {
			bucket.Blocks = append(bucket.Blocks, block.Block{Key: "-1", Value: ""})
		}
		requests = append(requests, bucketRequest.BucketRequest{
			BucketId: idx,
			Bucket:   bucket,
		})
	}

	// Write all updated buckets to Redis
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

	oldLeaves := make(map[string]int)

	for _, req := range requests {

		_, keyReadBefore := fakeReadMap[req.Key]
		var previousPositionLeaf = -1

		if !keyReadBefore { // if seeing key for first time in batch
			var exists bool
			previousPositionLeaf, exists = o.Keymap[req.Key]

			if !exists { // if key doesn't exist in Keymap
				if req.Value == "" { // New key: GET request
					// GET request for a new key, perform a random fake read, don't add a block, need to return -1
					previousPositionLeaf = crypto.GetRandomInt(1 << o.LogCapacity)
					// Skip adding to stash
				} else { // New key: PUT request
					// PUT request for a new key, add block to stash
					previousPositionLeaf = crypto.GetRandomInt(1 << o.LogCapacity)
					o.StashMap[req.Key] = block.Block{Key: req.Key, Value: req.Value}
				}
			}
			fakeReadMap[req.Key] = true // This key has now been visited; future appearances in batch should lead to fake reads

		} else { // if key seen before in batch
			previousPositionLeaf = crypto.GetRandomInt(1 << o.LogCapacity)
		}

		oldLeaves[req.Key] = previousPositionLeaf // keep track of old leaf for each key

		// ensure there are no duplicates in previousPositionLeaves - otherwise writepaths may overwrite content
		if _, alreadyExists := previousPositionLeavesMap[previousPositionLeaf]; !alreadyExists {
			previousPositionLeavesMap[previousPositionLeaf] = struct{}{}
			previousPositionLeaves = append(previousPositionLeaves, previousPositionLeaf) // previousPositionLeaves goes from index 0..batchSize - 1
		}

		// randomly remap key
		o.Keymap[req.Key] = crypto.GetRandomInt(1 << o.LogCapacity) // keep track of new leaf for each key
	}

	// perform read path, go through all paths at once and find non overlapping buckets, fetch all from redis at once - read path use redis MGET
	// adding all blocks to stash
	var bucketIndices map[int]struct{} = o.ReadPaths(previousPositionLeaves)

	/*
		Optimization v1
		previousPositionLeaves: all leaves for requested keys (these could be real or fake, but they were requested)
		newLeaves: for all requested keys, o.Keymap now has NEW leaf ids
	*/

	// Retrieve values from stash map for all keys in requests and load them into an array
	values := make([]string, len(requests))

	// Craft reply to requests
	for i, req := range requests {
		if req.Value != "" {
			// Replying to PUT requests
			o.StashMap[req.Key] = block.Block{Key: req.Key, Value: req.Value}
			values[i] = o.StashMap[req.Key].Value
		} else if block, exists := o.StashMap[req.Key]; exists {
			// Replying to GET requests that access existing data
			values[i] = block.Value
		} else {
			// Replying to GET requests trying to access non-existent keys
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

	o.WritePaths(oldLeaves, bucketIndices)

	// return the results to the batch in an array
	return values, nil
}
