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
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	executor "github.com/Haseeb1399/WorkingThesis/api/executor"
	loadbalancer "github.com/Haseeb1399/WorkingThesis/api/loadbalancer"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	executorNumber int
	executors      map[int]executor.ExecutorClient
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

func (lb *myLoadBalancer) AddKeys(ctx context.Context, req *loadbalancer.LoadBalanceRequest) (*loadbalancer.LoadBalanceResponse, error) {
	// fmt.Printf("Got Request for Size: %d \n", len(req.Keys))

	channelId := fmt.Sprintf("%d-%d-%d", req.RequestId, req.ObjectNum, req.TotalObjects)
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

	// fmt.Println("Waiting for Response")
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

	return &loadbalancer.LoadBalanceResponse{
		RequestId:    req.RequestId,
		ObjectNum:    req.ObjectNum,
		TotalObjects: req.TotalObjects,
		Keys:         sendKeys,
		Values:       sendVal,
	}, nil

}

func (lb *myLoadBalancer) executeBatch(elements []KVPair, executerNumber int) {
	client := lb.executors[executerNumber]

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

	resp, err := client.ExecuteBatch(context.Background(), req)
	if err != nil {
		log.Fatalf("Failed to Fetch Values! Error: %s", err)
	}

	for i, v := range channelIds {
		//Drop fake requests.
		if v == "" {
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

		responseStruct.m.Lock()
		responseStruct.channel <- newKVPair
		responseStruct.m.Unlock()
	}

	//Delegate any assumptions for the ORAM proxy Executor(Ordering of keys Etc)
}

func (lb *myLoadBalancer) checkQueues(ctx context.Context) {
	waitTime := 2 * time.Second

	timer := time.NewTicker(waitTime)
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Exiting out of CheckQueues")
			return
		case <-timer.C:
			fmt.Println("Timer Click")
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
					fakeRequests[i] = KVPair{Key: "Fake", Value: "", channelId: ""}
				}
				elements = append(elements, fakeRequests...)

				fmt.Println(len(elements), toAdd)
				lb.queueLock[i].Unlock()
				localMap[i] = elements
				localAdded[i] = toAdd
				// go lb.executeBatch(elements, i)
			}
			toSend := false
			for i := 0; i < lb.executorNumber; i++ {
				fmt.Println(len(localMap[i]))
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

func (lb *myLoadBalancer) connectToExecutors(hosts []string, ports []string, filePath string) {
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
				initMap[hashVal].Keys = append(initMap[hashVal].Keys, key)
				initMap[hashVal].Values = append(initMap[hashVal].Values, value)
			}
		}
	}

	for i, v := range hosts {
		fullAddr := v + ":" + ports[i]
		conn, err := grpc.NewClient(fullAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(644000*300), grpc.MaxCallSendMsgSize(644000*300)))
		if err != nil {
			log.Fatalf("Couldn't Connect to Executor Proxy at: %s %s. Error: %s", v, ports[i], err)
		}
		newClient := executor.NewExecutorClient(conn)
		// maxSizeOption := grpc.MaxCallRecvMsgSize()
		resp, err := newClient.InitDb(context.Background(), initMap[i])
		if err != nil {
			fmt.Print(err)
			log.Fatalf("Failed to initialize DB %s", fullAddr)
		} else if resp.Value {
			log.Printf("Initialized Executor: %d \n", i)
		}
		lb.executors[i] = newClient
	}
	log.Println("Connected to Executors!")
}

func main() {
	//CLI arguments
	rPtr := flag.Int("R", 10, "Real Number of Requests")
	nPtr := flag.Int("N", 10, "Numer of Executors")

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
		executorNumber: *nPtr,
		executors:      make(map[int]executor.ExecutorClient),
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
	service.connectToExecutors(hosts, ports, traceLoc)

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
