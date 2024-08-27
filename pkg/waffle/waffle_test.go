package main

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"testing"

	waffle_service "github.com/Haseeb1399/waffle-go/api/waffle"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var testCases = []TestCase{
	{
		name: "Valid Get Request",
		requestBatch: &waffle_service.PutBatchRequest{
			Keys:   []string{"K1", "K2", "K3"},
			Values: []string{"", "", ""},
		},
		expected: &waffle_service.ExecuteBatchResponse{

			Keys:   []string{"K1", "K2", "K3"},
			Values: []string{"V1", "V2", "V3"},
		},
	},
	{
		name: "Get Non-existant Value",
		requestBatch: &waffle_service.PutBatchRequest{

			Keys:   []string{"K100", "K200", "K10000"},
			Values: []string{"", "", ""},
		},
		expected: &waffle_service.ExecuteBatchResponse{

			Keys:   []string{"K100", "K200", "K10000"},
			Values: []string{"V100", "V200", "-1"},
		},
	},
	{
		name: "Normal Put Request",
		requestBatch: &waffle_service.PutBatchRequest{

			Keys:   []string{"K1", "K2", "K3"},
			Values: []string{"VT1", "VT2", "VT3"},
		},
		expected: &waffle_service.ExecuteBatchResponse{

			Keys:   []string{"K1", "K2", "K3"},
			Values: []string{"VT1", "VT2", "VT3"},
		},
	},
	{
		name: "Get & Puts in One",
		requestBatch: &waffle_service.PutBatchRequest{
			Keys:   []string{"K1", "K2", "K1", "K3", "K1", "K1", "K1"},
			Values: []string{"", "", "Test", "VT3", "", "Final", ""},
		},
		expected: &waffle_service.ExecuteBatchResponse{
			Keys:   []string{"K1", "K2", "K1", "K3", "K1", "K1", "K1"},
			Values: []string{"VT1", "VT2", "Test", "VT3", "Test", "Final", "Final"},
		},
	},
	{
		name: "Get & Puts With Missing Key",
		requestBatch: &waffle_service.PutBatchRequest{
			Keys:   []string{"K1", "K2", "K1", "k999999", "K3", "K1", "K1", "K1"},
			Values: []string{"", "", "Test", "", "VT3", "", "Final", ""},
		},
		expected: &waffle_service.ExecuteBatchResponse{

			Keys:   []string{"K1", "K2", "K1", "k999999", "K3", "K1", "K1", "K1"},
			Values: []string{"Final", "VT2", "Test", "-1", "VT3", "Test", "Final", "Final"},
		},
	},
	// {
	// 	name: "Missing Key, then Inserted",
	// 	requestBatch: &waffle_service.PutBatchRequest{
	// 		Keys:   []string{"K999999", "K999999", "K999999"},
	// 		Values: []string{"", "Hello", ""},
	// 	},
	// 	expected: &waffle_service.ExecuteBatchResponse{
	// 		Keys:   []string{"K999999", "K999999", "K999999"},
	// 		Values: []string{"-1", "Hello", "Hello"},
	// 	},
	// },
}

type TestCase struct {
	name         string
	requestBatch *waffle_service.PutBatchRequest
	expected     *waffle_service.ExecuteBatchResponse
}

func generateDataSet() *waffle_service.InitRequest {
	newReq := waffle_service.InitRequest{
		Keys:   make([]string, 0),
		Values: make([]string, 0),
	}

	for i := 0; i < 1000; i++ {
		newKey := fmt.Sprintf("K%d", i)
		newValue := fmt.Sprintf("V%d", i)
		newReq.Keys = append(newReq.Keys, newKey)
		newReq.Values = append(newReq.Values, newValue)
	}

	return &newReq
}

func TestExecutor(t *testing.T) {
	ctx := context.Background()
	fullAddr := "localhost:9090"
	conn, err := grpc.NewClient(fullAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(644000*300), grpc.MaxCallSendMsgSize(644000*300)))
	if err != nil {
		log.Fatalf("Couldn't Connect to Executor Proxy at localhost:9090. Error: %s", err)
	}
	newClient := waffle_service.NewKeyValueStoreClient(conn)
	//Testing Init DB
	resp, err := newClient.Init(ctx, generateDataSet())
	fmt.Println(resp)
	if err != nil {
		log.Fatalln("Failed to Init DB! Error: ", err)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Printf("Running test: %s \n", tc.name)
			resp, err := newClient.ExecuteMixBatch(ctx, tc.requestBatch)
			if err != nil {
				t.Errorf("ExecuteBatch Error = %v", err)
				return
			}
			if !reflect.DeepEqual(resp.Keys, tc.expected.Keys) || !reflect.DeepEqual(resp.Values, tc.expected.Values) {
				t.Errorf("ExecuteBatch Values not same! \n Expected Keys: %v, Got: %v \n Expected Values: %v, Got: %v \n", tc.expected.Keys, resp.Keys, tc.expected.Values, resp.Values)
			}
		})
	}
}
