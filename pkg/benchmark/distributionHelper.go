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

// PrintDistributionStats prints detailed statistics about the distributions with visualization
func PrintDistributionStats(distributions map[string]*SearchColDistribution) {
	// Define how many items to show by default
	defaultTopItems := 20

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

		// Find the highest frequency for scaling the visualization
		maxFreq := 0
		if len(pairs) > 0 {
			maxFreq = pairs[0].Count
		}

		// Display header for the table
		fmt.Println("\nRank  Value                 Count    Percentage  Distribution")
		fmt.Println("-------------------------------------------------------------")

		// Determine number of items to show
		itemsToShow := defaultTopItems
		if len(pairs) < itemsToShow {
			itemsToShow = len(pairs)
		}

		// Display items with visualization
		maxBars := 50 // maximum number of bars to display
		for i := 0; i < itemsToShow; i++ {
			percentage := float64(pairs[i].Count) / float64(dist.Total) * 100
			bars := generateBars(pairs[i].Count, maxFreq, maxBars)

			// Format the value column with appropriate padding based on content length
			valueStr := fmt.Sprintf("%-20s", pairs[i].Value)
			if len(pairs[i].Value) > 20 {
				valueStr = pairs[i].Value[:17] + "..."
			}

			fmt.Printf("%-5d %-20s %-8d (%5.2f%%) %s\n",
				i+1, valueStr, pairs[i].Count, percentage, bars)
		}

		// If there are too many unique values, summarize the rest
		if len(pairs) > itemsToShow {
			remaining := len(pairs) - itemsToShow
			remainingCount := 0
			for i := itemsToShow; i < len(pairs); i++ {
				remainingCount += pairs[i].Count
			}
			remainingPercentage := float64(remainingCount) / float64(dist.Total) * 100
			fmt.Printf("... %d more unique values with combined count %d (%.2f%%)\n",
				remaining, remainingCount, remainingPercentage)
		}
	}
}

// generateBars creates a text-based bar chart
func generateBars(count, maxCount, maxBars int) string {
	if maxCount == 0 {
		return ""
	}

	numBars := int(float64(count) * float64(maxBars) / float64(maxCount))
	return strings.Repeat("█", numBars)
}
