package oramClient

import (
	"context"
	"fmt"

	"github.com/project/ObliSql/api/oramExecutor"
	executor "github.com/project/ObliSql/api/oramExecutor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type OramClient struct {
	client oramExecutor.ExecutorClient
}

func (p *OramClient) CreateClient(address string, port int) error {
	// Establish a connection to the gRPC server
	fullAddr := address + ":" + fmt.Sprintf("%d", port)
	conn, err := grpc.NewClient(fullAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(644000*300), grpc.MaxCallSendMsgSize(644000*300)))
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	fmt.Println("Connected!")

	// Create a client for the PlainTextExecutor service
	newClient := executor.NewExecutorClient(conn)
	p.client = newClient
	return nil
}

func (p *OramClient) InitDB(keys []string, values []string) {
	ctx := context.Background()
	newReq := executor.RequestBatchORAM{
		Keys:      keys,
		Values:    values,
		RequestId: 1,
	}

	resp, err := p.client.InitDb(ctx, &newReq)

	if resp.Value {
		fmt.Println("Initialized DB!")
	}

	if err != nil {
		fmt.Println("Error initializing DB!")
	}
}

func (p *OramClient) MixBatch(keys []string, values []string, batchID int64) ([]string, error) {
	ctx := context.Background()
	newReq := executor.RequestBatchORAM{
		Keys:      keys,
		Values:    values,
		RequestId: 1,
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
