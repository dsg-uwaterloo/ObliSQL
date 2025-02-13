package benchmark

import (
	"strconv"
	"time"

	"math/rand"

	"github.com/project/ObliSql/api/resolver"
)

type Query struct {
	name         string
	requestQuery *resolver.ParsedQuery
}

// Helper function to generate a random date between a given start and end date
// func getRandomDateRangeWithMaxLength(start, end time.Time, maxLength int) (string, string) {
// 	startUnix := start.Unix()
// 	endUnix := end.Unix()

// 	// Generate a random start date between the provided start and end
// 	randomStartUnix := rand.Int63n(endUnix-startUnix) + startUnix
// 	randomStart := time.Unix(randomStartUnix, 0)

// 	// Calculate the maximum duration based on the maxLength provided
// 	maxDuration := int64(maxLength * 24 * 60 * 60) // convert maxLength from days to seconds
// 	availableDuration := endUnix - randomStartUnix

// 	// Ensure the end date is no later than the smaller of maxLength or the available time range
// 	if availableDuration > maxDuration {
// 		availableDuration = maxDuration
// 	}

// 	randomEndUnix := randomStartUnix + rand.Int63n(availableDuration)
// 	randomEnd := time.Unix(randomEndUnix, 0)

// 	// Format the dates in YYYY-MM-DD format
// 	startDate := randomStart.Format("2006-01-02")
// 	endDate := randomEnd.Format("2006-01-02")

// 	return startDate, endDate
// }

// Helper function to generate a random date between a given start and end date
// func getRandomDateRange(start, end time.Time) (string, string) {
// 	startUnix := start.Unix()
// 	endUnix := end.Unix()

// 	// Generate a random start date between the provided start and end
// 	randomStartUnix := rand.Int63n(endUnix-startUnix) + startUnix
// 	randomStart := time.Unix(randomStartUnix, 0)

// 	// Generate a random duration to add to the start date to ensure end > start
// 	maxDuration := endUnix - randomStartUnix
// 	randomEndUnix := randomStartUnix + rand.Int63n(maxDuration)

// 	randomEnd := time.Unix(randomEndUnix, 0)

// 	// Format the dates in YYYY-MM-DD format
// 	startDate := randomStart.Format("2006-01-02")
// 	endDate := randomEnd.Format("2006-01-02")

// 	return startDate, endDate
// }

func getRandomValue(values *[]string, rng *rand.Rand) string {
	if len(*values) == 0 {
		return ""
	}

	return (*values)[rng.Intn(len(*values))]
}

// func getRandomRangeWithLength(min, max, maxRangeLength int) (string, string) {
// 	start := rand.Intn(max-min+1) + min
// 	// Calculate the maximum allowable end based on the restricted range length
// 	maxEnd := start + maxRangeLength
// 	if maxEnd > max {
// 		maxEnd = max
// 	}
// 	end := rand.Intn(maxEnd-start+1) + start
// 	return strconv.Itoa(start), strconv.Itoa(end)
// }

// func getRandomRange(min, max int) (string, string) {
// 	start := rand.Intn(max-min+1) + min
// 	end := rand.Intn(max-start+1) + start
// 	return strconv.Itoa(start), strconv.Itoa(end)
// }

func getRandomNumber(max int) int {
	return rand.Intn(max) + 1
}

func getRandomNumberInRange(x, y int) int {
	if x >= y {
		return x
	}
	return rand.Intn(y-x) + x
}

func getRandomNumberAboveThousand(numbers []string) string {
	var filteredNumbers []int
	for _, numStr := range numbers {
		num, err := strconv.Atoi(numStr)
		if err == nil && num > 1000 {
			filteredNumbers = append(filteredNumbers, num)
		}
	}

	if len(filteredNumbers) == 0 {
		return ""
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return strconv.Itoa(filteredNumbers[rng.Intn(len(filteredNumbers))])
}

func getRandomRangeFromList(list *[]string, k int) (string, string) {
	if k <= 0 || len(*list) < k {
		return "0", "0"
	}

	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source)

	// Calculate the maximum starting index for a valid range of size k
	maxStart := len(*list) - k

	// Generate a random starting index
	randomStart := rng.Intn(maxStart + 1)

	// Return the first and last elements of the selected range
	return (*list)[randomStart], (*list)[randomStart+k-1]
}

func getRandomDateRange(start, end time.Time, length int) (string, string) {
	if length <= 0 || start.After(end) || int(end.Sub(start).Hours()/24)+1 < length {
		return "0", "0"
	}

	// Create a new random source
	source := rand.NewSource((time.Now().UnixNano()))
	rng := rand.New(source)

	// Calculate the maximum starting point for a valid range of the specified length
	totalDays := int(end.Sub(start).Hours() / 24)
	maxStartDays := totalDays - length + 1

	// Generate a random starting point in days
	randomStartDays := rng.Intn(maxStartDays + 1)

	// Compute the start and end dates based on the random start
	randomStartDate := start.AddDate(0, 0, randomStartDays)
	randomEndDate := randomStartDate.AddDate(0, 0, length-1)

	// Return the start and end dates as strings
	return randomStartDate.Format("2006-01-02"), randomEndDate.Format("2006-01-02")
}

func getTestCases(u_id, i_id, a_id, pageRank_list *[]string, pair_list *[][]string, seedVal int64) []Query {
	source := rand.NewSource(seedVal) // Fixed seed value
	rng := rand.New(source)
	// Define the date range for random date generation
	startDate := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2060, 12, 31, 0, 0, 0, 0, time.UTC)
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
				SearchVal:  []string{getRandomValue(u_id, rng)},
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
				SearchVal:  []string{getRandomValue(a_id, rng), getRandomValue(u_id, rng)},
				SearchType: []string{"point", "point"},
			},
		},
		// {
		// 	name: "Select without index",
		// 	//Select title from item where i_id = 500;
		// 	requestQuery: &resolver.ParsedQuery{
		// 		ClientId:   "1",
		// 		QueryType:  "select",
		// 		TableName:  "item",
		// 		ColToGet:   []string{"title"},
		// 		SearchCol:  []string{"i_id"},
		// 		SearchVal:  []string{getRandomValue(0, 999)},
		// 		SearchType: []string{"point"},
		// 	},
		// },
		{

			name: "Select with Order by (Order by included column) - ASC",
			//Select rating from review where u_id = 812;
			requestQuery: &resolver.ParsedQuery{
				ClientId:   "1",
				QueryType:  "select",
				TableName:  "review",
				ColToGet:   []string{"rating", "creation_date"},
				SearchCol:  []string{"i_id"},
				SearchVal:  []string{getRandomValue(i_id, rng)},
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
				SearchVal:  []string{getRandomValue(i_id, rng)},
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
				SearchVal:     []string{getRandomValue(i_id, rng)},
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
				SearchVal:     []string{getRandomValue(i_id, rng)},
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
				SearchVal:     []string{getRandomValue(i_id, rng)},
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
				ClientId:      "1",
				QueryType:     "aggregate",
				TableName:     "review",
				ColToGet:      []string{"rating", "rating"},
				SearchCol:     []string{"i_id", "i_id"},
				SearchVal:     []string{getRandomValue(i_id, rng), getRandomValue(i_id, rng)},
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
					start, end := getRandomRangeFromList(u_id, 5)
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
					start, end := getRandomDateRange(startDate, endDate, getRandomNumber(5))
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
				SearchVal:   []string{getRandomValue(i_id, rng)}, //getRandomValue(i_id, rng) 55861
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
				SearchVal:   []string{getRandomValue(i_id, rng), getRandomValue(u_id, rng)},
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
				SearchVal:     []string{getRandomValue(i_id, rng), getRandomValue(u_id, rng)},
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
				SearchVal:  []string{getRandomValue(i_id, rng)},
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
				SearchVal:  []string{getRandomValue(i_id, rng), getRandomValue(u_id, rng)},
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
				SearchVal:  []string{getRandomValue(u_id, rng), getRandomValue(u_id, rng)}, //using u_id since source/trust u_id reference user IDS.
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
				SearchVal:  []string{getRandomValue(i_id, rng)}, //using u_id since source/trust u_id reference user IDS.
				SearchType: []string{"point"},
				UpdateVal:  []string{"new-title"},
			},
		},
		// {
		// 	name: "Update without index",
		// 	//Select title from item where i_id = 500;
		// 	requestQuery: &resolver.ParsedQuery{
		// 		ClientId:   "1",
		// 		QueryType:  "update",
		// 		TableName:  "item",
		// 		ColToGet:   []string{"title"},
		// 		SearchCol:  []string{"i_id"},
		// 		SearchVal:  []string{getRandomValue(0, 49999)},
		// 		SearchType: []string{"point"},
		// 		UpdateVal:  []string{"New Title for testing!"},
		// 	},
		// },
		// {
		// 	name: "BDB1",
		// 	//SELECT pageURL, pageRank FROM rankings WHERE pageRank > X
		// 	requestQuery: &resolver.ParsedQuery{
		// 		ClientId:   "1",
		// 		QueryType:  "select",
		// 		TableName:  "rankings",
		// 		ColToGet:   []string{"pageURL", "pageRank"},
		// 		SearchCol:  []string{"pageRank"},
		// 		SearchVal:  []string{"1001", "80818"},
		// 		SearchType: []string{"range"},
		// 	},
		// },
		// {

		// 	name: "BDB1-Select",
		// 	//Select rating from review where u_id = 812;
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
		// 	name: "Cross Join - JoinBloomCheck",
		// 	//select review.rating,item.title from review,item where item.i_id=r.i_id and item.i_id = 757 and review.creation_date='2018-05-03';
		// 	//Find rating and title of a review for item X created on date Y.

		// 	requestQuery: &resolver.ParsedQuery{
		// 		ClientId:    "1",
		// 		QueryType:   "join",
		// 		TableName:   "review,item", //Make it into a list
		// 		ColToGet:    []string{"review.rating", "item.title"},
		// 		SearchCol:   []string{"review.creation_date", "item.i_id"},
		// 		SearchVal:   (*pair_list)[getRandomNumber(len(*pair_list)-1)], //getRandomValue(i_id, rng) 55861[]string{"2017-01-09", "28125"}
		// 		SearchType:  []string{"point"},
		// 		JoinColumns: []string{"i_id", "i_id"},
		// 	},
		// },
	}
	return testCases
}
