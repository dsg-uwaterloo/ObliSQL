package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/project/ObliSql/api/resolver"
	"github.com/project/ObliSql/pkg/benchmark"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	hPtr := flag.String("h", "localhost", "Resolver Host")
	pPtr := flag.String("p", "9900", "Resolver Host")
	sPtr := flag.Int("s", 2000, "Maximum in-flight requests")
	tPtr := flag.Int("t", 30, "Duration to run benchmark (in seconds)")

	flag.Parse()

	resolverAddr := *hPtr + ":" + *pPtr

	conn, err := grpc.NewClient(resolverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(644000*300),
		grpc.MaxCallSendMsgSize(644000*300),
	))
	defer conn.Close()
	if err != nil {
		log.Fatalf("Failed to open connection to Resolver: %v", err)
	}

	resolverClient := resolver.NewResolverClient(conn)

	res, err := resolverClient.ConnectPingResolver(context.Background(), &resolver.ClientConnectResolver{Id: "1"})

	if err != nil || res.Id != "1" {
		log.Fatalf("Could not connect to Resolver")
		return
	} else {
		fmt.Printf("Connected to Resolver on %s \n", resolverAddr)
	}

	benchmark.StartBench(resolverClient, *sPtr, *tPtr)
}
