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

	executor "github.com/Haseeb1399/WorkingThesis/api/executor"
	loadbalancer "github.com/Haseeb1399/WorkingThesis/api/loadbalancer"
	waffle_client "github.com/Haseeb1399/WorkingThesis/pkg/waffle_executor"
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

type myLoadBalancer struct {
	loadbalancer.UnimplementedLoadBalancerServer
	done           bool
	R              int
	B              int
	F              int
	D              int
	C              int
	N              int
	executorNumber int
	executorType   string
	executors      map[int][]*waffle_client.ProxyClient
	clientMutexes  map[int][]sync.Mutex
	exectorQueues  map[int][]KVPair
	channelMap     map[string]responseChannel
	queueLock      []sync.Mutex
	channelLock    sync.Mutex
	requestNumber  atomic.Int64
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

func (lb *myLoadBalancer) AddKeys(ctx context.Context, req *loadbalancer.LoadBalanceRequest) (*loadbalancer.LoadBalanceResponse, error) {
	// fmt.Printf("Got Request for Size: %d \n", len(req.Keys))
	reqNum := lb.requestNumber.Add(1)
	channelId := fmt.Sprintf("%d-%d-%d-%d", req.RequestId, req.ObjectNum, req.TotalObjects, reqNum)
	lb.channelLock.Lock()
	lb.channelMap[channelId] = responseChannel{
		m:       &sync.Mutex{},
		channel: make(chan KVPair),
	}
	lb.channelLock.Unlock()

	localMap := make(map[int][]KVPair)

	for i := 0; i < len(req.Keys); i++ {
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
		lb.queueLock[i].Lock()
		lb.exectorQueues[i] = append(lb.exectorQueues[i], localMap[i]...)
		lb.queueLock[i].Unlock()
	}

	recv_resp := make([]KVPair, 0, len(req.Keys))

	lb.channelLock.Lock()
	responseStruct := lb.channelMap[channelId]
	lb.channelLock.Unlock()

	responseStruct.m.Lock()
	myChan := responseStruct.channel
	responseStruct.m.Unlock()

	for i := 0; i < len(req.Keys); i++ {
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
		sendKeys = append(sendKeys, v.Key)
		sendVal = append(sendVal, v.Value)
	}

	fmt.Println("Finished: ", channelId)
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
	waitTime := 500 * time.Millisecond

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

			for i := 0; i < lb.executorNumber; i++ {
				lb.queueLock[i].Lock()
				currLen := len(lb.exectorQueues[i])
				toAdd := lb.R - currLen
				if toAdd < 0 {
					toAdd = 0
				}

				elements := lb.exectorQueues[i][:(lb.R - toAdd)]
				lb.exectorQueues[i] = lb.exectorQueues[i][(lb.R - toAdd):]

				fakeRequests := make([]KVPair, toAdd)

				for i := range fakeRequests {
					fakeRequests[i] = KVPair{Key: "Fake", Value: "", channelId: "noChannel"}
				}
				elements = append(elements, fakeRequests...)

				lb.queueLock[i].Unlock()
				localMap[i] = elements
				localAdded[i] = toAdd
				// go lb.executeBatch(elements, i)
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
				lb.queueLock[i].Lock()
				currLen := len(lb.exectorQueues[i])
				lb.queueLock[i].Unlock()
				if currLen >= lb.R {
					if i == lb.executorNumber-1 {
						for j := 0; j < lb.executorNumber; j++ {
							lb.queueLock[j].Lock()
							elements := lb.exectorQueues[j][:lb.R]
							lb.exectorQueues[j] = lb.exectorQueues[j][lb.R:]
							lb.queueLock[j].Unlock()
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

	fmt.Println(len(initMap[0].Keys), len(initMap[0].Values))

	lb.executors = make(map[int][]*waffle_client.ProxyClient)
	lb.clientMutexes = make(map[int][]sync.Mutex)

	for i, v := range hosts {
		lb.executors[i] = make([]*waffle_client.ProxyClient, numClient)
		lb.clientMutexes[i] = make([]sync.Mutex, numClient)

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
	nPtr := flag.Int("num", 1, "Numer of Executors")
	bPtr := flag.Int("B", 1200, "Batch Size for Executors (Waffle)")
	fPtr := flag.Int("F", 100, "Fake request size for Exectors (Waffle)")
	cPtr := flag.Int("C", 2, "Cache Size for Executors (Waffle)")
	nCPtr := flag.Int("N", 1, "Number of Cores for Executor (Waffle)")
	dPtr := flag.Int("D", 100000, "Number of Dummy Values")
	tPtr := flag.String("T", "Waffle", "Executor Type")

	flag.Parse()

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
	hosts := make([]string, 0)
	ports := make([]string, 0)

	for i := 0; i < *nPtr; i++ {
		hosts = append(hosts, "localhost")
		ports = append(ports, fmt.Sprintf("909%d", i))
	}

	//Define Service
	service := myLoadBalancer{
		done:           false,
		R:              *rPtr,
		B:              *bPtr,
		C:              *cPtr,
		F:              *fPtr,
		N:              *nCPtr,
		D:              *dPtr,
		executorNumber: *nPtr,
		executorType:   *tPtr,
		executors:      make(map[int][]*waffle_client.ProxyClient),
		clientMutexes:  make(map[int][]sync.Mutex),
		exectorQueues:  make(map[int][]KVPair),
		channelMap:     make(map[string]responseChannel),
		queueLock:      make([]sync.Mutex, len(hosts)),
		channelLock:    sync.Mutex{},
		requestNumber:  atomic.Int64{},
	}
	service.requestNumber.Store(0)

	serverRegister := grpc.NewServer()
	loadbalancer.RegisterLoadBalancerServer(serverRegister, &service)

	//Load Trace
	traceLoc := "../../tracefiles/serverInput.txt"
	//Connect to Executors
	service.connectToExecutors(hosts, ports, traceLoc, 1)

	//Launch Thread to check Queues
	go service.checkQueues(ctx)

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
		log.Fatalf("Error! Could not start loadBalancer! %s", err)
	}

}