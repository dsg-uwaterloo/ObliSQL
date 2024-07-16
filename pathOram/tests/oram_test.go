package oram_test

import (
    "fmt"
    "testing"

    "github.com/schollz/progressbar/v3"
    "pathOram/oram"
)

const (
    logCapacity = 10 // Logarithm base 2 of capacity (1024 buckets)
    Z           = 5  // Number of blocks per bucket
    stashSize   = 20 // Maximum number of blocks in stash
)

func TestORAMReadWrite(t *testing.T) {
    // Initialize your ORAM structure or use a mocked instance
    o, err := oram.NewORAM(logCapacity, Z, stashSize, "localhost:6379")
    if err != nil {
        t.Fatalf("Error initializing ORAM: %v", err)
    }
    defer o.Close()

    totalOperations := 1000 // Number of operations you plan to perform

    // Progress bar for writes
    writeProgress := progressbar.Default(int64(totalOperations), "Writing: ")

    // Loop for writes
    for i := 0; i < totalOperations; i++ {
        key := i
        value := fmt.Sprintf("Value%d", i)
        o.Put(key, value)
        writeProgress.Add(1)
    }
    writeProgress.Finish()

    // Progress bar for reads
    readProgress := progressbar.Default(int64(totalOperations), "Reading: ")

    // Verify each key-value pair
    for i := 0; i < totalOperations; i++ {
        expectedValue := fmt.Sprintf("Value%d", i)
        retrievedValue := o.Get(i)
        if retrievedValue != expectedValue {
            t.Errorf("Mismatched value for key %d: expected %s, got %s", i, expectedValue, retrievedValue)
        }
        readProgress.Add(1)
    }
    readProgress.Finish()
}
