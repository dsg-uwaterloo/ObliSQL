package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Haseeb1399/WorkingThesis/api/loadbalancer"
	"github.com/Haseeb1399/WorkingThesis/api/resolver"
)

func (c *myResolver) constructRequestAndUpdate(pkList []string, requestID int64, q *resolver.ParsedQuery) ([]string, []string, error) {
	ctx := context.Background()
	valReq := loadbalancer.LoadBalanceRequest{
		Keys:      make([]string, 0, len(pkList)*len(q.ColToGet)),
		Values:    make([]string, 0, len(pkList)*len(q.ColToGet)),
		RequestId: requestID,
	}
	if q.ColToGet[0] == "*" {
		return nil, nil, fmt.Errorf("encourted * in update operation")
	}
	updateCols := q.ColToGet

	for _, pk := range pkList {
		for ind, col := range updateCols {
			keyVal := fmt.Sprintf("%s/%s/%s", q.TableName, col, pk)
			valReq.Keys = append(valReq.Keys, keyVal)
			valReq.Values = append(valReq.Values, q.UpdateVal[ind])
		}
	}

	valueRes, err := c.conn.AddKeys(ctx, &valReq)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to update values: %w", err)
	}

	parsedKeys := make([]string, 0, len(valueRes.Keys))
	parsedValues := make([]string, 0, len(valueRes.Values))

	for ind, key := range valueRes.Keys {
		//Do only once, Remove from the index part. Don't do it twice.
		if valueRes.Values[ind] != "-1" {
			parsedKeys = append(parsedKeys, key)
			parsedValues = append(parsedValues, valueRes.Values[ind])
		}
	}

	return parsedKeys, parsedValues, nil
}

func (c *myResolver) doUpdate(q *resolver.ParsedQuery, localRequestID int64) (*queryResponse, error) {

	var filteredPks []string
	var err error

	if c.checkAllIndexExists([]string{q.TableName}, q.SearchCol) {
		filteredPks, err = c.filterPkUsingIndex(q, localRequestID)
		if err != nil {
			return nil, fmt.Errorf("error filtering multiple indexes: %w", err)
		}
	} else {
		if c.checkAnyIndexExists(q.TableName, q.SearchCol) {
			log.Fatalln("Mix of Index and Non-Index Columns not implemented.")
		} else {
			columData, err := c.getSearchColumns(q.TableName, q.SearchCol, localRequestID)
			if err != nil {
				return nil, fmt.Errorf("error filtering primary keys: %w", err)
			}
			filteredPks, err = c.filterPkFromColumns(columData, q, localRequestID)
			if err != nil {
				return nil, fmt.Errorf("error filtering primary keys: %w", err)
			}
		}
	}
	//Fatal Error for OR or any other operator.
	filteredPks = andFilter(filteredPks, q.SearchCol)
	updatedKeys, updatedValues, err := c.constructRequestAndUpdate(filteredPks, localRequestID, q)

	if err != nil {
		return nil, fmt.Errorf("error constructing request and fetching: %w", err)
	}

	return &queryResponse{
		Keys:   updatedKeys,
		Values: updatedValues,
	}, nil
}
