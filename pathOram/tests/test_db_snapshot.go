package main

import (
	"bufio"
	"crypto/sha256"
	"log"
	"os"
	"strings"

	"pathOram/pkg/oram/oram"
	"pathOram/pkg/oram/request"
	"pathOram/pkg/oram/utils"

	// Used to load .env file

	"github.com/schollz/progressbar/v3"
)

const (
	logCapacity = 19      // Logarithm base 2 of capacity (1024 buckets)
	Z           = 4       // Number of blocks per bucket
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

	//totalOperations := 1000 // Number of operations you plan to perform

	// Batch size for processing requests
	batchSize := 50 // Adjust batch size as needed

	// =============================================================

	var dataMap map[string]string = make(map[string]string)

	// Load data from tracefile and create Request objects
	var getRequests []request.Request
	file, err := os.Open("/Users/nachiketrao/Desktop/URA/Tracefiles/ThreeExecutors/serverInput_2.txt") // TODO: define tracefile path here
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
			dataMap[key] = value
			getRequests = append(getRequests, request.Request{Key: key, Value: ""})
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("error reading tracefile: %v", err)
	}

	// If you want to process GET requests and verify results, uncomment and modify below:
	// getRequests := make([]request.Request, totalOperations)
	// for i := 0; i < totalOperations; i++ {
	// 	key := strconv.Itoa(i)
	// 	getRequests[i] = request.Request{
	// 		Key:   key,
	// 		Value: "",
	// 	}
	// }

	readProgress := progressbar.Default(int64(len(getRequests)), "Reading: ")

	for i := 0; i < len(getRequests); i += batchSize {
		end := i + batchSize
		if end > len(getRequests) {
			end = len(getRequests)
		}
		results, err := o.Batching(getRequests[i:end], batchSize)
		if err != nil {
			log.Fatalf("Error during GET batching: %v", err)
		}

		// Verify the results
		for j := 0; j < end-i; j++ {
			key := getRequests[i+j].Key
			//expectedValue := fmt.Sprintf("Value%s", key)
			if results[j] != dataMap[key] {
				log.Printf("Mismatched value for key %s: expected %s, got %s", key, dataMap[key], results[j])
			}
		}
		readProgress.Add(end - i)
	}

	readProgress.Finish()

	utils.PrintStashMap(o)
}
