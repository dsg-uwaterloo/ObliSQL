
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
    "testing"

    "github.com/schollz/progressbar/v3"
    "pathOram/pkg/oram/oram"
	"pathOram/pkg/oram/request"
	"pathOram/pkg/oram/utils"
)

const (
    logCapacity = 10 // Logarithm base 2 of capacity (1024 buckets)
    Z           = 5  // Number of blocks per bucket
    stashSize   = 20 // Maximum number of blocks in stash
)

/*
This test case inserts 1000 values into the key-value store and then retrieves the key:values, asserting their correctness.
*/
func TestORAMReadWrite(t *testing.T) {
    // Initialize your ORAM structure or use a mocked instance
    o, err := oram.NewORAM(logCapacity, Z, stashSize, "127.0.0.1:6379")
    if err != nil {
        t.Fatalf("Error initializing ORAM: %v", err)
    }
    defer o.RedisClient.Close()

    totalOperations := 1000 // Number of operations you plan to perform

	// Create PUT requests
    putRequests := make([]request.Request, totalOperations)
    for i := 0; i < totalOperations; i++ {
        putRequests[i] = request.Request{
            Type:  "PUT",
            Key:   i,
            Value: fmt.Sprintf("Value%d", i),
        }
    }

	// Create GET requests
    getRequests := make([]request.Request, totalOperations)
    for i := 0; i < totalOperations; i++ {
        getRequests[i] = request.Request{
            Type: "GET",
            Key:  i,
			Value: "",
        }
    }

    // Batch size for processing requests
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

	//fmt.Println("Printing the tree")
	//utils.PrintTree(o)

	fmt.Println("Printing the stash...")
	utils.PrintStashMap(o)

    // Process GET requests in batches and verify results
	readProgress := progressbar.Default(int64(totalOperations), "Reading: ")

	for i := 0; i < totalOperations; i += batchSize {
		end := i + batchSize
		if end > totalOperations {
			end = totalOperations
		}
		results, err := o.Batching(getRequests[i:end], batchSize)
		if err != nil {
			t.Fatalf("Error during GET batching: %v", err)
		}

		// Verify each key-value pair for the current batch
		for j := 0; j < end-i; j++ {
			key := getRequests[i+j].Key
			expectedValue := fmt.Sprintf("Value%d", key)
			if results[j] != expectedValue {
				t.Errorf("Mismatched value for key %d: expected %s, got %s", key, expectedValue, results[j])
			}
		}
		readProgress.Add(end - i)
	}

	readProgress.Finish()

    fmt.Println("Printing the stash...")
	utils.PrintStashMap(o)
}
