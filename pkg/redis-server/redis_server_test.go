package main

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	pb "github.com/Haseeb1399/waffle-go/api/redis"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	testRedisAddress = "localhost:6379"  // Redis server address for testing
	testGRPCAddress  = "localhost:50051" // gRPC server address for testing
)

var (
	grpcServer *grpc.Server
	once       sync.Once
)

func startTestGRPCServer() {
	once.Do(func() {
		lis, err := net.Listen("tcp", testGRPCAddress)
		if err != nil {
			panic(err) // Using panic to indicate setup failure in test environment
		}

		grpcServer = grpc.NewServer()
		pb.RegisterRedisServiceServer(grpcServer, NewRedisServiceServer(testRedisAddress))

		go func() {
			if err := grpcServer.Serve(lis); err != nil {
				panic(err) // Using panic to indicate setup failure in test environment
			}
		}()

		// Allow some time for the server to start
		time.Sleep(time.Second)
	})
}

// Helper function to create a new Redis client for testing
func createRedisTestClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: testRedisAddress,
	})
}

// TestPut tests the Put method
func TestPut(t *testing.T) {
	startTestGRPCServer()

	conn, err := grpc.NewClient(testGRPCAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewRedisServiceClient(conn)

	// Test Put
	_, err = client.Put(context.Background(), &pb.PutRequest{Key: "test_key", Value: "test_value"})
	assert.NoError(t, err, "Put request failed")

	// Verify the value is set in Redis
	redisClient := createRedisTestClient()
	defer redisClient.Close()

	val, err := redisClient.Get(context.Background(), "test_key").Result()
	assert.NoError(t, err, "Failed to get value from Redis")
	assert.Equal(t, "test_value", val, "Value does not match")
}

// TestGet tests the Get method
func TestGet(t *testing.T) {
	startTestGRPCServer()

	conn, err := grpc.NewClient(testGRPCAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewRedisServiceClient(conn)

	// Set initial value in Redis
	redisClient := createRedisTestClient()
	defer redisClient.Close()
	redisClient.Set(context.Background(), "test_key", "test_value", 0)

	// Test Get
	resp, err := client.Get(context.Background(), &pb.GetRequest{Key: "test_key"})
	assert.NoError(t, err, "Get request failed")
	assert.Equal(t, "test_value", resp.Value, "Value does not match")
}

// TestPutBatch tests the PutBatch method
func TestPutBatch(t *testing.T) {
	startTestGRPCServer()

	conn, err := grpc.NewClient(testGRPCAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewRedisServiceClient(conn)

	// Test PutBatch
	_, err = client.PutBatch(context.Background(), &pb.BatchRequest{
		Keys:   []string{"key1", "key2"},
		Values: []string{"value1", "value2"},
	})
	assert.NoError(t, err, "PutBatch request failed")

	// Verify the values are set in Redis
	redisClient := createRedisTestClient()
	defer redisClient.Close()

	val1, err := redisClient.Get(context.Background(), "key1").Result()
	assert.NoError(t, err, "Failed to get value from Redis")
	assert.Equal(t, "value1", val1, "Value does not match for key1")

	val2, err := redisClient.Get(context.Background(), "key2").Result()
	assert.NoError(t, err, "Failed to get value from Redis")
	assert.Equal(t, "value2", val2, "Value does not match for key2")
}

// TestGetBatch tests the GetBatch method
func TestGetBatch(t *testing.T) {
	startTestGRPCServer()

	conn, err := grpc.NewClient(testGRPCAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewRedisServiceClient(conn)

	// Set initial values in Redis
	redisClient := createRedisTestClient()
	defer redisClient.Close()
	redisClient.MSet(context.Background(), "key1", "value1", "key2", "value2")

	// Test GetBatch
	resp, err := client.GetBatch(context.Background(), &pb.BatchRequest{
		Keys: []string{"key1", "key2"},
	})
	assert.NoError(t, err, "GetBatch request failed")
	assert.Equal(t, []string{"value1", "value2"}, resp.Values, "Values do not match")
}

// TestDeleteBatch tests the DeleteBatch method
func TestDeleteBatch(t *testing.T) {
	startTestGRPCServer()

	conn, err := grpc.NewClient(testGRPCAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewRedisServiceClient(conn)

	// Set initial values in Redis
	redisClient := createRedisTestClient()
	defer redisClient.Close()
	redisClient.MSet(context.Background(), "key1", "value1", "key2", "value2")

	// Test DeleteBatch
	_, err = client.DeleteBatch(context.Background(), &pb.BatchRequest{
		Keys: []string{"key1", "key2"},
	})
	assert.NoError(t, err, "DeleteBatch request failed")

	// Verify the values are deleted in Redis
	val1, err := redisClient.Get(context.Background(), "key1").Result()
	assert.Equal(t, redis.Nil, err, "Expected redis.Nil error for deleted key1")
	assert.Equal(t, "", val1, "Expected empty value for deleted key1")

	val2, err := redisClient.Get(context.Background(), "key2").Result()
	assert.Equal(t, redis.Nil, err, "Expected redis.Nil error for deleted key2")
	assert.Equal(t, "", val2, "Expected empty value for deleted key2")
}

func TestConcurrentReadWrite(t *testing.T) {
	startTestGRPCServer()

	conn, err := grpc.NewClient(testGRPCAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewRedisServiceClient(conn)

	var wg sync.WaitGroup
	numGoroutines := 20 // Number of concurrent goroutines for reads and writes

	// Start concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "concurrent_key_" + string(rune(i))
			value := "concurrent_value_" + string(rune(i))
			// log.Println("Writing: ", key, ":", value)
			_, err := client.Put(context.Background(), &pb.PutRequest{Key: key, Value: value})
			assert.NoError(t, err, "Put request failed")
		}(i)
	}
	wg.Wait()

	// Start concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "concurrent_key_" + string(rune(i))
			// log.Println("Getting: ", key)
			resp, err := client.Get(context.Background(), &pb.GetRequest{Key: key})
			assert.NoError(t, err, "Get request failed")
			expectedValue := "concurrent_value_" + string(rune(i))
			assert.Equal(t, expectedValue, resp.Value, "Value does not match")
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
}
