package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/Haseeb1399/WorkingThesis/api/loadbalancer"
	resolver "github.com/Haseeb1399/WorkingThesis/api/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func (c *myResolver) ExecuteQuery(ctx context.Context, q *resolver.ParsedQuery) (*resolver.QueryResponse, error) {
	if q.QueryType == "select" {
		resp, err := c.doSelect(q)
		if err != nil {
			log.Fatalf("Failed to execute Query!")
		}
		clientId, err := strconv.Atoi(q.ClientId)
		if err != nil {
			log.Fatalf("Error converting clientId to integer: %s \n", err)
		}

		return &resolver.QueryResponse{
			ClientId:  int64(clientId),
			RequestId: 1,
			Keys:      resp.Keys,
			Values:    resp.Values,
		}, nil
	} else {
		//More else ifs after other types
		clientId, err := strconv.Atoi(q.ClientId)
		if err != nil {
			log.Fatalf("Error converting clientId to integer: %s \n", err)
		}

		return &resolver.QueryResponse{
			ClientId:  int64(clientId),
			RequestId: 1,
			Keys:      nil,
			Values:    nil,
		}, nil
	}
}

func (c *myResolver) readMetaData(filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Error opening metadata file: %s \n", err)
		return
	}
	defer file.Close()

	configMap := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) == 2 {
			key := parts[0]
			value := parts[1]
			configMap[key] = value
		}
	}

	var m metaData
	for key, value := range configMap {
		switch key {
		case "colNames":
			m.colNames = strings.Split(value, " ")
		case "indexOn":
			m.indexOn = strings.Split(value, " ")
		case "pkEnd":
			pkEnd, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				log.Fatalf("Error converting pkEnd to integer: %s \n", err)
			}
			m.pkEnd = pkEnd
		case "pkStart":
			pkStart, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				log.Fatalf("Error converting pkStart to integer: %s \n", err)
			}
			m.pkStart = pkStart
		case "tableName":
			m.tableName = value
		default:
			log.Printf("Unknown key in MetaData : %s \n", key)
		}
	}
	c.metaData = m
}

func main() {

	lis, err := net.Listen("tcp", ":9900")
	if err != nil {
		log.Fatalf("Cannot create listener on port :9900 %s", err)

	}
	fmt.Println("Starting Resolver on: localhost:9900")

	metaDataLoc := "./metadata.txt"

	lb_host := "localhost"
	lb_port := "9500"
	lb_addr := lb_host + ":" + lb_port

	conn, err := grpc.NewClient(lb_addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(644000*300), grpc.MaxCallSendMsgSize(644000*300)))

	if err != nil {
		log.Fatalf("Failed to open connection to load balancer")
	}
	lbClient := loadbalancer.NewLoadBalancerClient(conn)

	service := myResolver{
		conn:      lbClient,
		done:      atomic.Int32{},
		requestId: atomic.Int64{},
		recvChan:  make(chan int32),
	}
	service.readMetaData(metaDataLoc)
	service.done.Store(0)
	serverRegister := grpc.NewServer()
	resolver.RegisterResolverServer(serverRegister, &service)

	err = serverRegister.Serve(lis)

	if err != nil {
		log.Fatalf("Error! Could not start loadBalancer! %s", err)
	}

}

//Fixes (comments in the doSelect function)
//Make it moduluar (Convert to functions) and remove repeated code.
//Create test file and move test cases to file. (Via Network messages)
//Tests should also check for values.
//Convert to modular format.
//Remove all assumptions (Single column) --> Searching, getting multiple columns.
// >, < , >=,<= cases as well.
