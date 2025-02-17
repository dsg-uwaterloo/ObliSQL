package benchmark

import (
	"math/rand"

	"github.com/project/ObliSql/api/resolver"
)

func getTestCasesEpinion(u_id, i_id, a_id, pageRank_list *[]string, pair_list *[][]string, seedVal int64) []Query {
	source := rand.NewSource(seedVal) // Fixed seed value
	rng := rand.New(source)
	// Define the date range for random date generation
	// startDate := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	// endDate := time.Date(2060, 12, 31, 0, 0, 0, 0, time.UTC)
	testCases := []Query{

		{

			name: "Select with Order by (Order by included column) - DESC",
			//Select rating from review where i_id = 812;
			//Epinions-3
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "select",
				TableName:  "review",
				ColToGet:   []string{"*"},
				SearchCol:  []string{"i_id"},
				SearchVal:  []string{getRandomValue(i_id, rng)},
				SearchType: []string{"point"},
				OrderBy:    []string{"creation_date,DESC"},
			},
		},
		{
			name: "Avg Aggregate",
			// Epinions-2
			//Select avg(rating) from review where i_id = 17;
			requestQuery: &resolver.ParsedQuery{
				ClientId:      "1",
				QueryType:     "aggregate",
				TableName:     "review",
				ColToGet:      []string{"rating"},
				SearchCol:     []string{"i_id"},
				SearchVal:     []string{getRandomValue(i_id, rng)},
				SearchType:    []string{"point"},
				AggregateType: []string{"avg"},
			},
		},
		{
			name: "Cross Join",
			//Epinion-7
			//select review.rating,item.title from review,item where item.i_id=r.i_id and r.i_id = 17;
			//More verbose documentation in the resolver that defines how I am returning the join.
			requestQuery: &resolver.ParsedQuery{
				ClientId:    "1",
				QueryType:   "join",
				TableName:   "review,item", //Make it into a list
				ColToGet:    []string{"review.*", "item.*"},
				SearchCol:   []string{"review.i_id"},
				SearchVal:   []string{getRandomValue(i_id, rng)}, //getRandomValue(i_id, rng) 55861
				SearchType:  []string{"point"},
				JoinColumns: []string{"i_id", "i_id"},
				OrderBy:     []string{"review.rating,DESC", "review.creation_date,DESC"},
			},
		},
		{
			name: "Join Aggregate with two search filters",
			//Epinions - 1
			//select avg(review.rating) from review,item where review.u_id=target.target_u_id and r.i_id = ? and t.source_u_id = ?
			requestQuery: &resolver.ParsedQuery{
				ClientId:      "1",
				QueryType:     "aggregate",
				TableName:     "review,trust", //Make it into a list
				ColToGet:      []string{"review.rating"},
				SearchCol:     []string{"review.i_id", "trust.source_u_id"},
				SearchVal:     []string{getRandomValue(i_id, rng), getRandomValue(u_id, rng)},
				SearchType:    []string{"point"},
				JoinColumns:   []string{"u_id", "target_u_id"},
				AggregateType: []string{"avg"},
			},
		},
		{
			name: "Update using two filters with index (AND) (Update Review)",
			//Epinion-4
			//UPDATE item SET title = ? WHERE i_id=?
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "update",
				TableName:  "review",
				ColToGet:   []string{"title"},
				SearchCol:  []string{"i_id"},
				SearchVal:  []string{getRandomValue(i_id, rng)},
				SearchType: []string{"point"},
				UpdateVal:  []string{"This is the new title"},
			},
		},
		{
			name: "Update using two filters with index (AND) (Update Review Rating)",
			//Epinions-5
			//UPDATE review SET rating = ? WHERE i_id=? AND u_id=?
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "update",
				TableName:  "review",
				ColToGet:   []string{"rating"},
				SearchCol:  []string{"i_id", "u_id"},
				SearchVal:  []string{getRandomValue(i_id, rng), getRandomValue(u_id, rng)},
				SearchType: []string{"point", "point"},
				UpdateVal:  []string{"6"}, //Adding a new Rating just to make sure it isn't something from the DB
			},
		},
		{
			name: "Update using two filters with index (AND) (Update Trust)",
			//UPDATE trust SET trust = ? WHERE source_u_id=? AND target_u_id=?
			//Epinions-6
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "update",
				TableName:  "trust",
				ColToGet:   []string{"trust"},
				SearchCol:  []string{"source_u_id", "target_u_id"},
				SearchVal:  []string{getRandomValue(u_id, rng), getRandomValue(u_id, rng)}, //using u_id since source/trust u_id reference user IDS.
				SearchType: []string{"point", "point"},
				UpdateVal:  []string{"10"},
			},
		},
	}
	return testCases
}
