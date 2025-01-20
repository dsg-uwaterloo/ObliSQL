package resolver

import (
	"context"
	"fmt"
	"reflect"
	"sort"
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

// func (c *myResolver) removeAdditionalColumns(q *resolver.ParsedQuery, keys []string, values []string) {

// }

// Order of tableNames matters. Either raise descriptive error or handle.
func (c *myResolver) parseJoinMapForTable(tableNames string) (map[string]interface{}, map[string]interface{}) {
	if c.JoinMap[tableNames] == nil {
		log.Fatal().Msgf("Nil Join Map! %s\n", tableNames)
	}
	columns, ok := c.JoinMap[tableNames].(map[string]interface{})["column"]
	if !ok {
		log.Fatal().Msgf("Error parsing JoinMap! \n")
	}
	values, ok := c.JoinMap[tableNames].(map[string]interface{})["values"]
	if !ok {
		log.Fatal().Msgf("Error parsing JoinMap! \n")
	}

	return columns.(map[string]interface{}), values.(map[string]interface{})
}
func (c *myResolver) CreateSearchMap(searchCol, searchVal []string) (map[string]map[string]string, []string, []string) {
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

	return result, tableNames, columns
}

func checkJoinColumnPresent(columns map[string]interface{}, joinColumns []string) bool {
	localMapColumns := []string{}

	// Get all the keys and sort them
	keys := make([]string, 0, len(columns))
	for k := range columns {
		keys = append(keys, k)
	}
	sort.Strings(keys) // Sort keys alphabetically

	for _, k := range keys {
		v := columns[k]

		if list, ok := v.([]interface{}); ok {
			for _, item := range list {
				if str, ok := item.(string); ok {
					localMapColumns = append(localMapColumns, str)
				} else {
					fmt.Printf("Item %v in list is not a string\n", item)
				}
			}
		} else {
			fmt.Printf("Value for key %s is not a list\n", k)
		}
	}

	sort.Strings(localMapColumns)
	sort.Strings(joinColumns)

	return reflect.DeepEqual(localMapColumns, joinColumns)
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

func requestUsingLocalMap(values map[string]interface{}, q *resolver.ParsedQuery, t1 string, t2 string) map[string][]string {
	joinMapValues := values[q.SearchVal[0]] //Assumption: Only one search filter column --> Change to more if needed, otherwise throw error.
	fetchColMap := make(map[string][]string)
	joinMapValuesMap, ok := joinMapValues.(map[string]interface{})
	if !ok {
		fmt.Println("joinMapValues is not a map[string]interface{}")
		return nil
	}

	for _, v := range q.ColToGet {
		splitString := strings.Split(v, ".")
		tableName := splitString[0]
		fetchCol := splitString[1]
		fetchColMap[tableName] = append(fetchColMap[tableName], fetchCol)

	}
	tablePkMap := make(map[string][]string)
	if table, ok := joinMapValuesMap[t1].([]interface{}); ok {
		for _, v1 := range table {
			if tableTwo, okTwo := joinMapValuesMap[t2].([]interface{}); okTwo {
				for _, v2 := range tableTwo {
					tablePkMap[t1] = append(tablePkMap[t1], fmt.Sprintf("%v", v1))
					tablePkMap[t2] = append(tablePkMap[t2], fmt.Sprintf("%v", v2))
				}
			} else {
				fmt.Printf("Table %s not found or not a list\n", t2)
			}
		}
	} else {
		fmt.Printf("Table %s not found or not a list\n", t1)
	}
	return tablePkMap
	// return returnKeys, returnValues
}

func (c *myResolver) indexFilterAndJoin(tableName string, searchMap *map[string]map[string]string, localRequestID int64) map[string][]string {
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
		tableName := strings.Split(k, "/")[0]
		tableNameKeyMap[tableName] = strings.Split(resp.Values[i], ",")
	}

	getCombo := generateCombinations(tableNameKeyMap, strings.Split(tableName, ","))

	joinFilter := c.Filters[tableName]

	foundPairs := []string{}

	for _, pair := range getCombo {
		found := joinFilter.Has(xxhash.Sum64([]byte(pair)))
		if found {
			foundPairs = append(foundPairs, pair)
		}
	}

	tablePkMap := constructTablePkMap(strings.Split(tableName, ","), foundPairs)

	// fmt.Println("tablePkMap:", tablePkMap)
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
	table1, table2 := getTableNames(q.TableName)
	var reqKeys []string
	var reqValues []string
	columns, values := c.parseJoinMapForTable(q.TableName)

	//Working only for single search filter. Extend to multiple filters.
	if checkJoinColumnPresent(columns, q.JoinColumns) {
		searchMap, tabNames, tabCols := c.CreateSearchMap(q.SearchCol, q.SearchVal)

		if checkJoinFilterColumnSame(tabCols, q.JoinColumns) {
			//Case 1: We are searching on the same columns that are being joined. (Best Case)

			//Search Column and Join Column are the same.
			//We have PK of the rows we want in the Map.
			//Directly Query the records.
			filteredKeys := requestUsingLocalMap(values, q, table1, table2)
			reqKeys, reqValues = c.constructRequestValues(filteredKeys, q)
		} else {
			//We are joining on different columns and searching on different columns.

			//Do the search columns have an index on them?
			if c.checkMultiTableIndexExists(tabNames, tabCols) {
				//All Search Columns do have an index!

				filteredKeys := c.indexFilterAndJoin(q.TableName, &searchMap, localRequestID)
				reqKeys, reqValues = c.constructRequestValues(filteredKeys, q) //Note: Test might fail sometimes due to reordering of keys in Map. Fix by sorting.
			}
		}

	} else {
		//Index on one of the joinable columns --> Index nested loop join
		//No index on any of the join columns. --> Linear scan entire tables. --> Block nested loop join
		//Leverage MetaData --> Find the smaller of the two.
		fmt.Println(checkJoinColumnPresent(columns, q.JoinColumns))
		log.Fatal().Msg("Joins without join-map not implemented")
	}
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
