package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"pathOram/pkg/oram/oram"
	"pathOram/pkg/oram/request"
	"pathOram/pkg/oram/utils"

	"github.com/schollz/progressbar/v3"
)

/*
The problem is - we need to pass in the same key to redis client for both generated and
testing redis client. otherwise it tries to decrypt with a different key -> doesn't find
the values it's looking for cuz the decryption results in an arbitrary value/object.
The batching mechanism returns a -1 for any key it didn't find.
*/

const (
	logCapacity = 22      // Logarithm base 2 of capacity (1024 buckets)
	Z           = 5       // Number of blocks per bucket
	stashSize   = 7000000 // Maximum number of blocks in stash
)

// This function simulates operations on ORAM and stores snapshots of internal data.
func main() {
	key_input := "oblisqloram"
	// Generate the SHA-256 hash of the input string
	hash := sha256.New()
	hash.Write([]byte(key_input))

	// Return the 256-bit (32-byte) hash as a byte slice
	key := hash.Sum(nil)

	// -----------

	// Initialize ORAM
	o, err := oram.NewORAM(logCapacity, Z, stashSize, "127.0.0.1:6379", false, key)
	if err != nil {
		log.Fatalf("Error initializing ORAM: %v", err)
	}
	defer o.RedisClient.Close()

	// totalOperations := 1000 // Number of operations you plan to perform

	// Use for testing:
	// Create PUT requests
	// putRequests := make([]request.Request, totalOperations)
	// for i := 0; i < totalOperations; i++ {
	// 	key := strconv.Itoa(i)
	// 	putRequests[i] = request.Request{
	// 		Key:   key,
	// 		Value: fmt.Sprintf("Value%d", i),
	// 	}
	// }

	fmt.Println("Loading data from tracefile now:")

	// Load data from tracefile and create Request objects
	var requests []request.Request
	file, err := os.Open("/Users/nachiketrao/Desktop/URA/serverInput.txt") // TODO: define tracefile path here
	if err != nil {
		log.Fatalf("failed to open tracefile: %v", err)
	}
	defer file.Close()

	const maxBufferSize = 1024 * 1024 // 1MB

	scanner := bufio.NewScanner(file)
	buffer := make([]byte, maxBufferSize)
	scanner.Buffer(buffer, maxBufferSize)

	for scanner.Scan() {
		line := scanner.Text()
		// Only process lines that start with "SET"
		if strings.HasPrefix(line, "SET") {
			parts := strings.SplitN(line, " ", 3)
			if len(parts) != 3 {
				continue // skip lines that don't have exactly 3 parts
			}
			key := parts[1]
			value := parts[2]
			requests = append(requests, request.Request{Key: key, Value: value})
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("error reading tracefile: %v", err)
	}

	fmt.Println("Finished scanning tracefile")

	fmt.Println("Setting values in DB...")

	// Initialize DB with tracefile contents and display a progress bar
	batchSize := 10
	bar := progressbar.Default(int64(len(requests)), "Setting values...")

	for start := 0; start < len(requests); start += batchSize {
		end := start + batchSize
		if end > len(requests) {
			end = len(requests) // Ensure we don't go out of bounds
		}

		o.Batching(requests[start:end], batchSize)

		// Increment the progress bar by the batch size or remaining items
		_ = bar.Add(end - start)
	}

	bar.Finish()
	fmt.Println("Finished Initializing DB!")

	// Optionally print the ORAM's internal tree (this might be useful for debugging)
	// utils.PrintTree(o)

	// Print the stash map to inspect the internal state of ORAM
	fmt.Println("Printing the stash...")
	utils.PrintStashMap(o)

	// =============================================================
	// Storing Keymap and StashMap in a JSON file
	a := o.Keymap
	b := o.StashMap

	// Prepare data to write to file
	data := map[string]interface{}{
		"Keymap":   a,
		"StashMap": b,
	}

	// Open or create the snapshot file
	file, err = os.Create("proxy_snapshot.json")
	if err != nil {
		log.Fatalf("Error creating file: %v", err)
	}
	defer file.Close()

	// Serialize data to JSON and write to the file
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Indentation for better readability
	if err := encoder.Encode(data); err != nil {
		log.Fatalf("Error encoding data to JSON: %v", err)
	}

	fmt.Println("Data written to file: proxy_snapshot.json")

	// =============================================================

	// Trigger a save to dump.rdb
	fmt.Println("Triggering RDB snapshot save (BGSAVE)...")
	err = o.RedisClient.Client.BgSave(o.RedisClient.Ctx).Err() // BGSAVE command
	if err != nil {
		log.Fatalf("Error triggering RDB snapshot save: %v", err)
	}
	fmt.Println("RDB snapshot save triggered successfully.")

}
