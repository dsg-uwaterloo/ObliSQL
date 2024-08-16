package main

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"

	"github.com/Haseeb1399/WorkingThesis/api/loadbalancer"
	"github.com/Haseeb1399/WorkingThesis/api/resolver"
)

func getTableNames(tablNames string) (string, string) {
	splitNames := strings.Split(tablNames, ",")
	if len(splitNames) > 2 {
		log.Fatalf("Join between more than 2 tables is not implemeneted")
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
	columns, ok := c.JoinMap[tableNames].(map[string]interface{})["column"]
	if !ok {
		log.Fatalf("Error parsing JoinMap! \n")
	}
	values, ok := c.JoinMap[tableNames].(map[string]interface{})["values"]
	if !ok {
		log.Fatalf("Error parsing JoinMap! \n")
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

func (c *myResolver) indexFilterAndJoin(searchMap map[string]map[string]string, joinColumns []string, joinMapValues map[string]interface{}, localRequestID int64) map[string][]string {
	lbReq := loadbalancer.LoadBalanceRequest{
		RequestId: localRequestID,
	}
	for tableName, searchObj := range searchMap {
		for searchCol, searchVal := range searchObj {
			c.constructPointIndexKey(searchCol, searchVal, tableName, &lbReq)
		}
	}
	resp, err := c.conn.AddKeys(context.Background(), &lbReq)
	if err != nil {
		log.Fatalf("Failed to fetch index keys from load balancer!: %s \n", err)
	}
	tableNameKeyMap := make(map[string][]string)

	for i, k := range resp.Keys {
		tableName := strings.Split(k, "/")[0]
		tableNameKeyMap[tableName] = strings.Split(resp.Values[i], ",")
	}

	tablePkMap := make(map[string][]string)

	for _, pkInfo := range joinMapValues {
		tempPkMap := make(map[string][]string)
		for tableName, pkList := range pkInfo.(map[string]interface{}) {
			newList := c.getListFromInterface(pkList)
			commonPk := FindCommonElements(newList, tableNameKeyMap[tableName])
			tempPkMap[tableName] = append(tempPkMap[tableName], commonPk...)
		}
		candidate := true
		for _, v := range tempPkMap {
			if len(v) == 0 {
				candidate = false
				break
			}
		}
		if !candidate {
			continue
		} else {
			for k, v := range tempPkMap {
				tablePkMap[k] = append(tablePkMap[k], v...)
			}
		}
	}
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

func (c *myResolver) doJoin(q *resolver.ParsedQuery) (*queryResponse, error) {
	localRequestID := c.requestId.Add(1)
	table1, table2 := getTableNames(q.TableName)
	var reqKeys []string
	var reqValues []string
	columns, values := c.parseJoinMapForTable(q.TableName)

	//Working only for single search filter. Extent to multiple filters.
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
				filteredKeys := c.indexFilterAndJoin(searchMap, q.JoinColumns, values, localRequestID)
				reqKeys, reqValues = c.constructRequestValues(filteredKeys, q) //Note: Test might fail sometimes due to reordering of keys in Map. Fix by sorting.
			}
		}

	} else {
		fmt.Println(checkJoinColumnPresent(columns, q.JoinColumns))
		log.Fatalln("Joins without join-map not implemented")
	}

	storeKeys, storeVals := c.simpleFetch(reqKeys, reqValues, localRequestID)

	//For Order by, change the way joins are done. Associate the keys from different tables together
	//Form a list of objects. Then for order-by, sort this array of objects according to query.

	return &queryResponse{
		Keys:   storeKeys,
		Values: storeVals,
	}, nil
}
