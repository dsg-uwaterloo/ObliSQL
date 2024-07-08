package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"reflect"
	"testing"
	"time"

	executor "github.com/Haseeb1399/WorkingThesis/api/executor"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

var testCases = []TestCase{
	{
		name: "Valid Get Request",
		requestBatch: &executor.RequestBatch{
			RequestId: 2,
			Keys:      []string{"K1", "K2", "K3"},
			Values:    []string{"", "", ""},
		},
		expected: &executor.RespondBatch{
			RequestId: 2,
			Keys:      []string{"K1", "K2", "K3"},
			Values:    []string{"V1", "V2", "V3"},
		},
	},
	{
		name: "Get Non-existant Value",
		requestBatch: &executor.RequestBatch{
			RequestId: 3,
			Keys:      []string{"K100", "K200", "K10000"},
			Values:    []string{"", "", ""},
		},
		expected: &executor.RespondBatch{
			RequestId: 3,
			Keys:      []string{"K100", "K200", "K10000"},
			Values:    []string{"V100", "V200", "-1"},
		},
	},
	{
		name: "Normal Put Request",
		requestBatch: &executor.RequestBatch{
			RequestId: 4,
			Keys:      []string{"K1", "K2", "K3"},
			Values:    []string{"VT1", "VT2", "VT3"},
		},
		expected: &executor.RespondBatch{
			RequestId: 4,
			Keys:      []string{"K1", "K2", "K3"},
			Values:    []string{"VT1", "VT2", "VT3"},
		},
	},
	{
		name: "Get & Puts in One",
		requestBatch: &executor.RequestBatch{
			RequestId: 5,
			Keys:      []string{"K1", "K2", "K1", "K3", "K1", "K1", "K1"},
			Values:    []string{"", "", "Test", "VT3", "", "Final", ""},
		},
		expected: &executor.RespondBatch{
			RequestId: 5,
			Keys:      []string{"K1", "K2", "K1", "K3", "K1", "K1", "K1"},
			Values:    []string{"VT1", "VT2", "Test", "VT3", "Test", "Final", "Final"},
		},
	},
	{
		name: "Get & Puts With Missing Key",
		requestBatch: &executor.RequestBatch{
			RequestId: 6,
			Keys:      []string{"K1", "K2", "K1", "k999999", "K3", "K1", "K1", "K1"},
			Values:    []string{"", "", "Test", "", "VT3", "", "Final", ""},
		},
		expected: &executor.RespondBatch{
			RequestId: 6,
			Keys:      []string{"K1", "K2", "K1", "k999999", "K3", "K1", "K1", "K1"},
			Values:    []string{"Final", "VT2", "Test", "-1", "VT3", "Test", "Final", "Final"},
		},
	},
	{
		name: "Missing Key, then Inserted",
		requestBatch: &executor.RequestBatch{
			RequestId: 7,
			Keys:      []string{"K999999", "K999999", "K999999"},
			Values:    []string{"", "Hello", ""},
		},
		expected: &executor.RespondBatch{
			RequestId: 7,
			Keys:      []string{"K999999", "K999999", "K999999"},
			Values:    []string{"-1", "Hello", "Hello"},
		},
	},
}

type TestCase struct {
	name         string
	requestBatch *executor.RequestBatch
	expected     *executor.RespondBatch
}

func generateDataSet() *executor.RequestBatch {
	newReq := executor.RequestBatch{
		RequestId: 1,
		Keys:      make([]string, 0),
		Values:    make([]string, 0),
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
	lis, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatalf("Cannot create listener on port :9090 %s", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	service := myExecutor{rdb: rdb}
	serverRegister := grpc.NewServer(grpc.MaxRecvMsgSize(644000*300), grpc.MaxSendMsgSize(644000*300))
	executor.RegisterExecutorServer(serverRegister, service)

	go func() {
		err = serverRegister.Serve(lis)
		if err != nil {
			log.Fatalf("Error! Could not start server %s", err)
		}
	}()

	time.Sleep(2 * time.Second)

	//Testing Init DB
	initSuccess, err := service.InitDb(ctx, generateDataSet())
	if err != nil {
		log.Fatalln("Failed to Init DB! Error: ", err)
	}

	if !initSuccess.Value {
		log.Fatalln("Init DB returned false! Failed to init DB")
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := service.ExecuteBatch(ctx, tc.requestBatch)
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
