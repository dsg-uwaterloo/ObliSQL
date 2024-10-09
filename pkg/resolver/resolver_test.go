package main

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/Haseeb1399/WorkingThesis/api/resolver"
	"golang.org/x/exp/rand"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type TestCase struct {
	name         string
	requestQuery *resolver.ParsedQuery
	expectedAns  *resolver.QueryResponse
}

func sortKeysAndValues(keys []string, values []string) ([]string, []string) {
	type keyValue struct {
		key   string
		value string
	}

	pairs := make([]keyValue, len(keys))
	for i := range keys {
		pairs[i] = keyValue{keys[i], values[i]}
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].key < pairs[j].key
	})

	sortedKeys := make([]string, len(keys))
	sortedValues := make([]string, len(values))
	for i, pair := range pairs {
		sortedKeys[i] = pair.key
		sortedValues[i] = pair.value
	}

	return sortedKeys, sortedValues
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var seededRand = rand.New(rand.NewSource(uint64(time.Now().UnixNano())))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
func sampleTestCases(testcases []TestCase, sampleSize int) []TestCase {
	rand.Seed(uint64(time.Now().UnixNano()))
	rand.Shuffle(len(testcases), func(i, j int) { testcases[i], testcases[j] = testcases[j], testcases[i] })
	if sampleSize > len(testcases) {
		sampleSize = len(testcases)
	}
	return testcases[:sampleSize]
}

func runTestCase(tc TestCase, resolverClient resolver.ResolverClient, wg *sync.WaitGroup) {
	defer wg.Done()

	fmt.Println("Starting Test", tc.name)
	resp, err := resolverClient.ExecuteQuery(context.Background(), tc.requestQuery)

	if err != nil {
		log.Printf("Test %s: Execute Query Error = %v\n", tc.name, err)
		return
	}
	sortedRespKeys, sortedRespValues := sortKeysAndValues(resp.Keys, resp.Values)
	sortedExpKeys, sortedExpValues := sortKeysAndValues(tc.expectedAns.Keys, tc.expectedAns.Values)

	if !reflect.DeepEqual(sortedRespKeys, sortedExpKeys) || !reflect.DeepEqual(sortedRespValues, sortedExpValues) {
		log.Printf("Test %s: Execute Query got incorrect values!\n", tc.name)
		log.Printf("Expected Keys: %+v \n Got Keys: %+v \n", sortedExpKeys, sortedRespKeys)
		log.Printf("Expected Values: %+v \n Got Values: %+v \n", sortedExpValues, sortedRespValues)
	} else {
		fmt.Println("Finished", tc.name)
	}
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

func getTestCasesWithInserts() []TestCase {
	testCases := []TestCase{
		{

			name: "Simple Select",
			//Select rating from review where u_id = 812;
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
			//Select * from review where a_id =10 and i_id = 7;
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
					"start_value",
					"2020-12-15",
				},
			},
		},
		{
			name: "Update using two filters with index (AND)",
			//Update review set comment = "This is the new comment" where a_id =10 and i_id = 7;
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "update",
				TableName:  "review",
				ColToGet:   []string{"rank", "comment"},
				SearchCol:  []string{"a_id", "i_id"},
				SearchVal:  []string{"10", "7"},
				SearchType: []string{"point", "point"},
				UpdateVal:  []string{"1", "This is the new comment"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys: []string{
					"review/rank/211",
					"review/comment/211",
				},
				Values: []string{
					"1",
					"This is the new comment",
				},
			},
		},
		{
			name: "Select without index",
			//Select title from item where i_id = 500;
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
			name: "Update without index",
			//Select title from item where i_id = 500;
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "update",
				TableName:  "item",
				ColToGet:   []string{"title"},
				SearchCol:  []string{"i_id"},
				SearchVal:  []string{"500"},
				SearchType: []string{"point"},
				UpdateVal:  []string{"New Title for testing!"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys: []string{
					"item/title/737",
				},
				Values: []string{
					"New Title for testing!",
				},
			},
		},
		{

			name: "Select with Order by (Order by included column) - ASC",
			//Select rating from review where u_id = 812;
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "select",
				TableName:  "review",
				ColToGet:   []string{"rating", "creation_date"},
				SearchCol:  []string{"u_id"},
				SearchVal:  []string{"812"},
				SearchType: []string{"point"},
				OrderBy:    []string{"creation_date,ASC"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys: []string{
					"review/creation_date/4349", "review/rating/4349", "review/creation_date/1529", "review/rating/1529",
				},
				Values: []string{
					"2020-12-20",
					"0",
					"2021-03-22",
					"2",
				},
			},
		},
		{

			name: "Select with Order by (Order by included column) - DESC",
			//Select rating from review where u_id = 812;
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "select",
				TableName:  "review",
				ColToGet:   []string{"rating", "creation_date"},
				SearchCol:  []string{"u_id"},
				SearchVal:  []string{"812"},
				SearchType: []string{"point"},
				OrderBy:    []string{"creation_date,DESC"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys: []string{
					"review/creation_date/1529", "review/rating/1529", "review/creation_date/4349", "review/rating/4349",
				},
				Values: []string{
					"2021-03-22",
					"2",
					"2020-12-20",
					"0",
				},
			},
		},
		{
			name: "Avg Aggregate",
			//Select avg(rating) from review where i_id = 17;
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
			name: "Sum Aggregate",
			//select sum(rating) from review where i_id = 7;
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
			name: "Count Aggregate",
			//select count(rating) from review where i_id = 7;
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
			name: "Sum & Count Aggregate",
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
			name: "Range",
			//select rating from review where u_id between 812 and 814;
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
			//select rating from review between creation_date 2021-12-01 and 2021-12-02
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
		{
			name: "Cross Join",
			//select review.rating,item.title from review,item where item.i_id=r.i_id and r.i_id = 17;
			//More verbose documentation in the resolver that defines how I am returning the join.
			requestQuery: &resolver.ParsedQuery{
				ClientId:    "1",
				QueryType:   "join",
				TableName:   "review,item", //Make it into a list
				ColToGet:    []string{"review.rating", "item.title"},
				SearchCol:   []string{"review.i_id"},
				SearchVal:   []string{"17"},
				SearchType:  []string{"point"},
				JoinColumns: []string{"i_id", "i_id"},
				OrderBy:     []string{"review.rating,DESC", "review.creation_date,DESC"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys:   []string{"review/rating/254", "item/title/146", "review/rating/255", "item/title/146", "review/rating/256", "item/title/146", "review/rating/257", "item/title/146"},
				Values: []string{"2", "test", "3", "test", "1", "test", "3", "test"},
			},
		},
		{
			name: "Join with two search filters",
			//select review.rating from review,item where review.u_id=target.target_u_id and r.i_id = ? and t.source_u_id = ?
			requestQuery: &resolver.ParsedQuery{
				ClientId:    "1",
				QueryType:   "join",
				TableName:   "review,trust", //Make it into a list
				ColToGet:    []string{"review.rating"},
				SearchCol:   []string{"review.i_id", "trust.source_u_id"},
				SearchVal:   []string{"43", "1030"},
				SearchType:  []string{"point"},
				JoinColumns: []string{"u_id", "target_u_id"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys:   []string{"review/rating/1421", "review/rating/1430", "review/rating/1464", "review/rating/1722", "review/rating/1717", "review/rating/1706", "review/rating/1729", "review/rating/1713"},
				Values: []string{"4", "0", "2", "0", "0", "4", "2", "4"},
			},
		},
		{
			name: "Join Aggregate with two search filters",
			//select avg(review.rating) from review,item where review.u_id=target.target_u_id and r.i_id = ? and t.source_u_id = ?
			requestQuery: &resolver.ParsedQuery{
				ClientId:      "1",
				QueryType:     "aggregate",
				TableName:     "review,trust", //Make it into a list
				ColToGet:      []string{"review.rating"},
				SearchCol:     []string{"review.i_id", "trust.source_u_id"},
				SearchVal:     []string{"43", "1030"},
				SearchType:    []string{"point"},
				JoinColumns:   []string{"u_id", "target_u_id"},
				AggregateType: []string{"avg"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys:   []string{""},
				Values: []string{"2"},
			},
		},
		//Test to ensure that the update actually happened (Done at the end to reduce chances of value returning from cache)
		{

			name: "Select using two filters -- Checking Update",
			//Select * from review where a_id =10 and i_id = 7;
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "select",
				TableName:  "review",
				ColToGet:   []string{"rank", "comment"},
				SearchCol:  []string{"a_id", "i_id"},
				SearchVal:  []string{"10", "7"},
				SearchType: []string{"point", "point"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys: []string{
					"review/rank/211",
					"review/comment/211",
				},
				Values: []string{
					"1",
					"This is the new comment",
				},
			},
		},
		{
			name: "Select without index -- Checking Update",
			//Select title from item where i_id = 500;
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
					"New Title for testing!",
				},
			},
		},
	}
	return testCases
}

func getTestCases() []TestCase {
	testCases := []TestCase{
		{

			name: "Simple Select",
			//Select rating from review where u_id = 812;
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
			//Select * from review where a_id =10 and i_id = 7;
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
					"start_value",
					"2020-12-15",
				},
			},
		},
		{
			name: "Select without index",
			//Select title from item where i_id = 500;
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

			name: "Select with Order by (Order by included column) - ASC",
			//Select rating from review where u_id = 812;
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "select",
				TableName:  "review",
				ColToGet:   []string{"rating", "creation_date"},
				SearchCol:  []string{"u_id"},
				SearchVal:  []string{"812"},
				SearchType: []string{"point"},
				OrderBy:    []string{"creation_date,ASC"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys: []string{
					"review/creation_date/4349", "review/rating/4349", "review/creation_date/1529", "review/rating/1529",
				},
				Values: []string{
					"2020-12-20",
					"0",
					"2021-03-22",
					"2",
				},
			},
		},
		{

			name: "Select with Order by (Order by included column) - DESC",
			//Select rating from review where u_id = 812;
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "select",
				TableName:  "review",
				ColToGet:   []string{"rating", "creation_date"},
				SearchCol:  []string{"u_id"},
				SearchVal:  []string{"812"},
				SearchType: []string{"point"},
				OrderBy:    []string{"creation_date,DESC"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys: []string{
					"review/creation_date/1529", "review/rating/1529", "review/creation_date/4349", "review/rating/4349",
				},
				Values: []string{
					"2021-03-22",
					"2",
					"2020-12-20",
					"0",
				},
			},
		},
		{
			name: "Avg Aggregate",
			//Select avg(rating) from review where i_id = 17;
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
			name: "Sum Aggregate",
			//select sum(rating) from review where i_id = 7;
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
			name: "Count Aggregate",
			//select count(rating) from review where i_id = 7;
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
			name: "Sum & Count Aggregate",
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
			name: "Range",
			//select rating from review where u_id between 812 and 814;
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
			//select rating from review between creation_date 2021-12-01 and 2021-12-02
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
		{
			name: "Cross Join",
			//select review.rating,item.title from review,item where item.i_id=r.i_id and r.i_id = 17;
			//More verbose documentation in the resolver that defines how I am returning the join.
			requestQuery: &resolver.ParsedQuery{
				ClientId:    "1",
				QueryType:   "join",
				TableName:   "review,item", //Make it into a list
				ColToGet:    []string{"review.rating", "item.title"},
				SearchCol:   []string{"review.i_id"},
				SearchVal:   []string{"17"},
				SearchType:  []string{"point"},
				JoinColumns: []string{"i_id", "i_id"},
				OrderBy:     []string{"review.rating,DESC", "review.creation_date,DESC"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys:   []string{"review/rating/254", "item/title/146", "review/rating/255", "item/title/146", "review/rating/256", "item/title/146", "review/rating/257", "item/title/146"},
				Values: []string{"2", "test", "3", "test", "1", "test", "3", "test"},
			},
		},
		{
			name: "Join with two search filters",
			//select review.rating from review,item where review.u_id=target.target_u_id and r.i_id = ? and t.source_u_id = ?
			requestQuery: &resolver.ParsedQuery{
				ClientId:    "1",
				QueryType:   "join",
				TableName:   "review,trust", //Make it into a list
				ColToGet:    []string{"review.rating"},
				SearchCol:   []string{"review.i_id", "trust.source_u_id"},
				SearchVal:   []string{"43", "1030"},
				SearchType:  []string{"point"},
				JoinColumns: []string{"u_id", "target_u_id"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys:   []string{"review/rating/1421", "review/rating/1430", "review/rating/1464", "review/rating/1722", "review/rating/1717", "review/rating/1706", "review/rating/1729", "review/rating/1713"},
				Values: []string{"4", "0", "2", "0", "0", "4", "2", "4"},
			},
		},
		{
			name: "Join Aggregate with two search filters",
			//select avg(review.rating) from review,item where review.u_id=target.target_u_id and r.i_id = ? and t.source_u_id = ?
			requestQuery: &resolver.ParsedQuery{
				ClientId:      "1",
				QueryType:     "aggregate",
				TableName:     "review,trust", //Make it into a list
				ColToGet:      []string{"review.rating"},
				SearchCol:     []string{"review.i_id", "trust.source_u_id"},
				SearchVal:     []string{"43", "1030"},
				SearchType:    []string{"point"},
				JoinColumns:   []string{"u_id", "target_u_id"},
				AggregateType: []string{"avg"},
			},
			expectedAns: &resolver.QueryResponse{
				Keys:   []string{""},
				Values: []string{"2"},
			},
		},
	}
	return testCases
}

func TestSelectSequential(t *testing.T) {
	fmt.Println("-------------------------------")
	fmt.Println("Testing Sequential Select")
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
			// fmt.Println("-----")
			// fmt.Println(resp.Keys)
			// fmt.Println("-----")
			// fmt.Println(resp.Values)
			// fmt.Println("-----")

			if err != nil {
				t.Errorf("Execute Query Error = %v", err)
			}
			sortedRespKeys, sortedRespValues := sortKeysAndValues(resp.Keys, resp.Values)
			sortedExpKeys, sortedExpValues := sortKeysAndValues(tc.expectedAns.Keys, tc.expectedAns.Values)

			if !reflect.DeepEqual(sortedRespKeys, sortedExpKeys) || !reflect.DeepEqual(sortedRespValues, sortedExpValues) {
				t.Errorf("Execute Query got incorrect values!")
				fmt.Printf("Expected Keys: % +v \n Got Keys: %+v \n", sortedExpKeys, sortedRespKeys)
				fmt.Printf("Expected Values: % +v \n Got Values: %+v \n", sortedExpValues, sortedRespValues)
			}
		})
	}

}

func TestSelectParallel(t *testing.T) {
	fmt.Println("-------------------------------")
	fmt.Println("Testing Parallel Select")
	resolverAddr := "localhost:9900"
	conn, err := grpc.NewClient(resolverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(644000*300), grpc.MaxCallSendMsgSize(644000*300)))
	if err != nil {
		log.Fatalf("Failed to open connection to Resolver: %v", err)
	}
	defer conn.Close()

	resolverClient := resolver.NewResolverClient(conn)
	testcases := getTestCases()

	sampleSize := 20 // Adjust the sample size as needed
	sampledTestCases := sampleTestCases(testcases, sampleSize)

	var wg sync.WaitGroup

	for _, tc := range sampledTestCases {
		wg.Add(1)
		go runTestCase(tc, resolverClient, &wg)
	}

	wg.Wait()
}

// func TestMixSequential(t *testing.T) {
// 	fmt.Println("-------------------------------")
// 	fmt.Println("Testing Sequential Select and Update")
// 	resolver_addr := "localhost:9900"
// 	conn, err := grpc.NewClient(resolver_addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(644000*300), grpc.MaxCallSendMsgSize(644000*300)))
// 	if err != nil {
// 		log.Fatalf("Failed to open connection to Resolver")
// 	}

// 	resolverClient := resolver.NewResolverClient(conn)
// 	testcases := getTestCasesWithInserts()

// 	for _, tc := range testcases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			fmt.Println("Starting Test", tc.name)
// 			resp, err := resolverClient.ExecuteQuery(context.Background(), tc.requestQuery)
// 			// fmt.Println("-----")
// 			// fmt.Println(resp.Keys)
// 			// fmt.Println("-----")
// 			// fmt.Println(resp.Values)
// 			// fmt.Println("-----")

// 			if err != nil {
// 				t.Errorf("Execute Query Error = %v", err)
// 			}
// 			sortedRespKeys, sortedRespValues := sortKeysAndValues(resp.Keys, resp.Values)
// 			sortedExpKeys, sortedExpValues := sortKeysAndValues(tc.expectedAns.Keys, tc.expectedAns.Values)

// 			if !reflect.DeepEqual(sortedRespKeys, sortedExpKeys) || !reflect.DeepEqual(sortedRespValues, sortedExpValues) {
// 				t.Errorf("Execute Query got incorrect values!")
// 				fmt.Printf("Expected Keys: % +v \n Got Keys: %+v \n", sortedExpKeys, sortedRespKeys)
// 				fmt.Printf("Expected Values: % +v \n Got Values: %+v \n", sortedExpValues, sortedRespValues)
// 			}
// 		})
// 	}

// }

// func TestUpdateSerializability(t *testing.T) {

// 	resolver_addr := "localhost:9900"
// 	conn, err := grpc.NewClient(resolver_addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(644000*300), grpc.MaxCallSendMsgSize(644000*300)))
// 	if err != nil {
// 		log.Fatalf("Failed to open connection to Resolver")
// 	}

// 	resolverClient := resolver.NewResolverClient(conn)

// 	numUpdates := 5
// 	// Channel to synchronize updates
// 	var wg sync.WaitGroup

// 	for i := 1; i <= numUpdates; i++ {
// 		wg.Add(1)
// 		testString := generateRandomString(10)
// 		go func(requestId string, client resolver.ResolverClient) {
// 			defer wg.Done()
// 			// Create an update query
// 			var queryType string
// 			if rand.Intn(2) == 0 {
// 				queryType = "update"
// 			} else {
// 				queryType = "select"
// 			}
// 			var query *resolver.ParsedQuery
// 			if queryType == "update" {
// 				query = &resolver.ParsedQuery{
// 					ClientId:   "1",
// 					QueryType:  "update",
// 					TableName:  "review",
// 					ColToGet:   []string{"comment"},
// 					SearchCol:  []string{"a_id", "i_id"},
// 					SearchVal:  []string{"10", "7"},
// 					SearchType: []string{"point", "point"},
// 					UpdateVal:  []string{testString},
// 				}
// 			} else {
// 				query = &resolver.ParsedQuery{
// 					ClientId:   "1",
// 					QueryType:  "select",
// 					TableName:  "review",
// 					ColToGet:   []string{"comment"},
// 					SearchCol:  []string{"a_id", "i_id"},
// 					SearchVal:  []string{"10", "7"},
// 					SearchType: []string{"point", "point"},
// 				}
// 			}

// 			// Execute the query
// 			resp, err := client.ExecuteQuery(context.Background(), query)
// 			if err != nil {
// 				log.Printf("Error executing %s. %v", queryType, err)
// 			} else {
// 				fmt.Println("---------")
// 				fmt.Println(resp.RequestId, queryType, resp.Keys, resp.Values)
// 				fmt.Println("---------")
// 			}
// 		}(testString, resolverClient)
// 	}

// 	// Wait for all updates to complete
// 	wg.Wait()
// }
