package resolver

import (
	"context"
	"fmt"
	"strings"

	"github.com/cespare/xxhash/v2"
	loadbalancer "github.com/project/ObliSql/api/loadbalancer"
	"github.com/project/ObliSql/api/resolver"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Create a map to track unique values for each table and maintain mapping
type TableTracking struct {
	uniqueKeys map[string]bool     // Track unique keys we'll actually request
	keyMapping map[string][]string // Track which original pairs each unique key came from
}

func constructJoinKeysOptimized(foundPairs []string, tableOrder []string, joinColMap *map[string]string) ([]string, map[string][]string) {
	tracking := make(map[string]*TableTracking) // One tracker per table

	// Initialize trackers for each table
	for _, table := range strings.Split(tableOrder[0], "/") {
		tracking[table] = &TableTracking{
			uniqueKeys: make(map[string]bool),
			keyMapping: make(map[string][]string),
		}
	}

	// Process all pairs
	for _, pair := range foundPairs {
		splitPair := strings.Split(pair, "/")
		splitTableOrder := strings.Split(tableOrder[0], "/")

		if len(splitPair) != len(splitTableOrder) {
			fmt.Println("Mismatch in values and table order:", splitPair, splitTableOrder)
			continue
		}

		// Handle each table's values
		for i, table := range splitTableOrder {
			newKey := fmt.Sprintf("%s/%s/%s", table, (*joinColMap)[table], splitPair[i])

			// Track the original pair this key came from
			tracking[table].keyMapping[newKey] = append(tracking[table].keyMapping[newKey], pair)

			// Mark this key as needed
			tracking[table].uniqueKeys[newKey] = true
		}
	}

	// Construct final deduplicated key list
	var joinCheck []string
	for _, tableTrack := range tracking {
		for key := range tableTrack.uniqueKeys {
			joinCheck = append(joinCheck, key)
		}
	}

	// Construct mapping of which pairs led to each key
	pairMapping := make(map[string][]string)
	for table, tableTrack := range tracking {
		for key, pairs := range tableTrack.keyMapping {
			pairMapping[fmt.Sprintf("%s:%s", table, key)] = pairs
		}
	}

	return joinCheck, pairMapping
}

func getTableNames(tablNames string) (string, string) {
	splitNames := strings.Split(tablNames, ",")
	if len(splitNames) > 2 {
		log.Fatal().Msg("Join between more than 2 tables is not implemeneted")
	}
	return splitNames[0], splitNames[1]
}

func (c *myResolver) orderTuplesJoin(keys []string, values []string, q *resolver.ParsedQuery) ([]string, []string) {

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
func filterMatchingPairsOptimized(
	pairMapping map[string][]string,
	storeKeys []string,
	storeValues []string,
) ([]string, []string) {

	// fmt.Println(pairMapping)
	// fmt.Println("---")
	// fmt.Println(storeKeys)

	// Maps to help construct the final result
	tableData := make(map[string]map[string]string)   // Maps table -> id -> value
	tableKeys := make(map[string]map[string]string)   // Maps table -> id -> original key
	joinKeyData := make(map[string]map[string]string) // Maps table -> id -> join column value
	validPairs := make(map[string]bool)               // Track which pairs are actually valid after checking values

	// Step 1: Separate join column keys and data keys
	for i, key := range storeKeys {
		parts := strings.Split(key, "/")
		if len(parts) < 3 {
			continue
		}

		table := parts[0]
		id := parts[2]

		// Check if this key is in our pairMapping (is it a join column?)
		isJoinKey := false
		for mappingKey := range pairMapping {
			if strings.HasSuffix(mappingKey, ":"+key) {
				isJoinKey = true
				// Store the join column value
				if _, exists := joinKeyData[table]; !exists {
					joinKeyData[table] = make(map[string]string)
				}
				joinKeyData[table][id] = storeValues[i]
				break
			}
		}
		if !isJoinKey {
			// This is a data column
			if _, exists := tableData[table]; !exists {
				tableData[table] = make(map[string]string)
				tableKeys[table] = make(map[string]string)
			}

			tableData[table][id] = storeValues[i]
			tableKeys[table][id] = key
		}
	}

	// Step 2: Validate pairs by checking actual join values
	for _, pairs := range pairMapping {
		for _, pair := range pairs {
			ids := strings.Split(pair, "/")
			if len(ids) != 2 {
				continue
			}

			// Get the tables involved in this pair
			var table1, table2 string
			for table := range joinKeyData {
				if _, hasID := joinKeyData[table][ids[0]]; hasID {
					table1 = table
				}
				if _, hasID := joinKeyData[table][ids[1]]; hasID {
					table2 = table
				}
			}

			// Check if join values match
			if joinVal1, ok1 := joinKeyData[table1][ids[0]]; ok1 {
				if joinVal2, ok2 := joinKeyData[table2][ids[1]]; ok2 {
					if joinVal1 == joinVal2 {
						validPairs[pair] = true
					}
				}
			}
		}
	}

	// Step 3: Construct the final result using only valid pairs
	var filteredKeys []string
	var filteredValues []string
	// seen := make(map[string]bool)

	for pair := range validPairs {
		ids := strings.Split(pair, "/")
		if len(ids) != 2 {
			continue
		}

		// Add data for both tables in the pair
		for table := range tableData {
			for _, id := range ids {
				if key, exists := tableKeys[table][id]; exists {
					if value, hasValue := tableData[table][id]; hasValue {
						// Use composite key to avoid duplicates within the same table
						// compositeKey := fmt.Sprintf("%s/%s", key, id)
						filteredKeys = append(filteredKeys, key)
						filteredValues = append(filteredValues, value)
						// if !seen[compositeKey] {
						// 	seen[compositeKey] = true
						// }
					}
				}
			}
		}
	}

	return filteredKeys, filteredValues
}

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

func generateCombinations(tableNameKeyMap map[string][]string, order []string) ([]string, []string) {
	result := []string{}
	orders := []string{} // Stores the order of table names in each combination

	// Helper function to recursively generate combinations
	var helper func(int, string, string)
	helper = func(depth int, combination string, orderTrack string) {
		if depth == len(order) {
			result = append(result, combination)
			orders = append(orders, orderTrack)
			return
		}

		tableName := order[depth]
		for _, value := range tableNameKeyMap[tableName] {
			newCombination := combination
			newOrderTrack := orderTrack

			if newCombination != "" {
				newCombination += "/"
				newOrderTrack += "/" // Maintain order tracking format
			}

			newCombination += value
			newOrderTrack += tableName

			helper(depth+1, newCombination, newOrderTrack)
		}
	}

	helper(0, "", "")
	return result, orders
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

func (c *myResolver) indexFilterAndJoin(tableName string, searchMap *map[string]map[string]string, joinColMap *map[string]string, localRequestID int64, span trace.Span) (map[string][]string, []string, []string, map[string][]string) {
	lbReq := loadbalancer.LoadBalanceRequest{
		RequestId: localRequestID,
	}
	span.AddEvent("Getting Index Keys")
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
	c.JoinFetchKeys.Add(int64(len(resp.Keys)))
	span.AddEvent("Got Index Keys")
	span.SetAttributes(
		attribute.String("IndexKeys", strings.Join(resp.Keys, ",")),
		attribute.String("IndexValues", strings.Join(resp.Values, ",")),
	)
	tableNameKeyMap := make(map[string][]string)
	foundPairs := []string{}
	joinCheck := []string{}
	pairMapping := make(map[string][]string)

	span.AddEvent("Parsing Index")
	for i, k := range resp.Keys {
		tableNameInner := strings.Split(k, "/")[0]
		splitValues := strings.Split(resp.Values[i], ",")
		if contains(splitValues, "-1") {
			return map[string][]string{}, joinCheck, foundPairs, pairMapping // Empty Response back, we can return
		}
		tableNameKeyMap[tableNameInner] = splitValues
	}
	span.AddEvent("Parsed Index")

	span.AddEvent("Get all Combos")
	getCombo, tableOrder := generateCombinations(tableNameKeyMap, strings.Split(tableName, ","))
	span.AddEvent("Got all Combos")

	if c.UseBloom {
		//Can have False Positives
		span.AddEvent("Checking Membership")
		joinFilter := c.Filters[tableName]
		for _, pair := range getCombo {
			found := joinFilter.Has(xxhash.Sum64([]byte(pair)))
			if found {
				foundPairs = append(foundPairs, pair)
			}
		}
		span.AddEvent("Checked Membership")

		if c.JoinBloomOptimized {
			span.AddEvent("Constructing Extra Join Keys")
			joinCheck, pairMapping = constructJoinKeysOptimized(foundPairs, tableOrder, joinColMap)
			span.AddEvent("Constructed Extra Join Keys")
			span.SetAttributes(
				attribute.String("Found Pairs", strings.Join(foundPairs, ",")),
			)
			//Optimistically fetch join column and verify before returning
			// span.AddEvent("Constructing Extra Join Keys")
			// for _, pair := range foundPairs {
			// 	splitPair := strings.Split(pair, "/")
			// 	splitTableOrder := strings.Split(tableOrder[0], "/")

			// 	if len(splitPair) != len(splitTableOrder) {
			// 		fmt.Println("Mismatch in values and table order:", splitPair, splitTableOrder)
			// 		continue
			// 	}
			// 	//Only works for Join of two Tables. Extend for 3 if needed.
			// 	newKeyOne := fmt.Sprintf("%s/%s/%s", splitTableOrder[0], (*joinColMap)[splitTableOrder[0]], splitPair[0])
			// 	newKeyTwo := fmt.Sprintf("%s/%s/%s", splitTableOrder[1], (*joinColMap)[splitTableOrder[1]], splitPair[1])
			// 	joinCheck = append(joinCheck, newKeyOne)
			// 	joinCheck = append(joinCheck, newKeyTwo)
			// }
			// span.AddEvent("Constructed Extra Join Keys")
		}

	} else {
		//Completely Correct
		// 1. Get the Join Column Values for each PK we got back from index.
		// 2. Verify which Pairs satisfy the join.
		lbReq1 := loadbalancer.LoadBalanceRequest{
			RequestId: localRequestID,
		}
		// Create a map to store join column values for each table
		span.AddEvent("Default - Join Key Constructing")
		for table, pks := range tableNameKeyMap {
			joinCol := (*joinColMap)[table]
			for _, pk := range pks {
				newKey := fmt.Sprintf("%s/%s/%s", table, joinCol, pk)
				lbReq1.Keys = append(lbReq1.Keys, newKey)
				lbReq1.Values = append(lbReq1.Values, "")
			}
		}
		span.AddEvent("Default - Join Key Constructed")
		span.AddEvent("Default - Fetching Join Columns")
		conn, err := c.GetBatchClient()
		if err != nil {
			log.Fatal().Msgf("Failed to get Batch Client!")
		}

		resp, err := conn.AddKeys(context.Background(), &lbReq1)
		c.JoinFetchKeys.Add(int64(len(resp.Keys)))
		if err != nil {
			log.Fatal().Msgf("Failed to fetch index keys from load balancer!: %s \n", err)
		}
		span.AddEvent("Default - Fetched Join Columns")
		span.SetAttributes(
			attribute.Int("joinKeys", len(lbReq.Keys)),
		)
		joinColumnValues := make(map[string]map[string]string)

		// Populate the map with join column values from the response
		span.AddEvent("Checking Join")
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
		span.AddEvent("Checked Join")

	}

	// fmt.Println(getCombo)
	// fmt.Println(foundPairs)
	tablePkMap := constructTablePkMap(strings.Split(tableName, ","), foundPairs)
	return tablePkMap, joinCheck, foundPairs, pairMapping
}

func (c *myResolver) constructRequestValues(pkMap map[string][]string, q *resolver.ParsedQuery) ([]string, []string) {
	requestKeys := []string{}
	requestValues := []string{}

	for _, v := range q.ColToGet {
		splitStrings := strings.Split(v, ".")
		parsedTableName, colToGet := splitStrings[0], splitStrings[1]
		if colToGet == "*" {
			searchCols := c.metaData[parsedTableName].ColNames
			for _, pkVal := range pkMap[parsedTableName] {
				for _, col := range searchCols {
					keyVal := fmt.Sprintf("%s/%s/%s", parsedTableName, col, pkVal)
					requestKeys = append(requestKeys, keyVal)
					requestValues = append(requestValues, "")

				}
			}
		} else {
			for _, pkVal := range pkMap[parsedTableName] {
				keyVal := fmt.Sprintf("%s/%s/%s", parsedTableName, colToGet, pkVal)
				requestKeys = append(requestKeys, keyVal)
				requestValues = append(requestValues, "")
			}
		}

	}
	return requestKeys, requestValues
}

func (c *myResolver) doJoin(q *resolver.ParsedQuery, localRequestID int64) (*queryResponse, error) {
	var reqKeys []string
	var reqValues []string
	pairMap := make(map[string][]string)
	ctx := context.Background()
	ctx, span := c.tracer.Start(ctx, "Join")

	defer span.End()
	span.SetAttributes(
		attribute.Int64("request_id", localRequestID),
		attribute.String("query", q.String()),
		attribute.String("searchVal", strings.Join(q.SearchVal, ",")),
	)

	if c.checkJoinColumnPresent(q.TableName) {
		span.AddEvent("Creating Search Map")
		searchMap, tabNames, tabCols := c.CreateSearchMap(q.SearchCol, q.SearchVal, strings.Split(q.TableName, ","))
		//Do the search columns have an index on them?
		span.AddEvent("Created Search Map")
		if c.checkMultiTableIndexExists(tabNames, tabCols) {
			//All Search Columns do have an index!

			joinColMap := createJoinColumnMap(q)
			span.AddEvent("Filtering Using Join Logic")
			filteredKeys, joinCheck, _, pairMapping := c.indexFilterAndJoin(q.TableName, &searchMap, &joinColMap, localRequestID, span)
			span.AddEvent("Filtered Using Join Logic")
			span.AddEvent("Constructing Request")
			reqKeys, reqValues = c.constructRequestValues(filteredKeys, q) //Note: Test might fail sometimes due to reordering of keys in Map. Fix by sorting.
			span.AddEvent("Constructed Request")

			if len(joinCheck) > 0 {
				pairMap = pairMapping
				reqKeys = append(reqKeys, joinCheck...)
				reqValues = append(reqValues, make([]string, len(joinCheck))...)
				span.AddEvent("Added Extra Join Keys")
			}
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
		span.AddEvent("Empty Request")
		c.joinRequests.Add(1)
		return &queryResponse{
			Keys:   []string{},
			Values: []string{},
		}, nil

	} else {
		// log.Info().Msgf("Join Request Key Size: %d, %s", len(reqKeys), reqKeys)
		span.AddEvent("Fetching Values")
		storeKeys, storeVals := c.simpleFetch(reqKeys, reqValues, localRequestID)
		c.JoinFetchKeys.Add(int64(len(storeKeys)))
		span.AddEvent("Fetched Values")

		if c.JoinBloomOptimized {
			span.SetAttributes(
				attribute.Int64("HasValue", 1),
				attribute.Int64("RequestKeySize:", int64(len(reqKeys))),
				attribute.String("Sending Request", strings.Join(reqKeys, ",")),
			)
			span.AddEvent("Find Matching Pairs")
			keys, vals := filterMatchingPairsOptimized(pairMap, storeKeys, storeVals)
			span.AddEvent("Found Matching Pairs")
			c.joinRequests.Add(1)

			span.SetAttributes(
				attribute.Int64("Return Key Size:", int64(len(keys))),
				attribute.String("KeyGoingAsResp", strings.Join(keys, ",")),
			)
			return &queryResponse{
				Keys:   keys,
				Values: vals,
			}, nil
		}
		//For Order by, change the way joins are done. Associate the keys from different tables together
		//Form a list of objects. Then for order-by, sort this array of objects according to query.
		c.joinRequests.Add(1)
		span.SetAttributes(
			attribute.Int64("HasValue", 2),
			attribute.Int64("ResponseKeySize:", int64(len(storeKeys))),
			attribute.String("storeKeys", strings.Join(storeKeys, ",")),
		)
		span.AddEvent("Sending Normal Response")
		return &queryResponse{
			Keys:   storeKeys,
			Values: storeVals,
		}, nil
	}
}
