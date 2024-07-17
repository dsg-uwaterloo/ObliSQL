package main

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"testing"

	"github.com/Haseeb1399/WorkingThesis/api/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type TestCase struct {
	name         string
	requestQuery *resolver.ParsedQuery
	expectedAns  *resolver.QueryResponse
}

func getLinearKeyNames(start int, end int, tableName string, colName string) []string {
	tempList := make([]string, 0)
	for i := start; i <= end; i++ {
		tempKey := tableName + "/" + colName + "/" + strconv.FormatInt(int64(i), 10)
		tempList = append(tempList, tempKey)
	}
	return tempList
}
func getRepeatedValueList(value string, length int) []string {
	tempList := make([]string, 0)
	for i := 1; i <= length; i++ {
		tempList = append(tempList, value)
	}
	return tempList
}

func getTestCases() []TestCase {
	testCases := []TestCase{
		{

			name: "Mix Range and Point",
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "select",
				TableName:  "review",
				ColToGet:   []string{"rating"},
				SearchCol:  []string{"u_id"},
				SearchVal:  []string{"812"},
				SearchType: []string{"point"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys: []string{
					"review/rating/1529", "review/rating/1529",
				},
				Values: []string{
					"2",
					"0",
				},
			},
		},
	}
	return testCases
}

func TestQueryOne(t *testing.T) {

	resolver_addr := "localhost:9900"
	conn, err := grpc.NewClient(resolver_addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(644000*300), grpc.MaxCallSendMsgSize(644000*300)))
	if err != nil {
		log.Fatalf("Failed to open connection to Resolver")
	}

	resolverClient := resolver.NewResolverClient(conn)
	testcases := getTestCases()

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Println("Starting Test", tc.name)
			resp, err := resolverClient.ExecuteQuery(context.Background(), tc.requestQuery)

			if err != nil {
				t.Errorf("Execute Query Error = %v", err)
			}

			if !reflect.DeepEqual(resp.Keys, tc.expectedAns.Keys) || !reflect.DeepEqual(resp.Values, tc.expectedAns.Values) {
				t.Errorf("Execute Query got incorrect values!")
				fmt.Printf("Expected Keys: % +v \n Got Keys: %+v \n", tc.expectedAns.Values, resp.Values)
			}
		})
	}

}
