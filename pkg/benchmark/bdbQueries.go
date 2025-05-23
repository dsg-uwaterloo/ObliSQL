package benchmark

import (
	"math/rand"
	"sync/atomic"

	"github.com/project/ObliSql/api/resolver"
)

// Global counter for BDB calls
var bdbCallCounter int64 = 0

func getTestCasesBDB(u_id, i_id, a_id, pageRank_list *[]string, pair_list *[][]string, seedVal int64) Query {
	// Increment counter atomically (thread-safe)
	callNum := atomic.AddInt64(&bdbCallCounter, 1)

	// Every 20th call returns a join query (5% = 1/20)
	isJoinQuery := (callNum % 20) == 0

	// Use the call number in the seed to ensure different random values per call
	source := rand.NewSource(seedVal + callNum)
	rng := rand.New(source)

	if isJoinQuery {
		// Return BDB3-Join query
		endingPoints := []string{"1980-01-02", "1980-01-03", "1980-01-04", "1980-01-05"}
		return Query{
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
		}
	}

	// For non-join queries, randomly choose between BDB1-Select and BDB2-Range
	if rng.Float64() < 0.5 {
		// Return BDB1-Select query
		return Query{
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
		}
	} else {
		// Return BDB2-Range query
		return Query{
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
		}
	}
}

// Optional: Reset counter function if you need to reset between different benchmark runs
func ResetBDBCallCounter() {
	atomic.StoreInt64(&bdbCallCounter, 0)
}
