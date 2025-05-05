package benchmark

import (
	"fmt"
	"sort"
	"strings"
)

// SearchColDistribution holds the frequency count of each value for a specific search column
type SearchColDistribution struct {
	ColName string
	Values  map[string]int
	Total   int
}

// AnalyzeSearchColDistribution analyzes the distribution of values for each search column
// across all queries in the provided slice
func AnalyzeSearchColDistribution(queries []Query) map[string]*SearchColDistribution {
	// Initialize distribution map
	distributions := make(map[string]*SearchColDistribution)

	// Process each query
	for _, query := range queries {
		// Skip if no search columns
		if query.requestQuery == nil || len(query.requestQuery.SearchCol) == 0 {
			continue
		}

		// Process each search column and its corresponding value
		for i, col := range query.requestQuery.SearchCol {
			// Clean column name (remove table prefix if present)
			cleanColName := col
			if strings.Contains(col, ".") {
				parts := strings.Split(col, ".")
				cleanColName = parts[len(parts)-1]
			}

			// Skip if not one of the target columns we're interested in
			if cleanColName != "u_id" && cleanColName != "a_id" && cleanColName != "i_id" {
				continue
			}

			// Initialize distribution for this column if not already exists
			if _, exists := distributions[cleanColName]; !exists {
				distributions[cleanColName] = &SearchColDistribution{
					ColName: cleanColName,
					Values:  make(map[string]int),
				}
			}

			// Get the value and increment the count
			// Handle range values (which are represented as two values)
			if i < len(query.requestQuery.SearchVal) && query.requestQuery.SearchType[i] == "point" {
				val := query.requestQuery.SearchVal[i]
				distributions[cleanColName].Values[val]++
				distributions[cleanColName].Total++
			} else if i < len(query.requestQuery.SearchVal) && query.requestQuery.SearchType[i] == "range" {
				// For ranges, we'll use a special representation
				if i+1 < len(query.requestQuery.SearchVal) {
					rangeKey := fmt.Sprintf("%s-%s", query.requestQuery.SearchVal[i], query.requestQuery.SearchVal[i+1])
					distributions[cleanColName].Values[rangeKey]++
					distributions[cleanColName].Total++
				}
			}
		}
	}

	return distributions
}

// PrintDistributionStats prints detailed statistics about the distributions
func PrintDistributionStats(distributions map[string]*SearchColDistribution) {
	for colName, dist := range distributions {
		fmt.Printf("\n=== %s Distribution ===\n", colName)
		fmt.Printf("Total occurrences: %d\n", dist.Total)
		fmt.Printf("Unique values: %d\n", len(dist.Values))

		// Sort values by frequency
		type valuePair struct {
			Value string
			Count int
		}
		pairs := make([]valuePair, 0, len(dist.Values))
		for val, count := range dist.Values {
			pairs = append(pairs, valuePair{val, count})
		}
		sort.Slice(pairs, func(i, j int) bool {
			return pairs[i].Count > pairs[j].Count
		})

		// Print top values
		fmt.Println("Top values:")
		limit := 10
		if len(pairs) < limit {
			limit = len(pairs)
		}
		for i := 0; i < limit; i++ {
			fmt.Printf("  %s: %d (%.2f%%)\n", pairs[i].Value, pairs[i].Count,
				float64(pairs[i].Count)/float64(dist.Total)*100)
		}
	}
}
