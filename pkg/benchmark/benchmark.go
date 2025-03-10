package benchmark

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/project/ObliSql/api/resolver"
)

type Ack struct {
	hadError     bool
	latency      time.Duration
	QueryType    string
	ErrorString  error
	responseSize int
}

func ReadCSV(filename string) ([][]string, error) {
	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	// Create CSV reader
	reader := csv.NewReader(file)
	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("error reading CSV: %v", err)
	}

	return records, nil
}

func ReadCSVColumn(filename string, columnIndex int, skipHeader bool) ([]string, error) {
	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println(err)
		return nil, fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	// Create CSV reader
	reader := csv.NewReader(file)
	// Read all records
	records, err := reader.ReadAll()
	fmt.Printf("Read %d records for %s\n", len(records), filename)
	if err != nil {
		return nil, fmt.Errorf("error reading CSV: %v", err)
	}

	// Check if file is empty
	if len(records) == 0 {
		return nil, fmt.Errorf("CSV file is empty")
	}

	// Check if columnIndex is valid
	if len(records[0]) <= columnIndex {
		return nil, fmt.Errorf("column index %d is out of range. File has %d columns", columnIndex, len(records[0]))
	}

	// Calculate starting index based on whether to skip header
	startIdx := 0
	if skipHeader {
		startIdx = 1
	}

	// Extract the specified column
	result := make([]string, 0, len(records)-startIdx)
	for i := startIdx; i < len(records); i++ {
		result = append(result, records[i][columnIndex])
	}

	return result, nil
}

func GetRandomClient(resolverClient *[]resolver.ResolverClient) resolver.ResolverClient {
	if len(*resolverClient) == 0 {
		return nil
	}
	randIdx := rand.Intn(len(*resolverClient))
	return (*resolverClient)[randIdx]
}

func asyncRequest(ctx context.Context, ackChannel *chan Ack, resolverClient *[]resolver.ResolverClient, request *Query, rateLimit *RateLimit, counter *atomic.Int64) {
	rateLimit.Acquire()
	conn := GetRandomClient(resolverClient)

	start := time.Now() // Record start time
	resp, err := conn.ExecuteQuery(ctx, request.requestQuery)
	latency := time.Since(start) // Calculate latency
	queryType := request.requestQuery.QueryType
	rateLimit.Release()

	if err != nil {
		// fmt.Println(err)
		*ackChannel <- Ack{hadError: true, latency: latency, QueryType: queryType, ErrorString: err, responseSize: -1}
	} else {
		if len(resp.Values) > 0 {
			counter.Add(1)
		}
		*ackChannel <- Ack{hadError: false, latency: latency, QueryType: queryType, ErrorString: nil, responseSize: len(resp.Values)}
	}
}

func sendRequestsForever(ctx context.Context, ackChannel *chan Ack, requests *[]Query, resolverClient *[]resolver.ResolverClient, rateLimit *RateLimit, counter *atomic.Int64) {
	for _, request := range *requests {
		select {
		case <-ctx.Done():
			return
		default:
			go asyncRequest(ctx, ackChannel, resolverClient, &request, rateLimit, counter)
		}
	}
}

// Modified getResponses to collect latencies
func getResponses(ctx context.Context, ackChannel *chan Ack, latencies *[]string) (int, int) {
	operations, errors := 0, 0
	for {
		select {
		case <-ctx.Done():
			return operations, errors
		case ack := <-*ackChannel:
			if ack.hadError {
				errors++
			} else {
				operations++
			}
			*latencies = append(*latencies, ack.latency.String()+"_"+ack.QueryType+"_"+strconv.Itoa(ack.responseSize))
		}
	}
}

func runBenchmark(resolverClient *[]resolver.ResolverClient, requests *[]Query, rateLimit *RateLimit, duration int, warmup bool) (int, int, time.Duration) {
	realRequestCounter := atomic.Int64{}
	latencies := []string{}
	ackChannel := make(chan Ack)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(duration)*time.Second)
	defer cancel()

	go sendRequestsForever(ctx, &ackChannel, requests, resolverClient, rateLimit, &realRequestCounter)
	ops, err := getResponses(ctx, &ackChannel, &latencies)
	if !warmup {
		fmt.Printf("Ops/s,%d,Err,%d\n", ops, err)
	}
	fmt.Printf("Non-Empty Requests: %d\n", realRequestCounter.Load())

	var totalLatency time.Duration
	for _, l := range latencies {
		parts := strings.Split(l, "_")
		if len(parts) > 0 {
			latency, err := time.ParseDuration(parts[0])
			if err == nil {
				totalLatency += latency
			}
		}
	}
	averageLatency := time.Duration(0)
	if len(latencies) > 0 {
		averageLatency = totalLatency / time.Duration(len(latencies))
	}
	// Write all latencies to file
	if !warmup {
		timestamp := time.Now().Format("2006-01-02_15-04-05")
		filename := fmt.Sprintf("latencies_benchmark_%s.txt", timestamp)
		file, err := os.Create(filename)
		if err != nil {
			fmt.Println("Error creating file:", err)
		} else {
			defer file.Close()
			for _, latency := range latencies {
				_, err = file.WriteString(fmt.Sprintf("%v\n", latency))
				if err != nil {
					fmt.Println("Error writing to file:", err)
					break
				}
			}
		}
	}
	// fmt.Printf("Average Latency: %v\n", averageLatency)
	return ops, err, averageLatency
}

func StartBench(resolverClient *[]resolver.ResolverClient, inFlight int, timeDuration int, queryType string, joinRange int, rangeSize int) {
	itemIDFile := os.Getenv("ITEM_ID_FILE")
	if itemIDFile == "" {
		itemIDFile = "../../pkg/benchmark/benchmarkIdLists/i_id.csv"
	}

	userIDFile := os.Getenv("USER_ID_FILE")
	if userIDFile == "" {
		userIDFile = "../../pkg/benchmark/benchmarkIdLists/u_id.csv"
	}

	aIDFile := os.Getenv("A_ID_FILE")
	if aIDFile == "" {
		aIDFile = "../../pkg/benchmark/benchmarkIdLists/a_id.csv"
	}

	id_creation_pair_File := os.Getenv("PAIR_ID_FILE")
	if id_creation_pair_File == "" {
		id_creation_pair_File = "../../pkg/benchmark/benchmarkIdLists/id_creation_pairs.csv"
	}

	pageRankFile := os.Getenv("pageRankFile")
	if pageRankFile == "" {
		pageRankFile = "../../pkg/benchmark/benchmarkIdLists/pageRank.csv"
	}

	item_id_list, err := ReadCSVColumn(itemIDFile, 0, true)
	if err != nil {
		log.Fatal(err)
	}

	user_id_list, err := ReadCSVColumn(userIDFile, 0, true)
	if err != nil {
		log.Fatal(err)
	}

	a_id_list, err := ReadCSVColumn(aIDFile, 0, true)
	if err != nil {
		log.Fatal(err)
	}

	pair_date_list, err := ReadCSV(id_creation_pair_File)
	if err != nil {
		log.Fatal(err)
	}

	pageRank_list, err := ReadCSVColumn(pageRankFile, 0, true)
	if err != nil {
		log.Fatal(err)
	}
	selectionSeed := int64(13091999) //Random Seed

	requestsWarmup := []Query{}
	for len(requestsWarmup) < 50000 {
		if queryType == "default" {
			requestsWarmup = append(requestsWarmup, getTestCases(&user_id_list, &item_id_list, &a_id_list, &pageRank_list, &pair_date_list, selectionSeed, joinRange, rangeSize)...)
		} else if queryType == "scaling" {
			requestsWarmup = append(requestsWarmup, getTestCasesScaling(&user_id_list, &item_id_list, &a_id_list, &pageRank_list, &pair_date_list, selectionSeed)...)

		} else if queryType == "epinions" {
			requestsWarmup = append(requestsWarmup, getTestCasesEpinion(&user_id_list, &item_id_list, &a_id_list, &pageRank_list, &pair_date_list, selectionSeed)...)

		} else if queryType == "bdb" {
			requestsWarmup = append(requestsWarmup, getTestCasesBDB(&user_id_list, &item_id_list, &a_id_list, &pageRank_list, &pair_date_list, selectionSeed)...)

		}
	}
	requestsBench := []Query{}
	for len(requestsBench) < 500000 {
		if queryType == "default" {
			requestsBench = append(requestsBench, getTestCases(&user_id_list, &item_id_list, &a_id_list, &pageRank_list, &pair_date_list, selectionSeed, joinRange, rangeSize)...)
		} else if queryType == "scaling" {
			requestsBench = append(requestsBench, getTestCasesScaling(&user_id_list, &item_id_list, &a_id_list, &pageRank_list, &pair_date_list, selectionSeed)...)

		} else if queryType == "epinions" {
			requestsBench = append(requestsBench, getTestCasesEpinion(&user_id_list, &item_id_list, &a_id_list, &pageRank_list, &pair_date_list, selectionSeed)...)

		} else if queryType == "bdb" {
			requestsBench = append(requestsBench, getTestCasesBDB(&user_id_list, &item_id_list, &a_id_list, &pageRank_list, &pair_date_list, selectionSeed)...)

		}
	}

	fmt.Println("In-Flight Requests:", inFlight)
	rateLimit := NewRateLimit(inFlight)
	ops1, err1, _ := runBenchmark(resolverClient, &requestsWarmup, rateLimit, 10, true)
	fmt.Printf("Warmup Done! %d %d\n", ops1, err1)
	fmt.Println("-------")
	fmt.Printf("Running Benchmark! %d seconds \n", timeDuration)
	rateLimitNew := NewRateLimit(inFlight)
	ops2, err2, lat2 := runBenchmark(resolverClient, &requestsBench, rateLimitNew, timeDuration, false)

	fmt.Printf("Total Ops: %d\n", ops2)
	fmt.Printf("Total Err: %d\n", err2)
	fmt.Printf("Average Latency: %v ms\n", lat2.Milliseconds())
}
