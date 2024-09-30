package oram_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"pathOram/pkg/oram/oram"
	"pathOram/pkg/oram/request"
	"pathOram/pkg/oram/utils"

	"github.com/schollz/progressbar/v3"
)

const (
	logCapacity = 10 // Logarithm base 2 of capacity (1024 buckets)
	Z           = 5  // Number of blocks per bucket
	stashSize   = 20 // Maximum number of blocks in stash
)

func TestORAMFakeReadDuplicateKeys(t *testing.T) {
	// Initialize ORAM structure
	o, err := oram.NewORAM(logCapacity, Z, stashSize, "127.0.0.1:6379")
	if err != nil {
		t.Fatalf("Error initializing ORAM: %v", err)
	}
	defer o.RedisClient.Close()

	totalOperations := 1000    // Number of operations to perform
	keys := []string{"1", "2"} // Keys to use for this test
	batchSize := 50            // Batch size for processing requests

	// Seed the random number generator for reproducibility
	rand.Seed(time.Now().UnixNano())

	// Map to keep track of expected values
	expectedValues := map[string]string{
		"1": "",
		"2": "",
	}

	// Slice to store the sequence of requests
	requests := make([]request.Request, totalOperations)

	// Generate random requests
	for i := 0; i < totalOperations; i++ {
		key := keys[rand.Intn(len(keys))] // Randomly pick key 1 or 2
		if rand.Float32() < 0.5 {
			// Write operation
			value := fmt.Sprintf("Value%d", i)
			requests[i] = request.Request{
				Key:   key,
				Value: value,
			}
		} else {
			// Read operation
			requests[i] = request.Request{
				Key:   key,
				Value: "",
			}
		}
	}

	// Process requests in batches
	progress := progressbar.Default(int64(totalOperations), "Processing: ")

	for i := 0; i < totalOperations; i += batchSize {
		end := i + batchSize
		if end > totalOperations {
			end = totalOperations
		}

		batch := requests[i:end]
		results, err := o.Batching(batch, batchSize)
		if err != nil {
			t.Fatalf("Error during batch operation: %v", err)
		}

		// Handle the results of the batch
		for j, req := range batch {
			if req.Value != "" {
				// Update expected value for PUT request
				expectedValues[req.Key] = req.Value
			} else {
				// Verify the result of the GET request
				expectedValue := expectedValues[req.Key]
				if expectedValue == "" {
					expectedValue = "-1"
				}
				if results[j] != expectedValue {
					t.Errorf("Mismatched value for key %s: expected %s, got %s", req.Key, expectedValue, results[j])
				}
			}
		}
		progress.Add(end - i)
	}

	progress.Finish()

	fmt.Println("Final stash state:")
	utils.PrintStashMap(o)
}
