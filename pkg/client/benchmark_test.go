package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	waffle_service "github.com/Haseeb1399/waffle-go/api/waffle"
	"golang.org/x/exp/rand"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	client waffle_service.KeyValueStoreClient
	once   sync.Once
)

func loadDbTrace(fileName string, traceSize int) ([]string, []string) {
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatalf("Failed to open file: %s", err)
	}
	defer file.Close()

	keys := []string{}
	values := []string{}

	// Create a new scanner for the file
	scanner := bufio.NewScanner(file)
	lineNumb := 0
	// Read the file line by line
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)

		if len(parts) >= 3 && parts[0] == "SET" {
			key := parts[1]
			value := parts[2]
			keys = append(keys, key)
			values = append(values, value)
		} else {
			fmt.Println("Line format is incorrect or missing key-value pair")
		}
		if lineNumb == traceSize {
			break
		} else {
			lineNumb++
		}
	}

	// Check for errors during scanning
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading file: %s", err)
	}

	return keys, values
}

func selectRandomKeys(keys []string, N int) []string {
	selectedKeys := make([]string, N)
	for i := 0; i < N; i++ {
		selectedKeys[i] = keys[rand.Intn(len(keys))]
	}
	return selectedKeys
}

func initializeDB(client waffle_service.KeyValueStoreClient, keys []string, values []string) {
	ctx := context.Background()

	// Initialize DB
	initObj := waffle_service.InitRequest{
		Keys:   keys,
		Values: values,
	}
	resp, err := client.Init(ctx, &initObj)
	if err != nil {
		log.Fatalf("Failed to Init DB! Error: %v", err)
	}
	fmt.Println("DB Initialized with response: ", resp)
}

func runBenchmark(client waffle_service.KeyValueStoreClient, keys []string, N int, b *testing.B) {
	ctx := context.Background()

	// Start the benchmark
	startTime := time.Now()
	keysOp := 0

	for i := 0; i < b.N; i++ {
		// Select N random keys from the provided keys
		selectedKeys := selectRandomKeys(keys, N)

		// Prepare the batch request with selected keys and empty values
		req := &waffle_service.PutBatchRequest{
			Keys:   selectedKeys,
			Values: make([]string, len(selectedKeys)), // All values are empty strings
		}

		resp, err := client.ExecuteMixBatch(ctx, req)
		if err != nil {
			b.Fatalf("ExecuteMixBatch failed: %v", err)
		}
		keysOp += len(resp.Keys)
	}
	duration := time.Since(startTime)

	// Calculate throughput
	throughput := float64(b.N) / duration.Seconds()
	throughputKV := float64(keysOp) / duration.Seconds()
	fmt.Printf("Throughput: %.2f requests per second\n", throughput)
	fmt.Printf("Throughput KV: %.2f requests per second\n", throughputKV)
}

func BenchmarkExecuteMixBatch(b *testing.B) {
	// Ensure the DB is initialized only once
	rand.Seed(uint64(time.Now().UnixNano())) // Seed random number generator

	serverTrace := "./server_input.txt"
	initKeys, initValues := loadDbTrace(serverTrace, 10000)
	fmt.Println("Trace Size: ", len(initKeys))

	once.Do(func() {

		fullAddr := "localhost:9090"
		conn, err := grpc.NewClient(fullAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(644000*300), grpc.MaxCallSendMsgSize(644000*300)))
		if err != nil {
			log.Fatalf("Couldn't Connect to Executor Proxy at localhost:9090. Error: %s", err)
		}
		client = waffle_service.NewKeyValueStoreClient(conn)

		// Initialize the database
		initializeDB(client, initKeys, initValues)
	})

	// Run the benchmark with N keys per batch operation
	N := 80 // Number of keys to choose for each batch operation
	runBenchmark(client, initKeys, N, b)
}
