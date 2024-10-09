package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Haseeb1399/WorkingThesis/api/loadbalancer"
	resolver "github.com/Haseeb1399/WorkingThesis/api/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	// OpenTelemetry imports
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func initTracer() (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	// Create the OTLP exporter over gRPC
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint("localhost:4317"), // Adjust endpoint for Grafana
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create a resource to identify this application
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("my-resolver"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create the TracerProvider with the exporter and resource
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set the global TracerProvider
	otel.SetTracerProvider(tp)

	return tp, nil
}

func (c *myResolver) ExecuteQuery(ctx context.Context, q *resolver.ParsedQuery) (*resolver.QueryResponse, error) {
	requestID := c.localRequestID.Add(1)
	clientId, errConv := strconv.Atoi(q.ClientId)
	if errConv != nil {
		return nil, fmt.Errorf("error converting clientId to integer: %w", errConv)
	}

	var resp *queryResponse
	var err error
	switch q.QueryType {
	case "select":
		resp, err = c.doSelect(q, requestID)
	case "aggregate":
		resp, err = c.doAggregate(q, requestID)
	case "join":
		resp, err = c.doJoin(q, requestID)
	case "update":
		resp, err = c.doUpdate(q, requestID)
	default:
		return nil, fmt.Errorf("unsupported query type: %s", q.QueryType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to execute %s query with id:%d. error: %w", q.QueryType, requestID, err)
	} else {
		c.requestsDone.Add(1)
		return &resolver.QueryResponse{
			ClientId:  int64(clientId),
			RequestId: requestID,
			Keys:      resp.Keys,
			Values:    resp.Values,
		}, nil
	}
}

func (lb *myResolver) ConnectPingResolver(ctx context.Context, req *resolver.ClientConnectResolver) (*resolver.ClientConnectResolver, error) {
	fmt.Println("Client Connected!")

	toRet := resolver.ClientConnectResolver{
		Id: req.Id,
	}

	return &toRet, nil
}

func (c *myResolver) readMetaData(filePath string) {
	//MetaData
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Error opening metadata file: %s\n", err)
		return
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("Error reading metadata file: %s\n", err)
		return
	}

	var data map[string]MetaData
	err = json.Unmarshal(byteValue, &data)
	if err != nil {
		log.Fatalf("Error unmarshaling JSON: %s\n", err)
		return
	}

	c.metaData = data
}

func (c *myResolver) readJoinMap(filePath string) {
	//MetaData
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Error opening metadata file: %s\n", err)
		return
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("Error reading metadata file: %s\n", err)
		return
	}

	var data map[string]interface{}
	err = json.Unmarshal(byteValue, &data)
	if err != nil {
		log.Fatalf("Error unmarshaling JSON: %s\n", err)
		return
	}

	c.JoinMap = data
	time.Sleep(2 * time.Second)
}

func main() {
	hPtr := flag.String("h", "localhost", "Address of Batch Manager")
	pPtr := flag.String("p", "9500", "Port of Batch Manager")

	flag.Parse()

	lis, err := net.Listen("tcp", ":9900")
	if err != nil {
		log.Fatalf("Cannot create listener on port :9900 %s", err)

	}
	fmt.Println("Starting Resolver on: localhost:9900")

	// Initialize OpenTelemetry Tracer
	tp, err := initTracer()
	if err != nil {
		log.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Fatalf("Error shutting down tracer provider: %v", err)
		}
	}()

	metaDataLoc := "./metadata.txt"
	joinMapLoc := "./JoinMaps/join_map.json"

	lb_host := *hPtr
	lb_port := *pPtr
	lb_addr := lb_host + ":" + lb_port

	conn, err := grpc.NewClient(lb_addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(644000*300), grpc.MaxCallSendMsgSize(644000*300)))

	if err != nil {
		log.Fatalf("Failed to open connection to load balancer")
	}
	lbClient := loadbalancer.NewLoadBalancerClient(conn)

	res, err := lbClient.ConnectPing(context.Background(), &loadbalancer.ClientConnect{Id: "1"})

	if err != nil || res.Id != "1" {
		log.Fatalf("Could not connect to batch manager")
		return
	} else {
		fmt.Println("Connected to Batch Manager!")
	}

	service := myResolver{
		conn:           lbClient,
		done:           atomic.Int32{},
		requestId:      atomic.Int64{},
		recvChan:       make(chan int32),
		localRequestID: atomic.Int64{},
		tracer:         tp,
		requestsDone:   atomic.Int64{},
	}
	service.readMetaData(metaDataLoc)
	service.readJoinMap(joinMapLoc)
	service.done.Store(0)
	serverRegister := grpc.NewServer()
	resolver.RegisterResolverServer(serverRegister, &service)

	// Start a background goroutine to gracefully shut down after 200 seconds
	time.AfterFunc(200*time.Second, func() {
		fmt.Println("200 seconds passed. Shutting down the resolver...")
		lis.Close()
		serverRegister.GracefulStop()
		os.Exit(0)
	})

	// Handle system signals (e.g., SIGINT, SIGTERM) for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Launch a goroutine to handle signal events
	go func() {
		<-sigChan
		fmt.Println("Received interrupt signal. Shutting down the resolver...")
		fmt.Printf("Total Requests Done: %d\n", service.requestsDone.Load())
		lis.Close()
		serverRegister.GracefulStop()
		os.Exit(0)
	}()

	err = serverRegister.Serve(lis)

	if err != nil {
		log.Fatalf("Error! %s", err)
	}

}
