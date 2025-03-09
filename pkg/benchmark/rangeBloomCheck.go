package benchmark

import "github.com/project/ObliSql/api/resolver"

func getTestRangeBloom(rangeSize int) []Query {
	return []Query{
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
					start, end := getRandomRangeScaling(0, 299995, rangeSize)
					return []string{start, end}
				}(),
				SearchType: []string{"range"},
			},
		},
	}
}
