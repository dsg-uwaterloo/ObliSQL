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

	executorPlaintxtApi "github.com/project/ObliSql/api/plaintextExecutor"
	executorPlaintxt "github.com/project/ObliSql/pkg/executorPlaintext"
	tracing "github.com/project/ObliSql/pkg/tracing"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	redisHost := flag.String("rh", "127.0.0.1", "Redis Host")
	redisPort := flag.String("rp", "6379", "Redis Host")
	addrPort := flag.String("p", "9090", "Executor Port")
	tracingBool := flag.Bool("t", false, "Tracing Boolean") //Default no tracing is on.
	traceFileLocation := flag.String("l", "../../tracefiles/serverInput.txt", "TraceFileLocation")

	flag.Parse()

	// Setting up Tracing
	tracingProvider, err := tracing.NewProvider(ctx, "plaintext-executor", "localhost:4317", !*tracingBool)
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

	log.Info().Msgf("gRPC server listening on %s", serverAddress)

	// Create a new gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(600*1024*1024),
		grpc.MaxSendMsgSize(600*1024*1024),
	)

	// Initialize the executor service with Redis connection and tracingProvider
	executor := executorPlaintxt.NewExecutor(*redisHost, *redisPort, tracer, *traceFileLocation)

	// Register the service with the gRPC server
	executorPlaintxtApi.RegisterPlainTextExecutorServer(grpcServer, executor)

	// Handle graceful shutdown
	go func() {
		// Wait for an interrupt signal or timeout

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		// Create a timer for 1 minute
		timer := time.NewTimer(10 * time.Minute)

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
