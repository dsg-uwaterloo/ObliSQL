package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	executor "pathOram/api/executor"

	"github.com/golang/protobuf/ptypes/wrappers"
	// "github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	"pathOram/pkg/oram/oram"
	"pathOram/pkg/oram/request"
)

const (
	logCapacity = 10   // Logarithm base 2 of capacity (1024 buckets)
	Z           = 5    // Number of blocks per bucket
	stashSize   = 2000 // Maximum number of blocks in stash
)

type myExecutor struct {
	executor.UnimplementedExecutorServer
	// rdb *redis.Client
	o *oram.ORAM
}

type StringPair struct {
	First  string
	Second string
}

func (e myExecutor) ExecuteBatch(ctx context.Context, req *executor.RequestBatch) (*executor.RespondBatch, error) {
	fmt.Printf("Got a request with ID: %d \n", req.RequestId)

	// set batchsize
	batchSize := 50

	// Batching(requests []request.Request, batchSize int)

	var replyKeys []string
	var replyVals []string

	for start := 0; start < len(req.Values); start += batchSize {

		var requestList []request.Request
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
			currentRequest := request.Request{
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

	return &executor.RespondBatch{
		RequestId: req.RequestId,
		Keys:      replyKeys,
		Values:    replyVals,
	}, nil
}

func (e myExecutor) InitDb(ctx context.Context, req *executor.RequestBatch) (*wrappers.BoolValue, error) {
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

		var requestList []request.Request

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
			currentRequest := request.Request{
				Key:   batchKeys[i],
				Value: batchValues[i],
			}

			requestList = append(requestList, currentRequest)
		}
		e.o.Batching(requestList, batchSize)
	}

	return &wrappers.BoolValue{Value: true}, nil

}

func main() {
	redisHost := flag.String("rh", "127.0.0.1", "Redis Host")
	redisPort := flag.String("rp", "6379", "Redis Host")
	addrPort := flag.String("p", "9090", "Executor Port")

	flag.Parse()

	lis, err := net.Listen("tcp", ":"+*addrPort)
	if err != nil {
		log.Fatalf("Cannot create listener on port :9090 %s", err)
	}

	addr := *redisHost + ":" + *redisPort

	oram_object, err := oram.NewORAM(logCapacity, Z, stashSize, addr, false, nil)
	if err != nil {
		log.Fatalf("Error initializing ORAM: %v", err)
	}
	defer oram_object.RedisClient.Close()

	fmt.Println("Connected to Redis Server on: ", addr)

	// service := myExecutor{rdb: rdb}
	service := myExecutor{o: oram_object}
	serverRegister := grpc.NewServer(grpc.MaxRecvMsgSize(644000*300), grpc.MaxSendMsgSize(644000*300))

	log.Println("Starting Server!")
	executor.RegisterExecutorServer(serverRegister, service)
	err = serverRegister.Serve(lis)
	if err != nil {
		log.Fatalf("Error! Could not start server %s", err)
	}

}
