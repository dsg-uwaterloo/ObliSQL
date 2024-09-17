package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/Haseeb1399/WorkingThesis/api/resolver"
)

type aggregateFunc func(*myResolver, *resolver.ParsedQuery, int, int64) (float64, error)

func (c *myResolver) issuePointFetch(tableName, colToGet, searchCol, searchVal string, requestID int64) (*queryResponse, error) {
	sumQuery := &resolver.ParsedQuery{
		QueryType:  "select",
		TableName:  tableName,
		ColToGet:   []string{colToGet},
		SearchCol:  []string{searchCol},
		SearchVal:  []string{searchVal},
		SearchType: []string{"point"},
	}

	resp, err := c.doSelect(sumQuery, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch values: %w", err)
	}
	return resp, nil
}

func (c *myResolver) doSum(q *resolver.ParsedQuery, ind int, requestID int64) (float64, error) {
	//Only handles case one where condition.
	//select rating from review where i_id = 17;

	resp, err := c.issuePointFetch(q.TableName, q.ColToGet[ind], q.SearchCol[ind], q.SearchVal[ind], requestID)
	if err != nil {
		return 0, err
	}

	var valueSum float64
	for _, v := range resp.Values {
		parsedValue, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse value as float: %w", err)
		}
		valueSum += parsedValue
	}

	return valueSum, nil
}

func (c *myResolver) doSumAndCount(q *resolver.ParsedQuery, ind int, requestID int64) (float64, float64, error) {
	//Only handles case one where condition.
	//select rating from review where i_id = 17;

	resp, err := c.issuePointFetch(q.TableName, q.ColToGet[ind], q.SearchCol[ind], q.SearchVal[ind], requestID)
	if err != nil {
		return 0, 0, err
	}

	var valueSum float64
	for _, v := range resp.Values {
		parsedValue, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse value as float: %w", err)
		}
		valueSum += parsedValue
	}

	return valueSum, float64(len(resp.Keys)), nil
}

func (c *myResolver) joinSumAndCount(q *resolver.ParsedQuery, ind int, requestID int64) (float64, float64, error) {
	joinQuery := &resolver.ParsedQuery{
		QueryType:   "join",
		TableName:   q.TableName,
		ColToGet:    q.ColToGet,
		SearchCol:   q.SearchCol,
		SearchVal:   q.SearchVal,
		SearchType:  q.SearchType,
		JoinColumns: q.JoinColumns,
	}
	resp, err := c.doJoin(joinQuery, requestID)
	if err != nil {
		return 0, 0, err
	}

	var valueSum float64
	for _, v := range resp.Values {
		parsedValue, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse value as float: %w", err)
		}
		valueSum += parsedValue
	}

	return valueSum, float64(len(resp.Keys)), nil
}

func (c *myResolver) doCount(q *resolver.ParsedQuery, ind int, requestID int64) (float64, error) {
	//Only handles case one where condition.
	resp, err := c.issuePointFetch(q.TableName, q.ColToGet[ind], q.SearchCol[ind], q.SearchVal[ind], requestID)
	if err != nil {
		return 0, err
	}
	return float64(len(resp.Values)), nil
}

func (c *myResolver) doAverage(q *resolver.ParsedQuery, ind int, requestID int64) (float64, error) {
	joinCheck := len(strings.Split(q.TableName, ",")) > 1
	var sumValue float64
	var countValue float64
	var err error

	if joinCheck {
		//We have one of the join queries
		sumValue, countValue, err = c.joinSumAndCount(q, ind, requestID)
		if err != nil {
			return 0, fmt.Errorf("error in count calculation: (JounSumCount) %w", err)
		}
	} else {
		//Only handles case one where condition.

		sumValue, countValue, err = c.doSumAndCount(q, ind, requestID)
		if err != nil {
			return 0, fmt.Errorf("error in count calculation: (doSumCount) %w", err)
		}
	}

	if countValue == 0 {
		return 0, nil
	}

	return sumValue / countValue, nil
}

func (c *myResolver) doAggregate(q *resolver.ParsedQuery, requestID int64) (*queryResponse, error) {
	respKeys := make([]string, len(q.AggregateType))
	respValues := make([]string, len(q.AggregateType))

	aggregateFunctions := map[string]aggregateFunc{
		"avg":   (*myResolver).doAverage,
		"sum":   (*myResolver).doSum,
		"count": (*myResolver).doCount,
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(q.AggregateType))

	//agggreType --> Avg
	for ind, aggType := range q.AggregateType {
		wg.Add(1)
		go func(index int, aggrType string) {
			defer wg.Done()

			fn, ok := aggregateFunctions[aggrType] //fn-->avg
			if !ok {
				errChan <- fmt.Errorf("%s aggregate not implemented", aggrType)
				return
			}

			result, err := fn(c, q, index, requestID) //Change name of this to something else.
			if err != nil {
				errChan <- fmt.Errorf("error performing %s aggregate: %w", aggrType, err)
				return
			}

			respValues[index] = strconv.FormatFloat(result, 'f', -1, 64)
		}(ind, aggType)
	}

	wg.Wait()
	close(errChan)

	if err := <-errChan; err != nil {
		return nil, err
	}

	return &queryResponse{
		Keys:   respKeys,
		Values: respValues,
	}, nil
}
