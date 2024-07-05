package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Haseeb1399/WorkingThesis/api/loadbalancer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
func (c *resolver) getFullTable(q *parsedQuery) (*queryResponse, error) {
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

func (c *resolver) getFullColumn(q *parsedQuery) (*queryResponse, error) {
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

func (c *resolver) doSelect(q *parsedQuery, isRange bool) (*queryResponse, error) {

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

func (c *resolver) readMetaData(filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Error opening metadata file: %s \n", err)
		return
	}
	defer file.Close()

	configMap := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) == 2 {
			key := parts[0]
			value := parts[1]
			configMap[key] = value
		}
	}

	var m metaData
	for key, value := range configMap {
		switch key {
		case "colNames":
			m.colNames = strings.Split(value, " ")
		case "indexOn":
			m.indexOn = strings.Split(value, " ")
		case "pkEnd":
			pkEnd, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				log.Fatalf("Error converting pkEnd to integer: %s \n", err)
			}
			m.pkEnd = pkEnd
		case "pkStart":
			pkStart, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				log.Fatalf("Error converting pkStart to integer: %s \n", err)
			}
			m.pkStart = pkStart
		case "tableName":
			m.tableName = value
		default:
			log.Printf("Unknown key in MetaData : %s \n", key)
		}
	}
	c.metaData = m
}

func main() {

	metaDataLoc := "./metadata.txt"

	lb_host := "localhost"
	lb_port := "9500"
	lb_addr := lb_host + ":" + lb_port

	conn, err := grpc.NewClient(lb_addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(644000*300), grpc.MaxCallSendMsgSize(644000*300)))

	if err != nil {
		log.Fatalf("Failed to open connection to load balancer")
	}
	lbClient := loadbalancer.NewLoadBalancerClient(conn)

	service := resolver{
		conn:      lbClient,
		done:      atomic.Int32{},
		requestId: atomic.Int64{},
		recvChan:  make(chan int32, 10005),
	}

	service.done.Store(0)
	service.readMetaData(metaDataLoc)

	var wg sync.WaitGroup
	wg.Add(5)
	start := time.Now()
	go func() {
		defer wg.Done()
		//Single Select with Index select c_balance where c_state = ON
		query := parsedQuery{
			client_id: "1",
			queryType: "select",
			tableName: "customer",
			colToGet:  []string{"c_balance"},
			searchCol: "c_state",
			searchVal: []string{"ON"},
		}

		resp1, err := service.doSelect(&query, false)
		if err != nil {
			log.Fatalln("Failed to do selection")
		}
		if len(resp1.Keys) == 13 {
			end := time.Now()
			fmt.Printf("Passed Single Query with Index. Returned 13 values. Took %vs to complete\n", end.Sub(start).Seconds())
			fmt.Println("--------------------------")
		} else {
			fmt.Println("Failed Single Query with Index")
		}

	}()

	go func() {
		defer wg.Done()
		//Single Select without Index
		query := parsedQuery{
			client_id: "1",
			queryType: "select",
			tableName: "customer",
			colToGet:  []string{"c_balance"},
			searchCol: "c_discount",
			searchVal: []string{"0.1400"},
		}

		resp2, err := service.doSelect(&query, false)
		if err != nil {
			log.Fatalln("Failed to do selection")
		}

		if len(resp2.Keys) == 582 {
			end := time.Now()
			fmt.Printf("Passed Single Query without Index. Returned 582 values. Took %vs to complete\n", end.Sub(start).Seconds())
			fmt.Println("--------------------------")
		} else {
			fmt.Println("Failed Single Query without Index")
		}
	}()

	go func() {
		defer wg.Done()
		// Select with range query (index)
		query := parsedQuery{
			client_id: "1",
			queryType: "select",
			tableName: "customer",
			colToGet:  []string{"c_balance"},
			searchCol: "c_since",
			searchVal: []string{"1711607656774", "1711607656776"},
		}

		resp3, err := service.doSelect(&query, true)

		if err != nil {
			log.Fatalln("Failed to do range selection")
		}

		if len(resp3.Keys) == 1701 {
			end := time.Now()
			fmt.Printf("Passed Range Query with Index. Returned 1701 values. Took %vs to complete\n", end.Sub(start).Seconds())
			fmt.Println("--------------------------")
		} else {
			fmt.Println("Failed Range Query with Index")
		}
	}()

	go func() {
		defer wg.Done()
		query := parsedQuery{
			client_id: "1",
			queryType: "select",
			tableName: "customer",
			colToGet:  []string{"c_balance"},
			searchCol: "c_d_id",
			searchVal: []string{"1", "4"},
		}

		resp4, err := service.doSelect(&query, true)

		if err != nil {
			log.Fatalln("Failed to do range selection")
		}
		if len(resp4.Keys) == 12000 {
			end := time.Now()
			fmt.Printf("Passed Range Query without Index. Returned 12000 values. Took %vs to complete\n", end.Sub(start).Seconds())
			fmt.Println("--------------------------")
		} else {
			fmt.Println("Failed Range Query without Index")
		}
	}()

	go func() {
		defer wg.Done()
		query := parsedQuery{
			client_id: "1",
			queryType: "select",
			tableName: "customer",
			colToGet:  []string{"c_balance"},
			searchCol: "",
			searchVal: []string{},
		}
		resp5, err := service.getFullColumn(&query)

		if err != nil {
			log.Fatalln("Failed to do fetch column")
		}

		if len(resp5.Keys) == 30000 {
			end := time.Now()
			fmt.Printf("Passed Fetch Column. Returned 30000 values. Took %vs to complete\n", end.Sub(start).Seconds())
			fmt.Println("--------------------------")
		} else {
			fmt.Println("Failed Fetch Column")
		}

	}()

	wg.Wait()
	end := time.Now()
	timeSeconds := end.Sub(start).Seconds()
	totalOps := 13 + 582 + 1701 + 12000 + 30000

	xops := float64(totalOps) / float64(timeSeconds)
	fmt.Printf("XOPS /s : %v \n", xops)

}

//Fixes (comments in the doSelect function)
//Make it moduluar (Convert to functions) and remove repeated code.
//Create test file and move test cases to file. (Via Network messages)
//Tests should also check for values.
//Convert to modular format.
//Remove all assumptions (Single column) --> Searching, getting multiple columns.
// >, < , >=,<= cases as well.
