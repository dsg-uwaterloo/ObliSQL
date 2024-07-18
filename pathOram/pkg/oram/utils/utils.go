package utils

import (
	"fmt"
	"strings"

	"pathOram/pkg/oram/oram"
	"pathOram/pkg/oram/bucket"
)

// PrintTree prints the ORAM tree structure by reading the file sequentially
func PrintTree(o *oram.ORAM) {
	totalBuckets := (1 << (o.LogCapacity + 1)) - 1
	for i := 0; i < totalBuckets; i++ {
		b, _ := o.RedisClient.ReadBucketFromDb(i)
		indent := strings.Repeat("  ", o.GetDepth(i))
		fmt.Printf("%sBucket %d:\n", indent, i)
		for _, blk := range b.Blocks {
			//j = j
			fmt.Printf("%s  Blockid %d: Key=%d, Value=%s\n", indent, blk.BlockId, blk.Key, blk.Value)
		}
	}
}

// PrintStashMap prints the contents of the stash map
func PrintStashMap(o *oram.ORAM) {
	fmt.Println("Stash Map contents:")
	for blockId, blk := range o.StashMap {
		fmt.Printf("BlockId: %d, Key: %d, Value: %s\n", blockId, blk.Key, blk.Value)
	}
}

// Exported put function used for testing by directly inserting bucket (escaping writepath)
func PutBucket(o *oram.ORAM, index int, b bucket.Bucket) {
	o.RedisClient.WriteBucketToDb(index, b)
}
