package main

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/Haseeb1399/WorkingThesis/api/resolver"
)

type aggregateFunc func(*myResolver, *resolver.ParsedQuery, int) (float64, error)

func (c *myResolver) issuePointFetch(tableName, colToGet, searchCol, searchVal string) (*queryResponse, error) {
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
		return nil, fmt.Errorf("failed to fetch values: %w", err)
	}
	return resp, nil
}

func (c *myResolver) doSum(q *resolver.ParsedQuery, ind int) (float64, error) {
	//Only handles case one where condition.
	resp, err := c.issuePointFetch(q.TableName, q.ColToGet[ind], q.SearchCol[ind], q.SearchVal[ind])
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

func (c *myResolver) doCount(q *resolver.ParsedQuery, ind int) (float64, error) {
	//Only handles case one where condition.
	resp, err := c.issuePointFetch(q.TableName, q.ColToGet[ind], q.SearchCol[ind], q.SearchVal[ind])
	if err != nil {
		return 0, err
	}
	return float64(len(resp.Values)), nil
}

func (c *myResolver) doAverage(q *resolver.ParsedQuery, ind int) (float64, error) {
	//Only handles case one where condition.

	var sumValue, countValue float64
	var sumErr, countErr error
	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()
		sumValue, sumErr = c.doSum(q, ind)
	}()

	go func() {
		defer wg.Done()
		countValue, countErr = c.doCount(q, ind)
	}()

	wg.Wait()

	if sumErr != nil {
		return 0, fmt.Errorf("error in sum calculation: %w", sumErr)
	}
	if countErr != nil {
		return 0, fmt.Errorf("error in count calculation: %w", countErr)
	}

	if countValue == 0 {
		return 0, nil
	}

	return sumValue / countValue, nil
}

func (c *myResolver) doAggregate(q *resolver.ParsedQuery) (*queryResponse, error) {
	respKeys := make([]string, len(q.AggregateType))
	respValues := make([]string, len(q.AggregateType))

	aggregateFunctions := map[string]aggregateFunc{
		"avg":   (*myResolver).doAverage,
		"sum":   (*myResolver).doSum,
		"count": (*myResolver).doCount,
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(q.AggregateType))

	for ind, aggType := range q.AggregateType {
		wg.Add(1)
		go func(index int, aggrType string) {
			defer wg.Done()

			fn, ok := aggregateFunctions[aggrType]
			if !ok {
				errChan <- fmt.Errorf("%s aggregate not implemented", aggrType)
				return
			}

			result, err := fn(c, q, index)
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
