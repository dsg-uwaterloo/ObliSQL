package executorPlaintxt

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/golang/protobuf/ptypes/wrappers"
	executorPlaintxt "github.com/project/ObliSql/api/plaintextExecutor"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/trace"
)

type Executor struct {
	executorPlaintxt.UnimplementedPlainTextExecutorServer
	rdb    *redis.Client
	tracer trace.Tracer
}

type StringPair struct {
	First  string
	Second string
}

func (e Executor) ExecuteBatch(ctx context.Context, req *executorPlaintxt.RequestBatch) (*executorPlaintxt.RespondBatch, error) {
	// log.Info().Msgf("Got a request with ID: %d", req.RequestId)
	threadCtx := context.Background()
	var runningKeys []StringPair
	localCache := make(map[string]string)

	//First Get All Keys
	serverResult, err := e.rdb.MGet(threadCtx, req.Keys...).Result()

	if err != nil {
		log.Fatal().Msgf("Error with Mget")
		return &executorPlaintxt.RespondBatch{
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

	var replyKeys []string
	var replyVals []string
	var pairs []interface{}
	for _, v := range runningKeys {
		if v.Second == "-1" {
			replyKeys = append(replyKeys, v.First)
			replyVals = append(replyVals, "-1")
		} else {
			replyKeys = append(replyKeys, v.First)
			replyVals = append(replyVals, v.Second)
		}
		pairs = append(pairs, v.First, v.Second)

	}
	//Push All Keys
	if len(pairs) != 0 {
		_, err = e.rdb.MSet(threadCtx, pairs...).Result()
	}

	if err != nil {
		return &executorPlaintxt.RespondBatch{
			RequestId: req.RequestId,
			Keys:      nil,
			Values:    nil,
		}, err
	}

	return &executorPlaintxt.RespondBatch{
		RequestId: req.RequestId,
		Keys:      replyKeys,
		Values:    replyVals,
	}, nil
}

func (e Executor) InitDb(ctx context.Context, req *executorPlaintxt.RequestBatch) (*wrappers.BoolValue, error) {
	log.Info().Msgf("Initialize DB with Key Size: %d \n", len(req.Keys))

	ctx, span := e.tracer.Start(ctx, "Init DB")
	defer span.End()

	threadCtx := context.Background()
	e.rdb.FlushAll(threadCtx)
	var pairs []interface{}

	for i := range req.Keys {
		pairs = append(pairs, req.Keys[i], req.Values[i])
	}
	span.AddEvent("Going to Redis")
	redisServerResp := e.rdb.MSet(threadCtx, pairs...)
	span.AddEvent("Set on Redis")

	_, err := redisServerResp.Result()

	log.Info().Msg("Initialization finished! \n")

	if err != nil {
		return &wrappers.BoolValue{Value: false}, err
	} else {
		return &wrappers.BoolValue{Value: true}, nil
	}

}

func loadTrace(tracefile string) (*executorPlaintxt.RequestBatch, error) {
	req := &executorPlaintxt.RequestBatch{}

	file, err := os.Open(tracefile)
	if err != nil {
		return nil, fmt.Errorf("Failed to open tracefiles!: %v", err)
	}
	defer file.Close()

	const maxBufferSize = 1024 * 1024 // 1MB

	scanner := bufio.NewScanner(file)
	buffer := make([]byte, maxBufferSize)
	scanner.Buffer(buffer, maxBufferSize)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "SET") {
			parts := strings.SplitN(line, " ", 3)
			if len(parts) != 3 {
				continue // Skip lines that don't have exactly 3 parts
			}
			key := parts[1]
			value := parts[2]
			req.Keys = append(req.Keys, key)
			req.Values = append(req.Values, value)
		}
	}
	return req, nil
}

func NewExecutor(redisHost string, redisPort string, traceProvider trace.Tracer) *Executor {
	addr := redisHost + ":" + redisPort

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	log.Info().Msg("Connected to Redis Host on: " + addr)
	service := Executor{
		rdb:    rdb,
		tracer: traceProvider,
	}

	traceFileloc := "../../tracefiles/serverInput.txt"
	traceData, err := loadTrace(traceFileloc)

	if err != nil {
		fmt.Println("Failed to Load Trace!")
	}
	service.InitDb(context.Background(), traceData)

	return &service

}
