package main

import (
	"context"
	"fmt"
	"log"
	"reflect"
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

// func getLinearKeyNames(start int, end int, tableName string, colName string) []string {
// 	tempList := make([]string, 0)
// 	for i := start; i <= end; i++ {
// 		tempKey := tableName + "/" + colName + "/" + strconv.FormatInt(int64(i), 10)
// 		tempList = append(tempList, tempKey)
// 	}
// 	return tempList
// }
// func getRepeatedValueList(value string, length int) []string {
// 	tempList := make([]string, 0)
// 	for i := 1; i <= length; i++ {
// 		tempList = append(tempList, value)
// 	}
// 	return tempList
// }

func getTestCases() []TestCase {
	testCases := []TestCase{
		{

			name: "Simple Select",
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
					"review/rating/1529", "review/rating/4349",
				},
				Values: []string{
					"2",
					"0",
				},
			},
		},
		{

			name: "Select using two filters with index (AND)",
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "select",
				TableName:  "review",
				ColToGet:   []string{"*"},
				SearchCol:  []string{"a_id", "i_id"},
				SearchVal:  []string{"10", "7"},
				SearchType: []string{"point", "point"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys: []string{
					"review/a_id/211", "review/u_id/211", "review/i_id/211", "review/rating/211", "review/rank/211", "review/comment/211", "review/creation_date/211",
				},
				Values: []string{
					"10",
					"238",
					"7",
					"2",
					"nan",
					"test",
					"2020-12-15",
				},
			},
		},
		{
			name: "Simple Select without index",
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "select",
				TableName:  "item",
				ColToGet:   []string{"title"},
				SearchCol:  []string{"i_id"},
				SearchVal:  []string{"500"},
				SearchType: []string{"point"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys: []string{
					"item/title/737",
				},
				Values: []string{
					"?M7s'eFjt#_ll_<;Fjt9yMqNae!Et]]1rF0]lsW@YgM,-}Hpx'CYM.V((:bh1F[/xr@GWs*U8?rNHV^OWHt0P?JnteKvy5r/>ASYo&%##|_fW]J`c~]x>ASYfm3cVE|%",
				},
			},
		},
		{
			name: "Simple Avg Aggregate",
			requestQuery: &resolver.ParsedQuery{
				ClientId:      "1",
				QueryType:     "aggregate",
				TableName:     "review",
				ColToGet:      []string{"rating"},
				SearchCol:     []string{"i_id"},
				SearchVal:     []string{"17"},
				SearchType:    []string{"point"},
				AggregateType: []string{"avg"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys: []string{
					"",
				},
				Values: []string{
					"2.25",
				},
			},
		},
		{
			name: "Simple Sum Aggregate",
			requestQuery: &resolver.ParsedQuery{
				ClientId:      "1",
				QueryType:     "aggregate",
				TableName:     "review",
				ColToGet:      []string{"rating"},
				SearchCol:     []string{"i_id"},
				SearchVal:     []string{"7"},
				SearchType:    []string{"point"},
				AggregateType: []string{"sum"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys: []string{
					"",
				},
				Values: []string{
					"6",
				},
			},
		},
		{
			name: "Simple Count Aggregate",
			requestQuery: &resolver.ParsedQuery{
				ClientId:      "1",
				QueryType:     "aggregate",
				TableName:     "review",
				ColToGet:      []string{"rating"},
				SearchCol:     []string{"i_id"},
				SearchVal:     []string{"7"},
				SearchType:    []string{"point"},
				AggregateType: []string{"count"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys: []string{
					"",
				},
				Values: []string{
					"3",
				},
			},
		},
		{
			name: "Simple Sum & Count Aggregate",
			// select sum(rating) from new_review where i_id =7
			// union all
			// select count(rating)  from new_review where i_id =7;
			requestQuery: &resolver.ParsedQuery{
				ClientId:      "1",
				QueryType:     "aggregate",
				TableName:     "review",
				ColToGet:      []string{"rating", "rating"},
				SearchCol:     []string{"i_id", "i_id"},
				SearchVal:     []string{"7", "7"},
				SearchType:    []string{"point", "point"},
				AggregateType: []string{"sum", "count"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys: []string{
					"", "",
				},
				Values: []string{
					"6",
					"3",
				},
			},
		},
		{
			name: "Simple Range",
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "select",
				TableName:  "review",
				ColToGet:   []string{"rating"},
				SearchCol:  []string{"u_id"},
				SearchVal:  []string{"812", "814"},
				SearchType: []string{"range"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys: []string{
					"review/rating/1529", "review/rating/4349", "review/rating/426", "review/rating/855", "review/rating/3442", "review/rating/4362",
				},
				//By default it's ordered in the way the keys appear in the index.
				Values: []string{
					"2",
					"0",
					"2",
					"1",
					"4",
					"3",
				},
			},
		},
		{
			name: "Date Range",
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "select",
				TableName:  "review",
				ColToGet:   []string{"rating"},
				SearchCol:  []string{"creation_date"},
				SearchVal:  []string{"2021-12-01", "2021-12-02"},
				SearchType: []string{"range"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys: []string{
					"review/rating/1579", "review/rating/2682", "review/rating/643", "review/rating/2032", "review/rating/3245",
				},
				//By default it's ordered in the way the keys appear in the index.
				Values: []string{
					"1",
					"0",
					"3",
					"3",
					"3",
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
				fmt.Printf("Expected Keys: % +v \n Got Keys: %+v \n", tc.expectedAns.Keys, resp.Keys)
				fmt.Printf("Expected Values: % +v \n Got Values: %+v \n", tc.expectedAns.Values, resp.Values)
			}
		})
	}

}
