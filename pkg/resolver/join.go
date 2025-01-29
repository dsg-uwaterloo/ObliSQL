package resolver

import (
	"context"
	"fmt"
	"strings"

	"github.com/cespare/xxhash/v2"
	loadbalancer "github.com/project/ObliSql/api/loadbalancer"
	"github.com/project/ObliSql/api/resolver"
	"github.com/rs/zerolog/log"
)

func getTableNames(tablNames string) (string, string) {
	splitNames := strings.Split(tablNames, ",")
	if len(splitNames) > 2 {
		log.Fatal().Msg("Join between more than 2 tables is not implemeneted")
	}
	return splitNames[0], splitNames[1]
}

func (c *myResolver) orderTuplesJoin(keys []string, values []string, q *resolver.ParsedQuery) ([]string, []string) {

	fmt.Println(keys)
	return keys, values
}

func createJoinColumnMap(query *resolver.ParsedQuery) map[string]string {
	joinColumnMap := make(map[string]string)

	// Split the TableName and JoinColumns into slices
	tableNames := strings.Split(query.TableName, ",")
	joinColumns := query.JoinColumns

	// Ensure the lengths of tableNames and joinColumns match
	if len(tableNames) != len(joinColumns) {
		log.Error().Msgf("Mismatch between table names and join columns: %v vs %v", tableNames, joinColumns)
		return joinColumnMap
	}

	// Populate the map
	for i, tableName := range tableNames {
		joinColumnMap[tableName] = joinColumns[i]
	}

	return joinColumnMap
}

// func (c *myResolver) removeAdditionalColumns(q *resolver.ParsedQuery, keys []string, values []string) {

// }

// Order of tableNames matters. Either raise descriptive error or handle.
// func (c *myResolver) parseJoinMapForTable(tableNames string) (map[string]interface{}, map[string]interface{}) {
// 	if c.JoinMap[tableNames] == nil {
// 		log.Fatal().Msgf("Nil Join Map! %s\n", tableNames)
// 	}
// 	columns, ok := c.JoinMap[tableNames].(map[string]interface{})["column"]
// 	if !ok {
// 		log.Fatal().Msgf("Error parsing JoinMap! \n")
// 	}
// 	values, ok := c.JoinMap[tableNames].(map[string]interface{})["values"]
// 	if !ok {
// 		log.Fatal().Msgf("Error parsing JoinMap! \n")
// 	}

// 	return columns.(map[string]interface{}), values.(map[string]interface{})
// }

func (c *myResolver) CreateSearchMap(searchCol, searchVal, tableNamesQuery []string) (map[string]map[string]string, []string, []string) {
	// Initialize the result map
	result := make(map[string]map[string]string)
	tableNames := []string{}
	columns := []string{}

	// Iterate over the searchCol slice
	for i, col := range searchCol {
		// Split the searchCol into table and column
		parts := strings.Split(col, ".")
		if len(parts) != 2 {
			continue // Skip if the format is incorrect
		}
		tableName := parts[0]
		columnName := parts[1]

		// Append the table name and column name to their respective slices
		tableNames = append(tableNames, tableName)
		columns = append(columns, columnName)

		// Initialize the map for the table if it doesn't exist
		if _, exists := result[tableName]; !exists {
			result[tableName] = make(map[string]string)
		}

		// Assign the searchVal to the corresponding column in the table
		result[tableName][columnName] = searchVal[i]
	}

	// Handle the case where searchCol has fewer elements than tableNamesQuery
	// This is useful for cross joins where the same search value is applied to multiple tables
	if len(searchCol) < len(tableNamesQuery) {
		// Get the last search value (assuming it should be applied to all remaining tables)
		lastSearchVal := searchVal[len(searchVal)-1]

		// Iterate over the remaining tables in tableNamesQuery
		for i := len(searchCol); i < len(tableNamesQuery); i++ {
			tableName := tableNamesQuery[i]

			// Initialize the map for the table if it doesn't exist
			if _, exists := result[tableName]; !exists {
				result[tableName] = make(map[string]string)
			}

			// Assign the last search value to the join column in the table
			// Assuming the join column is the same as the last column in searchCol
			joinColumn := strings.Split(searchCol[len(searchCol)-1], ".")[1]
			result[tableName][joinColumn] = lastSearchVal
		}
	}

	return result, tableNames, columns
}

func (c *myResolver) checkJoinColumnPresent(tableNames string) bool {

	return contains(c.JoinMap, tableNames)
}

func checkJoinFilterColumnSame(columnName []string, joinColumns []string) bool {
	for i, v := range columnName {
		if v != joinColumns[i] {
			return false
		}
	}
	return true
}

func generateCombinations(tableNameKeyMap map[string][]string, order []string) []string {
	result := []string{}

	// Helper function to recursively generate combinations
	var helper func([]string, int, string)
	helper = func(current []string, depth int, combination string) {
		if depth == len(order) {
			result = append(result, combination)
			return
		}

		tableName := order[depth]
		for _, value := range tableNameKeyMap[tableName] {
			newCombination := combination
			if newCombination != "" {
				newCombination += "/"
			}
			newCombination += value
			helper(current, depth+1, newCombination)
		}
	}

	helper([]string{}, 0, "")
	return result
}

func constructTablePkMap(order []string, foundPairs []string) map[string][]string {
	tablePkMap := make(map[string][]string)

	for _, pair := range foundPairs {
		values := strings.Split(pair, "/")
		for i, table := range order {
			if _, exists := tablePkMap[table]; !exists {
				tablePkMap[table] = []string{}
			}
			tablePkMap[table] = appendUnique(tablePkMap[table], values[i])
		}
	}

	return tablePkMap
}

// Helper function to append unique values to the slice
func appendUnique(slice []string, value string) []string {
	for _, v := range slice {
		if v == value {
			return slice
		}
	}
	return append(slice, value)
}

func (c *myResolver) indexFilterAndJoin(tableName string, searchMap *map[string]map[string]string, joinColMap *map[string]string, localRequestID int64) map[string][]string {
	lbReq := loadbalancer.LoadBalanceRequest{
		RequestId: localRequestID,
	}
	for tableName, searchObj := range *searchMap {
		for searchCol, searchVal := range searchObj {
			c.constructPointIndexKey(searchCol, searchVal, tableName, &lbReq)
		}
	}

	conn, err := c.GetBatchClient()
	if err != nil {
		log.Fatal().Msgf("Failed to get Batch Client!")
	}

	resp, err := conn.AddKeys(context.Background(), &lbReq)
	if err != nil {
		log.Fatal().Msgf("Failed to fetch index keys from load balancer!: %s \n", err)
	}
	tableNameKeyMap := make(map[string][]string)

	for i, k := range resp.Keys {
		tableNameInner := strings.Split(k, "/")[0]
		splitValues := strings.Split(resp.Values[i], ",")
		if contains(splitValues, "-1") {
			return map[string][]string{} // Empty Response back, we can return
		}
		tableNameKeyMap[tableNameInner] = splitValues
	}

	getCombo := generateCombinations(tableNameKeyMap, strings.Split(tableName, ","))
	foundPairs := []string{}

	if c.UseBloom {
		joinFilter := c.Filters[tableName]
		for _, pair := range getCombo {
			found := joinFilter.Has(xxhash.Sum64([]byte(pair)))
			if found {
				foundPairs = append(foundPairs, pair)
			}
		}
	} else {
		// 1. Get the Join Column Values for each PK we got back from index.
		// 2. Verify which Pairs satisfy the join.
		lbReq1 := loadbalancer.LoadBalanceRequest{
			RequestId: localRequestID,
		}
		// Create a map to store join column values for each table
		for table, pks := range tableNameKeyMap {
			joinCol := (*joinColMap)[table]
			for _, pk := range pks {
				newKey := fmt.Sprintf("%s/%s/%s", table, joinCol, pk)
				lbReq1.Keys = append(lbReq1.Keys, newKey)
				lbReq1.Values = append(lbReq1.Values, "")
			}
		}

		conn, err := c.GetBatchClient()
		if err != nil {
			log.Fatal().Msgf("Failed to get Batch Client!")
		}

		resp, err := conn.AddKeys(context.Background(), &lbReq1)
		if err != nil {
			log.Fatal().Msgf("Failed to fetch index keys from load balancer!: %s \n", err)
		}
		joinColumnValues := make(map[string]map[string]string)

		// Populate the map with join column values from the response
		for i, key := range resp.Keys {
			parts := strings.Split(key, "/")
			if len(parts) != 3 {
				continue // Skip invalid keys
			}
			tableName := parts[0]
			pk := parts[2]
			joinColVal := resp.Values[i]

			// Initialize the inner map if it doesn't exist
			if _, exists := joinColumnValues[tableName]; !exists {
				joinColumnValues[tableName] = make(map[string]string)
			}

			// Store the join column value for this PK
			joinColumnValues[tableName][pk] = joinColVal
		}

		// Verify which pairs satisfy the join condition
		for _, pair := range getCombo {
			pairParts := strings.Split(pair, "/")
			if len(pairParts) != 2 {
				continue // Skip invalid pairs
			}

			table1 := strings.Split(tableName, ",")[0]
			table2 := strings.Split(tableName, ",")[1]
			pk1 := pairParts[0]
			pk2 := pairParts[1]

			// Get the join column values for the pair
			joinVal1, ok1 := joinColumnValues[table1][pk1]
			joinVal2, ok2 := joinColumnValues[table2][pk2]

			if ok1 && ok2 && joinVal1 == joinVal2 {
				// The pair satisfies the join condition
				foundPairs = append(foundPairs, pair)
			}
		}
	}

	// fmt.Println(getCombo)
	// fmt.Println(foundPairs)
	tablePkMap := constructTablePkMap(strings.Split(tableName, ","), foundPairs)
	return tablePkMap
}

func (c *myResolver) constructRequestValues(pkMap map[string][]string, q *resolver.ParsedQuery) ([]string, []string) {
	requestKeys := []string{}
	requestValues := []string{}

	for _, v := range q.ColToGet {
		splitStrings := strings.Split(v, ".")
		parsedTableName, colToGet := splitStrings[0], splitStrings[1]

		for _, pkVal := range pkMap[parsedTableName] {
			keyVal := fmt.Sprintf("%s/%s/%s", parsedTableName, colToGet, pkVal)
			requestKeys = append(requestKeys, keyVal)
			requestValues = append(requestValues, "")
		}
	}

	return requestKeys, requestValues
}

func (c *myResolver) doJoin(q *resolver.ParsedQuery, localRequestID int64) (*queryResponse, error) {
	var reqKeys []string
	var reqValues []string

	//Working only for single search filter. Extend to multiple filters.
	if c.checkJoinColumnPresent(q.TableName) {

		searchMap, tabNames, tabCols := c.CreateSearchMap(q.SearchCol, q.SearchVal, strings.Split(q.TableName, ","))
		//Do the search columns have an index on them?
		if c.checkMultiTableIndexExists(tabNames, tabCols) {
			//All Search Columns do have an index!

			joinColMap := createJoinColumnMap(q)
			filteredKeys := c.indexFilterAndJoin(q.TableName, &searchMap, &joinColMap, localRequestID)
			reqKeys, reqValues = c.constructRequestValues(filteredKeys, q) //Note: Test might fail sometimes due to reordering of keys in Map. Fix by sorting.
		} else {
			fmt.Errorf("Joing within an Index not supported!")
		}

	} else {
		//Index on one of the joinable columns --> Index nested loop join
		//No index on any of the join columns. --> Linear scan entire tables. --> Block nested loop join
		//Leverage MetaData --> Find the smaller of the two.
		// fmt.Println(checkJoinColumnPresent(columns, q.JoinColumns))
		log.Fatal().Msg("Joins without filter not implemented")
	}

	if len(reqKeys) == 0 {
		c.joinRequests.Add(1)
		return &queryResponse{
			Keys:   []string{},
			Values: []string{},
		}, nil
	} else {
		// log.Info().Msgf("Join Request Key Size: %d, %s", len(reqKeys), reqKeys)
		storeKeys, storeVals := c.simpleFetch(reqKeys, reqValues, localRequestID)

		//For Order by, change the way joins are done. Associate the keys from different tables together
		//Form a list of objects. Then for order-by, sort this array of objects according to query.
		c.joinRequests.Add(1)
		return &queryResponse{
			Keys:   storeKeys,
			Values: storeVals,
		}, nil
	}
}
