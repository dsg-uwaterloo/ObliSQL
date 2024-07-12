package main

import (
	"context"
	"fmt"
	"log"
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

func filterPkFromColumns(colData map[string]*queryResponse, q *resolver.ParsedQuery) []string {
	filteredKeys := make([]string, 0)

	//Iterate over all the columns in colData.
	//Check if its a point or a range
	//Depending on what type it is, do filtering
	//Return back filteredKeys

	return filteredKeys
}

func filterPkUsingIndex(q *resolver.ParsedQuery) []string {
	//Entire filtering logic goes here.
	return nil
}

func (c *myResolver) perfSelect(q *resolver.ParsedQuery) (*queryResponse, error) {
	localRequestID := c.requestId.Add(1)

	if c.checkIndexExists(q) {

		filteredPk := filterPkUsingIndex(q)
		requestValues := c.constructRequestAndFetch(filteredPk, localRequestID, q)

		return &queryResponse{
			Keys:   requestValues.Keys,
			Values: requestValues.Values,
		}, nil

	} else {
		//Get Full Columns for all searchCols
		columData := make(map[string]*queryResponse)

		var wg sync.WaitGroup
		for _, v := range q.SearchCol {
			wg.Add(1)
			go func(columnName string) {
				defer wg.Done()
				resp, err := c.getFullColumn(q.TableName, columnName)
				if err != nil {
					log.Fatalln("Error in GetFullColumn!", err)
				}
				columData[v] = resp
			}(v)
		}
		wg.Wait()
		//Filter Out the primary keys we want.
		filteredPKs := filterPkFromColumns(columData, q)
		//Fetch values we want from the cloud
		requestValues := c.constructRequestAndFetch(filteredPKs, localRequestID, q)

		//Return to user.
		return &queryResponse{
			Keys:   requestValues.Keys,
			Values: requestValues.Values,
		}, nil

	}

	return nil, nil
}
