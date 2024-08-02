package main

import (
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/Haseeb1399/WorkingThesis/api/resolver"
)

func getTableNames(tablNames string) (string, string) {
	splitNames := strings.Split(tablNames, ",")
	if len(splitNames) > 2 {
		log.Fatalf("Join between more than 2 tables is not implemeneted")
	}
	return splitNames[0], splitNames[1]
}

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

func checkJoinColumnPresent(columns map[string]interface{}, joinColumns []string) bool {
	localMapColumns := []string{}

	for k, v := range columns {
		// Check if the value is a list
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

	return reflect.DeepEqual(localMapColumns, joinColumns)
}

func checkJoinFilterColumnSame(columnName string, joinColumns []string) bool {
	for _, v := range joinColumns {
		if v != columnName {
			return false
		}
	}
	return true
}

func requestUsingLocalMap(values map[string]interface{}, q *resolver.ParsedQuery, t1 string, t2 string) ([]string, []string) {
	joinMapValues := values[q.SearchVal[0]] //Assumption: Only one search filter column --> Change to more if needed, otherwise throw error.
	returnKeys := []string{}
	returnValues := []string{}

	fetchColMap := make(map[string][]string)
	joinMapValuesMap, ok := joinMapValues.(map[string]interface{})
	if !ok {
		fmt.Println("joinMapValues is not a map[string]interface{}")
		return nil, nil
	}

	for _, v := range q.ColToGet {
		splitString := strings.Split(v, ".")
		tableName := splitString[0]
		fetchCol := splitString[1]
		fetchColMap[tableName] = append(fetchColMap[tableName], fetchCol)

	}

	if table, ok := joinMapValuesMap[t1].([]interface{}); ok {
		for _, v1 := range table {
			if tableTwo, okTwo := joinMapValuesMap[t2].([]interface{}); okTwo {
				for _, v2 := range tableTwo {
					table1Val := fetchColMap[t1]
					for _, v := range table1Val {
						tempKey := t1 + "/" + v + "/" + fmt.Sprintf("%v", v1)
						returnKeys = append(returnKeys, tempKey)
						returnValues = append(returnValues, "")
					}
					table2Val := fetchColMap[t2]
					for _, v := range table2Val {
						tempKey := t2 + "/" + v + "/" + fmt.Sprintf("%v", v2)
						returnKeys = append(returnKeys, tempKey)
						returnValues = append(returnValues, "")
					}
				}
			} else {
				fmt.Printf("Table %s not found or not a list\n", t2)
			}
		}
	} else {
		fmt.Printf("Table %s not found or not a list\n", t1)
	}
	return returnKeys, returnValues
}

func (c *myResolver) doJoin(q *resolver.ParsedQuery) (*queryResponse, error) {
	localRequestID := c.requestId.Add(1)
	table1, table2 := getTableNames(q.TableName)
	var reqKeys []string
	var reqValues []string
	columns, values := c.parseJoinMapForTable(q.TableName)

	//Working only for single search filter. Extent to multiple filters.
	if checkJoinColumnPresent(columns, q.JoinColumns) {
		splitSearchString := strings.Split(q.SearchCol[0], ".") // review.item --> [review,item] //Extend this for multiple table searches.

		if c.checkAllIndexExists(splitSearchString[0], []string{splitSearchString[1]}) {
			//Search Filter is indexed.

			if checkJoinFilterColumnSame(splitSearchString[1], q.JoinColumns) {
				// Search Filter and Join Column are the same.
				//We have PK of the rows we want in the Map. (Best Case)
				//Directly Query the records.
				reqKeys, reqValues = requestUsingLocalMap(values, q, table1, table2)
			}

		}
	} else {
		log.Fatalln("Joins without join-map not implemented")
	}

	storeKeys, storeVals := c.simpleFetch(reqKeys, reqValues, localRequestID)

	return &queryResponse{
		Keys:   storeKeys,
		Values: storeVals,
	}, nil
}
