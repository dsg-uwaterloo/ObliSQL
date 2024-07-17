package main

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/Haseeb1399/WorkingThesis/api/loadbalancer"
)

func contain1s(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
func (c *resolver) getFull1Table(q *parsedQuery) (*queryResponse, error) {
	ctx := context.Background()
	startingKey := c.metaData.pkStart
	endingKey := c.metaData.pkEnd
	localRequestID := c.requestId.Add(1)

	req := loadbalancer.LoadBalanceRequest{
		Keys:         []string{},
		Values:       []string{},
		RequestId:    localRequestID,
		ObjectNum:    1,
		TotalObjects: 1,
	}

	for i := startingKey; i <= endingKey; i++ {
		for _, j := range c.metaData.colNames {
			temp := q.tableName + "/" + j + "/" + fmt.Sprintf("%d", i)
			req.Keys = append(req.Keys, temp)
			req.Values = append(req.Values, "")
		}
	}
	fmt.Println(len(req.Keys))
	fmt.Println(len(c.metaData.colNames))
	fullTable, err := c.conn.AddKeys(ctx, &req)

	if err != nil {
		log.Fatalln("Failed to fetch full table!", err)
		return nil, err
	}

	return &queryResponse{
		Keys:   fullTable.Keys,
		Values: fullTable.Values,
	}, nil

}

func (c *resolver) getFullCol1umn(q *parsedQuery) (*queryResponse, error) {
	ctx := context.Background()
	startingKey := c.metaData.pkStart
	endingKey := c.metaData.pkEnd
	localRequestID := c.requestId.Add(1)

	req := loadbalancer.LoadBalanceRequest{
		Keys:         []string{},
		Values:       []string{},
		RequestId:    localRequestID,
		ObjectNum:    1,
		TotalObjects: 1,
	}

	for i := startingKey; i <= endingKey; i++ {
		temp := q.tableName + "/" + q.colToGet[0] + "/" + fmt.Sprintf("%d", i)
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

func (c *resolver) doSel1ect(q *parsedQuery, isRange bool) (*queryResponse, error) {

	if isRange {
		if contains(c.metaData.indexOn, q.searchCol) {
			//Indexed Search

			ctx := context.Background()
			localRequestID := c.requestId.Add(1)

			req := loadbalancer.LoadBalanceRequest{
				Keys:         []string{},
				Values:       []string{},
				RequestId:    localRequestID,
				ObjectNum:    1,
				TotalObjects: 1,
			}

			//Two ranges or more ranges case
			//Age > 10
			//Age < 10 (Also <= or >=)

			startVal, _ := strconv.ParseInt(q.searchVal[0], 10, 64) //1
			endVal, _ := strconv.ParseInt(q.searchVal[1], 10, 64)   //4

			for v := startVal; v <= endVal; v++ {
				indexKey := q.tableName + "/" + q.searchCol + "_index" + "/" + strconv.FormatInt(v, 10)
				req.Keys = append(req.Keys, indexKey)
				req.Values = append(req.Values, "")
			}

			resp, err := c.conn.AddKeys(ctx, &req)
			if err != nil {
				log.Fatalln("Failed to fetch Index Value")
			}
			indexedKeys := make([]string, 0)

			for _, val := range resp.Values {
				respVal := strings.Split(val, ",")
				for i, v := range respVal {
					if i == len(respVal)-1 {
						re := regexp.MustCompile(`^\d+`)
						numbers := re.FindString(v)
						indexedKeys = append(indexedKeys, numbers)
					} else {
						indexedKeys = append(indexedKeys, v)
					}

				}
			}
			//Increase bandwith size. Experiment performance with dividing this into multiple objects.
			reqTwo := loadbalancer.LoadBalanceRequest{
				Keys:         []string{},
				Values:       []string{},
				RequestId:    localRequestID,
				ObjectNum:    1,
				TotalObjects: 1,
			}

			//Multiple columns to get c_balance,c_id
			for _, v := range indexedKeys {
				temp := q.tableName + "/" + q.colToGet[0] + "/" + v
				reqTwo.Keys = append(reqTwo.Keys, temp)
				reqTwo.Values = append(reqTwo.Values, "")
			}
			respTwo, err := c.conn.AddKeys(ctx, &reqTwo)
			if err != nil {
				log.Fatalln("Failed to fetch Actual Values!")
			}
			return &queryResponse{
				Keys:   respTwo.Keys,
				Values: respTwo.Values,
			}, nil
		} else {
			//Not Indexed range search
			startVal, _ := strconv.ParseInt(q.searchVal[0], 10, 64)
			endVal, _ := strconv.ParseInt(q.searchVal[1], 10, 64)

			rangeList := make([]string, 0)
			for v := startVal; v <= endVal; v++ {
				indexKey := strconv.FormatInt(v, 10)
				rangeList = append(rangeList, indexKey)
			}

			ctx := context.Background()
			startingKey := c.metaData.pkStart //Fix for multiple tables.
			endingKey := c.metaData.pkEnd
			localRequestID := c.requestId.Add(1)

			req := loadbalancer.LoadBalanceRequest{
				Keys:         []string{},
				Values:       []string{},
				RequestId:    localRequestID,
				ObjectNum:    1,
				TotalObjects: 1,
			}

			for i := startingKey; i <= endingKey; i++ {
				temp := q.tableName + "/" + q.searchCol + "/" + fmt.Sprintf("%d", i)
				req.Keys = append(req.Keys, temp)
				req.Values = append(req.Values, "")
			}
			fullTable, err := c.conn.AddKeys(ctx, &req)
			if err != nil {
				log.Fatalf("Failed to fetch full table! Error: %s \n", err)
			}
			neededPk := make([]string, 0)

			for i, key := range fullTable.Keys {
				value := strings.Replace(fullTable.Values[i], "|", "", -1)
				if contains(rangeList, value) {
					neededPk = append(neededPk, strings.Split(key, "/")[2])
				}
			}

			reqTwo := loadbalancer.LoadBalanceRequest{
				Keys:         []string{},
				Values:       []string{},
				RequestId:    localRequestID,
				ObjectNum:    1,
				TotalObjects: 1,
			}

			for _, v := range neededPk {
				temp := q.tableName + "/" + q.colToGet[0] + "/" + v
				reqTwo.Keys = append(reqTwo.Keys, temp)
				reqTwo.Values = append(reqTwo.Values, "")
			}
			respTwo, err := c.conn.AddKeys(ctx, &reqTwo)

			if err != nil {
				log.Fatalln("Failed to fetch Actual Values!")
			}

			return &queryResponse{
				Keys:   respTwo.Keys,
				Values: respTwo.Values,
			}, nil
		}

	} else {
		//If it is not a range select.

		if contains(c.metaData.indexOn, q.searchCol) {
			ctx := context.Background()
			localRequestID := c.requestId.Add(1)

			req := loadbalancer.LoadBalanceRequest{
				Keys:         []string{},
				Values:       []string{},
				RequestId:    localRequestID,
				ObjectNum:    1,
				TotalObjects: 1,
			}
			//We can filter using an index
			indexKey := q.tableName + "/" + q.searchCol + "_index" + "/" + q.searchVal[0]
			req.Keys = append(req.Keys, indexKey)
			req.Values = append(req.Values, "")
			resp, err := c.conn.AddKeys(ctx, &req)
			if err != nil {
				log.Fatalln("Failed to fetch Index Value")
			}
			respVal := strings.Split(resp.Values[0], ",")
			indexedKeys := make([]string, 0)

			for i, v := range respVal {
				if i == len(respVal)-1 {
					re := regexp.MustCompile(`^\d+`)
					numbers := re.FindString(v)
					indexedKeys = append(indexedKeys, numbers)
				} else {
					indexedKeys = append(indexedKeys, v)
				}

			}

			reqTwo := loadbalancer.LoadBalanceRequest{
				Keys:         []string{},
				Values:       []string{},
				RequestId:    localRequestID,
				ObjectNum:    1,
				TotalObjects: 1,
			}

			for _, v := range indexedKeys {
				temp := q.tableName + "/" + q.colToGet[0] + "/" + v
				reqTwo.Keys = append(reqTwo.Keys, temp)
				reqTwo.Values = append(reqTwo.Values, "")
			}
			respTwo, err := c.conn.AddKeys(ctx, &reqTwo)
			if err != nil {
				log.Fatalln("Failed to fetch Actual Values!")
			}
			return &queryResponse{
				Keys:   respTwo.Keys,
				Values: respTwo.Values,
			}, nil

		} else {
			//Search Column does not have an index on it. Do a full table scan + Filtering.
			ctx := context.Background()
			startingKey := c.metaData.pkStart
			endingKey := c.metaData.pkEnd
			localRequestID := c.requestId.Add(1)

			req := loadbalancer.LoadBalanceRequest{
				Keys:         []string{},
				Values:       []string{},
				RequestId:    localRequestID,
				ObjectNum:    1,
				TotalObjects: 1,
			}

			for i := startingKey; i <= endingKey; i++ {
				temp := q.tableName + "/" + q.searchCol + "/" + fmt.Sprintf("%d", i)
				req.Keys = append(req.Keys, temp)
				req.Values = append(req.Values, "")
			}
			fullTable, err := c.conn.AddKeys(ctx, &req)
			if err != nil {
				log.Fatalf("Failed to fetch full table! Error: %s \n", err)
			}
			neededPk := make([]string, 0)

			for i, key := range fullTable.Keys {
				value := strings.Replace(fullTable.Values[i], "|", "", -1)
				if value == q.searchVal[0] {
					neededPk = append(neededPk, strings.Split(key, "/")[2])
				}
			}

			reqTwo := loadbalancer.LoadBalanceRequest{
				Keys:         []string{},
				Values:       []string{},
				RequestId:    localRequestID,
				ObjectNum:    1,
				TotalObjects: 1,
			}

			for _, v := range neededPk {
				temp := q.tableName + "/" + q.colToGet[0] + "/" + v
				reqTwo.Keys = append(reqTwo.Keys, temp)
				reqTwo.Values = append(reqTwo.Values, "")
			}
			respTwo, err := c.conn.AddKeys(ctx, &reqTwo)

			if err != nil {
				log.Fatalln("Failed to fetch Actual Values!")
			}

			return &queryResponse{
				Keys:   respTwo.Keys,
				Values: respTwo.Values,
			}, nil

		}
	}
}
