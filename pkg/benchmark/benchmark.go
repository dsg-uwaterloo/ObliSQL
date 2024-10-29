package benchmark

import (
	"context"
	"fmt"
	"time"

	"github.com/project/ObliSql/api/resolver"
)

type Ack struct {
	had_error bool
}

func asyncRequest(ctx context.Context, ack_channel chan Ack, resolverClient resolver.ResolverClient, request Query, rateLimit *RateLimit) {
	rateLimit.Acquire()
	_, err := resolverClient.ExecuteQuery(ctx, request.requestQuery)
	rateLimit.Release()
	if err != nil {
		ack_channel <- Ack{had_error: true}
	} else {
		ack_channel <- Ack{had_error: false}
	}
}

func sendRequestsForever(ctx context.Context, ack_channel chan Ack, requests []Query, resolverClient resolver.ResolverClient, rateLimit *RateLimit) {
	for _, request := range requests {
		select {
		case <-ctx.Done():
			return
		default:
			go asyncRequest(ctx, ack_channel, resolverClient, request, rateLimit)
		}
	}
	ctx.Done()
}

func getResponses(ctx context.Context, ack_channel chan Ack) (int, int) {
	Operations, Errors := 0, 0
	for {
		select {
		case <-ctx.Done():
			return Operations, Errors
		case ack := <-ack_channel:
			if ack.had_error {
				Errors++
			} else {
				Operations++
			}
		}
	}

}

func runBenchmark(resolverClient resolver.ResolverClient, requests []Query, rateLimit *RateLimit, duration int, warmup bool) (int, int) {
	ack_channel := make(chan Ack)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(duration)*time.Second)
	defer cancel()
	go sendRequestsForever(ctx, ack_channel, requests, resolverClient, rateLimit)
	Ops, Err := getResponses(ctx, ack_channel)
	if !warmup {

		fmt.Printf("Ops/s,%d,Err,%d\n", Ops, Err)

	}
	return Ops, Err
}

func StartBench(resolverClient resolver.ResolverClient, inFlight int, timeDuration int) {
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
	Ops1, Err1 := runBenchmark(resolverClient, requestsWarmup, rateLimit, 10, true)
	fmt.Printf("Warmup Done! %d %d \n", Ops1, Err1)
	fmt.Println("-------")
	fmt.Printf("Running Benchmark! %d seconds \n", timeDuration)
	rateLimitNew := NewRateLimit(inFlight)
	Ops2, Err2 := runBenchmark(resolverClient, requestsBench, rateLimitNew, timeDuration, false)

	fmt.Printf("Total Ops: %d\n", Ops2)
	fmt.Printf("Total Err: %d\n", Err2)
}
