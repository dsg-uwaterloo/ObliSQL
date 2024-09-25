package main

import (
	"context"
	"flag"
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
	fmt.Println("Ops/s, Err")
	fmt.Println(Ops, Err)
}

func main() {
	hPtr := flag.String("h", "localhost", "Resolver Host")
	pPtr := flag.String("p", "9900", "Resolver Host")

	flag.Parse()

	resolverAddr := *hPtr + ":" + *pPtr
	fmt.Println(resolverAddr)
	conn, err := grpc.NewClient(resolverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(644000*300),
		grpc.MaxCallSendMsgSize(644000*300),
	))
	if err != nil {
		log.Fatalf("Failed to open connection to Resolver: %v", err)
	}

	resolverClient := resolver.NewResolverClient(conn)

	res, err := resolverClient.ConnectPingResolver(context.Background(), &resolver.ClientConnectResolver{Id: "1"})

	if err != nil || res.Id != "1" {
		log.Fatalf("Could not connect to Resolver")
		return
	} else {
		fmt.Println("Connected to Resolver!")
	}

	requests := []Query{}

	for len(requests) < 500000 {
		requests = append(requests, getTestCases()...)
	}

	rateLimit := NewRateLimit(2000)

	defer conn.Close()

	runBenchmark(resolverClient, requests, rateLimit, 10)
}
