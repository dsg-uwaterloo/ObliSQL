package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	loadBalancer "github.com/project/ObliSql/api/loadbalancer"
	batcher "github.com/project/ObliSql/pkg/batchManager"
	"github.com/project/ObliSql/pkg/tracing"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	addrPort := flag.String("p", "9500", "Batcher Port")
	rPtr := flag.Int("R", 800, "Real Number of Requests")
	timeOutPtr := flag.Int("Z", 500, "Queue Wait time in Milliseconds")
	nPtr := flag.Int("num", 1, "Number of Executors")
	tPtr := flag.String("T", "Waffle", "Executor Type")
	numCPtr := flag.Int("X", 1, "Number of clients to Waffle")
	hostsPtr := flag.String("hosts", "localhost", "Comma-separated list of host addresses")
	portsPtr := flag.String("ports", "9090", "Comma-separated list of port numbers")
	fakeReqPtr := flag.Bool("fr", false, "Turn off Fake Requests")
	tracingBool := flag.Bool("t", false, "Tracing Boolean") //Default no tracing is on.

	flag.Parse()

	// Setting up Tracing
	tracingProvider, err := tracing.NewProvider(ctx, "batcher", "localhost:4317", !*tracingBool)
	if err != nil {
		log.Fatal().Msgf("Failed to create tracing provider; %v", err)
	}
	stopTracingProvider, err := tracingProvider.RegisterAsGlobal()
	if err != nil {
		log.Fatal().Msgf("Failed to register tracing provider; %v", err)
	}
	defer stopTracingProvider(ctx)

	tracer := otel.Tracer("")

	// Define the address where the gRPC server will be listening
	serverAddress := "0.0.0.0:" + *addrPort

	// Create a listener on TCP port
	lis, err := net.Listen("tcp", serverAddress)
	if err != nil {
		log.Fatal().Msgf("Failed to listen on %s: %v", serverAddress, err)
	}
	defer lis.Close()

	log.Info().Msgf("Batcher server listening on %s", serverAddress)

	// Create a new gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(600*1024*1024), // 600 MB
		grpc.MaxSendMsgSize(600*1024*1024), // 600 MB
	)

	// Initialize the batcher service with Redis connection and tracingProvider
	batchService := batcher.NewBatcher(ctx, *rPtr, *nPtr, *timeOutPtr, *tPtr, *hostsPtr, *portsPtr, *numCPtr, *fakeReqPtr, tracer)

	// Register the service with the gRPC server
	loadBalancer.RegisterLoadBalancerServer(grpcServer, batchService)

	// startTime := time.Now()
	// ticker := time.NewTicker(1 * time.Second)
	// defer ticker.Stop()

	// go func() {
	// 	for {
	// 		select {
	// 		case <-ticker.C:
	// 			log.Info().Msgf("Total Non-Fake Keys Seen by Resolver after %d Seconds: %d", int(time.Since(startTime).Seconds()), batchService.TotalKeysSeen.Load())
	// 		case <-ctx.Done():
	// 			return
	// 		}
	// 	}
	// }()

	// Handle graceful shutdown
	go func() {
		// Wait for an interrupt signal or timeout

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		// Create a timer for 1 minute
		timer := time.NewTimer(5 * time.Minute)

		select {
		case sig := <-sigCh:
			fmt.Printf("Received signal: %v. Shutting down server...\n", sig)
		case <-timer.C:
			fmt.Println("Timeout reached. Shutting down server...")
		}
		// Gracefully stop the gRPC server
		grpcServer.GracefulStop()
		cancel()
	}()

	// Start serving incoming gRPC connections
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Msgf("Failed to serve gRPC: %v", err)
	}

}
