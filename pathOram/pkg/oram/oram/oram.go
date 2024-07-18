package oram

import (
	"fmt"

	"pathOram/pkg/oram/block"
	"pathOram/pkg/oram/bucket"
	"pathOram/pkg/oram/crypto"
	"pathOram/pkg/oram/redis"
)

//  Core ORAM functions

type ORAM struct {
	LogCapacity   int // height of the tree or logarithm base 2 of capacity (i.e. capacity is 2 to the power of this value)
	Z             int // number of blocks in a bucket (typically, 3 to 7)
	RedisClient   *redis.RedisClient
	StashMap      map[int]block.Block
	StashSize     int // Maximum number of blocks the stash can hold
	keyMap        map[int]int
	blockIds      int // keeps track of the blockIds used so far
}


func NewORAM(LogCapacity, Z, StashSize int, redisAddr string) (*ORAM, error) {
	key, err := crypto.GenerateRandomKey()
	if err != nil {
		return nil, err
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
		StashMap:    make(map[int]block.Block),
		keyMap:      make(map[int]int),
		blockIds:    0,
	}

	oram.initialize()
	return oram, nil
}

// Initializing ORAM and populating with key = -1
func (o *ORAM) initialize() {
	totalBuckets := (1 << (o.LogCapacity + 1)) - 1
	for i := 0; i < totalBuckets; i++ {
		bucket := bucket.Bucket{
			Blocks: make([]block.Block, o.Z),
		}
		for j := range bucket.Blocks {
			bucket.Blocks[j].Key = -1
			bucket.Blocks[j].BlockId = -1
			bucket.Blocks[j].Value = ""
		}
		o.RedisClient.WriteBucketToDb(i, bucket)
	}
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

// Function to put a value into the datastore
func (o *ORAM) Put(key int, value string) (string) {
	//create a block
	newblock := block.Block{
		BlockId: key,
		Key:     key,
		Value:   value,
	}

    returnvalue := o.Access(false, newblock.BlockId, newblock) // Assuming Access function is defined later
	return returnvalue.Value
}

// Function to get a value from the datastore
func (o *ORAM) Get(key int) (string) {
	//create dummy block
	dummyBlock := block.Block{
		BlockId: -1,
		Key:     -1,
		Value:   "",
	}

	returnvalue := o.Access(true, key, dummyBlock) // Assuming Access function is defined later
	return returnvalue.Value
}

// Read the path from the root to the given leaf and optionally populate the stash
func (o *ORAM) ReadPath(leaf int, putInStash bool) (map[int]struct{}) {

	// Calculate the maximum valid leaf value
	maxLeaf := (1 << o.LogCapacity) - 1

	// Validate the leaf value
	if leaf < 0 || leaf > maxLeaf {
		fmt.Println("invalid leaf value: %d, valid range is [0, %d]", leaf, maxLeaf)
		return nil
	}

	//fmt.Println("the leaf give is: ", leaf)
	path := make(map[int]struct{})

	// Collect all bucket indices from root to leaf
	for level := 0; level <= o.LogCapacity; level++ {
		bucket := o.bucketForLevelLeaf(level, leaf)
		//fmt.Println(bucket)
		path[bucket] = struct{}{}
	}

	// Read the blocks from each bucket

	if putInStash { // TODO: even if fake path is read, and nothing is put in the stash, still write the path back
		for bucket := range path {
			bucketData, _ := o.RedisClient.ReadBucketFromDb(bucket)
			for _, block := range bucketData.Blocks {
				// Skip "empty" blocks
				if block.Key != -1 {
					o.StashMap[block.Key] = block
				}
			}
		}
	}

	return path
}

// Write the path back, ensuring all buckets contain Z blocks
func (o *ORAM) WritePath(leaf int) {
    // Step 1: Get all blocks from stash
    currentStash := make(map[int]block.Block)
	for blockId, block := range o.StashMap {
		currentStash[blockId] = block
	}

    toDelete := make(map[int]struct{}) // track blocks that need to be deleted from stash
    requests := make([]struct {
        bucketId int
        bucket   bucket.Bucket
    }, 0) // storage SET requests (batching)

    // Step 2: Follow the path from leaf to root (greedy)
    for level := o.LogCapacity; level >= 0; level-- {
        toInsert := make(map[int]block.Block) // blocks to be inserted in the bucket (up to Z)
        toDeleteLocal := make([]int, 0)   // indices/blockIds/keys of blocks in currentStash to delete

        for key, currentBlock := range currentStash {
            currentBlockLeaf := o.keyMap[currentBlock.Key]
            if o.canInclude(currentBlockLeaf, leaf, level) {
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
        bucketId := o.bucketForLevelLeaf(level, leaf)
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
			bkt.Blocks[i] = block.Block{Key: -1, BlockId: -1, Value: ""}
			i++
		}

        requests = append(requests, struct {
            bucketId int
            bucket   bucket.Bucket
        }{
            bucketId: bucketId,
            bucket:   bkt,
        })
    }

    // Set the requests in Redis
    for _, request := range requests {
        if err := o.RedisClient.WriteBucketToDb(request.bucketId, request.bucket); err != nil {
            fmt.Printf("Error writing bucket %d: %v\n", request.bucketId, err)
        }
    }

    // Update the stash map by removing the newly inserted blocks
    for key := range toDelete {
        delete(o.StashMap, key)
    }
}

// Access simulates the access operation.
func (o *ORAM) Access(read bool, blockId int, data block.Block) (block.Block) {
	//blockId is treated the same as Key
    // Step 1: Remap block
    previousPositionLeaf, exists := o.keyMap[blockId]
    if !exists {
        previousPositionLeaf = crypto.GetRandomInt(1 << (o.LogCapacity - 1)) //TODO: only for PUT, for GET, assume key always exists
		o.StashMap[blockId] = data
    }
	
    o.keyMap[blockId] = crypto.GetRandomInt(1 << (o.LogCapacity - 1))

    // Step 2: Read path
    o.ReadPath(previousPositionLeaf, true) // Stash updated

    // Step 3: Update or read from stash
    if !read {
        o.StashMap[blockId] = data
    }
    
	returnValue := o.StashMap[blockId]

    // Step 4: Write path
    o.WritePath(previousPositionLeaf) // Stash updated

	return returnValue // returns the block
}