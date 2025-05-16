package benchmark

import (
	"github.com/project/ObliSql/api/resolver"
)

func getTestCasesBDB(u_id, i_id, a_id, pageRank_list *[]string, pair_list *[][]string, seedVal int64) []Query {
	// source := rand.NewSource(seedVal) // Fixed seed value
	// rng := rand.New(source)
	// Define the date range for random date generation
	// startDate := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	// endDate := time.Date(2060, 12, 31, 0, 0, 0, 0, time.UTC)
	testCases := []Query{

		// {
		// 	name: "BDB1-Select",
		// 	requestQuery: &resolver.ParsedQuery{
		// 		ClientId:   "1",
		// 		QueryType:  "select",
		// 		TableName:  "rankings",
		// 		ColToGet:   []string{"pageURL", "pageRank"},
		// 		SearchCol:  []string{"pageRank"},
		// 		SearchVal:  []string{(getRandomValue(pageRank_list, rng))},
		// 		SearchType: []string{"point"},
		// 	},
		// },
		// {
		// 	name: "BDB2-Range",
		// 	requestQuery: &resolver.ParsedQuery{
		// 		ClientId:  "1",
		// 		QueryType: "select",
		// 		TableName: "rankings",
		// 		ColToGet:  []string{"pageURL", "pageRank"},
		// 		SearchCol: []string{"pageRank"},
		// 		SearchVal: func() []string {
		// 			start, end := getRandomRangeFromList(pageRank_list, 5)
		// 			return []string{start, end}
		// 		}(),
		// 		SearchType: []string{"range"},
		// 	},
		// },
		{
			name: "BDB3-Join",
			requestQuery: &resolver.ParsedQuery{
				ClientId:  "1",
				QueryType: "bdb3",
				TableName: "rankings,uservisits",
				ColToGet:  []string{"uservisits.sourceIP", "uservisits.adRevenue", "rankings.pageRank"},
				SearchCol: []string{"uservisits.visitDate"},
				SearchVal: func() []string {
					start, end := "1980-01-01", "1980-01-01"
					return []string{start, end}
				}(),
				SearchType:  []string{"range"},
				JoinColumns: []string{"pageURL", "destURL"},
			},
		},
	}
	return testCases
}
