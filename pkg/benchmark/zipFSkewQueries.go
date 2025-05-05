package benchmark

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/project/ObliSql/api/resolver"
)

func dateSkewHelper(zipfDate *rand.Zipf) (string, string) {
	startDate := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	startOffset := int(zipfDate.Uint64())
	rangeLength := getRandomNumber(5)
	start := startDate.AddDate(0, 0, startOffset)
	end := start.AddDate(0, 0, rangeLength)

	return start.Format("2006-01-02"), end.Format("2006-01-02")
}

func rangeHelper(Zip_u_id *rand.Zipf) (string, string) {
	startingPoint := Zip_u_id.Uint64()
	endingPoint := startingPoint + uint64(getRandomNumber(5))

	return strconv.FormatUint(startingPoint, 10), strconv.FormatUint(endingPoint, 10)
}

func getZipfQueries(Zip_u_id, Zip_i_id, Zip_a_id, zipfDate *rand.Zipf) []Query {
	testCases := []Query{
		{

			name: "Simple Select",
			//Select rating from review where u_id = 812;
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "select",
				TableName:  "review",
				ColToGet:   []string{"rating"},
				SearchCol:  []string{"u_id"},
				SearchVal:  []string{strconv.FormatUint(Zip_u_id.Uint64(), 10)},
				SearchType: []string{"point"},
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
				SearchVal:  []string{strconv.FormatUint(Zip_a_id.Uint64(), 10), strconv.FormatUint(Zip_i_id.Uint64(), 10)},
				SearchType: []string{"point", "point"},
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
				SearchCol:  []string{"i_id"},
				SearchVal:  []string{strconv.FormatUint(Zip_i_id.Uint64(), 10)},
				SearchType: []string{"point"},
				OrderBy:    []string{"creation_date,ASC"},
			},
		},
		{

			name: "Select with Order by (Order by included column) - DESC",
			//Select rating from review where u_id = 812;
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "select",
				TableName:  "review",
				ColToGet:   []string{"*"},
				SearchCol:  []string{"i_id"},
				SearchVal:  []string{strconv.FormatUint(Zip_i_id.Uint64(), 10)},
				SearchType: []string{"point"},
				OrderBy:    []string{"creation_date,DESC"},
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
				SearchVal:     []string{strconv.FormatUint(Zip_i_id.Uint64(), 10)},
				SearchType:    []string{"point"},
				AggregateType: []string{"avg"},
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
				SearchVal:     []string{strconv.FormatUint(Zip_i_id.Uint64(), 10)},
				SearchType:    []string{"point"},
				AggregateType: []string{"sum"},
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
				SearchVal:     []string{strconv.FormatUint(Zip_i_id.Uint64(), 10)},
				SearchType:    []string{"point"},
				AggregateType: []string{"count"},
			},
		},
		{
			name: "Sum & Count Aggregate",
			// select sum(rating) from new_review where i_id =7
			// union all
			// select count(rating)  from new_review where i_id =7;
			requestQuery: &resolver.ParsedQuery{
				ClientId:  "1",
				QueryType: "aggregate",
				TableName: "review",
				ColToGet:  []string{"rating", "rating"},
				SearchCol: []string{"i_id", "i_id"},
				SearchVal: func() []string {
					val := strconv.FormatUint(Zip_i_id.Uint64(), 10)
					return []string{val, val}
				}(),
				SearchType:    []string{"point", "point"},
				AggregateType: []string{"sum", "count"},
			},
		},
		{
			name: "Range",
			//select rating from review where u_id between 812 and 814;
			requestQuery: &resolver.ParsedQuery{
				ClientId:  "1",
				QueryType: "select",
				TableName: "review",
				ColToGet:  []string{"rating"},
				SearchCol: []string{"u_id"}, //Change to fetch at most 10 rows 299995
				SearchVal: func() []string {
					start, end := rangeHelper(Zip_u_id)
					return []string{start, end}
				}(),
				SearchType: []string{"range"},
			},
		},
		{
			name: "Date Range",
			//select rating from review between creation_date 2021-12-01 and 2021-12-02
			requestQuery: &resolver.ParsedQuery{
				ClientId:  "1",
				QueryType: "select",
				TableName: "review",
				ColToGet:  []string{"rating"},
				SearchCol: []string{"creation_date"},
				SearchVal: func() []string {
					start, end := dateSkewHelper(zipfDate)
					return []string{start, end}
				}(),
				SearchType: []string{"range"},
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
				SearchVal:   []string{strconv.FormatUint(Zip_i_id.Uint64(), 10)}, //getRandomValue(i_id, rng) 55861
				SearchType:  []string{"point"},
				JoinColumns: []string{"i_id", "i_id"},
				OrderBy:     []string{"review.rating,DESC", "review.creation_date,DESC"},
			},
		},
		{
			name: "Join with two search filters",
			//select review.rating from review,item where review.u_id=trust.target_u_id and review.i_id = ? and trust.source_u_id = ?
			requestQuery: &resolver.ParsedQuery{
				ClientId:    "1",
				QueryType:   "join",
				TableName:   "review,trust", //Make it into a list
				ColToGet:    []string{"review.rating"},
				SearchCol:   []string{"review.i_id", "trust.source_u_id"},
				SearchVal:   []string{strconv.FormatUint(Zip_i_id.Uint64(), 10), strconv.FormatUint(Zip_u_id.Uint64(), 10)},
				SearchType:  []string{"point"},
				JoinColumns: []string{"u_id", "target_u_id"},
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
				SearchVal:     []string{strconv.FormatUint(Zip_i_id.Uint64(), 10), strconv.FormatUint(Zip_u_id.Uint64(), 10)},
				SearchType:    []string{"point"},
				JoinColumns:   []string{"u_id", "target_u_id"},
				AggregateType: []string{"avg"},
			},
		},
		{
			name: "Update using two filters with index (AND) (Update Review)",
			//UPDATE item SET title = ? WHERE i_id=?
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "update",
				TableName:  "review",
				ColToGet:   []string{"title"},
				SearchCol:  []string{"i_id"},
				SearchVal:  []string{strconv.FormatUint(Zip_i_id.Uint64(), 10)},
				SearchType: []string{"point"},
				UpdateVal:  []string{"This is the new title"},
			},
		},
		{
			name: "Update using two filters with index (AND) (Update Review Rating)",
			//UPDATE review SET rating = ? WHERE i_id=? AND u_id=?
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "update",
				TableName:  "review",
				ColToGet:   []string{"rating"},
				SearchCol:  []string{"i_id", "u_id"},
				SearchVal:  []string{strconv.FormatUint(Zip_i_id.Uint64(), 10), strconv.FormatUint(Zip_u_id.Uint64(), 10)},
				SearchType: []string{"point", "point"},
				UpdateVal:  []string{"6"}, //Adding a new Rating just to make sure it isn't something from the DB
			},
		},
		{
			name: "Update using two filters with index (AND) (Update Trust)",
			//UPDATE trust SET trust = ? WHERE source_u_id=? AND target_u_id=?
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "update",
				TableName:  "trust",
				ColToGet:   []string{"trust"},
				SearchCol:  []string{"source_u_id", "target_u_id"},
				SearchVal:  []string{strconv.FormatUint(Zip_u_id.Uint64(), 10), strconv.FormatUint(Zip_u_id.Uint64(), 10)}, //using u_id since source/trust u_id reference user IDS.
				SearchType: []string{"point", "point"},
				UpdateVal:  []string{"10"},
			},
		},
		{
			name: "Update Item Description",
			//UPDATE trust SET trust = ? WHERE source_u_id=? AND target_u_id=?
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "update",
				TableName:  "item",
				ColToGet:   []string{"description"},
				SearchCol:  []string{"i_id"},
				SearchVal:  []string{strconv.FormatUint(Zip_i_id.Uint64(), 10)}, //using u_id since source/trust u_id reference user IDS.
				SearchType: []string{"point"},
				UpdateVal:  []string{"new-title"},
			},
		},
	}
	return testCases
}
