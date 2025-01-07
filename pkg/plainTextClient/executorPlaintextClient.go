package ptClient

import (
	"context"
	"fmt"

	executor "github.com/project/ObliSql/api/plaintextExecutor"
	executorPlaintxt "github.com/project/ObliSql/api/plaintextExecutor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PlainTextClient struct {
	client executorPlaintxt.PlainTextExecutorClient
}

func (p *PlainTextClient) CreateClient(address string, port int) error {
	// Establish a connection to the gRPC server
	fullAddr := address + ":" + fmt.Sprintf("%d", port)
	conn, err := grpc.NewClient(fullAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(600*1024*1024), grpc.MaxCallSendMsgSize(600*1024*1024)))
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	fmt.Println("Connected!")

	// Create a client for the PlainTextExecutor service
	newClient := executor.NewPlainTextExecutorClient(conn)
	p.client = newClient
	return nil
}

func (p *PlainTextClient) InitDB(keys []string, values []string) {
	ctx := context.Background()
	newReq := executor.RequestBatch{
		Keys:      keys,
		Values:    values,
		RequestId: 1,
	}

	_, err := p.client.InitDb(ctx, &newReq)
	if err != nil {
		fmt.Println("Error initializing DB!")
	}
}

func (p *PlainTextClient) MixBatch(keys []string, values []string, batchID int64) ([]string, error) {
	ctx := context.Background()
	newReq := executor.RequestBatch{
		Keys:      keys,
		Values:    values,
		RequestId: batchID,
	}

	resp, err := p.client.ExecuteBatch(ctx, &newReq)
	if err != nil {
		fmt.Println("Error Executing Batch!")
		return nil, err
	}

	ret := []string{}
	for i, v := range resp.Keys {
		newVal := v + ":" + resp.Values[i]
		ret = append(ret, newVal)
	}
	return ret, nil
}
