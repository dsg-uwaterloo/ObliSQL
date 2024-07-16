package main

import (
	"fmt"
	"pathOram/oram"
)

/*
Bucket indices go 0, 1, 2, 3, ....

Levels go 0, 1, 2, 3, ..., logcapacity where logcapacity = height of tree - 1

Leaf ids go from 0, 1, 2, 3, 4, ..., 2^logcapacity - 1
*/
func main() {
	logCapacity := 2 // height of tree - 1
	Z := 4
	redisAddr := "127.0.0.1:6379" // local host default redis port


	// Create a new ORAM instance
	oramInstance, _ := oram.NewORAM(logCapacity, Z, 1000, redisAddr)


	// bucket := oram.Bucket{
	// 	Blocks: []oram.Block{
	// 		{BlockId: 192, Key: 1234, Value: "Block1234"},
	// 		{BlockId: 193, Key: 5678, Value: "Block5678"},
	// 	},
	// }
	// oramInstance.PutBucket(0, bucket) // Writing to the root bucket (index 0)

	oramInstance.PrintTree()

	fmt.Println()

	oramInstance.ReadPath(3, true)

	oramInstance.PrintStashMap()

	fmt.Println("--------ORAM core functionality tests--------")

	value := oramInstance.Put(555, "Waterloo")

	// Print the retrieved key
	fmt.Println("Read key 555 value:", value)

	oramInstance.PrintTree()

	//value = oramInstance.Put(555, "Toronto")

	fmt.Println()

	value = oramInstance.Get(555)

	// Print the retrieved key
	fmt.Println("Read key 555 value:", value)

	oramInstance.PrintTree()

	oramInstance.Close()
}
