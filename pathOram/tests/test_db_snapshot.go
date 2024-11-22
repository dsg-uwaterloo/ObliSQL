package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"strconv"

	"pathOram/pkg/oram/oram"
	"pathOram/pkg/oram/request"
	"pathOram/pkg/oram/utils"

	// Used to load .env file

	"github.com/schollz/progressbar/v3"
)

const (
	logCapacity = 10 // Logarithm base 2 of capacity (1024 buckets)
	Z           = 5  // Number of blocks per bucket
	stashSize   = 20 // Maximum number of blocks in stash
)

// This function simulates operations on ORAM and stores snapshots of internal data.
func main() {

	key_input := "ura"
	// Generate the SHA-256 hash of the input string
	hash := sha256.New()
	hash.Write([]byte(key_input))

	// Return the 256-bit (32-byte) hash as a byte slice
	key := hash.Sum(nil)

	// --------------------

	// Initialize ORAM
	o, err := oram.NewORAM(logCapacity, Z, stashSize, "127.0.0.1:6379", true, key)
	if err != nil {
		log.Fatalf("Error initializing ORAM: %v", err)
	}
	defer o.RedisClient.Close()

	// // Print the Keymap after initializing ORAM
	// fmt.Println("Keymap after ORAM initialization:")
	// for key, value := range o.Keymap {
	// 	fmt.Printf("Key: %s, Value: %d\n", key, value)
	// }

	totalOperations := 1000 // Number of operations you plan to perform

	// Batch size for processing requests
	batchSize := 50 // Adjust batch size as needed

	// =============================================================

	// If you want to process GET requests and verify results, uncomment and modify below:
	getRequests := make([]request.Request, totalOperations)
	for i := 0; i < totalOperations; i++ {
		key := strconv.Itoa(i)
		getRequests[i] = request.Request{
			Key:   key,
			Value: "",
		}
	}

	readProgress := progressbar.Default(int64(totalOperations), "Reading: ")

	for i := 0; i < totalOperations; i += batchSize {
		end := i + batchSize
		if end > totalOperations {
			end = totalOperations
		}
		results, err := o.Batching(getRequests[i:end], batchSize)
		if err != nil {
			log.Fatalf("Error during GET batching: %v", err)
		}

		// Verify the results
		for j := 0; j < end-i; j++ {
			key := getRequests[i+j].Key
			expectedValue := fmt.Sprintf("Value%s", key)
			if results[j] != expectedValue {
				log.Printf("Mismatched value for key %s: expected %s, got %s", key, expectedValue, results[j])
			}
		}
		readProgress.Add(end - i)
	}

	readProgress.Finish()

	utils.PrintStashMap(o)
}
