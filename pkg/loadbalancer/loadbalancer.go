package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"hash/fnv"
	"log"
	"net"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"net/http"
	_ "net/http/pprof"

	executor "github.com/Haseeb1399/WorkingThesis/api/executor"
	loadbalancer "github.com/Haseeb1399/WorkingThesis/api/loadbalancer"
	waffle_client "github.com/Haseeb1399/WorkingThesis/pkg/waffle_executor"
	queue "github.com/golang-collections/collections/queue"
	"google.golang.org/grpc"
)

type KVPair struct {
	channelId  string
	Key        string
	Value      string
	sortingKey int
}

type responseChannel struct {
	m       *sync.Mutex
	channel chan KVPair
}

type executorQueue struct {
	m     *sync.Mutex
	queue *queue.Queue
}

type myLoadBalancer struct {
	loadbalancer.UnimplementedLoadBalancerServer
	done              bool
	R                 int
	B                 int
	F                 int
	D                 int
	C                 int
	N                 int
	executorNumber    int
	executorType      string
	waitTime          int
	executors         map[int][]*waffle_client.ProxyClient
	clientMutexes     map[int][]sync.Mutex
	executorQueueList map[int]executorQueue
	channelMap        map[string]responseChannel
	updateCacheMap    ConcurrentMap
	channelLock       sync.Mutex
	requestNumber     atomic.Int64
}

func hashString(s string, N uint32) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32() % N
}

func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
func separateKeysValues(pairs []string) ([]string, []string) {
	var keys []string
	var values []string

	for _, pair := range pairs {
		// Split the pair into two parts on the first occurrence of ':'
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) == 2 {
			keys = append(keys, parts[0])
			values = append(values, parts[1])
		}
	}

	return keys, values
}

func (lb *myLoadBalancer) ConnectPing(ctx context.Context, req *loadbalancer.ClientConnect) (*loadbalancer.ClientConnect, error) {
	fmt.Println("Resolver Connected!")

	toRet := loadbalancer.ClientConnect{
		Id: req.Id,
	}

	return &toRet, nil
}

func (lb *myLoadBalancer) AddKeys(ctx context.Context, req *loadbalancer.LoadBalanceRequest) (*loadbalancer.LoadBalanceResponse, error) {
	// fmt.Printf("Got Request for Size: %d \n", len(req.Keys))
	reqNum := lb.requestNumber.Add(1)
	recv_resp := make([]KVPair, 0, len(req.Keys))
	localUpdateMap := make(map[string]int)

	channelId := fmt.Sprintf("%d-%d-%d-%d", req.RequestId, req.ObjectNum, req.TotalObjects, reqNum)
	lb.channelLock.Lock()
	lb.channelMap[channelId] = responseChannel{
		m:       &sync.Mutex{},
		channel: make(chan KVPair),
	}
	lb.channelLock.Unlock()

	localMap := make(map[int][]KVPair)

	for i := 0; i < len(req.Keys); i++ {

		//Insert Operation
		if req.Values[i] != "" {
			//Insert if a higher version does not exist.
			err := lb.updateCacheMap.Set(req.Keys[i], req.Values[i], req.RequestId)
			//Error only happens when trying to overwrite a newer value in the cache. Abort request.
			if err != nil {
				return nil, fmt.Errorf("update request aborted: %w", err)
			}
			localUpdateMap[req.Keys[i]] = 1 //Local state that I (the thread) added this to the cache and have to remove it.

		} else {
			//Get Operation
			cachedVal, version, isPresent := lb.updateCacheMap.Get(req.Keys[i], req.RequestId)
			if isPresent {
				recv_resp = append(recv_resp, KVPair{
					Key:   req.Keys[i],
					Value: cachedVal.(string),
				})
				continue //No need to request key from the Server.
			}

			if version == -2 {
				return nil, fmt.Errorf("get failed due to lower version, retry request")
			}
		}

		hashVal := hashString(req.Keys[i], uint32(lb.executorNumber))
		currentPair := KVPair{
			channelId:  channelId,
			Key:        req.Keys[i],
			Value:      req.Values[i],
			sortingKey: i,
		}
		localMap[int(hashVal)] = append(localMap[int(hashVal)], currentPair)
	}
	for i := 0; i < lb.executorNumber; i++ {
		lb.executorQueueList[i].m.Lock()

		for j := 0; j < len(localMap[i]); j++ {
			lb.executorQueueList[i].queue.Enqueue(localMap[i][j])
		}

		lb.executorQueueList[i].m.Unlock()
	}

	lb.channelLock.Lock()
	responseStruct := lb.channelMap[channelId]
	lb.channelLock.Unlock()

	responseStruct.m.Lock()
	myChan := responseStruct.channel
	responseStruct.m.Unlock()

	expectedNonCachedResp := len(req.Keys) - len(recv_resp)
	for i := 0; i < expectedNonCachedResp; i++ {
		item := <-myChan
		recv_resp = append(recv_resp, item)
	}

	close(myChan)
	lb.channelLock.Lock()
	delete(lb.channelMap, channelId)
	lb.channelLock.Unlock()

	sort.Slice(recv_resp, func(i, j int) bool {
		return recv_resp[i].sortingKey < recv_resp[j].sortingKey
	})

	sendKeys := make([]string, 0, len(req.Keys))
	sendVal := make([]string, 0, len(req.Keys))

	for _, v := range recv_resp {
		// _, ok := localUpdateMap[v.Key]
		// if ok {
		// 	//Delete the version that I added.
		// 	lb.updateCacheMap.DeleteVersion(v.Key, req.RequestId)
		// }
		sendKeys = append(sendKeys, v.Key)
		sendVal = append(sendVal, v.Value)
	}

	return &loadbalancer.LoadBalanceResponse{
		RequestId:    req.RequestId,
		ObjectNum:    req.ObjectNum,
		TotalObjects: req.TotalObjects,
		Keys:         sendKeys,
		Values:       sendVal,
	}, nil

}

func (lb *myLoadBalancer) executeBatch(elements []KVPair, executorNumber int) {
	var client *waffle_client.ProxyClient
	var clientIndex int
	// Try to find an available client
	for {
		found := false
		for i := 0; i < len(lb.executors[executorNumber]); i++ {
			if lb.clientMutexes[executorNumber][i].TryLock() {
				client = lb.executors[executorNumber][i]
				clientIndex = i
				found = true
				break
			}
		}
		if found {
			break
		}
		// If no client is available, wait a bit and try again
		time.Sleep(10 * time.Millisecond)
	}
	// fmt.Println("Using Client Index: ", clientIndex)
	defer lb.clientMutexes[executorNumber][clientIndex].Unlock()

	channelIds := make([]string, 0, len(elements))
	executeKeys := make([]string, 0, len(elements))
	executeVals := make([]string, 0, len(elements))
	executeSortKeys := make([]int, 0, len(elements))

	for i := 0; i < len(elements); i++ {
		channelIds = append(channelIds, elements[i].channelId)
		executeKeys = append(executeKeys, elements[i].Key)
		executeVals = append(executeVals, elements[i].Value)
		executeSortKeys = append(executeSortKeys, elements[i].sortingKey)
	}

	req := &executor.RequestBatch{
		RequestId: lb.requestNumber.Add(1),
		Keys:      executeKeys,
		Values:    executeVals,
	}
	resp := &executor.RespondBatch{
		Keys:   []string{},
		Values: []string{},
	}

	if lb.executorType == "Waffle" {
		execResp, err := client.MixBatch(req.Keys, req.Values)
		if err != nil {
			log.Fatalf("Failed to Fetch Values! Error: %s", err)
		}
		Keys, Values := separateKeysValues(execResp)
		resp.Keys = Keys
		resp.Values = Values
	} else {
		// resp, err := client.ExecuteBatch(context.Background(), req)
		// if err != nil {
		// 	log.Fatalf("Failed to Fetch Values! Error: %s", err)
		// }
	}

	for i, v := range channelIds {
		//Drop fake requests.
		if v == "noChannel" {
			continue
		}
		newKVPair := KVPair{
			Key:        resp.Keys[i],
			Value:      resp.Values[i],
			sortingKey: executeSortKeys[i],
		}

		lb.channelLock.Lock()
		responseStruct := lb.channelMap[v]
		lb.channelLock.Unlock()

		if responseStruct.m == nil {
			fmt.Println("Hello!!", v, newKVPair)
		}
		responseStruct.m.Lock()
		responseStruct.channel <- newKVPair
		responseStruct.m.Unlock()
	}

	//Delegate any assumptions for the ORAM proxy Executor(Ordering of keys Etc)
}

func (lb *myLoadBalancer) checkQueues(ctx context.Context) {
	waitTime := time.Duration(lb.waitTime) * time.Millisecond

	timer := time.NewTicker(waitTime)
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Exiting out of CheckQueues")
			return
		case <-timer.C:
			// fmt.Println("Timer Click")
			localMap := make(map[int][]KVPair)
			localAdded := make(map[int]int)
			elements := []KVPair{}
			for i := 0; i < lb.executorNumber; i++ {
				lb.executorQueueList[i].m.Lock()
				currLen := lb.executorQueueList[i].queue.Len()
				toAdd := lb.R - currLen
				if toAdd < 0 {
					toAdd = 0
				}

				for lb.executorQueueList[i].queue.Len() > 0 {
					currElement := lb.executorQueueList[i].queue.Dequeue()
					elements = append(elements, currElement.(KVPair))
				}

				for toAdd > 0 {
					temp := KVPair{Key: "Fake", Value: "", channelId: "noChannel"}
					elements = append(elements, temp)
					toAdd--
				}

				lb.executorQueueList[i].m.Unlock()
				localMap[i] = elements
				localAdded[i] = toAdd
			}
			toSend := false
			for i := 0; i < lb.executorNumber; i++ {
				//fmt.Println(len(localMap[i]))
				if localAdded[i] != lb.R {
					toSend = true
				}
			}
			if toSend {
				for i := 0; i < lb.executorNumber; i++ {
					go lb.executeBatch(localMap[i], i)
				}
			}
			timer.Reset(waitTime)
		default:
			for i := 0; i < lb.executorNumber; i++ {
				lb.executorQueueList[i].m.Lock()
				currLen := lb.executorQueueList[i].queue.Len()
				lb.executorQueueList[i].m.Unlock()
				if currLen >= lb.R {
					if i == lb.executorNumber-1 {
						for j := 0; j < lb.executorNumber; j++ {
							lb.executorQueueList[j].m.Lock()
							elements := []KVPair{}
							for elementNum := 0; elementNum < lb.R; elementNum++ {
								currEle := lb.executorQueueList[j].queue.Dequeue()
								elements = append(elements, currEle.(KVPair))
							}
							lb.executorQueueList[j].m.Unlock()
							go lb.executeBatch(elements, j)
						}
						timer.Reset(waitTime)
					}
				} else {
					break
				}
			}
		}
	}
}

func (lb *myLoadBalancer) connectToExecutors(hosts []string, ports []string, filePath string, numClient int) {
	initMap := make(map[int]*executor.RequestBatch, lb.executorNumber)
	for i := 0; i < lb.executorNumber; i++ {
		initMap[i] = &executor.RequestBatch{}
	}
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Error opening metadata file: %s \n", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, " ")
		if len(parts) == 3 {
			op := parts[0]
			if op == "SET" {
				key := parts[1]
				hashVal := int(hashString(key, uint32(lb.executorNumber)))
				value := parts[2]
				if contains(initMap[hashVal].Keys, key) {
					fmt.Println("Duplicate Key: ", key)
				}
				initMap[hashVal].Keys = append(initMap[hashVal].Keys, key)
				initMap[hashVal].Values = append(initMap[hashVal].Values, value)
			}
		} else {
			fmt.Println("Invalid!", parts)
		}
	}

	fmt.Println("Total Keys and Values: ", len(initMap[0].Keys), len(initMap[0].Values))

	lb.executors = make(map[int][]*waffle_client.ProxyClient)
	lb.clientMutexes = make(map[int][]sync.Mutex)

	for i, v := range hosts {
		lb.executors[i] = make([]*waffle_client.ProxyClient, numClient)
		lb.clientMutexes[i] = make([]sync.Mutex, numClient)
		lb.executorQueueList[i] = executorQueue{
			m:     &sync.Mutex{},
			queue: queue.New(),
		}

		port, err := strconv.Atoi(ports[i])
		if err != nil {
			log.Fatalf("Error converting port to integer: %s", err)
		}

		for j := 0; j < numClient; j++ {
			client := &waffle_client.ProxyClient{}
			err := client.Init(v, port)
			if err != nil {
				log.Fatalf("Couldn't Connect to Executor Proxy %d-%d. Error: %s", i, j, err)
			}

			if j == 0 {
				client.InitArgs(int64(lb.B), int64(lb.R), int64(lb.F), int64(lb.D), int64(lb.C), int64(lb.N))
				client.InitDB(initMap[i].Keys, initMap[i].Values)
			}
			client.GetClientID()
			lb.executors[i][j] = client
		}
	}
	log.Println("Connected to Executors!")
}

func main() {
	//CLI arguments
	rPtr := flag.Int("R", 800, "Real Number of Requests")
	timeOutPtr := flag.Int("Z", 500, "Queue Wait time in Milliseconds")
	nPtr := flag.Int("num", 1, "Numer of Executors")
	bPtr := flag.Int("B", 1200, "Batch Size for Executors (Waffle)")
	fPtr := flag.Int("F", 100, "Fake request size for Exectors (Waffle)")
	cPtr := flag.Int("C", 2, "Cache Size for Executors (Waffle)")
	nCPtr := flag.Int("N", 1, "Number of Cores for Executor (Waffle)")
	dPtr := flag.Int("D", 100000, "Number of Dummy Values")
	tPtr := flag.String("T", "Waffle", "Executor Type")
	numCPtr := flag.Int("X", 1, "Number of clients to Waffle")
	hostsPtr := flag.String("hosts", "localhost", "Comma-separated list of host addresses (e.g., 'localhost,host2,host3')")
	portsPtr := flag.String("ports", "9090", "Comma-separated list of port numbers (e.g., '9090,9091,9092')")

	flag.Parse()

	// Start pprof server
	go func() {
		log.Println("Starting pprof server on :6060")
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			log.Fatalf("pprof server failed: %v", err)
		}
	}()

	//Gracefully exist go routines by passing in main context

	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	lis, err := net.Listen("tcp", ":9500")
	if err != nil {
		log.Fatalf("Cannot create listener on port :9500 %s", err)

	}
	fmt.Println("Starting Load Balancer on: localhost:9500")

	//Executor Information
	// Split hosts and ports based on user input
	hosts := strings.Split(*hostsPtr, ",")
	ports := strings.Split(*portsPtr, ",")

	// Check if the number of hosts and ports match the number of executors
	if len(hosts) != *nPtr || len(ports) != *nPtr {
		log.Fatalf("The number of hosts (%d) or ports (%d) does not match the number of executors (%d).", len(hosts), len(ports), *nPtr)
	}

	//Define Service
	service := myLoadBalancer{
		done:              false,
		R:                 *rPtr,
		B:                 *bPtr,
		C:                 *cPtr,
		F:                 *fPtr,
		N:                 *nCPtr,
		D:                 *dPtr,
		executorNumber:    *nPtr,
		executorType:      *tPtr,
		waitTime:          *timeOutPtr,
		executors:         make(map[int][]*waffle_client.ProxyClient),
		clientMutexes:     make(map[int][]sync.Mutex),
		executorQueueList: make(map[int]executorQueue),
		channelMap:        make(map[string]responseChannel),
		updateCacheMap:    NewConcurrentMap(),
		channelLock:       sync.Mutex{},
		requestNumber:     atomic.Int64{},
	}
	service.requestNumber.Store(0)

	// Print the variables used for initialization
	fmt.Printf("Initialization with:  - R: %d, Z: %d, num: %d, B: %d, F: %d, C: %d, N: %d, D: %d, X: %d, T: %s\n",
		*rPtr, *timeOutPtr, *nPtr, *bPtr, *fPtr, *cPtr, *nCPtr, *dPtr, *numCPtr, *tPtr)

	serverRegister := grpc.NewServer()
	loadbalancer.RegisterLoadBalancerServer(serverRegister, &service)

	//Load Trace
	traceLoc := "../../tracefiles/serverInput.txt"
	//Connect to Executors
	service.connectToExecutors(hosts, ports, traceLoc, *numCPtr)

	//Launch Thread to check Queues
	go service.checkQueues(ctx)

	time.AfterFunc(200*time.Second, func() {
		fmt.Println("200 seconds passed. Shutting down...")
		cancel()
		lis.Close()
		serverRegister.GracefulStop()
		os.Exit(0)
	})

	//Launch routine to listen to sig events.
	go func() {
		<-sigChan
		cancel()
		lis.Close()
		serverRegister.GracefulStop()
		os.Exit(0)
	}()
	//Start serving
	err = serverRegister.Serve(lis)

	if err != nil {
		log.Fatalf("Error! %s", err)
	}

}
