package main

import (
	"context"
	"crypto/sha256"
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
	logCap := flag.Int("l", 22, "Logarithm base 2 of capacity")
	zVal := flag.Int("z", 5, "Number of blocks per bucket")
	stashSize := flag.Int("s", 8000000, "Maximum number of blocks in Stash")
	traceLocation := flag.String("tl", "../../tracefiles/serverInputTEST.txt", "Location to tracefile for initializing DB")
	useSnapshot := flag.Bool("snapshot", false, "Use database snapshot") // use flag like -snapshot
	batchSize := flag.Int("br", 10, "Batch size for ORAM")

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

	// Use a standard key for encryption
	key_input := "oblisqloram"
	// Generate the SHA-256 hash of the input string
	hash := sha256.New()
	hash.Write([]byte(key_input))

	// Return the 256-bit (32-byte) hash as a byte slice
	key := hash.Sum(nil)

	// Initialize the executor service with Redis connection and tracingProvider

	executor, err := oramexecutor.NewORAM(*logCap, *zVal, *stashSize, redisAddress, *traceLocation, *useSnapshot, *batchSize, key)

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
