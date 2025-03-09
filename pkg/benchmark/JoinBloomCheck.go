package benchmark

import "github.com/project/ObliSql/api/resolver"

func getTestJoinBloom(pair_list *[][]string) []Query {
	return []Query{
		{
			name: "Cross Join - JoinBloomCheck",
			requestQuery: &resolver.ParsedQuery{
				ClientId:    "1",
				QueryType:   "join",
				TableName:   "review,item",
				ColToGet:    []string{"review.rating", "item.title"},
				SearchCol:   []string{"review.creation_date", "item.i_id"},
				SearchVal:   (*pair_list)[getRandomNumber(len(*pair_list)-1)],
				SearchType:  []string{"point"},
				JoinColumns: []string{"i_id", "i_id"},
			},
		},
	}
}
