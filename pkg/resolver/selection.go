package main

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/Haseeb1399/WorkingThesis/api/loadbalancer"
	"github.com/Haseeb1399/WorkingThesis/api/resolver"
)

func (c *myResolver) checkIndexExists(q *resolver.ParsedQuery) bool {
	//Iterate over all searchColumns and check if any have an index.

	for _, v := range q.SearchCol {
		if contains(c.metaData[q.TableName].IndexOn, v) {
			continue
		} else {
			return false
		}
	}
	return true
}

func (c *myResolver) getRangeColumnType(tableName string, colName string) string {
	return c.metaData[tableName].ColTypes[colName]
}

func (c *myResolver) rangeParser(op string) (string, bool) {
	switch op {
	case "<", ">", "<=", ">=":
		return op, true
	default:
		return op, false
	}
}

func parseValuesAndRemoveNull(resp *loadbalancer.LoadBalanceResponse) map[string][]string {
	indexedKeys := make(map[string][]string)
	for j, val := range resp.Values {
		if val == "-1" {
			continue
		}

		respVal := strings.Split(val, ",")
		tempList := make([]string, 0)
		for i, v := range respVal {
			if i == len(respVal)-1 {
				re := regexp.MustCompile(`^\d+`)
				numbers := re.FindString(v)
				tempList = append(tempList, numbers)
			} else {
				tempList = append(tempList, v)
			}

		}
		indexedKeys[resp.Keys[j]] = tempList
	}
	return indexedKeys
}

func (c *myResolver) constructPointIndexKey(searchCol string, searchValue string, tableName string, lbReq *loadbalancer.LoadBalanceRequest) {
	indexKey := tableName + "/" + searchCol + "_index" + "/" + searchValue
	lbReq.Keys = append(lbReq.Keys, indexKey)
	lbReq.Values = append(lbReq.Values, "")
}

func (c *myResolver) constructRangeIndexKeyInt(searchCol string, searchValueStart int64, searchValueEnd int64, tableName string, lbReq *loadbalancer.LoadBalanceRequest) {
	for v := searchValueStart; v <= searchValueEnd; v++ {
		indexKey := tableName + "/" + searchCol + "_index" + "/" + strconv.FormatInt(v, 10)
		lbReq.Keys = append(lbReq.Keys, indexKey)
		lbReq.Values = append(lbReq.Values, "")
	}
}

func (c *myResolver) constructRangeIndexDate(searchCol string, searchValueStart string, searchValueEnd string, tableName string, lbReq *loadbalancer.LoadBalanceRequest) {

	dateRangeValues, err := getDatesInRange(searchValueStart, searchValueEnd)
	if err != nil {
		log.Fatalln("Failed to parse date range into points")
	}
	for _, v := range dateRangeValues {
		indexKey := tableName + "/" + searchCol + "_index" + "/" + v
		lbReq.Keys = append(lbReq.Keys, indexKey)
		lbReq.Values = append(lbReq.Values, "")
	}
}

func (c *myResolver) constructRequestAndFetch(pkList []string, requestID int64, q *resolver.ParsedQuery) ([]string, []string) {
	ctx := context.Background()
	valReq := loadbalancer.LoadBalanceRequest{
		Keys:      []string{},
		Values:    []string{},
		RequestId: requestID,
	}
	searchCols := []string{}

	if q.ColToGet[0] == "*" {
		searchCols = c.metaData[q.TableName].ColNames
	} else {
		searchCols = q.ColToGet
	}

	for _, pk := range pkList {
		for _, col := range searchCols {
			keyVal := q.TableName + "/" + col + "/" + pk
			valReq.Keys = append(valReq.Keys, keyVal)
			valReq.Values = append(valReq.Values, "")
		}
	}

	valueRes, err := c.conn.AddKeys(ctx, &valReq)

	parsedKeys := []string{}
	parsedValues := []string{}

	for ind, key := range valueRes.Keys {
		if valueRes.Values[ind] == "-1" {
			continue
		} else {
			parsedKeys = append(parsedKeys, key)
			parsedValues = append(parsedValues, valueRes.Values[ind])
		}
	}

	if err != nil {
		log.Fatalf("Failed to fetch Values!")
	}
	return parsedKeys, parsedValues
}

func (c *myResolver) getFullColumn(tableName string, colName string, localRequestID int64) (*queryResponse, error) {

	//No conversion needed, already integers
	startingKey := c.metaData[tableName].PkStart
	endingKey := c.metaData[tableName].PkEnd

	ctx := context.Background()

	req := loadbalancer.LoadBalanceRequest{
		Keys:         []string{},
		Values:       []string{},
		RequestId:    localRequestID,
		ObjectNum:    1,
		TotalObjects: 1,
	}

	for i := startingKey; i <= endingKey; i++ {
		temp := tableName + "/" + colName + "/" + fmt.Sprintf("%d", i)
		req.Keys = append(req.Keys, temp)
		req.Values = append(req.Values, "")
	}
	fullCol, err := c.conn.AddKeys(ctx, &req)

	if err != nil {
		log.Fatalln("Failed to fetch full column!")
		return nil, err
	}

	return &queryResponse{
		Keys:   fullCol.Keys,
		Values: fullCol.Values,
	}, nil

}

func (c *myResolver) getSearchColumns(tableName string, searchCol []string, localRequestID int64) map[string]*queryResponse {
	columData := make(map[string]*queryResponse)
	var wg sync.WaitGroup
	for _, v := range searchCol {
		wg.Add(1)
		go func(columnName string) {
			defer wg.Done()
			resp, err := c.getFullColumn(tableName, columnName, localRequestID)
			if err != nil {
				log.Fatalln("Error in GetFullColumn!", err)
			}
			columData[v] = resp
		}(v)
	}
	wg.Wait()

	return columData
}

func (c *myResolver) filterPkFromColumns(colData map[string]*queryResponse, q *resolver.ParsedQuery, localRequestID int64) []string {
	keyMap := make(map[string][]string)

	for k, v := range colData {
		columnIndex := getIndexFromArray(q.SearchCol, k)
		searchType := q.SearchType[columnIndex]

		if searchType == "point" {
			searchValue := q.SearchVal[columnIndex]

			for idx, colValue := range v.Values {
				if colValue == searchValue {
					splitStrings := strings.Split(v.Keys[idx], "/")
					keyMap[k] = append(keyMap[k], splitStrings[2])
				}
			}
		} else if searchType == "range" {
			log.Fatalf("Range search for non-index column not implemented.")
		} else {
			log.Fatalf("FilterPkFromColumn - Not Implemented!")
		}

	}

	filteredKeys := findStringIntersection(keyMap)

	return filteredKeys
}

func (c *myResolver) filterPkUsingIndex(q *resolver.ParsedQuery, localRequestID int64) []string {
	ctx := context.Background()
	indexReqKeys := loadbalancer.LoadBalanceRequest{
		Keys:      []string{},
		Values:    []string{},
		RequestId: localRequestID,
	}
	counter := 0
	//Differentiate Between Point and Range
	for i, v := range q.SearchType {
		if v == "point" {
			c.constructPointIndexKey(q.SearchCol[i], q.SearchVal[counter], q.TableName, &indexReqKeys)
			counter++
		} else if v == "range" {
			parsedPart, singleOp := c.rangeParser(q.SearchVal[counter])

			if singleOp {
				log.Fatalf("Range Operations using >, <, >=, <= are not yet implemented")
			} else {
				columnType := c.getRangeColumnType(q.TableName, q.SearchCol[i])
				if columnType == "int" {
					startingPoint, _ := strconv.ParseInt(parsedPart, 10, 64)
					endingPoint, _ := strconv.ParseInt(q.SearchVal[counter+1], 10, 64)
					c.constructRangeIndexKeyInt(q.SearchCol[i], startingPoint, endingPoint, q.TableName, &indexReqKeys)
					counter++
				} else if columnType == "date" {
					starting_date := q.SearchVal[counter]
					ending_date := q.SearchVal[counter+1]
					c.constructRangeIndexDate(q.SearchCol[i], starting_date, ending_date, q.TableName, &indexReqKeys)
					counter += 2
				} else {
					log.Fatalf("Range operations on %s are not implemented \n", columnType)
				}

			}
		}

	}

	resp, err := c.conn.AddKeys(ctx, &indexReqKeys)
	if err != nil {
		log.Fatalln("Failed to Fetch Index Value")
	}

	respKeys := []string{}

	parsedKeyMap := parseValuesAndRemoveNull(resp)

	for _, v := range parsedKeyMap {
		respKeys = append(respKeys, v...)
	}

	return respKeys
}

func (c *myResolver) doSelect(q *resolver.ParsedQuery) (*queryResponse, error) {
	localRequestID := c.requestId.Add(1)

	var filteredPks []string
	if c.checkIndexExists(q) {
		filteredPks = c.filterPkUsingIndex(q, localRequestID)
	} else {
		//Get Full Columns for all searchCols
		columData := c.getSearchColumns(q.TableName, q.SearchCol, localRequestID)
		//Filter Out the primary keys we want.
		filteredPks = c.filterPkFromColumns(columData, q, localRequestID)
	}
	requestKeys, requestVales := c.constructRequestAndFetch(filteredPks, localRequestID, q)

	return &queryResponse{
		Keys:   requestKeys,
		Values: requestVales,
	}, nil
}
