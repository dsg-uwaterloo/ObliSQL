/*
Approach to debugging:
use new readpaths, old writepath,
remove batching in some parts if needed
make sure mget and mset are working as they should be
use printtree() and printstash() where needed
*/

package oram_test

import (
	"fmt"
	"strconv"
	"testing"

	"pathOram/pkg/oram/oram"
	"pathOram/pkg/oram/request"
	"pathOram/pkg/oram/utils"

	"github.com/schollz/progressbar/v3"
)

const (
	logCapacity = 10  // Logarithm base 2 of capacity (1024 buckets)
	Z           = 4   // Number of blocks per bucket
	stashSize   = 100 // Maximum number of blocks in stash
)

/*
This test case inserts 1000 values into the key-value store and then retrieves the key:values, asserting their correctness.
*/
/*
This test case inserts 1000 values into the key-value store and then retrieves them 5 times,
asserting their correctness in each iteration.
*/
func TestORAMReadWriteRepeatedReads(t *testing.T) {
	// Initialize ORAM
	o, err := oram.NewORAM(logCapacity, Z, stashSize, "127.0.0.1:6379", false, nil)
	if err != nil {
		t.Fatalf("Error initializing ORAM: %v", err)
	}
	defer o.RedisClient.Close()

	totalOperations := 1000 // Number of operations

	// Create PUT requests
	putRequests := make([]request.Request, totalOperations)
	for i := 0; i < totalOperations; i++ {
		key := strconv.Itoa(i)
		putRequests[i] = request.Request{
			Key:   key,
			Value: fmt.Sprintf("Value%d", i),
		}
	}

	// Create GET requests
	getRequests := make([]request.Request, totalOperations)
	for i := 0; i < totalOperations; i++ {
		key := strconv.Itoa(i)
		getRequests[i] = request.Request{
			Key:   key,
			Value: "",
		}
	}

	batchSize := 50 // Adjust batch size as needed

	// Process PUT requests in batches
	writeProgress := progressbar.Default(int64(totalOperations), "Writing: ")
	for i := 0; i < totalOperations; i += batchSize {
		end := i + batchSize
		if end > totalOperations {
			end = totalOperations
		}
		_, err := o.Batching(putRequests[i:end], batchSize)
		if err != nil {
			t.Fatalf("Error during PUT batching: %v", err)
		}
		writeProgress.Add(end - i)
	}
	writeProgress.Finish()

	fmt.Println("Stash after writes:")
	utils.PrintStashMap(o)

	// Read all values 5 times
	for readIteration := 1; readIteration <= 5; readIteration++ {
		t.Logf("--- Reading iteration %d ---", readIteration)

		readProgress := progressbar.Default(int64(totalOperations), fmt.Sprintf("Reading (Iter %d): ", readIteration))
		for i := 0; i < totalOperations; i += batchSize {
			end := i + batchSize
			if end > totalOperations {
				end = totalOperations
			}
			results, err := o.Batching(getRequests[i:end], batchSize)
			if err != nil {
				t.Fatalf("Error during GET batching (Iter %d): %v", readIteration, err)
			}

			// Verify each key-value pair
			for j := 0; j < end-i; j++ {
				key := getRequests[i+j].Key
				expectedValue := fmt.Sprintf("Value%s", key)
				if results[j] != expectedValue {
					t.Errorf("Mismatch in Iter %d for key %s: expected %s, got %s", readIteration, key, expectedValue, results[j])
				}
			}
			readProgress.Add(end - i)
		}
		readProgress.Finish()

		fmt.Printf("Stash after read iteration %d:\n", readIteration)
		utils.PrintStashMap(o)
	}
}
