package benchmark

import (
	"math/rand"

	"github.com/project/ObliSql/api/resolver"
)

func getTestCasesBDB(u_id, i_id, a_id, pageRank_list *[]string, pair_list *[][]string, seedVal int64) []Query {
	source := rand.NewSource(seedVal) // Fixed seed value
	rng := rand.New(source)
	// Define the date range for random date generation
	// startDate := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	// endDate := time.Date(2060, 12, 31, 0, 0, 0, 0, time.UTC)
	testCases := []Query{

		{
			name: "BDB1-Select",
			//Select rating from review where u_id = 812;
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "select",
				TableName:  "rankings",
				ColToGet:   []string{"pageURL", "pageRank"},
				SearchCol:  []string{"pageRank"},
				SearchVal:  []string{(getRandomValue(pageRank_list, rng))},
				SearchType: []string{"point"},
			},
		},
		{
			name: "BDB1-Range",
			//Select rating from review where u_id = 812;
			requestQuery: &resolver.ParsedQuery{
				ClientId:  "1",
				QueryType: "select",
				TableName: "rankings",
				ColToGet:  []string{"pageURL", "pageRank"},
				SearchCol: []string{"pageRank"},
				SearchVal: func() []string {
					start, end := getRandomRangeFromList(pageRank_list, 5)
					return []string{start, end}
				}(),
				SearchType: []string{"range"},
			},
		},
	}
	return testCases
}
