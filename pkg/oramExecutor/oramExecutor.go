package oramexecutor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	executor "github.com/project/ObliSql/api/oramExecutor"
	// "github.com/redis/go-redis/v9"
	"github.com/schollz/progressbar/v3"
)

const (
	logCapacity = 15     // Logarithm base 2 of capacity (1024 buckets)
	Z           = 5      // Number of blocks per bucket
	stashSize   = 100000 // Maximum number of blocks in stash
)

type MyOram struct {
	executor.UnimplementedExecutorServer
	// rdb *redis.Client
	o *ORAM
}

type StringPair struct {
	First  string
	Second string
}

func (e MyOram) ExecuteBatch(ctx context.Context, req *executor.RequestBatchORAM) (*executor.RespondBatchORAM, error) {
	fmt.Printf("Got a request with ID: %d \n", req.RequestId)

	// set batchsize
	batchSize := 60

	// Batching(requests []request.Request, batchSize int)

	var replyKeys []string
	var replyVals []string

	for start := 0; start < len(req.Values); start += batchSize {

		var requestList []Request
		var returnValues []string

		end := start + batchSize
		if end > len(req.Values) {
			end = len(req.Values) // Ensure we don't go out of bounds
		}

		// Slice the keys and values for the current batch
		batchKeys := req.Keys[start:end]
		batchValues := req.Values[start:end]

		for i := range batchKeys {
			// Read operation
			currentRequest := Request{
				Key:   batchKeys[i],
				Value: batchValues[i],
			}

			requestList = append(requestList, currentRequest)
		}

		returnValues, _ = e.o.Batching(requestList, batchSize)

		replyKeys = append(replyKeys, batchKeys...)
		replyVals = append(replyVals, returnValues...)

	}

	return &executor.RespondBatchORAM{
		RequestId: req.RequestId,
		Keys:      replyKeys,
		Values:    replyVals,
	}, nil
}

func NewORAM(LogCapacity, Z, StashSize int, redisAddr string, tracefile string, useSnapshot bool, key []byte) (*MyOram, error) {
	// If key is not provided (nil or empty), generate a random key
	if len(key) == 0 {
		var err error
		key, err = GenerateRandomKey()
		if err != nil {
			return nil, err
		}
	}

	client, err := NewRedisClient(redisAddr, key)
	if err != nil {
		return nil, err
	}

	oram := &ORAM{
		RedisClient: client,
		LogCapacity: LogCapacity,
		Z:           Z,
		StashSize:   StashSize,
		StashMap:    make(map[string]Block),
		keyMap:      make(map[string]int),
	}

	if useSnapshot {
		// Load the Stashmap and Keymap into memory
		// Allow redis to update state using dump.rdb
		oram.loadSnapshotMaps()
	} else {
		// Clear the Redis database to ensure a fresh start
		if err := client.FlushDB(); err != nil {
			return nil, fmt.Errorf("failed to flush Redis database: %v", err)
		}
		oram.initialize()

		// Load data from tracefile and create Request objects
		var requests []Request
		file, err := os.Open(tracefile)
		if err != nil {
			return nil, fmt.Errorf("failed to open tracefile: %v", err)
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
				requests = append(requests, Request{Key: key, Value: value})
			}
		}

		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading tracefile: %v", err)
		}

		// Initialize DB with tracefile contents and display a progress bar
		batchSize := 10
		bar := progressbar.Default(int64(len(requests)), "Setting values...")

		for start := 0; start < len(requests); start += batchSize {
			end := start + batchSize
			if end > len(requests) {
				end = len(requests) // Ensure we don't go out of bounds
			}

			oram.Batching(requests[start:end], batchSize)

			// Increment the progress bar by the batch size or remaining items
			_ = bar.Add(end - start)
		}

		bar.Finish()
		fmt.Println("Finished Initializing DB!")
	}

	myOram := &MyOram{
		o: oram,
	}

	return myOram, nil
}

// Load Keymap and Stashmap into memory
func (oram *ORAM) loadSnapshotMaps() {
	// Read from snapshot.json
	// Open the file for reading
	file, err := os.Open("proxy_snapshot.json")
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return // No need to return anything here, just exit the function
	}
	defer file.Close()

	// Decode JSON data into a map
	var data map[string]interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		fmt.Printf("Error decoding JSON data: %v\n", err)
		return // No need to return anything here, just exit the function
	}

	// Load Keymap
	if keymap, ok := data["Keymap"].(map[string]interface{}); ok {
		// Convert map[string]interface{} to map[string]int for Keymap
		for key, value := range keymap {
			if val, ok := value.(float64); ok { // JSON numbers are decoded as float64
				oram.keyMap[key] = int(val) // Convert float64 to int
			}
		}
	} else {
		fmt.Println("Error: Keymap data is not of expected type")
	}

	// Load StashMap
	if stashmap, ok := data["StashMap"].(map[string]interface{}); ok {
		// Convert map[string]interface{} to map[string]block.Block for StashMap
		for key, value := range stashmap {
			// Assuming block.Block is a struct, and you need to decode the value into that type
			stashBlock, ok := value.(map[string]interface{}) // Assuming stash block is a map
			if !ok {
				fmt.Println("Error: StashMap block is not of expected type")
				continue
			}

			// Marshal and unmarshal to convert the stash block map to block.Block
			stashBlockData, err := json.Marshal(stashBlock)
			if err != nil {
				fmt.Printf("Error marshaling stash block data: %v\n", err)
				continue
			}
			var blockData Block
			err = json.Unmarshal(stashBlockData, &blockData)
			if err != nil {
				fmt.Printf("Error unmarshaling stash block: %v\n", err)
				continue
			}
			// Add the block to StashMap
			oram.StashMap[key] = blockData
		}
	} else {
		fmt.Println("Error: StashMap data is not of expected type")
	}
}
