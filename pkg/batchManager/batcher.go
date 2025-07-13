package batcher

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	loadBalancer "github.com/project/ObliSql/api/loadbalancer"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type KVPair struct {
	channelId  string
	Key        string
	Value      string
	sortingKey int
	RequestID  int
}

type Batch struct {
	input            []*KVPair
	responseChan     chan *[]string
	ctx              context.Context // Add context here
	enqueueTime      time.Time
	dequeueTime      time.Time
	processingTime   time.Duration
	aggregateBatchID int64
}

type responseChannel struct {
	m       *sync.RWMutex
	channel chan KVPair
}

type myBatcher struct {
	loadBalancer.UnimplementedLoadBalancerServer
	R                 int
	executorNumber    int
	waitTime          int
	executorType      string
	executorNum       int
	executors         map[int][]ExecutorClient
	numClients        int
	tracer            trace.Tracer
	fakeRequestsOff   bool
	executorChannels  map[int]chan *KVPair // Per-executor channels
	channelMap        map[string]responseChannel
	channelLock       sync.RWMutex
	requestNumber     atomic.Int64
	batchChannel      map[int]chan *Batch
	executorWorkerIds map[int][]int
	TotalKeysSeen     atomic.Int64
	aggBatchIds       atomic.Int64
	TotalFakeAdded    atomic.Int64
}

func extractTableName(key string) string {
	parts := strings.Split(key, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return key
}

func assignTableToExecutor(key string, N uint32) uint32 {
	tableName := extractTableName(key)
	switch tableName {
	case "rankings":
		return 0
	case "uservisits":
		return 1 % N
	default:
		h := fnv.New32a()
		h.Write([]byte(tableName))
		return h.Sum32() % N
	}
}

func (lb *myBatcher) ConnectPing(ctx context.Context, req *loadBalancer.ClientConnect) (*loadBalancer.ClientConnect, error) {
	log.Info().Msgf("Resolver Connected!")

	toRet := loadBalancer.ClientConnect{
		Id: req.Id,
	}

	return &toRet, nil
}

// func (lb *myBatcher) InitDB(ctx context.Context, req *loadBalancer.LoadBalanceRequest) (*wrapperspb.BoolValue, error) {

// 	initMap := make(map[int]*executor.RequestBatch, lb.executorNumber)
// 	for i := 0; i < lb.executorNumber; i++ {
// 		initMap[i] = &executor.RequestBatch{}
// 	}

// 	totalKeys := 0
// 	for i, k := range req.Keys {
// 		totalKeys++
// 		key := k
// 		val := req.Values[i]
// 		hashVal := int(hashString(key, uint32(lb.executorNumber)))

// 		if contains(initMap[hashVal].Keys, key) {
// 			log.Fatal().Msg("Fatal: Duplicate Key seen. Should not happen!")
// 			return wrapperspb.Bool(false), fmt.Errorf("Fatal: Duplicate Key seen. ")
// 		}
// 		initMap[hashVal].Keys = append(initMap[hashVal].Keys, key)
// 		initMap[hashVal].Values = append(initMap[hashVal].Values, val)
// 	}

// 	fmt.Println("Total Keys: ", totalKeys)

// 	for i := 0; i < lb.executorNumber; i++ {
// 		firstClient := lb.executors[i][0]
// 		firstClient.InitDB(initMap[i].Keys, initMap[i].Values)
// 	}

// 	return wrapperspb.Bool(true), nil
// }

func (lb *myBatcher) connectToExecutors(ctx context.Context, hosts []string, ports []string, numClient int) {
	ctx, span := lb.tracer.Start(ctx, "Connecting")
	defer span.End()

	lb.executors = make(map[int][]ExecutorClient)
	workerId := 0
	for i := range hosts {
		lb.executors[i] = make([]ExecutorClient, numClient)
		lb.executorWorkerIds[i] = []int{}
		for j := 0; j < numClient; j++ {
			lb.batchChannel[workerId] = make(chan *Batch, lb.R)
			lb.executorWorkerIds[i] = append(lb.executorWorkerIds[i], workerId)
			workerId++
		}
	}
	workerId = 0
	for i, v := range hosts {

		port, err := strconv.Atoi(ports[i])
		if err != nil {
			log.Fatal().Msgf("Error converting port to integer: %s", err)
		}
		span.AddEvent(fmt.Sprintf("Launching Clients for: %d", i))
		for j := 0; j < numClient; j++ {
			client, err := NewExecutorClient(lb.executorType, v, port, lb.tracer)
			if err != nil {
				log.Fatal().Msgf("Couldn't Connect to Executor Proxy %d-%d. Error: %s", i, j, err)
			}

			lb.executors[i][j] = client
			go lb.batchWorker(client, i, workerId)
			workerId++
		}
		span.AddEvent(fmt.Sprintf("Finished Launching Clients for: %d", i))
	}
	log.Info().Msgf("Connected to Executors!")
}

func (lb *myBatcher) AddKeys(ctx context.Context, req *loadBalancer.LoadBalanceRequest) (*loadBalancer.LoadBalanceResponse, error) {
	ctx, span := lb.tracer.Start(ctx, "Add Keys")
	defer span.End()

	fmt.Println(req.RequestId)

	span.SetAttributes(
		attribute.Int64("request_id", req.RequestId),
		attribute.Int("num_keys", len(req.Keys)),
	)

	reqNum := lb.requestNumber.Add(1)
	recv_resp := make([]KVPair, 0, len(req.Keys))

	channelId := fmt.Sprintf("%d-%d", req.RequestId, reqNum)
	localRespChannel := make(chan KVPair, len(req.Keys))

	span.AddEvent("Adding Channel to Global Map")
	lb.channelLock.Lock()
	lb.channelMap[channelId] = responseChannel{
		m:       &sync.RWMutex{},
		channel: localRespChannel,
	}
	lb.channelLock.Unlock()
	span.AddEvent("Added Channel to Global Map")

	span.AddEvent("Adding Keys to Channels")
	sent := 0
	for i, key := range req.Keys {
		// if !lb.bloomFilter.TestString(key) {
		// 	kv := KVPair{
		// 		channelId:  "-",
		// 		Key:        key,
		// 		Value:      "-1",
		// 		sortingKey: i,
		// 		RequestID:  int(req.RequestId),
		// 	}
		// 	recv_resp = append(recv_resp, kv)
		// 	continue
		// }
		value := req.Values[i]
		hashVal := int(assignTableToExecutor(key, uint32(lb.executorNumber)))
		kv := &KVPair{
			channelId:  channelId,
			Key:        key,
			Value:      value,
			sortingKey: i,
			RequestID:  int(req.RequestId),
		}
		// Block if the channel is full
		sent++
		lb.executorChannels[hashVal] <- kv

	}
	span.AddEvent("Finished Adding Keys to Channels")
	span.SetAttributes(
		attribute.Int("after_bloom_check", sent),
	)

	span.AddEvent("Waiting for Responses")
	for i := 0; i < len(req.Keys); i++ {
		item := <-localRespChannel
		recv_resp = append(recv_resp, item)
	}
	span.AddEvent("Got all Responses")

	close(localRespChannel)

	lb.channelLock.Lock()
	delete(lb.channelMap, channelId)
	lb.channelLock.Unlock()

	sort.Slice(recv_resp, func(i, j int) bool {
		return recv_resp[i].sortingKey < recv_resp[j].sortingKey
	})

	sendKeys := make([]string, 0, len(req.Keys))
	sendVal := make([]string, 0, len(req.Keys))

	for _, v := range recv_resp {
		sendKeys = append(sendKeys, v.Key)
		sendVal = append(sendVal, v.Value)
	}

	return &loadBalancer.LoadBalanceResponse{
		RequestId:    req.RequestId,
		ObjectNum:    req.ObjectNum,
		TotalObjects: req.TotalObjects,
		Keys:         sendKeys,
		Values:       sendVal,
	}, nil
}

func (lb *myBatcher) centralCoordinator() {
	log.Info().Msgf("Launching Central Coordinator with timeOut: %d", lb.waitTime)
	waitDuration := time.Duration(lb.waitTime) * time.Millisecond
	timer := time.NewTicker(waitDuration)
	defer timer.Stop()

	for {
		ready := false

		for !ready {
			ready = true
			for i := 0; i < lb.executorNumber; i++ {
				count := len(lb.executorChannels[i])
				if count < lb.R {
					ready = false
					break
				}
			}
			if ready {
				break
			}

			select {
			case <-timer.C:
				// Time to send whatever we have
				ready = true
			default:
				// Do nothing, just continue the loop
				// time.Sleep(time.Millisecond * 10)
			}
		}
		timer.Reset(waitDuration)
		//Do we want all of them to Reach R or do we want to have at-least one has reached R.

		// Collect batches
		allZero := true
		batches := make(map[int][]*KVPair)
		fakeCount := 0
		for i := 0; i < lb.executorNumber; i++ {
			n := len(lb.executorChannels[i])
			if n != 0 {
				allZero = false
			}

			if n > lb.R {
				n = lb.R //Label this better
			}
			batch := make([]*KVPair, 0, n)
			for j := 0; j < n; j++ {
				kv := <-lb.executorChannels[i]
				batch = append(batch, kv)
			}
			// Add fake requests to make up to R if n < lb.R
			if !lb.fakeRequestsOff {
				// log.Info().Msgf("Adding Fake Requests", len(batch))
				for len(batch) < lb.R {
					fakeCount += 1
					temp := &KVPair{Key: "Fake", Value: "", channelId: "noChannel"}
					batch = append(batch, temp)
				}
			}
			batches[i] = batch
		}
		if allZero {
			continue //Skip sending fake requests only after timeout
		}
		// Dispatch batches together
		for i := 0; i < lb.executorNumber; i++ {
			batch := batches[i]
			if len(batch) == 0 {
				if lb.fakeRequestsOff {
					log.Info().Msg("Skipping over a batch, Happens when fake requests are turned off")
					continue
				} else {
					log.Fatal().Msg("Should never have an empty batch!")
				}
			}
			lb.TotalKeysSeen.Add(int64(len(batch)))
			lb.TotalFakeAdded.Add(int64(fakeCount))
			go func(executorID int, batch []*KVPair) {
				lb.executeBatch(executorID, batch)
			}(i, batch)
		}
		timer.Reset(waitDuration)
	}
}

func (lb *myBatcher) executeBatch(executorNumber int, batch []*KVPair) {

	ctx, span := lb.tracer.Start(context.Background(), "Execute Batch")
	defer span.End()

	span.AddEvent("Collecting request IDs")
	requestIDs := make([]int, len(batch))
	for i, kv := range batch {
		requestIDs[i] = kv.RequestID
	}
	span.AddEvent("Collected request IDs")
	span.SetAttributes(
		attribute.IntSlice("batch_request_ids", requestIDs),
		attribute.Int("numKeys", len(requestIDs)),
	)

	respChann := make(chan *[]string)
	newBatch := &Batch{
		responseChan: respChann,
		input:        batch,
		ctx:          ctx,
		enqueueTime:  time.Now(),
	}
	// fmt.Printf("Channel length before adding value: %d\n", len(lb.executorChannels[executorNumber]))
	workerIds := lb.executorWorkerIds[executorNumber]
	var selectedWorkerId int
	minLen := int(^uint(0) >> 1) // Max int
	for _, id := range workerIds {
		length := len(lb.batchChannel[id])
		if length < minLen {
			minLen = length
			selectedWorkerId = id
		}
	}
	// log.Info().Msgf("Selected WorkerID: %d", selectedWorkerId)
	span.AddEvent("Adding Batch to Worker Pool")
	lb.batchChannel[selectedWorkerId] <- newBatch
	span.AddEvent("Added Batch to Worker Pool")

	resp := <-respChann
	span.AddEvent("Got Response for Batch from Worker Pool")

	span.SetAttributes(
		attribute.Float64("Time between Enqueue & Dequeue", float64(newBatch.dequeueTime.Sub(newBatch.enqueueTime).Seconds())),
		attribute.Float64("Processing Time", float64(newBatch.processingTime.Seconds())),
		attribute.Int("assigned_aggBatchID", int(newBatch.aggregateBatchID)),
	)

	Keys, Values := separateKeysValues(resp)

	span.AddEvent("Collecing channel Ids. Locking")
	channelCache := make(map[string]chan KVPair, len(batch))
	lb.channelLock.RLock()
	for _, v := range batch {
		if v.channelId != "noChannel" {
			channelCache[v.channelId] = lb.channelMap[v.channelId].channel
		}
	}
	lb.channelLock.RUnlock()
	span.AddEvent("Collected Channel Ids, Unlocked")

	// Now process each batch using the preloaded channelMapCache
	span.AddEvent("Sending responses to their channels")
	for i, v := range batch {
		if v.channelId == "noChannel" {
			continue
		}
		lb.TotalKeysSeen.Add(1)
		newKVPair := KVPair{
			Key:        Keys[i],
			Value:      Values[i],
			sortingKey: v.sortingKey,
		}
		responseChannel := channelCache[v.channelId]
		responseChannel <- newKVPair
	}
	span.AddEvent("Sent responses to their channels")

}

func (lb *myBatcher) batchWorker(client ExecutorClient, idx int, workerId int) {
	log.Info().Msgf("Batch Worker for executor: %d is up. WorkerID: %d ", idx, workerId)

	for {
		var allBatches []*Batch

		// Drain the channel and collect all the batches
	DrainLoop:
		for {
			select {
			case batch := <-lb.batchChannel[workerId]:
				batch.dequeueTime = time.Now()
				allBatches = append(allBatches, batch) // Collect all batches
			default:
				// Exit the loop when the channel is empty
				break DrainLoop
			}
		}

		if len(allBatches) == 0 {
			continue
		}

		// log.Info().Msgf("Worker %d Drained %d batches from batchChannel[%d]", workerId, len(allBatches), idx)

		// Aggregate keys and values from all collected batches
		var aggregatedKeys []string
		var aggregatedValues []string
		var batchInputs []*KVPair

		aggregateBatchId := lb.aggBatchIds.Add(1)
		startTime := time.Now()

		// Collect request IDs for tracing
		var requestIDs []int
		for _, batch := range allBatches {
			batch.aggregateBatchID = aggregateBatchId //Assign a batch ID
			for _, pair := range batch.input {
				aggregatedKeys = append(aggregatedKeys, pair.Key)
				aggregatedValues = append(aggregatedValues, pair.Value)
				batchInputs = append(batchInputs, pair)
				requestIDs = append(requestIDs, pair.RequestID)
			}
		}

		// Create span for MixBatch operation
		_, span := lb.tracer.Start(context.Background(), "MixBatch")
		span.SetAttributes(
			attribute.Int("executor_id", idx),
			attribute.Int("worker_id", workerId),
			attribute.Int64("aggregate_batch_id", aggregateBatchId),
			attribute.Int("num_keys", len(aggregatedKeys)),
			attribute.IntSlice("request_ids", requestIDs),
		)

		// Send all the aggregated keys and values to MixBatch in one call
		// log.Info().Msg("Sending all aggregated keys and values to MixBatch")
		// log.Debug().Msgf("Sending Request to Waffle: %d. WorkerID: %d", idx, workerId)
		// log.Info().Msgf("Sending Key Length: %d, ID: %d", len(aggregatedKeys), workerId)

		execResp, err := client.MixBatch(aggregatedKeys, aggregatedValues, aggregateBatchId)
		finishTime := time.Since(startTime)

		span.SetAttributes(
			attribute.Float64("mixbatch_duration_millisecond", float64(finishTime.Milliseconds())),
		)
		span.End()
		if err != nil {
			log.Fatal().Msgf("Failed to Fetch Values from MixBatch! Error: %s", err)
		}

		// Send the response of each batch to its designated channel
		currentIndex := 0
		for _, batch := range allBatches {
			batch.processingTime = finishTime
			sendSlice := execResp[currentIndex : currentIndex+len(batch.input)]
			batch.responseChan <- &sendSlice
			currentIndex += len(batch.input)
		}
	}
}

func NewBatcher(ctx context.Context, R int, executorNumber int, waitTime int, executorType string, executorHosts string, executorPorts string, numClients int, fakeReqPtr bool, tracer trace.Tracer) *myBatcher {
	hosts := strings.Split(executorHosts, ",")
	ports := strings.Split(executorPorts, ",")

	if len(hosts) != executorNumber || len(ports) != executorNumber {
		log.Fatal().Msgf("The number of hosts (%d) or ports (%d) does not match the number of executors (%d).", len(hosts), len(ports), executorNumber)
	}

	service := myBatcher{
		R:                 R,
		executorNumber:    executorNumber,
		waitTime:          waitTime,
		executorType:      executorType,
		numClients:        numClients,
		tracer:            tracer,
		channelMap:        make(map[string]responseChannel),
		channelLock:       sync.RWMutex{},
		batchChannel:      make(map[int]chan *Batch),
		executorChannels:  make(map[int]chan *KVPair), // Initialize executorChannels
		TotalKeysSeen:     atomic.Int64{},
		aggBatchIds:       atomic.Int64{},
		executorWorkerIds: make(map[int][]int),
		fakeRequestsOff:   fakeReqPtr,
		TotalFakeAdded:    atomic.Int64{},
	}

	// // Initialize batchChannel before any goroutines start
	// for i := 0; i < executorNumber; i++ {
	// 	service.batchChannel[i] = make(chan *Batch, 3*R) //Some Factor of R. Experiment and see.
	// }

	service.connectToExecutors(ctx, hosts, ports, numClients)
	log.Info().Msgf("Fake Requests Off?: %t", service.fakeRequestsOff)

	log.Info().Msgf("Number of Executors: %d", executorNumber)
	for i := 0; i < executorNumber; i++ {
		service.executorChannels[i] = make(chan *KVPair, 1000000)
	}
	go service.centralCoordinator()

	return &service
}
