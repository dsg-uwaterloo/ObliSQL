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

	"github.com/project/ObliSql/api/oramExecutor"
	oramexecutor "github.com/project/ObliSql/pkg/oramExecutor"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

func main() {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	redisHost := flag.String("rh", "127.0.0.1", "Redis Host")
	redisPort := flag.String("rp", "6379", "Redis Host")
	addrPort := flag.String("p", "9090", "Executor Port")
	logCap := flag.Int("l", 16, "Logarithm base 2 of capacity")
	zVal := flag.Int("z", 5, "Number of blocks per bucket")
	stashSize := flag.Int("s", 100000, "Maximum number of blocks in Stash")
	traceLocation := flag.String("tl", "../../tracefiles/serverInputTEST.txt", "Location to tracefile for initializing DB")

	flag.Parse()

	serverAddress := "0.0.0.0:" + *addrPort
	redisAddress := *redisHost + ":" + *redisPort

	lis, err := net.Listen("tcp", serverAddress)
	if err != nil {
		log.Fatal().Msgf("Failed to listen on %s: %v", serverAddress, err)
	}
	defer lis.Close()

	log.Info().Msgf("gRPC server listening on %s", serverAddress)

	// Create a new gRPC server
	grpcServer := grpc.NewServer()

	// Initialize the executor service with Redis connection and tracingProvider

	executor, err := oramexecutor.NewORAM(*logCap, *zVal, *stashSize, redisAddress, *traceLocation)

	if err != nil {
		log.Fatal().Msgf("Failed to initialize ORAM! %s \n", err)
	}

	// Register the service with the gRPC server
	oramExecutor.RegisterExecutorServer(grpcServer, executor)

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
