package benchmark

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync/atomic"
	"time"

	"github.com/project/ObliSql/api/resolver"
)

type Ack struct {
	hadError bool
	latency  time.Duration
}

func GetRandomClient(resolverClient *[]resolver.ResolverClient) resolver.ResolverClient {
	if len(*resolverClient) == 0 {
		return nil
	}
	randIdx := rand.Intn(len(*resolverClient))
	return (*resolverClient)[randIdx]
}

func asyncRequest(ctx context.Context, ackChannel *chan Ack, resolverClient *[]resolver.ResolverClient, request *Query, rateLimit *RateLimit, counter *atomic.Int64) {
	rateLimit.Acquire()
	conn := GetRandomClient(resolverClient)

	start := time.Now() // Record start time
	resp, err := conn.ExecuteQuery(ctx, request.requestQuery)
	latency := time.Since(start) // Calculate latency
	rateLimit.Release()

	if err != nil {
		*ackChannel <- Ack{hadError: true, latency: latency}
	} else {
		if len(resp.Values) > 0 {
			counter.Add(1)
		}
		*ackChannel <- Ack{hadError: false, latency: latency}
	}
}

func sendRequestsForever(ctx context.Context, ackChannel *chan Ack, requests *[]Query, resolverClient *[]resolver.ResolverClient, rateLimit *RateLimit, counter *atomic.Int64) {
	for _, request := range *requests {
		select {
		case <-ctx.Done():
			return
		default:
			go asyncRequest(ctx, ackChannel, resolverClient, &request, rateLimit, counter)
		}
	}
}

// Modified getResponses to collect latencies
func getResponses(ctx context.Context, ackChannel *chan Ack, latencies *[]time.Duration) (int, int) {
	operations, errors := 0, 0
	for {
		select {
		case <-ctx.Done():
			return operations, errors
		case ack := <-*ackChannel:
			if ack.hadError {
				errors++
			} else {
				operations++
			}
			*latencies = append(*latencies, ack.latency) // Collect latency
		}
	}
}

func runBenchmark(resolverClient *[]resolver.ResolverClient, requests *[]Query, rateLimit *RateLimit, duration int, warmup bool) (int, int, time.Duration) {
	realRequestCounter := atomic.Int64{}
	latencies := []time.Duration{}
	ackChannel := make(chan Ack)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(duration)*time.Second)
	defer cancel()

	go sendRequestsForever(ctx, &ackChannel, requests, resolverClient, rateLimit, &realRequestCounter)
	ops, err := getResponses(ctx, &ackChannel, &latencies)
	if !warmup {
		fmt.Printf("Ops/s,%d,Err,%d\n", ops, err)
	}
	fmt.Printf("Non-Empty Requests: %d\n", realRequestCounter.Load())

	var totalLatency time.Duration
	for _, l := range latencies {
		totalLatency += l
	}
	averageLatency := time.Duration(0)
	if len(latencies) > 0 {
		averageLatency = totalLatency / time.Duration(len(latencies))
	}
	// Write all latencies to file
	if !warmup {
		timestamp := time.Now().Format("2006-01-02_15-04-05")
		filename := fmt.Sprintf("latencies_benchmark_%s.txt", timestamp)
		file, err := os.Create(filename)
		if err != nil {
			fmt.Println("Error creating file:", err)
		} else {
			defer file.Close()
			for _, latency := range latencies {
				_, err = file.WriteString(fmt.Sprintf("%v\n", latency))
				if err != nil {
					fmt.Println("Error writing to file:", err)
					break
				}
			}
		}
	}
	// fmt.Printf("Average Latency: %v\n", averageLatency)
	return ops, err, averageLatency
}

func StartBench(resolverClient *[]resolver.ResolverClient, inFlight int, timeDuration int) {
	requestsWarmup := []Query{}
	for len(requestsWarmup) < 50000 {
		requestsWarmup = append(requestsWarmup, getTestCases()...)
	}

	requestsBench := []Query{}
	for len(requestsBench) < 500000 {
		requestsBench = append(requestsBench, getTestCases()...)
	}

	fmt.Println("In-Flight Requests:", inFlight)

	rateLimit := NewRateLimit(inFlight)
	ops1, err1, _ := runBenchmark(resolverClient, &requestsWarmup, rateLimit, 10, true)
	fmt.Printf("Warmup Done! %d %d\n", ops1, err1)
	fmt.Println("-------")
	fmt.Printf("Running Benchmark! %d seconds \n", timeDuration)
	rateLimitNew := NewRateLimit(inFlight)
	ops2, err2, lat2 := runBenchmark(resolverClient, &requestsBench, rateLimitNew, timeDuration, false)

	fmt.Printf("Total Ops: %d\n", ops2)
	fmt.Printf("Total Err: %d\n", err2)
	fmt.Printf("Average Latency: %v ms\n", lat2.Milliseconds())
}
