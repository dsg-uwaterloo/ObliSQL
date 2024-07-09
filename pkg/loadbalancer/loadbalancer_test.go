package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	executor "github.com/Haseeb1399/WorkingThesis/api/executor"
	"github.com/Haseeb1399/WorkingThesis/api/loadbalancer"
	"google.golang.org/grpc"
)

var localKeyValMap = make(map[string]string)

type TestCase struct {
	name         string
	requestBatch *loadbalancer.LoadBalanceRequest
	expected     *loadbalancer.LoadBalanceResponse
}

func readTrace(filePath string) {
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
				value := parts[2]
				localKeyValMap[key] = value
			}
		}
	}
}

func getRandomKeyValuePairs(numPairs int) map[string]string {
	// Step 1: Convert map entries to a slice
	entries := make([][2]string, 0, len(localKeyValMap))
	for key, value := range localKeyValMap {
		entries = append(entries, [2]string{key, value})
	}

	// Step 2: Shuffle the slice of entries
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(entries), func(i, j int) { entries[i], entries[j] = entries[j], entries[i] })

	// Step 3: Select the first 100 elements from the shuffled slice
	if len(entries) < numPairs {
		numPairs = len(entries)
	}

	randomKeyValuePairs := make(map[string]string, numPairs)
	for i := 0; i < numPairs; i++ {
		randomKeyValuePairs[entries[i][0]] = entries[i][1]
	}

	return randomKeyValuePairs
}
func initLb(hosts []string, ports []string, traceLoc string) *myLoadBalancer {
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	lis, err := net.Listen("tcp", ":9500")
	if err != nil {
		log.Fatalf("Cannot create listener on port :9500 %s", err)
	}
	fmt.Println("Starting Load Balancer on: localhost:9500")
	//Define Service
	service := myLoadBalancer{
		done:           false,
		R:              100, //Fixing R to 100 for testing.
		executorNumber: len(hosts),
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
	// Load Trace

	// Connect to Executors
	service.connectToExecutors(hosts, ports, traceLoc)

	// Launch Thread to check Queues
	go service.checkQueues(ctx)

	// Launch routine to listen to sig events.
	go func() {
		<-sigChan
		cancel()
		lis.Close()
		serverRegister.GracefulStop()
		os.Exit(0)
	}()

	go func() {
		//Start serving
		err = serverRegister.Serve(lis)

		if err != nil {
			log.Fatalf("Error! Could not start loadBalancer! %s", err)
		}
	}()

	return &service
}

func getTestCases(numKeys int) []TestCase {
	getRandomKeys := getRandomKeyValuePairs(numKeys)
	lbKeys := make([]string, 0, numKeys)
	lbValues := make([]string, 0, numKeys)
	lbExpectedVal := make([]string, 0, numKeys)

	for k, v := range getRandomKeys {
		lbKeys = append(lbKeys, k)
		lbValues = append(lbValues, "") //All Gets
		lbExpectedVal = append(lbExpectedVal, v)
	}

	testCases := []TestCase{
		{
			name: "Valid Full R Requests",
			requestBatch: &loadbalancer.LoadBalanceRequest{
				RequestId:    1,
				Keys:         lbKeys,
				Values:       lbValues,
				ObjectNum:    1,
				TotalObjects: 1,
			},
			expected: &loadbalancer.LoadBalanceResponse{
				RequestId:    1,
				Keys:         lbKeys,
				Values:       lbExpectedVal,
				ObjectNum:    1,
				TotalObjects: 1,
			},
		},
		{
			name: "Single Request",
			requestBatch: &loadbalancer.LoadBalanceRequest{
				RequestId:    1,
				Keys:         lbKeys[:1],
				Values:       lbValues[:1],
				ObjectNum:    1,
				TotalObjects: 1,
			},
			expected: &loadbalancer.LoadBalanceResponse{
				RequestId:    1,
				Keys:         lbKeys[:1],
				Values:       lbExpectedVal[:1],
				ObjectNum:    1,
				TotalObjects: 1,
			},
		},
	}
	return testCases
}

func TestSingleExecutor(t *testing.T) {
	//Init Load Balancer
	//Assumption: The Executors are already Running!
	//R = 100

	ctx := context.Background()
	traceLoc := "../../tracefiles/serverInput.txt"
	hosts := []string{"localhost"}
	ports := []string{"9090"}

	service := initLb(hosts, ports, traceLoc)

	time.Sleep(2 * time.Second)

	//Read trace into local storage
	readTrace(traceLoc)
	testCases := getTestCases(100)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := service.AddKeys(ctx, tc.requestBatch)
			if err != nil {
				t.Errorf("ExecuteBatch Error = %v", err)
				return
			}
			if !reflect.DeepEqual(resp.Keys, tc.expected.Keys) || !reflect.DeepEqual(resp.Values, tc.expected.Values) {
				t.Errorf("ExecuteBatch Values not same! \n Expected Keys: %v, Got: %v \n Expected Values: %v, Got: %v \n", tc.expected.Keys, resp.Keys, tc.expected.Values, resp.Values)
			}
		})
	}

}

func TestTwoExecutor(t *testing.T) {
	//Init Load Balancer
	//Assumption: The Executors are already Running!
	//R = 100

	ctx := context.Background()
	traceLoc := "../../tracefiles/serverInput.txt"
	hosts := []string{"localhost", "localhost"}
	ports := []string{"9090", "9091"}

	service := initLb(hosts, ports, traceLoc)

	//Read trace into local storage
	readTrace(traceLoc)
	testCases := getTestCases(200)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := service.AddKeys(ctx, tc.requestBatch)
			if err != nil {
				t.Errorf("ExecuteBatch Error = %v", err)
				return
			}
			if !reflect.DeepEqual(resp.Keys, tc.expected.Keys) || !reflect.DeepEqual(resp.Values, tc.expected.Values) {
				t.Errorf("ExecuteBatch Values not same! \n Expected Keys: %v, \n Got: %v \n Expected Values: %v, \n Got: %v \n", tc.expected.Keys, resp.Keys, tc.expected.Values, resp.Values)
			}
		})
	}
}
