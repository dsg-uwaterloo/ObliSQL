// THIS FILE DOESN't APPLY TO THE CURRENT VERSION OF ENHANCED ORAM EXECUTABLE

// package main

// import (
// 	"fmt"

// 	"pathOram/pkg/oram/block"
// 	"pathOram/pkg/oram/bucket"
// 	"pathOram/pkg/oram/oram"
// 	"pathOram/pkg/oram/utils"
// )

// func main() {
// 	logCapacity := 2 // height of tree - 1
// 	Z := 4
// 	redisAddr := "127.0.0.1:6379" // local host default redis port

// 	// Create a new ORAM instance
// 	oramInstance, err := oram.NewORAM(logCapacity, Z, 1000, redisAddr)
// 	if err != nil {
// 		fmt.Printf("Error creating ORAM instance: %v\n", err)
// 		return
// 	}
// 	defer oramInstance.RedisClient.Close()

// 	// Create a bucket and write it to the root bucket (index 0)
// 	bucket := bucket.Bucket{
// 		Blocks: []block.Block{
// 			{Key: "1234", Value: "Block1234"},
// 			{Key: "5678", Value: "Block5678"},
// 		},
// 	}
// 	utils.PutBucket(oramInstance, 0, bucket)

// 	// Print the ORAM tree
// 	utils.PrintTree(oramInstance)
// 	fmt.Println()

// 	// Perform read path operation
// 	oramInstance.ReadPath("3", true)

// 	// Print the stash map
// 	utils.PrintStashMap(oramInstance)

// 	fmt.Println("--------ORAM core functionality tests--------")

// 	// Put a value into the ORAM
// 	oramInstance.Put(555, "Waterloo")

// 	// Retrieve and print the value for key 555
// 	value := oramInstance.Get(555)
// 	fmt.Println("Read key 555 value:", value)

// 	// Print the ORAM tree again
// 	utils.PrintTree(oramInstance)

// 	// Uncomment if you want to update the value for key 555
// 	// oramInstance.Put(555, "Toronto")

// 	// Retrieve and print the updated value for key 555
// 	value = oramInstance.Get(555)
// 	fmt.Println("Read key 555 value:", value)

// 	// Print the ORAM tree one last time
// 	utils.PrintTree(oramInstance)
// }