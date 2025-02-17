package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/project/ObliSql/api/resolver"
	"github.com/project/ObliSql/pkg/benchmark"
	"golang.org/x/exp/rand"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func splitCommaSeparated(input string) []string {
	return strings.Split(input, ",")
}
func main() {
	hPtr := flag.String("h", "localhost", "Resolver Hosts")
	pPtr := flag.String("p", "9900", "Resolver Hosts")
	sPtr := flag.Int("s", 2000, "Maximum in-flight requests")
	tPtr := flag.Int("t", 30, "Duration to run benchmark (in seconds)")
	qTypePtr := flag.String("q", "default", "Query Type (Scaling,BDB,Epinions,Default)")

	flag.Parse()

	rand.Seed(uint64(time.Now().UnixNano()))

	hosts := *hPtr
	ports := *pPtr
	resolverAddrs := []string{}
	splitHosts := splitCommaSeparated(hosts)
	splitPorts := splitCommaSeparated(ports)

	for i, host := range splitHosts {
		addr := host + ":" + splitPorts[i]
		resolverAddrs = append(resolverAddrs, addr)
	}

	clients := []resolver.ResolverClient{}

	for _, addr := range resolverAddrs {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(644000*300),
			grpc.MaxCallSendMsgSize(644000*300),
		))
		if err != nil {
			log.Printf("Failed to connect to Resolver %s: %v", addr, err)
			continue
		}

		client := resolver.NewResolverClient(conn)
		res, err := client.ConnectPingResolver(context.Background(), &resolver.ClientConnectResolver{Id: "1"})
		if err != nil || res.Id != "1" {
			log.Printf("Failed to ping Resolver %s: %v", addr, err)
			conn.Close()
			continue
		}
		fmt.Printf("Connected to Resolver %s\n", addr)
		clients = append(clients, client)
	}

	if len(clients) == 0 {
		log.Fatal("No resolvers available to connect.")
	}
	fmt.Println("Query Type:", *qTypePtr)

	benchmark.StartBench(&clients, *sPtr, *tPtr, *qTypePtr)
}
