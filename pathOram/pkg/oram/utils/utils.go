package utils

import (
	"fmt"
	"strings"

	"pathOram/pkg/oram/bucket"
	"pathOram/pkg/oram/oram"
)

// PrintTree prints the ORAM tree structure by reading the file sequentially
// Print only the blocks (and corresponding buckets) that have a non dummy key:value pair
func PrintTree(o *oram.ORAM) {
	totalBuckets := (1 << (o.LogCapacity + 1)) - 1
	for i := 0; i < totalBuckets; i++ {
		b, _ := o.RedisClient.ReadBucketFromDb(i)
		indent := strings.Repeat("  ", o.GetDepth(i))
		for _, blk := range b.Blocks {
			if blk.GetKey() != "-1" {
				fmt.Printf("%sBucket %d:\n", indent, i)
				fmt.Printf("%s  Key=%s, Value=%s\n", indent, blk.GetKey(), blk.GetValue())
			}
		}
	}
}

// PrintStashMap prints the contents of the stash map
func PrintStashMap(o *oram.ORAM) {
	fmt.Println("Stash Map contents:")
	for _, blk := range o.StashMap {
		fmt.Printf("Key: %s, Value: %s\n", blk.GetKey(), blk.GetValue())
	}
}

// Exported put function used for testing by directly inserting bucket (escaping writepath)
func PutBucket(o *oram.ORAM, index int, b bucket.Bucket) {
	o.RedisClient.WriteBucketToDb(index, b)
}
