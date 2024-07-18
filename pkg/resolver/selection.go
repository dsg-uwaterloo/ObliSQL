package main

import (
	"context"
	"fmt"
	"log"
	"regexp"
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

func parseValues(resp *loadbalancer.LoadBalanceResponse) map[string][]string {
	indexedKeys := make(map[string][]string)
	for j, val := range resp.Values {
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

func (c *myResolver) constructRequestAndFetch(pkList []string, requestID int64, q *resolver.ParsedQuery) *loadbalancer.LoadBalanceResponse {
	ctx := context.Background()
	valReq := loadbalancer.LoadBalanceRequest{
		Keys:      []string{},
		Values:    []string{},
		RequestId: requestID,
	}
	for _, pk := range pkList {
		for _, col := range q.ColToGet {
			keyVal := q.TableName + "/" + col + "/" + pk
			valReq.Keys = append(valReq.Keys, keyVal)
			valReq.Values = append(valReq.Values, "")
		}
	}

	valueRes, err := c.conn.AddKeys(ctx, &valReq)

	if err != nil {
		log.Fatalf("Failed to fetch Values!")
	}
	return valueRes
}

func (c *myResolver) getFullColumn(tableName string, colName string) (*queryResponse, error) {

	//No conversion needed, already integers
	startingKey := c.metaData[tableName].PkStart
	endingKey := c.metaData[tableName].PkEnd

	ctx := context.Background()
	localRequestID := c.requestId.Add(1)

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

func (c *myResolver) getSearchColumns(tableName string, searchCol []string) map[string]*queryResponse {
	columData := make(map[string]*queryResponse)
	var wg sync.WaitGroup
	for _, v := range searchCol {
		wg.Add(1)
		go func(columnName string) {
			defer wg.Done()
			resp, err := c.getFullColumn(tableName, columnName)
			if err != nil {
				log.Fatalln("Error in GetFullColumn!", err)
			}
			columData[v] = resp
		}(v)
	}
	wg.Wait()

	return columData
}

func (c *myResolver) filterPkFromColumns(colData map[string]*queryResponse, q *resolver.ParsedQuery) []string {
	filteredKeys := make([]string, 0)

	//Iterate over all the columns in colData.
	//Check if its a point or a range
	//Depending on what type it is, do filtering
	//Return back filteredKeys

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
		} else if v == "range" {
			log.Fatalln("Not implemented")
		}

	}
	resp, err := c.conn.AddKeys(ctx, &indexReqKeys)
	if err != nil {
		log.Fatalln("Failed to Fetch Index Value")
	}

	respKeys := []string{}
	parsedKeyMap := parseValues(resp)

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
		columData := c.getSearchColumns(q.TableName, q.SearchCol)
		//Filter Out the primary keys we want.
		filteredPks = c.filterPkFromColumns(columData, q)

	}
	requestValues := c.constructRequestAndFetch(filteredPks, localRequestID, q)

	return &queryResponse{
		Keys:   requestValues.Keys,
		Values: requestValues.Values,
	}, nil
}
