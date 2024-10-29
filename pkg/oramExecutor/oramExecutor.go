package oramexecutor

import (
	"context"
	"fmt"

	executor "github.com/project/ObliSql/api/oramExecutor"

	"github.com/golang/protobuf/ptypes/wrappers"
	// "github.com/redis/go-redis/v9"
)

const (
	logCapacity = 10   // Logarithm base 2 of capacity (1024 buckets)
	Z           = 5    // Number of blocks per bucket
	stashSize   = 2000 // Maximum number of blocks in stash
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
	batchSize := 50

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

		// TODO: create a list of Request objects with Type and key and value

		// TODO: call o.Batching(requests, batch size) this will return a list of VALUES

		// TODO: handle it like replyKeys = append(replyKeys, rec_block.Key)
		// replyVals = append(replyVals, rec_block.Value)

	}

	return &executor.RespondBatchORAM{
		RequestId: req.RequestId,
		Keys:      replyKeys,
		Values:    replyVals,
	}, nil
}

func (e MyOram) InitDb(ctx context.Context, req *executor.RequestBatchORAM) (*wrappers.BoolValue, error) {
	fmt.Printf("Initialize DB with Key Size: %d \n", len(req.Keys))

	e.o.RedisClient.FlushData()
	e.o.ClearKeymap()
	e.o.ClearStash()
	// var pairs []interface{}

	// TODO: split keys and values by batchsize
	// TODO: call o.Batching(requests, batch size) this will return a list of VALUES - can be neglected

	// TODO: need to tune oram.go to distinguish between put and get requests based on presence of value in requests object

	// set batchsize
	batchSize := 50

	for start := 0; start < len(req.Values); start += batchSize {
		var requestList []Request

		end := start + batchSize
		if end > len(req.Values) {
			end = len(req.Values) // Ensure we don't go out of bounds
		}

		// Slice the keys and values for the current batch
		batchKeys := req.Keys[start:end]
		batchValues := req.Values[start:end]

		for i := range batchKeys {
			//pairs = append(pairs, req.Keys[i], req.Values[i])
			// Read operation
			currentRequest := Request{
				Key:   batchKeys[i],
				Value: batchValues[i],
			}

			requestList = append(requestList, currentRequest)
		}
		e.o.Batching(requestList, batchSize)
	}
	fmt.Println("Finished Initializing DB!")
	return &wrappers.BoolValue{Value: true}, nil

}

func NewORAM(LogCapacity, Z, StashSize int, redisAddr string) (*MyOram, error) {
	key, err := GenerateRandomKey()
	if err != nil {
		return nil, err
	}

	client, err := NewRedisClient(redisAddr, key)
	if err != nil {
		return nil, err
	}

	// Clear the Redis database to ensure a fresh start
	if err := client.FlushDB(); err != nil {
		return nil, fmt.Errorf("failed to flush Redis database: %v", err)
	}

	oram := &ORAM{
		RedisClient: client,
		LogCapacity: LogCapacity,
		Z:           Z,
		StashSize:   StashSize,
		StashMap:    make(map[string]Block),
		keyMap:      make(map[string]int),
	}

	oram.initialize()

	myOram := &MyOram{
		o: oram,
	}

	return myOram, nil
}
