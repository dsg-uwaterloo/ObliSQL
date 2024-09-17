package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Haseeb1399/WorkingThesis/api/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

func runBenchmark(resolverClient resolver.ResolverClient, requests []Query, rateLimit *RateLimit, duration int) {
	ack_channel := make(chan Ack)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(duration)*time.Second)
	defer cancel()
	go sendRequestsForever(ctx, ack_channel, requests, resolverClient, rateLimit)
	Ops, Err := getResponses(ctx, ack_channel)

	fmt.Println(Ops, Err)
}

func main() {
	resolverAddr := "localhost:9900"
	conn, err := grpc.NewClient(resolverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(644000*300),
		grpc.MaxCallSendMsgSize(644000*300),
	))
	if err != nil {
		log.Fatalf("Failed to open connection to Resolver: %v", err)
	}

	requests := []Query{}

	for len(requests) < 1000000 {
		requests = append(requests, getTestCases()...)
	}

	rateLimit := NewRateLimit(8000)

	defer conn.Close()

	resolverClient := resolver.NewResolverClient(conn)
	runBenchmark(resolverClient, requests, rateLimit, 10)

	log.Println("Benchmark completed.")
}
