package main

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/Haseeb1399/WorkingThesis/api/loadbalancer"
	"github.com/Haseeb1399/WorkingThesis/api/resolver"
)

func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
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

func intersection(list1, list2 []string) []string {
	m := make(map[string]bool)
	var result []string

	// Populate the map with elements of list1
	for _, item := range list1 {
		m[item] = true
	}

	// Check for elements in list2 that are in the map
	for _, item := range list2 {
		if _, found := m[item]; found {
			result = append(result, item)
		}
	}

	return result
}
func (c *myResolver) getFullTable(q *parsedQuery) (*queryResponse, error) {
	ctx := context.Background()
	startingKey := c.metaData[q.tableName].PkStart
	endingKey := c.metaData[q.tableName].PkEnd
	localRequestID := c.requestId.Add(1)

	req := loadbalancer.LoadBalanceRequest{
		Keys:      []string{},
		Values:    []string{},
		RequestId: localRequestID,
	}

	for i := startingKey; i <= endingKey; i++ {
		for _, j := range c.metaData[q.tableName].ColNames {
			temp := q.tableName + "/" + j + "/" + fmt.Sprintf("%d", i)
			req.Keys = append(req.Keys, temp)
			req.Values = append(req.Values, "")
		}
	}
	fmt.Println(len(req.Keys))
	fmt.Println(len(c.metaData[q.tableName].ColNames))
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

func (c *myResolver) getFullColumn(q *parsedQuery) (*queryResponse, error) {
	ctx := context.Background()
	startingKey := c.metaData[q.tableName].PkStart
	endingKey := c.metaData[q.tableName].PkEnd
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

func (c *myResolver) doSelect(q *resolver.ParsedQuery) (*queryResponse, error) {
	//Step One, Check if there is an Index or not
	indexedSearch := true
	localRequestID := c.requestId.Add(1)
	ctx := context.Background()

	for _, v := range q.SearchCol {
		if contains(c.metaData[q.TableName].IndexOn, v) {
			continue
		} else {
			indexedSearch = false
			break
		}
	}

	if indexedSearch {
		searchValues := q.SearchVal
		keyTypeMap := make(map[string][]string)
		counter := 0
		//All Columns have an index on them.
		indexReq := loadbalancer.LoadBalanceRequest{
			Keys:      []string{},
			Values:    []string{},
			RequestId: localRequestID,
		}
		for i, v := range q.SearchType {

			if v == "point" {
				indexKey := q.TableName + "/" + q.SearchCol[i] + "_index" + "/" + searchValues[counter]
				indexReq.Keys = append(indexReq.Keys, indexKey)
				keyTypeMap["point"] = append(keyTypeMap["point"], indexKey)
				indexReq.Values = append(indexReq.Values, "") //Get Request
				counter++
			} else if v == "range" {
				_, err := strconv.Atoi(searchValues[counter])
				if err != nil {
					//Either a >, <, >= or <=
				} else {
					startingPoint, _ := strconv.ParseInt(searchValues[counter], 10, 64)
					endingPoint, _ := strconv.ParseInt(searchValues[counter+1], 10, 64)
					counter += 2
					for v := startingPoint; v <= endingPoint; v++ {
						indexKey := q.TableName + "/" + q.SearchCol[i] + "_index" + "/" + strconv.FormatInt(v, 10)
						keyTypeMap["range"] = append(keyTypeMap["range"], indexKey)
						indexReq.Keys = append(indexReq.Keys, indexKey)
						indexReq.Values = append(indexReq.Values, "") //Get Request
					}
				}
			}
		}
		resp, err := c.conn.AddKeys(ctx, &indexReq)
		if err != nil {
			log.Fatalln("Failed to fetch Index Value")
		}
		//Parsing Out Ids
		indexPkeys := parseValues(resp)
		indexPointIntersect := make([]string, 0)
		indexRangeAgg := make([]string, 0)

		for typeOf, keyList := range keyTypeMap {

			if typeOf == "point" {
				for i, val := range keyList {
					if i == 0 {
						indexPointIntersect = indexPkeys[val]
					} else {
						intersectedList := intersection(indexPointIntersect, indexPkeys[val])
						indexPointIntersect = intersectedList
					}
				}
			} else if typeOf == "range" {
				for _, val := range keyList {
					indexRangeAgg = append(indexRangeAgg, indexPkeys[val]...)
				}
			}
		}
		filteredKeys := intersection(indexPointIntersect, indexRangeAgg)

		valReq := loadbalancer.LoadBalanceRequest{
			Keys:      []string{},
			Values:    []string{},
			RequestId: localRequestID,
		}
		for _, pk := range filteredKeys {
			for _, col := range q.ColToGet {
				keyVal := q.TableName + "/" + col + "/" + pk
				valReq.Keys = append(valReq.Keys, keyVal)
				valReq.Values = append(valReq.Values, "")
			}
		}
		respTwo, err := c.conn.AddKeys(ctx, &valReq)

		if err != nil {
			log.Fatalln("Failed to fetch Actual Values!")
		}

		return &queryResponse{
			Keys:   respTwo.Keys,
			Values: respTwo.Values,
		}, nil

	} else {
		//At-least one of the search columns does not have an index on it.

	}

	return &queryResponse{
		Keys:   nil,
		Values: nil,
	}, nil
}
