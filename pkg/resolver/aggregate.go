package main

import (
	"log"
	"strconv"

	"github.com/Haseeb1399/WorkingThesis/api/resolver"
)

//	sumQuery := &resolver.ParsedQuery{
//		QueryType:  "select",
//		TableName:  tableName,
//		ColToGet:   []string{q.ColToGet[colToGet]},
//		SearchCol:  []string{q.SearchCol[ind]},
//		SearchVal:  []string{q.SearchVal[ind]},
//		SearchType: []string{"point"},
//	}
func (c *myResolver) issuePointFetch(tableName string, colToGet string, searchCol string, searchVal string) *queryResponse {
	sumQuery := &resolver.ParsedQuery{
		QueryType:  "select",
		TableName:  tableName,
		ColToGet:   []string{colToGet},
		SearchCol:  []string{searchCol},
		SearchVal:  []string{searchVal},
		SearchType: []string{"point"},
	}

	resp, err := c.doSelect(sumQuery)

	if err != nil {
		log.Fatalln("Failed to fetch Values!")
	}
	return resp
}

func (c *myResolver) doSum(q *resolver.ParsedQuery, ind int) float64 {

	resp := c.issuePointFetch(q.TableName, q.ColToGet[ind], q.SearchCol[ind], q.SearchVal[ind])

	valueSum := 0.0
	for _, v := range resp.Values {
		parsedValue, err := strconv.ParseFloat(v, 64)
		if err != nil {
			log.Fatalf("Failed to parse value as float!")
		}
		valueSum += parsedValue
	}

	return valueSum
}

func (c *myResolver) doCount(q *resolver.ParsedQuery, ind int) int {
	resp := c.issuePointFetch(q.TableName, q.ColToGet[ind], q.SearchCol[ind], q.SearchVal[ind])

	return len(resp.Values)
}

func (c *myResolver) doAverage(q *resolver.ParsedQuery, ind int) float64 {
	// Create channels to receive the results from the goroutines
	sumChan := make(chan float64)
	countChan := make(chan int)

	// Start the goroutines
	go func() {
		sumChan <- c.doSum(q, ind)
	}()
	go func() {
		countChan <- c.doCount(q, ind)
	}()

	// Receive the results from the channels
	sumValue := <-sumChan
	countValue := <-countChan

	// Avoid division by zero
	if countValue == 0 {
		return 0
	}

	return sumValue / float64(countValue)
}

func (c *myResolver) doAggregate(q *resolver.ParsedQuery) (*queryResponse, error) {
	respKeys := []string{}
	respValues := []string{}

	for ind, aggType := range q.AggregateType {
		if aggType == "avg" {
			resp := c.doAverage(q, ind)
			respKeys = append(respKeys, "")
			respValues = append(respValues, strconv.FormatFloat(resp, 'f', -1, 64))
		} else if aggType == "sum" {
			log.Fatalf("Sum Aggregate Not Implemented")
		} else if aggType == "count" {
			log.Fatalf("Count Aggregate Not Implemented")
		} else if aggType == "min" {
			log.Fatalf("Min Aggregate Not Implemeneted")
		} else if aggType == "max" {
			log.Fatalf("Max Aggregate Not Implemented")
		}
	}

	return &queryResponse{
		Keys:   respKeys,
		Values: respValues,
	}, nil

}
