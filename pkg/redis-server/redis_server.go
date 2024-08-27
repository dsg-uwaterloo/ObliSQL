package main

import (
	"context"
	"fmt"
	"log"
	"net"

	pb "github.com/Haseeb1399/waffle-go/api/redis"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

// RedisServiceServer is the server struct for the gRPC service
type RedisServiceServer struct {
	pb.UnimplementedRedisServiceServer
	redisClient *redis.Client
}

// NewRedisServiceServer creates a new RedisServiceServer
func NewRedisServiceServer(redisAddr string) *RedisServiceServer {
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	return &RedisServiceServer{redisClient: rdb}
}

// Put implements the Put RPC method
func (s *RedisServiceServer) Put(ctx context.Context, req *pb.PutRequest) (*pb.Empty, error) {
	err := s.redisClient.Set(ctx, req.Key, req.Value, 0).Err()
	if err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

// Get implements the Get RPC method
func (s *RedisServiceServer) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	val, err := s.redisClient.Get(ctx, req.Key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("key not found")
	} else if err != nil {
		return nil, err
	}
	return &pb.GetResponse{Value: val}, nil
}

// PutBatch implements the PutBatch RPC method
func (s *RedisServiceServer) PutBatch(ctx context.Context, req *pb.BatchRequest) (*pb.Empty, error) {
	if len(req.Keys) != len(req.Values) {
		return nil, fmt.Errorf("keys and values must have the same length")
	}

	// Prepare arguments for MSet
	args := make([]interface{}, 0, len(req.Keys)*2)
	for i, key := range req.Keys {
		args = append(args, key, req.Values[i])
	}

	// Use MSet to set all keys in one atomic operation
	err := s.redisClient.MSet(ctx, args...).Err()
	if err != nil {
		return nil, err
	}

	return &pb.Empty{}, nil
}

// GetBatch implements the GetBatch RPC method
func (s *RedisServiceServer) GetBatch(ctx context.Context, req *pb.BatchRequest) (*pb.BatchResponse, error) {
	values, err := s.redisClient.MGet(ctx, req.Keys...).Result()
	if err != nil {
		return nil, err
	}

	var result pb.BatchResponse
	for _, v := range values {
		if v != nil {
			result.Values = append(result.Values, v.(string))
		} else {
			result.Values = append(result.Values, "")
		}
	}

	return &result, nil
}

// DeleteBatch implements the DeleteBatch RPC method
func (s *RedisServiceServer) DeleteBatch(ctx context.Context, req *pb.BatchRequest) (*pb.Empty, error) {
	err := s.redisClient.Del(ctx, req.Keys...).Err()
	if err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

func main() {
	// Set up a TCP listener on port 50051
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Create a new gRPC server
	grpcServer := grpc.NewServer()

	// Register our RedisServiceServer with the gRPC server
	pb.RegisterRedisServiceServer(grpcServer, NewRedisServiceServer("localhost:6379"))

	// Start the gRPC server
	log.Println("Starting gRPC server on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

//This will run on the Redis-Machine / "The Cloud"
