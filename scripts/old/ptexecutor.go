// package batcher

// import (
// 	"bufio"
// 	"context"
// 	"fmt"
// 	"os"
// 	"sort"
// 	"strconv"
// 	"strings"
// 	"sync"
// 	"sync/atomic"
// 	"time"

// 	loadBalancer "github.com/project/ObliSql/api/loadbalancer"
// 	executor "github.com/project/ObliSql/api/plaintextExecutor"
// 	ptClient "github.com/project/ObliSql/pkg/batchManager/plainTextExecutor"
// 	"github.com/rs/zerolog/log"
// 	"go.opentelemetry.io/otel/attribute"
// 	"go.opentelemetry.io/otel/trace"
// )

// type KVPair struct {
// 	channelId  string
// 	Key        string
// 	Value      string
// 	sortingKey int
// 	RequestID  int
// }

// type Batch struct {
// 	input            []*KVPair
// 	responseChan     chan *[]string
// 	ctx              context.Context // Add context here
// 	enqueueTime      time.Time
// 	dequeueTime      time.Time
// 	processingTime   time.Duration
// 	aggregateBatchID int64
// }

// type responseChannel struct {
// 	m       *sync.RWMutex
// 	channel chan KVPair
// }

// type myBatcher struct {
// 	loadBalancer.UnimplementedLoadBalancerServer
// 	R                 int
// 	executorNumber    int
// 	waitTime          int
// 	executorType      string
// 	executorNum       int
// 	executors         map[int][]*ptClient.PlainTextClient
// 	numClients        int
// 	tracer            trace.Tracer
// 	executorChannels  map[int]chan *KVPair // Per-executor channels
// 	channelMap        map[string]responseChannel
// 	channelLock       sync.RWMutex
// 	requestNumber     atomic.Int64
// 	batchChannel      map[int]chan *Batch
// 	executorWorkerIds map[int][]int
// 	TotalKeysSeen     atomic.Int64
// 	aggBatchIds       atomic.Int64
// }

// func (lb *myBatcher) ConnectPing(ctx context.Context, req *loadBalancer.ClientConnect) (*loadBalancer.ClientConnect, error) {
// 	log.Info().Msgf("Resolver Connected!")

// 	toRet := loadBalancer.ClientConnect{
// 		Id: req.Id,
// 	}

// 	return &toRet, nil
// }

// func (lb *myBatcher) connectToExecutors(ctx context.Context, hosts []string, ports []string, filePath string, numClient int) {
// 	ctx, span := lb.tracer.Start(ctx, "Connecting")
// 	defer span.End()

// 	initMap := make(map[int]*executor.RequestBatch, lb.executorNumber)
// 	for i := 0; i < lb.executorNumber; i++ {
// 		initMap[i] = &executor.RequestBatch{}
// 	}
// 	file, err := os.Open(filePath)
// 	if err != nil {
// 		log.Fatal().Msgf("Error opening metadata file: %s", err)
// 	}
// 	defer file.Close()

// 	scanner := bufio.NewScanner(file)
// 	totalKeys := 0
// 	for scanner.Scan() {
// 		line := scanner.Text()
// 		parts := strings.Split(line, " ")
// 		if len(parts) == 3 {
// 			op := parts[0]
// 			if op == "SET" {
// 				totalKeys++
// 				key := parts[1]
// 				hashVal := int(hashString(key, uint32(lb.executorNumber)))
// 				value := parts[2]

// 				if contains(initMap[hashVal].Keys, key) {
// 					fmt.Println("Duplicate Key: ", key)
// 				}
// 				initMap[hashVal].Keys = append(initMap[hashVal].Keys, key)
// 				initMap[hashVal].Values = append(initMap[hashVal].Values, value)
// 			}
// 		} else {
// 			log.Info().Msg("Invalid! " + strings.Join(parts, " "))
// 		}
// 	}

// 	fmt.Println("Total Keys: ", totalKeys)

// 	lb.executors = make(map[int][]*ptClient.PlainTextClient)
// 	workerId := 0
// 	// First, initialize all batchChannels and workerIds
// 	for i := range hosts {
// 		lb.executors[i] = make([]*ptClient.PlainTextClient, numClient)
// 		lb.executorWorkerIds[i] = []int{}
// 		for j := 0; j < numClient; j++ {
// 			// Initialize batchChannel[workerId] before starting batchWorker
// 			lb.batchChannel[workerId] = make(chan *Batch, lb.R)
// 			lb.executorWorkerIds[i] = append(lb.executorWorkerIds[i], workerId)
// 			workerId++
// 		}
// 	}
// 	workerId = 0
// 	for i, v := range hosts {
// 		// lb.executors[i] = make([]*waffleExecutor.ProxyClient, numClient)
// 		// lb.executorWorkerIds[i] = []int{}

// 		port, err := strconv.Atoi(ports[i])
// 		if err != nil {
// 			log.Fatal().Msgf("Error converting port to integer: %s", err)
// 		}
// 		span.AddEvent(fmt.Sprintf("Launching Clients for: %d", i))
// 		for j := 0; j < numClient; j++ {
// 			client := &ptClient.PlainTextClient{}
// 			err := client.CreateClient(v, port)
// 			if err != nil {
// 				log.Fatal().Msgf("Couldn't Connect to Executor Proxy %d-%d. Error: %s", i, j, err)
// 			}

// 			if j == 0 {
// 				client.InitDB(initMap[i].Keys, initMap[i].Values)
// 			}

// 			// client.GetClientID()
// 			lb.executors[i][j] = client
// 			go lb.batchWorker(client, i, workerId)
// 			workerId++
// 		}
// 		span.AddEvent(fmt.Sprintf("Finished Launching Clients for: %d", i))
// 	}
// 	log.Info().Msgf("Connected to Executors!")
// }

// func (lb *myBatcher) AddKeys(ctx context.Context, req *loadBalancer.LoadBalanceRequest) (*loadBalancer.LoadBalanceResponse, error) {

// 	ctx, span := lb.tracer.Start(ctx, "Add Keys")
// 	defer span.End()

// 	span.SetAttributes(
// 		attribute.Int64("request_id", req.RequestId),
// 		attribute.Int("num_keys", len(req.Keys)),
// 	)

// 	reqNum := lb.requestNumber.Add(1)
// 	recv_resp := make([]KVPair, 0, len(req.Keys))

// 	channelId := fmt.Sprintf("%d-%d", req.RequestId, reqNum)
// 	localRespChannel := make(chan KVPair, len(req.Keys))

// 	span.AddEvent("Adding Channel to Global Map")
// 	lb.channelLock.Lock()
// 	lb.channelMap[channelId] = responseChannel{
// 		m:       &sync.RWMutex{},
// 		channel: localRespChannel,
// 	}
// 	lb.channelLock.Unlock()
// 	span.AddEvent("Added Channel to Global Map")

// 	span.AddEvent("Adding Keys to Channels")
// 	sent := 0
// 	for i, key := range req.Keys {
// 		// if !lb.bloomFilter.TestString(key) {
// 		// 	kv := KVPair{
// 		// 		channelId:  "-",
// 		// 		Key:        key,
// 		// 		Value:      "-1",
// 		// 		sortingKey: i,
// 		// 		RequestID:  int(req.RequestId),
// 		// 	}
// 		// 	recv_resp = append(recv_resp, kv)
// 		// 	continue
// 		// }
// 		value := req.Values[i]
// 		hashVal := int(hashString(key, uint32(lb.executorNumber))) //Rename it to somthing else
// 		kv := &KVPair{
// 			channelId:  channelId,
// 			Key:        key,
// 			Value:      value,
// 			sortingKey: i,
// 			RequestID:  int(req.RequestId),
// 		}
// 		// Block if the channel is full
// 		sent++
// 		lb.executorChannels[hashVal] <- kv

// 	}
// 	span.AddEvent("Finished Adding Keys to Channels")
// 	span.SetAttributes(
// 		attribute.Int("after_bloom_check", sent),
// 	)

// 	span.AddEvent("Waiting for Responses")
// 	for i := 0; i < len(req.Keys); i++ {
// 		item := <-localRespChannel
// 		recv_resp = append(recv_resp, item)
// 	}
// 	span.AddEvent("Got all Responses")

// 	close(localRespChannel)

// 	lb.channelLock.Lock()
// 	delete(lb.channelMap, channelId)
// 	lb.channelLock.Unlock()

// 	sort.Slice(recv_resp, func(i, j int) bool {
// 		return recv_resp[i].sortingKey < recv_resp[j].sortingKey
// 	})

// 	sendKeys := make([]string, 0, len(req.Keys))
// 	sendVal := make([]string, 0, len(req.Keys))

// 	for _, v := range recv_resp {
// 		sendKeys = append(sendKeys, v.Key)
// 		sendVal = append(sendVal, v.Value)
// 	}

// 	return &loadBalancer.LoadBalanceResponse{
// 		RequestId:    req.RequestId,
// 		ObjectNum:    req.ObjectNum,
// 		TotalObjects: req.TotalObjects,
// 		Keys:         sendKeys,
// 		Values:       sendVal,
// 	}, nil
// }

// func (lb *myBatcher) centralCoordinator() {
// 	log.Info().Msgf("Launching Central Coordinator with timeOut: %d", lb.waitTime)
// 	waitDuration := time.Duration(lb.waitTime) * time.Millisecond
// 	timer := time.NewTicker(waitDuration)
// 	defer timer.Stop()

// 	for {
// 		ready := false

// 		for !ready {
// 			ready = true
// 			for i := 0; i < lb.executorNumber; i++ {
// 				count := len(lb.executorChannels[i])
// 				if count < lb.R {
// 					ready = false
// 					break
// 				}
// 			}
// 			if ready {
// 				break
// 			}

// 			select {
// 			case <-timer.C:
// 				// Time to send whatever we have
// 				ready = true
// 			default:
// 				// Do nothing, just continue the loop
// 				// time.Sleep(time.Millisecond * 10)
// 			}
// 		}
// 		timer.Reset(waitDuration)
// 		//Do we want all of them to Reach R or do we want to have at-least one has reached R.

// 		// Collect batches
// 		allZero := true
// 		batches := make(map[int][]*KVPair)
// 		for i := 0; i < lb.executorNumber; i++ {
// 			n := len(lb.executorChannels[i])
// 			if n != 0 {
// 				allZero = false
// 			}

// 			if n > lb.R {
// 				n = lb.R //Label this better
// 			}
// 			batch := make([]*KVPair, 0, n)
// 			for j := 0; j < n; j++ {
// 				kv := <-lb.executorChannels[i]
// 				batch = append(batch, kv)
// 			}
// 			// Add fake requests to make up to R if n < lb.R
// 			for len(batch) < lb.R {
// 				temp := &KVPair{Key: "Fake", Value: "", channelId: "noChannel"}
// 				batch = append(batch, temp)
// 			}
// 			batches[i] = batch
// 		}
// 		if allZero {
// 			continue //Skip sending fake requests only after timeout
// 		}
// 		// Dispatch batches together
// 		for i := 0; i < lb.executorNumber; i++ {
// 			batch := batches[i]
// 			if len(batch) == 0 {
// 				//Add error for this should never happen.
// 				continue
// 			}
// 			go func(executorID int, batch []*KVPair) {
// 				lb.executeBatch(executorID, batch)
// 			}(i, batch)
// 		}
// 	}
// }

// func (lb *myBatcher) executeBatch(executorNumber int, batch []*KVPair) {

// 	ctx, span := lb.tracer.Start(context.Background(), "Execute Batch")
// 	defer span.End()

// 	span.AddEvent("Collecting request IDs")
// 	requestIDs := make([]int, len(batch))
// 	for i, kv := range batch {
// 		requestIDs[i] = kv.RequestID
// 	}
// 	span.AddEvent("Collected request IDs")
// 	span.SetAttributes(
// 		attribute.IntSlice("batch_request_ids", requestIDs),
// 		attribute.Int("numKeys", len(requestIDs)),
// 	)

// 	respChann := make(chan *[]string)
// 	newBatch := &Batch{
// 		responseChan: respChann,
// 		input:        batch,
// 		ctx:          ctx,
// 		enqueueTime:  time.Now(),
// 	}
// 	// fmt.Printf("Channel length before adding value: %d\n", len(lb.executorChannels[executorNumber]))
// 	workerIds := lb.executorWorkerIds[executorNumber]
// 	var selectedWorkerId int
// 	minLen := int(^uint(0) >> 1) // Max int
// 	for _, id := range workerIds {
// 		length := len(lb.batchChannel[id])
// 		if length < minLen {
// 			minLen = length
// 			selectedWorkerId = id
// 		}
// 	}

// 	span.AddEvent("Adding Batch to Worker Pool")
// 	lb.batchChannel[selectedWorkerId] <- newBatch
// 	span.AddEvent("Added Batch to Worker Pool")

// 	resp := <-respChann
// 	span.AddEvent("Got Response for Batch from Worker Pool")

// 	span.SetAttributes(
// 		attribute.Float64("Time between Enqueue & Dequeue", float64(newBatch.dequeueTime.Sub(newBatch.enqueueTime).Seconds())),
// 		attribute.Float64("Processing Time", float64(newBatch.processingTime.Seconds())),
// 		attribute.Int("assigned_aggBatchID", int(newBatch.aggregateBatchID)),
// 	)

// 	Keys, Values := separateKeysValues(resp)

// 	span.AddEvent("Collecing channel Ids. Locking")
// 	channelCache := make(map[string]chan KVPair, len(batch))
// 	lb.channelLock.RLock()
// 	for _, v := range batch {
// 		if v.channelId != "noChannel" {
// 			channelCache[v.channelId] = lb.channelMap[v.channelId].channel
// 		}
// 	}
// 	lb.channelLock.RUnlock()
// 	span.AddEvent("Collected Channel Ids, Unlocked")

// 	// Now process each batch using the preloaded channelMapCache
// 	span.AddEvent("Sending responses to their channels")
// 	for i, v := range batch {
// 		if v.channelId == "noChannel" {
// 			continue
// 		}
// 		lb.TotalKeysSeen.Add(1)
// 		newKVPair := KVPair{
// 			Key:        Keys[i],
// 			Value:      Values[i],
// 			sortingKey: v.sortingKey,
// 		}
// 		responseChannel := channelCache[v.channelId]
// 		responseChannel <- newKVPair
// 	}
// 	span.AddEvent("Sent responses to their channels")

// }

// func (lb *myBatcher) batchWorker(client *ptClient.PlainTextClient, idx int, workerId int) {
// 	log.Info().Msgf("Batch Worker for executor: %d is up.", idx)

// 	for {
// 		var allBatches []*Batch
// 		// keys := 0
// 		// Drain the channel and collect all the batches
// 	DrainLoop:
// 		for {
// 			select {
// 			case batch := <-lb.batchChannel[workerId]:
// 				batch.dequeueTime = time.Now()
// 				allBatches = append(allBatches, batch) // Collect all batches
// 				// keys += len(batch.input)
// 				// if keys >= lb.R {
// 				// 	break DrainLoop
// 				// }
// 			default:
// 				// Exit the loop when the channel is empty
// 				break DrainLoop
// 			}
// 		}

// 		if len(allBatches) == 0 {
// 			continue
// 		}

// 		// log.Info().Msgf("Worker %d Drained %d batches from batchChannel[%d]", workerId, len(allBatches), idx)

// 		// Aggregate keys and values from all collected batches
// 		var aggregatedKeys []string
// 		var aggregatedValues []string
// 		var batchInputs []*KVPair

// 		aggregateBatchId := lb.aggBatchIds.Add(1)
// 		startTime := time.Now()
// 		for _, batch := range allBatches {
// 			batch.aggregateBatchID = aggregateBatchId //Assign a batch ID
// 			for _, pair := range batch.input {
// 				aggregatedKeys = append(aggregatedKeys, pair.Key)
// 				aggregatedValues = append(aggregatedValues, pair.Value)
// 				batchInputs = append(batchInputs, pair)
// 			}
// 		}

// 		// Send all the aggregated keys and values to MixBatch in one call
// 		log.Info().Msgf("Sending all aggregated keys and values to MixBatch %d. Size: %d", idx, len(aggregatedKeys))
// 		execResp, err := client.MixBatch(aggregatedKeys, aggregatedValues, aggregateBatchId)
// 		log.Info().Msgf("Got aggregated keys and values to MixBatch %d. Size: %d", idx, len(aggregatedKeys))
// 		finishTime := time.Since(startTime)
// 		if err != nil {
// 			log.Fatal().Msgf("Failed to Fetch Values from MixBatch! Error: %s", err)
// 		}

// 		// Send the response of each batch to its designated channel
// 		currentIndex := 0
// 		for _, batch := range allBatches {
// 			batch.processingTime = finishTime
// 			sendSlice := execResp[currentIndex : currentIndex+len(batch.input)]
// 			batch.responseChan <- &sendSlice
// 			currentIndex += len(batch.input)
// 		}
// 	}
// }

// func NewBatcher(ctx context.Context, R int, executorNumber int, waitTime int, executorType string, executorHosts string, executorPorts string, numClients int, tracer trace.Tracer, traceLoc string) *myBatcher {
// 	hosts := strings.Split(executorHosts, ",")
// 	ports := strings.Split(executorPorts, ",")

// 	if len(hosts) != executorNumber || len(ports) != executorNumber {
// 		log.Fatal().Msgf("The number of hosts (%d) or ports (%d) does not match the number of executors (%d).", len(hosts), len(ports), executorNumber)
// 	}

// 	service := myBatcher{
// 		R:                 R,
// 		executorNumber:    executorNumber,
// 		waitTime:          waitTime,
// 		executorType:      executorType,
// 		numClients:        numClients,
// 		tracer:            tracer,
// 		channelMap:        make(map[string]responseChannel),
// 		channelLock:       sync.RWMutex{},
// 		batchChannel:      make(map[int]chan *Batch),
// 		executorChannels:  make(map[int]chan *KVPair), // Initialize executorChannels
// 		executorWorkerIds: make(map[int][]int),
// 		TotalKeysSeen:     atomic.Int64{},
// 		aggBatchIds:       atomic.Int64{},
// 	}

// 	// // Initialize batchChannel before any goroutines start
// 	// for i := 0; i < executorNumber; i++ {
// 	// 	service.batchChannel[i] = make(chan *Batch, 3*R) //Some Factor of R. Experiment and see.
// 	// }

// 	service.connectToExecutors(ctx, hosts, ports, traceLoc, numClients)

// 	for i := 0; i < executorNumber; i++ {
// 		service.executorChannels[i] = make(chan *KVPair, 1000000)
// 	}
// 	go service.centralCoordinator()

// 	return &service
// }

// // func (lb *myBatcher) batchWorker(client *waffleExecutor.ProxyClient, idx int) {
// // 	log.Info().Msgf("Batch Worker for executor: %d is up.", idx)

// // 	for batch := range lb.batchChannel[idx] {
// // 		_, span := lb.tracer.Start(batch.ctx, "Batch Worker")
// // 		span.AddEvent("Parsing Keys to lists")
// // 		inp := batch.input
// // 		keys := []string{}
// // 		values := []string{}
// // 		for _, pair := range inp {
// // 			keys = append(keys, pair.Key)
// // 			values = append(values, pair.Value)
// // 		}
// // 		span.AddEvent("Parsed the Keys")

// // 		span.AddEvent("Sending to Waffle")
// // 		execResp, err := client.MixBatch(keys, values)
// // 		span.AddEvent("Got back from Waffle")
// // 		if err != nil {
// // 			log.Fatal().Msgf("Failed to Fetch Values! Error: %s", err)
// // 		}
// // 		batch.responseChan <- &execResp
// // 		span.End()
// // 	}
// // }