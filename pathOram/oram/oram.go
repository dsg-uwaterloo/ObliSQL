package oram

import (
	mathrand "math/rand"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"io"
    "github.com/go-redis/redis/v8" // using go-redis library
	"context"
)

// Block represents a key-value pair
type Block struct {
	BlockId int
	Key   int    // dummy can have key -1
	Value string
}

// Bucket represents a collection of blocks
type Bucket struct {
	Blocks        []Block
	RealBlockCount int
}

// ############################### Cryptography functions #######################################

func getRandomInt(max int) int {
    return mathrand.Intn(max)
}

func generateRandomKey() ([]byte, error) {
	key := make([]byte, 32) // AES-256
	_, err := io.ReadFull(rand.Reader, key)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func encrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, aes.BlockSize+len(data))
	iv := ciphertext[:aes.BlockSize]

	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], data)

	return ciphertext, nil
}

func decrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(data) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := data[:aes.BlockSize]
	ciphertext := data[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return ciphertext, nil
}

// ##################################### Redis/Datastore access functions #######################################

// Write a bucket to Redis
func (o *ORAM) writeBucketToDb(index int, bucket Bucket) error {

	// Ensure bucket has exactly o.Z blocks
    if len(bucket.Blocks) < o.Z {
        for len(bucket.Blocks) < o.Z {
            bucket.Blocks = append(bucket.Blocks, Block{Key: -1, BlockId: -1, Value: ""})
        }
    }

	data, err := json.Marshal(bucket)
	if err != nil {
		return err
	}

	encryptedData, err := encrypt(data, o.encryptionKey)
	if err != nil {
		return err
	}

    key := fmt.Sprintf("bucket:%d", index)
	//key := index

    err = o.redisClient.Set(o.ctx, key, encryptedData, 0).Err() // the bucket index is the key
    return err
}

// Read a bucket from Redis
func (o *ORAM) readBucketFromDb(index int) (Bucket, error) {
    key := fmt.Sprintf("bucket:%d", index)
	// key := index
    data, err := o.redisClient.Get(o.ctx, key).Bytes()
    if err != nil {
        return Bucket{}, err
    }

	decryptedData, err := decrypt(data, o.encryptionKey)
	if err != nil {
		return Bucket{}, err
	}

    var bucket Bucket
    err = json.Unmarshal(decryptedData, &bucket)
    if err != nil {
        return Bucket{}, err
    }

    return bucket, nil
}

// Close closes the Redis client connection
func (o *ORAM) Close() error {
    return o.redisClient.Close()
}

// ######################################### Testing/Debugging helper functions ######################################

// PrintTree prints the ORAM tree structure by reading the file sequentially
func (o *ORAM) PrintTree() {
	totalBuckets := (1 << (o.logCapacity + 1)) - 1
	for i := 0; i < totalBuckets; i++ {
		bucket, _ := o.readBucketFromDb(i)
		indent := strings.Repeat("  ", o.getDepth(i))
		fmt.Printf("%sBucket %d:\n", indent, i)
		for j, block := range bucket.Blocks {
			j=j
			fmt.Printf("%s  Blockid %d: Key=%d, Value=%s\n", indent, block.BlockId, block.Key, block.Value)
		}
	}
}

// PrintStashMap prints the contents of the stash map
func (o *ORAM) PrintStashMap() {
	fmt.Println("Stash Map contents:")
	for blockId, block := range o.stashMap {
		fmt.Printf("BlockId: %d, Key: %d, Value: %s\n", blockId, block.Key, block.Value)
	}
}

// Exported put function used for testing by directly inserting bucket (escaping writepath)
func (o *ORAM) PutBucket(index int, bucket Bucket) { 
	o.writeBucketToDb(index, bucket)
}

// ############################################ Core ORAM functions #####################################################

type ORAM struct {
	logCapacity int // height of the tree or logarithm base 2 of capacity (i.e. capacity is 2 to the power of this value)
	Z           int // number of blocks in a bucket (typically, 3 to 7)
	redisClient *redis.Client
	stashMap   map[int]Block
	stashSize   int // Maximum number of blocks the stash can hold // TODO: not needed
	keyMap		map[int]int
	blockIds	int // keeps track of the blockIds used so far
	ctx         context.Context // Context field to associate with ORAM operations
	encryptionKey []byte // Encryption key for buckets
}


func NewORAM(logCapacity, Z int, stashSize int, redisAddr string) (*ORAM, error) {
	// configuring redis client
	// Create a Redis client with context
    ctx := context.Background()
	client := redis.NewClient(&redis.Options{
        Addr: redisAddr,
    })
    _, err := client.Ping(ctx).Result()
    if err != nil {
        return nil, err
    }

	key, err := generateRandomKey()
	if err != nil {
		return nil, err
	}

	oram := &ORAM{
		redisClient: client,
		logCapacity: logCapacity,
		Z:           Z,
		stashSize:   stashSize,
		stashMap:    make(map[int]Block),
		keyMap:      make(map[int]int),
		blockIds:		0,
		ctx:		ctx,
		encryptionKey: key,
	}

	// Initialization block
	oram.initialize()

	return oram, nil
}

// Initializing ORAM and populating with key = -1
func (o *ORAM) initialize() {
	totalBuckets := (1 << (o.logCapacity + 1)) - 1
	for i := 0; i < totalBuckets; i++ {
		bucket := Bucket{
			Blocks: make([]Block, o.Z),
			RealBlockCount: 0,
		}
		for j := range bucket.Blocks {
			bucket.Blocks[j].Key = -1
			bucket.Blocks[j].BlockId = -1
			bucket.Blocks[j].Value = ""
		}
		o.writeBucketToDb(i, bucket)
	}
}

// getDepth calculates the depth of a bucket in the tree
func (o *ORAM) getDepth(bucketIndex int) int {
	depth := 0
	for (1<<depth)-1 <= bucketIndex {
		depth++
	}
	return depth - 1
}

// Calculate the bucket index for a given level and leaf
func (o *ORAM) bucketForLevelLeaf(level, leaf int) int {
	return ((leaf + (1 << (o.logCapacity))) >> (o.logCapacity - level)) - 1
}

// on this level, do paths of entryLeaf and leaf share the same bucket
func (o *ORAM) canInclude(entryLeaf, leaf, level int) bool {
	return entryLeaf>>(o.logCapacity-level) == leaf>>(o.logCapacity-level)
}

// Function to put a value into the datastore
func (o *ORAM) Put(key int, value string) (string) {
	//create a block
	newblock := Block{
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
	newblock := Block{
		BlockId: -1,
		Key:     -1,
		Value:   "",
	}

	returnvalue := o.Access(true, key, newblock) // Assuming Access function is defined later
	return returnvalue.Value
}

// Read the path from the root to the given leaf and optionally populate the stash
func (o *ORAM) ReadPath(leaf int, putInStash bool) (map[int]struct{}) {

	// Calculate the maximum valid leaf value
	maxLeaf := (1 << o.logCapacity) - 1

	// Validate the leaf value
	if leaf < 0 || leaf > maxLeaf {
		fmt.Println("invalid leaf value: %d, valid range is [0, %d]", leaf, maxLeaf)
		return nil
	}

	//fmt.Println("the leaf give is: ", leaf)
	path := make(map[int]struct{})

	// Collect all bucket indices from root to leaf
	for level := 0; level <= o.logCapacity; level++ {
		bucket := o.bucketForLevelLeaf(level, leaf)
		//fmt.Println(bucket)
		path[bucket] = struct{}{}
	}

	// Read the blocks from each bucket

	if putInStash { // TODO: even if fake path is read, and nothing is put ins tash, still write the path back
		for bucket := range path {
			bucketData, _ := o.readBucketFromDb(bucket)
			for _, block := range bucketData.Blocks {
				// Skip "empty" blocks
				if block.Key != -1 {
					o.stashMap[block.Key] = block
				}
			}
		}
	}

	return path
}

// Write the path back, ensuring all buckets contain Z blocks
func (o *ORAM) WritePath(leaf int) {
    // Step 1: Get all blocks from stash
    currentStash := make(map[int]Block)
	for blockId, block := range o.stashMap {
		currentStash[blockId] = block
	}

    toDelete := make(map[int]struct{}) // track blocks that need to be deleted from stash
    requests := make([]struct {
        bucketId int
        bucket   Bucket
    }, 0) // storage SET requests (batching)

    // Step 2: Follow the path from leaf to root (greedy)
    for level := o.logCapacity; level >= 0; level-- {
        toInsert := make(map[int]Block) // blocks to be inserted in the bucket (up to Z)
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
        bucket := Bucket{
            Blocks: make([]Block, o.Z),
        }

        i := 0
		for _, block := range toInsert {
			bucket.Blocks[i] = block
			i++
			if i >= o.Z {
				break
			}
		}

		// If there are fewer blocks than o.Z, fill the remaining slots with dummy blocks
		for i < o.Z {
			bucket.Blocks[i] = Block{Key: -1, BlockId: -1, Value: ""}
			i++
		}

        requests = append(requests, struct {
            bucketId int
            bucket   Bucket
        }{
            bucketId: bucketId,
            bucket:   bucket,
        })
    }

    // Set the requests in Redis
    for _, request := range requests {
        if err := o.writeBucketToDb(request.bucketId, request.bucket); err != nil {
            fmt.Printf("Error writing bucket %d: %v\n", request.bucketId, err)
        }
    }

    // Update the stash map by removing the newly inserted blocks
    for key := range toDelete {
        delete(o.stashMap, key)
    }
}

// Access simulates the access operation.
func (o *ORAM) Access(read bool, blockId int, data Block) (Block) {
	//blockId is treated the same as Key
    // Step 1: Remap block
    previousPositionLeaf, exists := o.keyMap[blockId]
    if !exists {
        previousPositionLeaf = getRandomInt(1 << (o.logCapacity - 1)) //TODO: onyl for PUT
		o.stashMap[blockId] = data
    }
	
    o.keyMap[blockId] = getRandomInt(1 << (o.logCapacity - 1))

    // Step 2: Read path
    o.ReadPath(previousPositionLeaf, true) // Stash updated

    // Step 3: Update or read from stash
    if !read {
        o.stashMap[blockId] = data
    }
    
	returnValue := o.stashMap[blockId]

    // Step 4: Write path
    o.WritePath(previousPositionLeaf) // Stash updated

	return returnValue // returns the block
}