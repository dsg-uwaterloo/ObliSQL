package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	waffle "github.com/Haseeb1399/waffle-go/api/gen-go/waffle" // import the generated Go package from Thrift IDL

	"github.com/apache/thrift/lib/go/thrift"
)

type ProxyClient struct {
	client   *waffle.WaffleThriftClient
	response *waffle.WaffleThriftResponseClient
}

func loadDbTrace(fileName string, traceSize int) map[string]map[string][]interface{} {
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatalf("Failed to open file: %s", err)
	}
	defer file.Close()

	result := make(map[string]map[string][]interface{})
	tempKeys := []interface{}{}
	tempValues := []interface{}{}

	// Create a new scanner for the file
	scanner := bufio.NewScanner(file)
	i := 0 // Initialize the batch index

	// Read the file line by line
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)

		if len(parts) >= 2 {
			key := parts[1]
			var value string
			if parts[0] == "SET" && len(parts) > 2 {
				value = parts[2]
			} else {
				value = ""
			}
			// value = ""
			// Append key and value to their respective temporary slices
			tempKeys = append(tempKeys, key)
			tempValues = append(tempValues, value)

			// If the current batch size reaches the traceSize, add it to the result and reset
			if len(tempKeys) == traceSize {
				newKey := fmt.Sprintf("B%d", i)
				result[newKey] = map[string][]interface{}{
					"keys":   tempKeys,
					"values": tempValues,
				}
				tempKeys = []interface{}{}
				tempValues = []interface{}{}
				i++ // Increment batch index for next batch
			}
		} else {
			fmt.Println("Line format is incorrect or missing key-value pair", len(parts))
		}
	}

	// Append any remaining key-value pairs that didn't fill up the last batch
	if len(tempKeys) > 0 {
		newKey := fmt.Sprintf("B%d", i)
		result[newKey] = map[string][]interface{}{
			"keys":   tempKeys,
			"values": tempValues,
		}
	}

	// Check for errors during scanning
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading file: %s", err)
	}
	return result
}

func loadInitTrace(fileName string) ([]string, []string) {
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatalf("Failed to open file: %s", err)
	}
	defer file.Close()

	keys := []string{}
	values := []string{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if parts[0] == "SET" {
			keys = append(keys, parts[1])
			values = append(values, parts[2])
		}
	}

	return keys, values
}

func (p *ProxyClient) Init(host string, port int) error {
	// Create a configuration for the socket
	conf := &thrift.TConfiguration{
		// Set your configuration options here (if needed)
	}

	// Use NewTSocketConf instead of NewTSocket
	socket := thrift.NewTSocketConf(fmt.Sprintf("%s:%d", host, port), conf)

	transport := thrift.NewTFramedTransportConf(socket, conf)
	protocol := thrift.NewTBinaryProtocolFactoryConf(conf)
	client := waffle.NewWaffleThriftClientFactory(transport, protocol)
	response := waffle.NewWaffleThriftResponseClientFactory(transport, protocol)

	if err := transport.Open(); err != nil {
		return fmt.Errorf("error opening transport: %v", err)
	}

	p.client = client
	p.response = response
	return nil
}
func (p *ProxyClient) GetClientID() (int64, error) {
	ctx := context.Background()
	clientID, err := p.client.GetClientID(ctx)
	if err != nil {
		return 0, fmt.Errorf("error calling GetClientID: %v", err)
	}
	return clientID, nil
}

func (p *ProxyClient) RegisterClientID(blockID int32, clientID int64) error {
	ctx := context.Background()
	err := p.client.RegisterClientID(ctx, blockID, clientID)
	if err != nil {
		return fmt.Errorf("error calling RegisterClientID: %v", err)
	}
	return nil
}

func (p *ProxyClient) AsyncGet(seqID *waffle.SequenceID, key string) error {
	ctx := context.Background()
	err := p.client.AsyncGet(ctx, seqID, key)
	if err != nil {
		return fmt.Errorf("error calling AsyncGet: %v", err)
	}
	return nil
}

func (p *ProxyClient) AsyncPut(seqID *waffle.SequenceID, key, value string) error {
	ctx := context.Background()
	err := p.client.AsyncPut(ctx, seqID, key, value)
	if err != nil {
		return fmt.Errorf("error calling AsyncPut: %v", err)
	}
	return nil
}

func (p *ProxyClient) AsyncGetBatch(seqID *waffle.SequenceID, keys []string) error {
	ctx := context.Background()
	err := p.client.AsyncGetBatch(ctx, seqID, keys)
	if err != nil {
		return fmt.Errorf("error calling AsyncGetBatch: %v", err)
	}
	return nil
}

func (p *ProxyClient) AsyncPutBatch(seqID *waffle.SequenceID, keys, values []string) error {
	ctx := context.Background()
	err := p.client.AsyncPutBatch(ctx, seqID, keys, values)
	if err != nil {
		return fmt.Errorf("error calling AsyncPutBatch: %v", err)
	}
	return nil
}

func (p *ProxyClient) Get(key string) (string, error) {
	ctx := context.Background()
	result, err := p.client.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("error calling Get: %v", err)
	}
	return result, nil
}

func (p *ProxyClient) Put(key, value string) error {
	ctx := context.Background()
	err := p.client.Put(ctx, key, value)
	if err != nil {
		return fmt.Errorf("error calling Put: %v", err)
	}
	return nil
}

func (p *ProxyClient) GetBatch(keys []string) ([]string, error) {
	ctx := context.Background()
	results, err := p.client.GetBatch(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("error calling GetBatch: %v", err)
	}
	return results, nil
}

func (p *ProxyClient) PutBatch(keys, values []string) error {
	ctx := context.Background()
	err := p.client.PutBatch(ctx, keys, values)
	if err != nil {
		return fmt.Errorf("error calling PutBatch: %v", err)
	}
	return nil
}

func (p *ProxyClient) AsyncResponse(seqID *waffle.SequenceID, opCode int32, result []string) error {
	ctx := context.Background()
	err := p.response.AsyncResponse(ctx, seqID, opCode, result)
	if err != nil {
		return fmt.Errorf("error calling AsyncResponse: %v", err)
	}
	return nil
}

func (p *ProxyClient) MixBatch(keys, values []string) ([]string, error) {
	ctx := context.Background()
	results, err := p.client.MixBatch(ctx, keys, values)
	if err != nil {
		return nil, fmt.Errorf("error calling PutBatch: %v", err)
	}
	return results, nil
}

func (p *ProxyClient) InitDB(keys, values []string) {
	ctx := context.Background()
	err := p.client.InitDb(ctx, keys, values)

	if err != nil {
		fmt.Errorf("Error Initializing DB: %v", err)
	}
}

func (p *ProxyClient) InitArgs(B, R, F, D, C, N int64) {
	ctx := context.Background()
	err := p.client.InitArgs_(ctx, B, R, F, D, C, N)
	if err != nil {
		fmt.Errorf("Error Initializing Arguments: %v", err)
	}
}

func runBenchmark(traceFile map[string]map[string][]interface{}, clients []*ProxyClient, timeVal time.Duration) {
	var wg sync.WaitGroup
	startTime := time.Now()
	operationsCount := 0
	opsMutex := sync.Mutex{} // Mutex to protect operationsCount

	// Create a channel to manage client availability
	clientChan := make(chan *ProxyClient, len(clients))
	for _, client := range clients {
		clientChan <- client
	}

	// Create a context with a timeout to stop counting operations after the time has elapsed
	ctx, cancel := context.WithTimeout(context.Background(), timeVal*time.Second)
	defer cancel()

	// Start the benchmark loop
benchmarkLoop:
	for {
		select {
		case <-ctx.Done():
			// Stop processing new operations after this point
			break benchmarkLoop
		default:
			for _, batch := range traceFile {
				select {
				case <-ctx.Done():
					// Stop processing new operations after this point
					break benchmarkLoop
				case client := <-clientChan:
					wg.Add(1)

					// Launch a goroutine for each batch operation
					go func(client *ProxyClient, batch map[string][]interface{}) {
						defer wg.Done()

						batchKeys := batch["keys"]
						batchValues := batch["values"]
						// Convert batchKeys and batchValues to []string
						keys := make([]string, len(batchKeys))
						values := make([]string, len(batchValues))
						for i, key := range batchKeys {
							keys[i] = key.(string)
						}
						for i, value := range batchValues {
							values[i] = value.(string)
						}

						// Execute batch using the client
						resp, err := client.MixBatch(keys, values)
						if err != nil {
							fmt.Printf("Error executing MixBatch: %v\n", err)
						}

						// Only update operation count if the time hasn't elapsed
						select {
						case <-ctx.Done():
							// Do nothing if the time has elapsed
						default:
							opsMutex.Lock()
							operationsCount += len(resp)
							opsMutex.Unlock()
						}

						// Return the client to the channel
						clientChan <- client
					}(client, batch)

				default:
					// No available clients, wait and retry
					time.Sleep(10 * time.Millisecond)
				}
			}
		}
	}

	// Allow any running goroutines to finish (no more new ones will be started)
	wg.Wait()

	// Calculate throughput (using elapsed time up to the timeout)
	elapsedTime := time.Since(startTime)
	throughput := float64(operationsCount) / elapsedTime.Seconds()

	fmt.Printf("Operations completed: %d\n", operationsCount)
	fmt.Printf("Elapsed time: %v\n", elapsedTime)
	fmt.Printf("Throughput: %.2f ops/sec\n", throughput)
}

func main() {
	// Initialize 4 clients

	initTrace := "./server_input.txt"
	keys, values := loadInitTrace(initTrace)

	numClients := 2
	clients := make([]*ProxyClient, numClients)
	for i := 0; i < numClients; i++ {
		client := &ProxyClient{}
		err := client.Init("localhost", 9090)
		if err != nil {
			fmt.Printf("Error initializing client %d: %v\n", i, err)
			return
		}
		client.GetClientID()
		clients[i] = client
	}
	//Use one of the Clients to init waffle.

	clients[0].InitArgs(1200, 800, 100, 100000, 2, 1)
	clients[0].InitDB(keys, values)

	benchTrace := "./benchmark_input.txt"
	traceFile := loadDbTrace(benchTrace, 800)

	fmt.Println("WarmUp: ")
	runBenchmark(traceFile, clients, 10)
	fmt.Println()
	fmt.Println("Benchmark: ")
	runBenchmark(traceFile, clients, 30)
}
