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
	file, err := os.Open("proxy_snapshot.json")
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
			if block.Key != "-1" {
				// fmt.Println("Read block from Tree and put in stash with key: ", block.Key)
				o.StashMap[block.Key] = block
			}
		}
	}

}

func (o *ORAM) WritePath(bucketInfo []int, requests *[]bucketRequest.BucketRequest) {

	buckedId := bucketInfo[0]
	leaf := bucketInfo[1]
	level := bucketInfo[2]

	//newBucketsWritten := make(map[int]struct{})

	// Step 1: Get all blocks from stash
	currentStash := make(map[string]block.Block)
	for blockId, block := range o.StashMap {
		currentStash[blockId] = block
	}

	toDelete := make(map[string]struct{}) // track blocks that need to be deleted from stash

	toInsert := make(map[string]block.Block) // blocks to be inserted in the bucket (up to Z)

	toDeleteLocal := make([]string, 0) // indices/blockIds/keys of blocks in currentStash to delete

	for key, currentBlock := range currentStash {
		currentBlockLeaf := o.Keymap[currentBlock.Key]
		if o.canInclude(currentBlockLeaf, leaf, level) {
			// fmt.Println("This block can be written to Tree with key: ", currentBlock.Key)
			toInsert[currentBlock.Key] = currentBlock
			toDelete[currentBlock.Key] = struct{}{}
			toDeleteLocal = append(toDeleteLocal, key) // add the key for block we want to delete
			if len(toInsert) == o.Z {
				break
			}
		}
	}

	// Delete inserted blocks from currentStash
	for _, key := range toDeleteLocal {
		delete(currentStash, key)
	}

	// Prepare the bucket for writing

	bkt := bucket.Bucket{
		Blocks: make([]block.Block, o.Z),
	}

	i := 0
	for _, block := range toInsert {
		bkt.Blocks[i] = block
		i++
		if i >= o.Z {
			break
		}
	}

	// If there are fewer blocks than o.Z, fill the remaining slots with dummy blocks
	for i < o.Z {
		bkt.Blocks[i] = block.Block{Key: "-1", Value: ""}
		i++
	}

	*requests = append(*requests, bucketRequest.BucketRequest{
		BucketId: buckedId,
		Bucket:   bkt,
	})

	// Update the stash map by removing the newly inserted blocks
	for key := range toDelete {
		delete(o.StashMap, key)
	}

}

func (o *ORAM) WritePaths(previousPositionLeaves []int) {

	requests := make([]bucketRequest.BucketRequest, 0)
	bucketsToFillMap := make(map[int][]int) // Map to store bucketId and its associated [leaf, level]

	for _, leaf := range previousPositionLeaves {
		for level := o.LogCapacity; level >= 0; level-- {
			bucketId := o.bucketForLevelLeaf(level, leaf)
			bucketsToFillMap[bucketId] = []int{leaf, level} // Store leaf and level for each bucketId
		}
	}

	// Extract keys from the map
	var bucketIds []int
	for bucketId := range bucketsToFillMap {
		bucketIds = append(bucketIds, bucketId)
	}

	// Sort bucketIds in descending order
	sort.Slice(bucketIds, func(i, j int) bool {
		return bucketIds[i] > bucketIds[j]
	})

	// Create a nested list [bucketId:[leaf, level]]
	var nestedList [][]int
	for _, bucketId := range bucketIds {
		// Access leaf and level for the current bucketId
		leafLevel := bucketsToFillMap[bucketId]
		nestedList = append(nestedList, []int{bucketId, leafLevel[0], leafLevel[1]})
	}

	// Now we have all the bucketIDs that we need to fill

	// Iterate through the nested list
	for _, entry := range nestedList {
		o.WritePath(entry, &requests)
	}

	// Set the requests in Redis
	if err := o.RedisClient.WriteBucketsToDb(requests); err != nil {
		fmt.Println("Error writing buckets : ", err)
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
	var previousPositionLeaves []int

	for _, req := range requests {

		// TODO: if it's a get request, don't add the new block in
		_, keyReadBefore := fakeReadMap[req.Key]
		var previousPositionLeaf = -1

		if !keyReadBefore {
			var exists bool
			previousPositionLeaf, exists = o.Keymap[req.Key]
			// if !exists {
			// 	previousPositionLeaf = crypto.GetRandomInt(1 << (o.LogCapacity - 1))
			// 	o.StashMap[req.Key] = block.Block{Key: req.Key, Value: req.Value}
			// }
			// fakeReadMap[req.Key] = true
			if !exists {
				if req.Value == "" {
					// GET request for a new key, perform a random fake read, don't add a block
					previousPositionLeaf = crypto.GetRandomInt(1 << (o.LogCapacity - 1))
					fakeReadMap[req.Key] = true
					// Skip adding to stash
				} else {
					// PUT request for a new key, add block to stash
					previousPositionLeaf = crypto.GetRandomInt(1 << (o.LogCapacity - 1))
					o.StashMap[req.Key] = block.Block{Key: req.Key, Value: req.Value}
					fakeReadMap[req.Key] = true
				}
			}
		} else {
			previousPositionLeaf = crypto.GetRandomInt(1 << (o.LogCapacity - 1))
		}

		// ensure there are no duplicates in previousPositionLeaves - otherwise writepaths may overwrite content
		if _, alreadyExists := previousPositionLeavesMap[previousPositionLeaf]; !alreadyExists {
			previousPositionLeavesMap[previousPositionLeaf] = struct{}{}
			previousPositionLeaves = append(previousPositionLeaves, previousPositionLeaf) // previousPositionLeaves goes from index 0..batchSize - 1
		}

		// randomly remap key
		o.Keymap[req.Key] = crypto.GetRandomInt(1 << (o.LogCapacity - 1))
	}

	// perform read path, go through all paths at once and find non overlapping buckets, fetch all from redis at once - read path use redis MGET
	// adding all blocks to stash
	o.ReadPaths(previousPositionLeaves)

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

	o.WritePaths(previousPositionLeaves)

	// return the results to the batch in an array
	return values, nil
}
