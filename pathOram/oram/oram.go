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

    err = o.redisClient.Set(o.ctx, key, encryptedData, 0).Err() // the bucket index is the key
    return err
}

// Read a bucket from Redis
func (o *ORAM) readBucketFromDb(index int) (Bucket, error) {
    key := fmt.Sprintf("bucket:%d", index)
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
