package main

import (
	"context"
	"fmt"
	"log"
	"net"

	executor "github.com/Haseeb1399/WorkingThesis/api/executor"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

type myExecutor struct {
	executor.UnimplementedExecutorServer
	rdb *redis.Client
}

type StringPair struct {
	First  string
	Second string
}

func (e myExecutor) ExecuteBatch(ctx context.Context, req *executor.RequestBatch) (*executor.RespondBatch, error) {
	fmt.Printf("Got a request with ID: %d \n", req.RequestId)
	threadCtx := context.Background()
	var runningKeys []StringPair
	localCache := make(map[string]string)

	//First Get All Keys
	serverResult, err := e.rdb.MGet(threadCtx, req.Keys...).Result()

	if err != nil {
		log.Fatalf("Error with Mget")
		return &executor.RespondBatch{
			RequestId: req.RequestId,
			Keys:      nil,
			Values:    nil,
		}, err
	}

	for i, v := range serverResult {
		if v == nil {
			runningKeys = append(runningKeys, StringPair{First: req.Keys[i], Second: "-1"})
		} else {
			runningKeys = append(runningKeys, StringPair{First: req.Keys[i], Second: v.(string)})
		}

	}

	for i, v := range req.Values {
		if v != "" {
			//Put Request
			localCache[req.Keys[i]] = v
			runningKeys[i].Second = v
		} else {
			//Its a Get Request made after a Put request to the same key.
			val, ok := localCache[req.Keys[i]]
			if ok {
				runningKeys[i].Second = val
			}
		}
	}

	var ansKeys []string
	var ansVals []string
	var replyKeys []string
	var replyVals []string
	var pairs []interface{}
	for _, v := range runningKeys {
		if v.Second == "-1" {
			replyKeys = append(replyKeys, v.First)
			replyVals = append(replyVals, "-1")
		}
		ansKeys = append(ansKeys, v.First)
		ansVals = append(ansVals, v.Second)
		replyKeys = append(replyKeys, v.First)
		replyVals = append(replyVals, v.Second)
		pairs = append(pairs, v.First, v.Second)
	}
	//Push All Keys
	if len(pairs) != 0 {
		_, err = e.rdb.MSet(threadCtx, pairs...).Result()
	}

	if err != nil {
		return &executor.RespondBatch{
			RequestId: req.RequestId,
			Keys:      nil,
			Values:    nil,
		}, err
	}

	return &executor.RespondBatch{
		RequestId: req.RequestId,
		Keys:      replyKeys,
		Values:    replyVals,
	}, nil
}

func (e myExecutor) InitDb(ctx context.Context, req *executor.RequestBatch) (*wrappers.BoolValue, error) {
	fmt.Printf("Initialize DB with Key Size: %d \n", len(req.Keys))

	threadCtx := context.Background()

	var pairs []interface{}

	for i := range req.Keys {
		pairs = append(pairs, req.Keys[i], req.Values[i])
	}

	redisServerResp := e.rdb.MSet(threadCtx, pairs...)

	_, err := redisServerResp.Result()

	if err != nil {
		return &wrappers.BoolValue{Value: false}, err
	} else {
		return &wrappers.BoolValue{Value: true}, nil
	}

}

func main() {
	lis, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatalf("Cannot create listener on port :9090 %s", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	service := myExecutor{rdb: rdb}
	serverRegister := grpc.NewServer(grpc.MaxRecvMsgSize(644000*300), grpc.MaxSendMsgSize(644000*300))

	log.Println("Starting Server!")
	executor.RegisterExecutorServer(serverRegister, service)
	err = serverRegister.Serve(lis)
	if err != nil {
		log.Fatalf("Error! Could not start server %s", err)
	}

}
