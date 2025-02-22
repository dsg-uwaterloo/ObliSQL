package oramexecutor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	executor "github.com/project/ObliSql/api/oramExecutor"
	// "github.com/redis/go-redis/v9"
	"github.com/schollz/progressbar/v3"
)

const (
	logCapacity = 15     // Logarithm base 2 of capacity (1024 buckets)
	Z           = 5      // Number of blocks per bucket
	stashSize   = 100000 // Maximum number of blocks in stash
)

type Operation struct {
	RequestID uint64
	Key       string
	Value     string
	Index     int
}

type ResponseTracker struct {
	Values    []string
	Count     int32 // Use atomic operations
	Completed chan struct{}
}

type MyOram struct {
	executor.UnimplementedExecutorServer
	o *ORAM

	opMutex sync.Mutex
	opCond  *sync.Cond
	opQueue []Operation

	trackerMutex sync.Mutex
	trackers     map[uint64]*ResponseTracker

	requestID uint64 // atomic
	batchSize int
}

type StringPair struct {
	First  string
	Second string
}

func (e *MyOram) ExecuteBatch(ctx context.Context, req *executor.RequestBatchORAM) (*executor.RespondBatchORAM, error) {
	if len(req.Keys) != len(req.Values) {
		return nil, fmt.Errorf("keys and values length mismatch")
	}

	// Generate unique request ID
	id := atomic.AddUint64(&e.requestID, 1)
	numOps := len(req.Keys)

	// Setup response tracker
	tracker := &ResponseTracker{
		Values:    make([]string, numOps),
		Count:     0,
		Completed: make(chan struct{}),
	}

	// Register tracker
	e.trackerMutex.Lock()
	e.trackers[id] = tracker
	e.trackerMutex.Unlock()

	// Queue operations
	e.opMutex.Lock()
	for i := 0; i < numOps; i++ {
		e.opQueue = append(e.opQueue, Operation{
			RequestID: id,
			Key:       req.Keys[i],
			Value:     req.Values[i],
			Index:     i,
		})
	}
	e.opCond.Signal() // Notify batch processor
	e.opMutex.Unlock()

	// Wait for completion
	<-tracker.Completed

	// Return response with original request ID
	return &executor.RespondBatchORAM{
		RequestId: req.RequestId,
		Keys:      req.Keys,
		Values:    tracker.Values,
	}, nil
}

func (e *MyOram) processBatches() {
	for {
		e.opMutex.Lock()
		// Wait for operations to process
		for len(e.opQueue) == 0 {
			e.opCond.Wait()
		}

		// Determine batch size
		batchSize := e.batchSize
		if len(e.opQueue) < batchSize {
			e.opCond.Wait()
		}
		batchOps := e.opQueue[:batchSize]
		e.opQueue = e.opQueue[batchSize:]
		e.opMutex.Unlock()

		// Prepare ORAM batch request
		var requestList []Request
		for _, op := range batchOps {
			requestList = append(requestList, Request{
				Key:   op.Key,
				Value: op.Value,
			})
		}

		// Execute ORAM batch
		returnValues, err := e.o.Batching(requestList, batchSize)
		if err != nil {
			// Handle error (e.g., log and continue)
			fmt.Printf("ORAM batch error: %v\n", err)
			continue
		}

		// Distribute responses to trackers
		e.trackerMutex.Lock()
		for i, op := range batchOps {
			tracker, exists := e.trackers[op.RequestID]
			if !exists {
				continue // Tracker already removed
			}

			// Update value atomically
			tracker.Values[op.Index] = returnValues[i]
			count := atomic.AddInt32(&tracker.Count, 1)

			// Check if all responses received
			if int(count) == len(tracker.Values) {
				close(tracker.Completed)
				delete(e.trackers, op.RequestID)
			}
		}
		e.trackerMutex.Unlock()
	}
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
		o:         oram,
		batchSize: 60, // Set from config or constant
		trackers:  make(map[uint64]*ResponseTracker),
		opQueue:   make([]Operation, 0),
	}
	myOram.opCond = sync.NewCond(&myOram.opMutex)
	go myOram.processBatches() // Start batch processing

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
