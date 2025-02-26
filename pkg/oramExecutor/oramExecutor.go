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

type KVPair struct {
	channelId string
	Key       string
	Value     string
}

type responseChannel struct {
	m       *sync.RWMutex
	channel chan KVPair
}

type MyOram struct {
	executor.UnimplementedExecutorServer
	o *ORAM

	batchSize int

	channelMap    map[string]responseChannel
	requestNumber atomic.Int64
	channelLock   sync.RWMutex

	oramExecutorChannel chan *KVPair
}

type StringPair struct {
	First  string
	Second string
}

func (e *MyOram) ExecuteBatch(ctx context.Context, req *executor.RequestBatchORAM) (*executor.RespondBatchORAM, error) {
	if len(req.Keys) != len(req.Values) {
		return nil, fmt.Errorf("keys and values length mismatch")
	}

	reqNum := e.requestNumber.Add(1) // New id for this client/batch channel

	recv_resp := make([]KVPair, 0, len(req.Keys)) // This will store completed key value pairs

	channelId := fmt.Sprintf("%d-%d", req.RequestId, reqNum)
	localRespChannel := make(chan KVPair, len(req.Keys))

	e.channelLock.Lock() // Add channel to global map
	e.channelMap[channelId] = responseChannel{
		m:       &sync.RWMutex{},
		channel: localRespChannel,
	}
	e.channelLock.Unlock()

	sent := 0
	for i, key := range req.Keys {
		value := req.Values[i]
		kv := &KVPair{
			channelId: channelId,
			Key:       key,
			Value:     value,
		}
		// Block if the channel is full
		sent++
		e.oramExecutorChannel <- kv
	}

	// Finished adding keys to ORAM channel

	// Now wait for responses
	for i := 0; i < len(req.Keys); i++ {
		item := <-localRespChannel
		recv_resp = append(recv_resp, item)
	}

	close(localRespChannel)

	e.channelLock.Lock()
	delete(e.channelMap, channelId)
	e.channelLock.Unlock()

	sendKeys := make([]string, 0, len(req.Keys))
	sendVal := make([]string, 0, len(req.Keys))

	for _, v := range recv_resp {
		sendKeys = append(sendKeys, v.Key)
		sendVal = append(sendVal, v.Value)
	}

	// Return response with original request ID
	return &executor.RespondBatchORAM{
		RequestId: req.RequestId,
		Keys:      sendKeys,
		Values:    sendVal,
	}, nil
}

func (e *MyOram) processBatches() {
	for {

		if len(e.oramExecutorChannel) >= e.batchSize {
			var requestList []Request

			var chanIds []string

			for i := 0; i < e.batchSize; i++ {
				op := <-e.oramExecutorChannel // Read from channel

				chanIds = append(chanIds, op.channelId)

				requestList = append(requestList, Request{
					Key:   op.Key,
					Value: op.Value,
				})
			}
			// Execute ORAM batch
			returnValues, err := e.o.Batching(requestList, e.batchSize)
			if err != nil {
				// Handle error (e.g., log and continue)
				fmt.Printf("ORAM batch error: %v\n", err)
				continue
			}

			channelCache := make(map[string]chan KVPair, e.batchSize)

			e.channelLock.RLock()
			for _, v := range chanIds {

				channelCache[v] = e.channelMap[v].channel

			}
			e.channelLock.RUnlock()

			for i := 0; i < e.batchSize; i++ {
				newKVPair := KVPair{
					Key:   requestList[i].Key,
					Value: returnValues[i],
				}
				responseChannel := channelCache[chanIds[i]]
				responseChannel <- newKVPair
			}
		}
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
		fmt.Println("ORAM snapshot loaded successfully!")
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
		batchSize := 60
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
		o:                   oram,
		batchSize:           60, // Set from config or constant
		channelMap:          make(map[string]responseChannel),
		channelLock:         sync.RWMutex{},
		oramExecutorChannel: make(chan *KVPair),
	}

	myOram.oramExecutorChannel = make(chan *KVPair, 100000)

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
