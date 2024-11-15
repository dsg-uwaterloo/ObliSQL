package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	resolverAPI "github.com/project/ObliSql/api/resolver"
	"github.com/project/ObliSql/pkg/resolver"
	"github.com/project/ObliSql/pkg/tracing"
	"go.opentelemetry.io/otel"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	hPtr := flag.String("h", "0.0.0.0", "Address of Resolver")
	pPtr := flag.String("p", "9900", "Port for Resolver")
	bHostPtr := flag.String("bh", "localhost", "Address of Batchers (Comma Separated)")
	bPortPtr := flag.String("bp", "9500", "Port of Batcher (Comma Separated)")
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
	serverAddress := *hPtr + ":" + *pPtr

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

	// Locations: --UPDATE TO BE PASSED VIA COMMAND LINE.
	metaDataLoc := "../../metaData/metadata.txt"
	joinMapLoc := "../../metaData/JoinMaps/join_map.json"
	traceLoc := "../../tracefiles/serverInput50.txt"

	bHostList := strings.Split(*bHostPtr, ",")
	pHostList := strings.Split(*bPortPtr, ",")

	resolverService := resolver.NewResolver(ctx, bHostList, pHostList, traceLoc, metaDataLoc, joinMapLoc, tracer)
	resolverAPI.RegisterResolverServer(grpcServer, resolverService)

	// Handle graceful shutdown
	go func() {
		// Wait for an interrupt signal or timeout

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		// Create a timer for 5 minute. Automatically quit after 5 minutes.
		timer := time.NewTimer(5 * time.Minute)

		select {
		case sig := <-sigCh:
			fmt.Println(resolverService.Created.Load(), resolverService.Inserted.Load())
			fmt.Printf("Received signal: %v. Shutting down server...\n", sig)
			grpcServer.GracefulStop()
			cancel()
			return
		case <-timer.C:
			fmt.Println("Timeout reached. Shutting down server...")
			grpcServer.GracefulStop()
			cancel()
			return
		}
		return
	}()

	// Start serving incoming gRPC connections
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Msgf("Failed to serve gRPC: %v", err)
	}
}
