package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Haseeb1399/WorkingThesis/api/loadbalancer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	ports := []string{"9500"}
	lb_addr := hosts[0] + ":" + ports[0]

	conn, err := grpc.NewClient(lb_addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(644000*300), grpc.MaxCallSendMsgSize(644000*300)))
	if err != nil {
		log.Fatalf("Failed to open connection to load balancer")
	}
	lbClient := loadbalancer.NewLoadBalancerClient(conn)

	//Read trace into local storage
	readTrace(traceLoc)
	testCases := getTestCases(100)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := lbClient.AddKeys(ctx, tc.requestBatch)
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
	hosts := []string{"localhost"}
	ports := []string{"9500"}
	lb_addr := hosts[0] + ":" + ports[0]

	conn, err := grpc.NewClient(lb_addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(644000*300), grpc.MaxCallSendMsgSize(644000*300)))
	if err != nil {
		log.Fatalf("Failed to open connection to load balancer")
	}
	lbClient := loadbalancer.NewLoadBalancerClient(conn)

	//Read trace into local storage

	readTrace(traceLoc)
	testCases := getTestCases(200)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Println("Starting Test", tc.name)
			resp, err := lbClient.AddKeys(ctx, tc.requestBatch)
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
