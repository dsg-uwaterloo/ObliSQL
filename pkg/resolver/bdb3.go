package resolver

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/cespare/xxhash/v2"
	loadbalancer "github.com/project/ObliSql/api/loadbalancer"
	"github.com/project/ObliSql/api/resolver"
	"github.com/rs/zerolog/log"
)

// AvgMetrics holds the two averages for a given IP.
type AvgMetrics struct {
	PageRank  float64
	AdRevenue float64
}

func ComputeAverages(keys, vals []string) map[string]AvgMetrics {
	// intermediate sums and counts
	type accum struct {
		sumRank    float64
		sumRevenue float64
		count      int
	}
	statsMap := make(map[string]*accum, len(vals)/3)

	for i := 0; i+2 < len(vals); i += 3 {
		pr, err := strconv.ParseFloat(vals[i], 64)
		if err != nil {
			continue
		}
		ip := vals[i+1]
		ar, err := strconv.ParseFloat(vals[i+2], 64)
		if err != nil {
			continue
		}

		a, ok := statsMap[ip]
		if !ok {
			a = &accum{}
			statsMap[ip] = a
		}
		a.sumRank += pr
		a.sumRevenue += ar
		a.count++
	}

	// build final averages map
	result := make(map[string]AvgMetrics, len(statsMap))
	for ip, a := range statsMap {
		if a.count == 0 {
			continue
		}
		result[ip] = AvgMetrics{
			PageRank:  a.sumRank / float64(a.count),
			AdRevenue: a.sumRevenue / float64(a.count),
		}
	}
	return result
}

func generateTuples(values []string, max int) []string {
	var tuples []string
	for _, group := range values {
		// split the comma-separated IDs
		ids := strings.Split(group, ",")
		for _, idStr := range ids {
			idStr = strings.TrimSpace(idStr)
			// optionally validate it's an integer
			if _, err := strconv.Atoi(idStr); err != nil {
				continue // skip non-numeric entries
			}
			// for each i in [0..max], append "i/ID"
			for i := 0; i <= max; i++ {
				tuples = append(tuples, fmt.Sprintf("%d/%s", i, idStr))
			}
		}
	}
	return tuples
}

func (c *myResolver) doBDB3Join(q *resolver.ParsedQuery, localRequestID int64) (*queryResponse, error) {
	// query := `
	// 	SELECT
	// 	UV.sourceIP,
	// 	SUM(UV.adRevenue)  AS totalRevenue,
	// 	AVG(R.pageRank)    AS avgPageRank
	// 	FROM benchmark.rankings    R
	// 	JOIN benchmark.uservisits  UV
	// 	ON R.pageURL = UV.destURL
	// 	WHERE UV.visitDate
	// 		BETWEEN DATE '1980-01-01' AND DATE '1980-04-02'
	// 	GROUP BY UV.sourceIP
	// 	ORDER BY totalRevenue DESC
	// 	LIMIT 1;
	// `

	//Added Later on to support additional benchmarks for review. Code only supports BDB3 Join Query

	// Step 1: Generate a list of keys for start and end range
	ctx := context.Background()
	indexReqKeys := loadbalancer.LoadBalanceRequest{
		Keys:      []string{},
		Values:    []string{},
		RequestId: localRequestID,
	}

	splitResult := strings.Split(q.SearchCol[0], ".")
	if len(splitResult) != 2 {
		return nil, fmt.Errorf("invalid format for SearchCol[0]: expected two values separated by a comma")
	}

	tableName, colName := splitResult[0], splitResult[1]
	start, end := q.SearchVal[0], q.SearchVal[1]

	// Step 2: Pass through Bloom Filter. Fetch Values that go pass through (We now have PKs for usevisits where date condition is True)
	c.constructRangeIndexDate(colName, start, end, tableName, &indexReqKeys) //Function Handles Bloom Filter
	conn, err := c.GetBatchClient()

	if err != nil {
		log.Fatal().Msgf("Failed to get batch Client!")
	}

	resp, err := conn.AddKeys(ctx, &indexReqKeys)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch index value: %w", err)
	}

	joinFilter := c.Filters[q.TableName]

	// // Step 3: Go over all possible Keys
	// totalRankingIds := c.metaData["rankings"].PkEnd
	// joinFilter := c.Filters[q.TableName]
	// crossProduct := generateTuples(resp.Values, totalRankingIds)
	// foundPairs := []string{}
	// for _, pair := range crossProduct {
	// 	pairBytes := []byte(pair) // Concatenate the strings and convert to []byte
	// 	found := joinFilter.Has(xxhash.Sum64(pairBytes))
	// 	if found {
	// 		foundPairs = append(foundPairs, pair) // Concatenate with a delimiter for string representation
	// 	}
	// }

	// Step 3(2): Fetch DestURLs using the PKs u got. Then Fetch Pks from Rankings for those pageUrls and then make a join.

	tableNameKeyMap := make(map[string][]string)

	for i, k := range resp.Keys {
		tableNameInner := strings.Split(k, "/")[0]
		splitValues := strings.Split(resp.Values[i], ",")
		if contains(splitValues, "-1") {
			continue
		} else {
			tableNameKeyMap[tableNameInner] = splitValues
		}
	}

	valReq := loadbalancer.LoadBalanceRequest{
		Keys:   []string{},
		Values: []string{},
	}
	splitValuesAll := []string{}
	for i, _ := range resp.Keys {
		splitValues := strings.Split(resp.Values[i], ",")
		splitValuesAll = append(splitValuesAll, splitValues...)
	}

	for _, pk := range splitValuesAll {
		keyVal := fmt.Sprintf("%s/%s/%s", "uservisits", "destURL", pk)
		valReq.Keys = append(valReq.Keys, keyVal)
		valReq.Values = append(valReq.Values, "")

	}

	valResp, err := conn.AddKeys(ctx, &valReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch DestURL value: %w", err)
	}

	indexReqKeysRanking := loadbalancer.LoadBalanceRequest{
		Keys:      []string{},
		Values:    []string{},
		RequestId: localRequestID,
	}

	for _, val := range valResp.Values {
		keyVal := fmt.Sprintf("%s/%s/%s", "rankings", "pageURL_index", val) //rankings/pageURL_index/
		indexReqKeysRanking.Keys = append(indexReqKeysRanking.Keys, keyVal)
		indexReqKeysRanking.Values = append(indexReqKeysRanking.Values, "")
	}

	rankingResp, err := conn.AddKeys(ctx, &indexReqKeysRanking)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pageURL value: %w", err)
	}

	for i, k := range rankingResp.Keys {
		parts := strings.Split(k, "/")
		if len(parts) == 0 {
			log.Warn().Msgf("Invalid key format: %s", k)
			continue
		}
		tableNameInner := parts[0]

		splitValues := strings.Split(rankingResp.Values[i], ",")
		if contains(splitValues, "-1") {
			continue
		}

		tableNameKeyMap[tableNameInner] = append(tableNameKeyMap[tableNameInner], splitValues...)
	}
	getCombo, _ := generateCombinations(tableNameKeyMap, strings.Split(q.TableName, ","))

	foundPairs2 := []string{}
	for _, pair := range getCombo {
		found := joinFilter.Has(xxhash.Sum64([]byte(pair)))
		if found {
			foundPairs2 = append(foundPairs2, pair)
		}
	}
	resultKeys := []string{}
	resultValues := []string{}

	for _, pair := range foundPairs2 {
		parts := strings.Split(pair, "/")
		if len(parts) != 2 {
			log.Warn().Msgf("Invalid pair format: %s", pair)
			continue
		}
		//parts[0] --> rankings
		//parts[1] --> uservisits
		pageRank := fmt.Sprintf("%s/%s/%s", "rankings", "pageRank", parts[0])
		sourceIP := fmt.Sprintf("%s/%s/%s", "uservisits", "sourceIP", parts[1])
		adRevenue := fmt.Sprintf("%s/%s/%s", "uservisits", "adRevenue", parts[1])
		resultKeys = append(resultKeys, pageRank)
		resultKeys = append(resultKeys, sourceIP)
		resultKeys = append(resultKeys, adRevenue)
		resultValues = append(resultValues, "")
		resultValues = append(resultValues, "")
		resultValues = append(resultValues, "")
	}

	fetchKeys := loadbalancer.LoadBalanceRequest{
		Keys:      resultKeys,
		Values:    resultValues,
		RequestId: localRequestID,
	}

	valueResp, err := conn.AddKeys(ctx, &fetchKeys)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pageURL value: %w", err)
	}

	averages := ComputeAverages(valueResp.Keys, valueResp.Values)

	returnKeys := []string{}
	returnValues := []string{}

	for ip, metrics := range averages {
		returnKeys = append(returnKeys, fmt.Sprintf("sourceIP/%s", ip))
		returnValues = append(returnValues, fmt.Sprintf("avgPageRank=%f || avgAdRevenue=%f", metrics.PageRank, metrics.AdRevenue))

	}

	// fmt.Println(len(resultKeys))

	return &queryResponse{
		Keys:   returnKeys,
		Values: returnValues,
	}, nil
}
