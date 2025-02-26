package resolver

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/cespare/xxhash/v2"
	"github.com/rs/zerolog/log"

	loadbalancer "github.com/project/ObliSql/api/loadbalancer"
	"github.com/project/ObliSql/api/resolver"
	"go.opentelemetry.io/otel/attribute"
)

func (c *myResolver) orderTuples(orderBy string, tableName string, keys []string, values []string) ([]string, []string) {
	//Parse out Order Column and Order Type (ASC,DESC)
	//Only works on columns being fetched --> Fix this in future.
	//Also make it work for Joins (Columns from different tables)

	orderCommand := strings.Split(orderBy, ",")
	orderColumn, orderType := orderCommand[0], orderCommand[1]
	orderColumnType := c.getColumnType(tableName, orderColumn)

	mapVal := createMapFromLists(keys, values)
	sortedMap := sortMapByColumn(mapVal, orderColumn, orderColumnType, orderType)

	sortedKeys, sortedValues := mapToList(sortedMap, tableName)

	return sortedKeys, sortedValues
}

func (c *myResolver) checkAllIndexExists(tableName []string, searchCol []string) bool {
	//Checks if an index exists on columns within searchCol for only one table.
	for _, tv := range tableName {
		for _, v := range searchCol {
			if !contains(c.metaData[tv].IndexOn, v) {
				return false
			}
		}
	}
	return true
}

func (c *myResolver) checkMultiTableIndexExists(tableName []string, searchCol []string) bool {
	if len(tableName) != len(searchCol) {
		return false
	}
	for i, tv := range tableName {
		if !contains(c.metaData[tv].IndexOn, searchCol[i]) {
			return false
		}
	}
	return true
}

func (c *myResolver) checkAnyIndexExists(tableName string, searchCol []string) bool {
	for _, v := range searchCol {
		if contains(c.metaData[tableName].IndexOn, v) {
			return true
		}
	}
	return false
}

func (c *myResolver) getColumnType(tableName, colName string) string {
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

func andFilter(filterKeys []string, searchCol []string) []string {
	newKeys := filterKeys
	if len(searchCol) > 1 {
		//Only handle AND case with point queries.
		filteredQuery := detectRepeats(filterKeys)
		newKeys = filteredQuery
	}
	return newKeys
}

func parseValuesAndRemoveNull(resp *loadbalancer.LoadBalanceResponse) []string {
	indexedKeys := []string{}

	for _, val := range resp.Values {

		respVal := strings.Split(val, ",")
		tempList := make([]string, 0, len(respVal))
		for i, v := range respVal {
			if i == len(respVal)-1 {
				re := regexp.MustCompile(`^\d+`)
				numbers := re.FindString(v)
				tempList = append(tempList, numbers)
			} else {
				tempList = append(tempList, v)
			}
		}
		indexedKeys = append(indexedKeys, tempList...)
	}
	return indexedKeys
}

func mapToList(sortedMap []map[string]string, TableName string) ([]string, []string) {
	sortedKeys := []string{}
	sortedValues := []string{}

	v := sortedMap[0]

	keys := make([]string, 0, len(v))
	for ik := range v {
		if ik != "id" {
			keys = append(keys, ik)
		}
	}

	// Sort the keys alphabetically. This is done to ensure tests have static order.
	sort.Strings(keys)

	for _, v := range sortedMap {
		pkVal := v["id"]

		for _, ik := range keys {
			key := TableName + "/" + ik + "/" + pkVal
			val := v[ik]
			sortedKeys = append(sortedKeys, key)
			sortedValues = append(sortedValues, val)
		}
	}
	return sortedKeys, sortedValues
}

func (c *myResolver) constructPointIndexKey(searchCol, searchValue, tableName string, lbReq *loadbalancer.LoadBalanceRequest) {
	indexKey := fmt.Sprintf("%s/%s_index/%s", tableName, searchCol, searchValue)
	lbReq.Keys = append(lbReq.Keys, indexKey)
	lbReq.Values = append(lbReq.Values, "")
}

func (c *myResolver) constructRangeIndexKeyInt(searchCol string, searchValueStart, searchValueEnd int64, tableName string, lbReq *loadbalancer.LoadBalanceRequest) {
	filterKey := fmt.Sprintf("%s_%s_index", tableName, searchCol)
	for v := searchValueStart; v <= searchValueEnd; v++ {
		indexKey := fmt.Sprintf("%s/%s_index/%d", tableName, searchCol, v)

		if c.UseBloom {
			c.Created.Add(1)
			isPresent := c.Filters[filterKey].Has(xxhash.Sum64([]byte(indexKey)))
			// fmt.Printf("Searching for %s in %s Found? %t\n", indexKey, filterKey, isPresent)
			if isPresent {
				c.Inserted.Add(1)
				lbReq.Keys = append(lbReq.Keys, indexKey)
				lbReq.Values = append(lbReq.Values, "")
			}

		} else {
			lbReq.Keys = append(lbReq.Keys, indexKey)
			lbReq.Values = append(lbReq.Values, "")
		}
	}
}

func (c *myResolver) constructRangeIndexDate(searchCol, searchValueStart, searchValueEnd, tableName string, lbReq *loadbalancer.LoadBalanceRequest) {
	dateRangeValues, err := getDatesInRange(searchValueStart, searchValueEnd)
	filterKey := fmt.Sprintf("%s_%s_index", tableName, searchCol)
	if err != nil {
		log.Fatal().Msgf("Failed to parse date range into points: %v", err)
	}
	for _, v := range dateRangeValues {
		indexKey := fmt.Sprintf("%s/%s_index/%s", tableName, searchCol, v)
		if c.UseBloom {
			c.Created.Add(1)
			isPresent := c.Filters[filterKey].Has(xxhash.Sum64([]byte(indexKey)))
			if isPresent {
				c.Inserted.Add(1)
				lbReq.Keys = append(lbReq.Keys, indexKey)
				lbReq.Values = append(lbReq.Values, "")
			}

		} else {
			lbReq.Keys = append(lbReq.Keys, indexKey)
			lbReq.Values = append(lbReq.Values, "")
		}
	}
}

func (c *myResolver) constructRequestAndFetch(pkList []string, requestID int64, q *resolver.ParsedQuery) ([]string, []string, error) {
	ctx := context.Background()

	if len(pkList) == 0 {
		return []string{}, []string{}, nil
	}

	valReq := loadbalancer.LoadBalanceRequest{
		Keys:      make([]string, 0, len(pkList)*len(q.ColToGet)),
		Values:    make([]string, 0, len(pkList)*len(q.ColToGet)),
		RequestId: requestID,
	}
	searchCols := q.ColToGet
	if q.ColToGet[0] == "*" {
		searchCols = c.metaData[q.TableName].ColNames
	}

	for _, pk := range pkList {
		for _, col := range searchCols {
			keyVal := fmt.Sprintf("%s/%s/%s", q.TableName, col, pk)
			valReq.Keys = append(valReq.Keys, keyVal)
			valReq.Values = append(valReq.Values, "")
		}
	}
	c.SelectFetchKeys.Add(int64(len(valReq.Keys)))

	conn, err := c.GetBatchClient()
	if err != nil {
		log.Fatal().Msgf("Failed to get Batch Client!")
	}
	valueRes, err := conn.AddKeys(ctx, &valReq)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch values: %w", err)
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

func (c *myResolver) getFullColumn(tableName, colName string, localRequestID int64) (*queryResponse, error) {
	startingKey := c.metaData[tableName].PkStart
	endingKey := c.metaData[tableName].PkEnd

	ctx := context.Background()

	req := loadbalancer.LoadBalanceRequest{
		Keys:         make([]string, 0, endingKey-startingKey+1),
		Values:       make([]string, 0, endingKey-startingKey+1),
		RequestId:    localRequestID,
		ObjectNum:    1,
		TotalObjects: 1,
	}

	for i := startingKey; i <= endingKey; i++ {
		temp := fmt.Sprintf("%s/%s/%d", tableName, colName, i)
		req.Keys = append(req.Keys, temp)
		req.Values = append(req.Values, "")
	}
	c.SelectFetchKeys.Add(int64(len(req.Keys)))

	conn, err := c.GetBatchClient()
	if err != nil {
		log.Fatal().Msgf("Failed to get Batch Client!")
	}

	fullCol, err := conn.AddKeys(ctx, &req)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch full column: %w", err)
	}

	return &queryResponse{
		Keys:   fullCol.Keys,
		Values: fullCol.Values,
	}, nil
}

func (c *myResolver) getSearchColumns(tableName string, searchCol []string, localRequestID int64) (map[string]*queryResponse, error) {
	columData := make(map[string]*queryResponse)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, v := range searchCol {
		wg.Add(1)
		go func(columnName string) {
			defer wg.Done()
			resp, err := c.getFullColumn(tableName, columnName, localRequestID)
			if err != nil {
				log.Printf("Error in GetFullColumn: %v", err)
				return
			}
			mu.Lock()
			columData[columnName] = resp
			mu.Unlock()
		}(v)
	}
	wg.Wait()

	return columData, nil
}

func (c *myResolver) filterPkFromColumns(colData map[string]*queryResponse, q *resolver.ParsedQuery, localRequestID int64) ([]string, error) {
	keyMap := make(map[string][]string)
	for k, v := range colData {
		columnIndex := getIndexFromArray(q.SearchCol, k)
		searchType := q.SearchType[columnIndex]

		switch searchType {
		case "point":
			searchValue := q.SearchVal[columnIndex]
			for idx, colValue := range v.Values {
				if colValue == searchValue {
					splitStrings := strings.Split(v.Keys[idx], "/")
					keyMap[k] = append(keyMap[k], splitStrings[2])
				}
			}
		case "range":
			searchValue := []string{}
			parsedPart, singleOp := c.rangeParser(q.SearchVal[0])
			if singleOp {
				return nil, fmt.Errorf("Range Operations supported: Start_date <= Column <= End_Date")
			}
			columnType := c.getColumnType(q.TableName, q.SearchCol[0])
			switch columnType {
			case "int":
				startingPoint, _ := strconv.ParseInt(parsedPart, 10, 64)   //starting point
				endingPoint, _ := strconv.ParseInt(q.SearchVal[1], 10, 64) // Ending point
				for v := startingPoint; v <= endingPoint; v++ {
					indexKey := fmt.Sprintf("%d", v)
					searchValue = append(searchValue, indexKey)
				}
			case "date":
				dateRangeValues, err := getDatesInRange(parsedPart, q.SearchVal[0])

				if err != nil {
					log.Fatal().Msgf("Failed to parse Date Range into Parts: %v", err)
				}
				for _, v := range dateRangeValues {
					indexKey := fmt.Sprintf("%s", v)
					searchValue = append(searchValue, indexKey)
				}
			default:
				return nil, fmt.Errorf("Range Operations on %s are not implemented", columnType)
			}
			genKeySet := make(map[string]struct{}, len(searchValue))
			for _, genKey := range searchValue {
				genKeySet[genKey] = struct{}{}
			}
			for idx, val := range v.Values {
				if _, exists := genKeySet[val]; exists {
					splitStrings := strings.Split(v.Keys[idx], "/")
					keyMap[k] = append(keyMap[k], splitStrings[2])
				}
			}
		default:
			log.Printf("FilterPkFromColumn - Not Implemented for search type: %s", searchType)
		}
	}
	//Remove intersection logic from here and return a list of keys.
	return findStringIntersection(keyMap), nil
}

func (c *myResolver) filterPkUsingIndex(q *resolver.ParsedQuery, localRequestID int64) ([]string, error) {
	indexReqKeys := loadbalancer.LoadBalanceRequest{
		Keys:      []string{},
		Values:    []string{},
		RequestId: localRequestID,
	}

	for i, v := range q.SearchType {
		switch v {
		case "point":
			c.constructPointIndexKey(q.SearchCol[i], q.SearchVal[i], q.TableName, &indexReqKeys)
		case "range":
			parsedPart, singleOp := c.rangeParser(q.SearchVal[i])
			if singleOp {
				//Fix error message. We only do between.
				return nil, fmt.Errorf("range operations supported: Start_Date <= Column <= End_Date")
			}
			columnType := c.getColumnType(q.TableName, q.SearchCol[i]) //Return the type of column it is (int, varchar, date, etc)
			switch columnType {
			case "int":
				startingPoint, _ := strconv.ParseInt(parsedPart, 10, 64)     //starting point
				endingPoint, _ := strconv.ParseInt(q.SearchVal[i+1], 10, 64) //Ending Point
				c.constructRangeIndexKeyInt(q.SearchCol[i], startingPoint, endingPoint, q.TableName, &indexReqKeys)

				// fmt.Printf("Range: %d, %d, Returned Index Keys: %v \n", startingPoint, endingPoint, indexReqKeys.Keys)
			case "date":
				c.constructRangeIndexDate(q.SearchCol[i], parsedPart, q.SearchVal[i+1], q.TableName, &indexReqKeys)
				// log.Info().Msgf("Date Range Generated: %d %v", len(indexReqKeys.Keys), indexReqKeys.Keys)
			default:
				return nil, fmt.Errorf("range operations on %s are not implemented", columnType)
			}
		default:
			return nil, fmt.Errorf("unknown search type: %s", v)
		}
	}
	c.SelectIndexKeys.Add(int64(len(indexReqKeys.Keys)))
	// log.Debug().Msgf(strconv.Itoa(len(indexReqKeys.Keys)))

	if len(indexReqKeys.Keys) == 0 {
		//Filter resulted in no valid finds
		return nil, nil
	}
	resp, err := c.indexFetchUtil(&indexReqKeys, localRequestID)
	// resp, err := conn.AddKeys(ctx, &indexReqKeys)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch index value: %w", err)
	}
	parsedKeyMap := parseValuesAndRemoveNull(resp) //Parses (2,3,4) --> [2,3,4] and ignores any -1 froms the executor (key didn't exist)
	// log.Info().Msgf("Date Range Parsed: %d", len(parsedKeyMap))
	return parsedKeyMap, nil
}

func (c *myResolver) doSelect(q *resolver.ParsedQuery, localRequestID int64) (*queryResponse, error) {
	ctx := context.Background()
	ctx, span := c.tracer.Start(ctx, "selection")
	defer span.End()
	span.SetAttributes(
		attribute.Int64("request_id", localRequestID),
		attribute.String("query", q.String()),
	)

	var filteredPks []string
	var err error

	span.AddEvent("Starting Selection")

	if c.checkAllIndexExists([]string{q.TableName}, q.SearchCol) {
		//If all searchColumns have an index on them.
		filteredPks, err = c.filterPkUsingIndex(q, localRequestID)
	} else {
		if c.checkAnyIndexExists(q.TableName, q.SearchCol) {
			log.Fatal().Msgf("Mix of index and non-index column not implemented.")
		} else {

			columData, err := c.getSearchColumns(q.TableName, q.SearchCol, localRequestID)
			if err != nil {
				return nil, fmt.Errorf("error filtering primary keys: %w", err)
			}
			filteredPks, err = c.filterPkFromColumns(columData, q, localRequestID)
		}
	}
	span.AddEvent("Finished Indexing")

	if err != nil {
		return nil, fmt.Errorf("error filtering primary keys: %w", err)
	}

	if filteredPks == nil {
		//Resolver side Filtering resulted in empty list
		c.selectRequests.Add(1)
		return &queryResponse{
			Keys:   []string{},
			Values: []string{},
		}, nil
	}

	//Fatal Error for OR or anything else.
	filteredPks = andFilter(filteredPks, q.SearchCol)
	//ALl Pks we need.

	requestKeys, requestValues, err := c.constructRequestAndFetch(filteredPks, localRequestID, q)
	if err != nil {
		return nil, fmt.Errorf("error constructing request and fetching: %w", err)
	}
	span.AddEvent("Finished Selection")

	//If there is an OrderBy
	if len(q.OrderBy) > 0 {
		if len(requestKeys) != 0 {
			for _, v := range q.OrderBy {

				requestKeys, requestValues = c.orderTuples(v, q.TableName, requestKeys, requestValues)
			}
		}
	}
	c.selectRequests.Add(1)
	return &queryResponse{
		Keys:   requestKeys,
		Values: requestValues,
	}, nil
}

//Intersection Filtering
//Check again for reordered test key/values.
