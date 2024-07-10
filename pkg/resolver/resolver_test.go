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
			//Select c_balance,c_state,c_since
			//from customer
			//where c_since between 774 and 779
			//and c_state = ke

			name: "Mix Range and Point",
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "select",
				TableName:  "customer",
				ColToGet:   []string{"c_balance", "c_state", "c_since"},
				SearchCol:  []string{"c_since", "c_state"},
				SearchVal:  []string{"1711607656774", "1711607656779", "ke"},
				SearchType: []string{"range", "point"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys: []string{
					"customer/c_balance/9762", "customer/c_state/9762", "customer/c_since/9762",
					"customer/c_balance/10002", "customer/c_state/10002", "customer/c_since/10002",
					"customer/c_balance/12592", "customer/c_state/12592", "customer/c_since/12592",
				},
				Values: []string{
					"-10.00", "ke", "1711607656774",
					"-10.00", "ke", "1711607656774",
					"-10.00", "ke", "1711607656779",
				},
			},
		},
		{
			//Select c_balance,c_since
			//from customer
			//where c_since < 775

			name: "Less than Range Test",
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "select",
				TableName:  "customer",
				ColToGet:   []string{"c_balance"},
				SearchCol:  []string{"c_since"},
				SearchVal:  []string{"<", "1711607656758"},
				SearchType: []string{"range"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys:   getLinearKeyNames(1, 438, "customer", "c_balance"),
				Values: getRepeatedValueList("-10.00", 438),
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
