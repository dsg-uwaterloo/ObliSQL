package benchmark

import (
	"math/rand"

	"github.com/project/ObliSql/api/resolver"
)

func getTestCasesBDB(u_id, i_id, a_id, pageRank_list *[]string, pair_list *[][]string, seedVal int64) []Query {
	source := rand.NewSource(seedVal) // Fixed seed value
	rng := rand.New(source)

	// Generate a random number to decide the query type
	isJoinQuery := rng.Float64() < 0.05 // 5% probability for BDB3-Join
	endingPoints := []string{"1980-01-02", "1980-01-03", "1980-01-04", "1980-01-05"}

	if isJoinQuery {
		return []Query{
			{
				name: "BDB3-Join",
				requestQuery: &resolver.ParsedQuery{
					ClientId:  "1",
					QueryType: "bdb3",
					TableName: "rankings,uservisits",
					ColToGet:  []string{"uservisits.sourceIP", "uservisits.adRevenue", "rankings.pageRank"},
					SearchCol: []string{"uservisits.visitDate"},
					SearchVal: func() []string {
						start, end := "1980-01-01", endingPoints[rng.Intn(len(endingPoints))]
						return []string{start, end}
					}(),
					SearchType:  []string{"range"},
					JoinColumns: []string{"pageURL", "destURL"},
				},
			},
		}
	}

	// Return other queries if not BDB3-Join
	testCases := []Query{
		{
			name: "BDB1-Select",
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
			name: "BDB2-Range",
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
