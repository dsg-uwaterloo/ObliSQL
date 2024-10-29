package resolver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"

	loadBalancer "github.com/project/ObliSql/api/loadbalancer"
	"github.com/project/ObliSql/api/resolver"
	cuckoo "github.com/seiflotfy/cuckoofilter"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func (c *myResolver) ExecuteQuery(ctx context.Context, q *resolver.ParsedQuery) (*resolver.QueryResponse, error) {
	requestID := c.localRequestID.Add(1)
	clientId, errConv := strconv.Atoi(q.ClientId)
	if errConv != nil {
		return nil, fmt.Errorf("error converting clientId to integer: %w", errConv)
	}

	var resp *queryResponse
	var err error
	switch q.QueryType {
	case "select":
		resp, err = c.doSelect(q, requestID)
	case "aggregate":
		resp, err = c.doAggregate(q, requestID)
	case "join":
		resp, err = c.doJoin(q, requestID)
	// case "update":
	// 	resp, err = c.doUpdate(q, requestID)
	default:
		return nil, fmt.Errorf("unsupported query type: %s", q.QueryType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to execute %s query with id:%d. error: %w", q.QueryType, requestID, err)
	} else {
		c.requestsDone.Add(1)
		return &resolver.QueryResponse{
			ClientId:  int64(clientId),
			RequestId: requestID,
			Keys:      resp.Keys,
			Values:    resp.Values,
		}, nil
	}
}

func (r *myResolver) ConnectPingResolver(ctx context.Context, req *resolver.ClientConnectResolver) (*resolver.ClientConnectResolver, error) {
	fmt.Println("Client Connected!")

	toRet := resolver.ClientConnectResolver{
		Id: req.Id,
	}

	return &toRet, nil
}

func (r *myResolver) InitDB(ctx context.Context, filePath string) bool {

	req := loadBalancer.LoadBalanceRequest{}

	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal().Msgf("Error opening metadata file: %s", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	totalKeys := 0
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, " ")
		if len(parts) == 3 {
			op := parts[0]
			if op == "SET" {
				totalKeys++
				key := parts[1]
				value := parts[2]
				req.Keys = append(req.Keys, key)
				req.Values = append(req.Values, value)
			}
		} else {
			log.Info().Msg("Invalid! " + strings.Join(parts, " "))
		}
	}
	log.Info().Msgf("Total Keys Read: %d", totalKeys)

	r.createFilters(req.Keys)

	success, err := r.conn.InitDB(ctx, &req)
	if err != nil {
		log.Fatal().Msgf("Failed to Initialize DB! %s \n", err)
	}
	log.Info().Msgf("Initialized DB! Total Keys: %d", totalKeys)
	return success.Value
}

func (r *myResolver) readJoinMap(filePath string) {
	//MetaData
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal().Msgf("Error opening metadata file: %s\n", err)
		return
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		log.Fatal().Msgf("Error reading metadata file: %s\n", err)
		return
	}

	var data map[string]interface{}
	err = json.Unmarshal(byteValue, &data)
	if err != nil {
		log.Fatal().Msgf("Error unmarshaling JSON: %s\n", err)
		return
	}

	r.JoinMap = data
	time.Sleep(1 * time.Second)
}

func (r *myResolver) readMetaData(filePath string) {
	//MetaData
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal().Msgf("Error opening metadata file: %s\n", err)
		return
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		log.Fatal().Msgf("Error reading metadata file: %s\n", err)
		return
	}

	var data map[string]MetaData
	err = json.Unmarshal(byteValue, &data)
	if err != nil {
		log.Fatal().Msgf("Error unmarshaling JSON: %s\n", err)
		return
	}
	// for table, meta := range data {
	// 	fmt.Printf("Table: %s, MetaData: %+v\n", table, meta)
	// }
	r.metaData = data
}
func (r *myResolver) createFilters(keys []string) {
	// Map to store separate filters for each table/index combination
	filters := make(map[string]*cuckoo.Filter)

	// Iterate over the keys
	for _, key := range keys {
		// Check if the key contains "_index/"
		if strings.Contains(key, "_index/") {
			// Extract table and column information from the key (assuming it's formatted like "table/column_index/...").
			parts := strings.Split(key, "_index/")
			if len(parts) < 2 {
				continue // Skip invalid keys
			}
			tableAndColumn := parts[0]
			tableParts := strings.Split(tableAndColumn, "/")
			if len(tableParts) < 2 {
				continue // Skip if we can't extract both table and column info
			}
			table := tableParts[0]
			column := tableParts[1]

			// Check if the table exists in the metadata
			if tableMeta, exists := r.metaData[table]; exists {
				// Check if the column is an index column in the metadata
				for _, indexColumn := range tableMeta.IndexOn {
					if indexColumn == column {
						// Create a unique key for this filter (table/column)
						filterKey := fmt.Sprintf("%s/%s", table, column)

						// If no filter exists for this combination, create one
						if _, ok := filters[filterKey]; !ok {
							filters[filterKey] = cuckoo.NewFilter(1000000)
						}

						// Add the key to the corresponding filter
						filters[filterKey].InsertUnique([]byte(key))
						break
					}
				}
			}
		}
	}
	// for k, v := range filters {
	// 	fmt.Println(k, ":", v.Count())
	// }

	r.Filters = filters
}

func NewResolver(ctx context.Context, lbAddr string, lbPort string, traceLocation string, metaDataLoc string, joinMapLoc string, tracer trace.Tracer) *myResolver {
	fullAddr := lbAddr + ":" + lbPort

	conn, err := grpc.NewClient(fullAddr, grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(600*1024*1024), grpc.MaxCallSendMsgSize(600*1024*1024)))

	if err != nil {
		log.Fatal().Msg("Could not connect to batcher.")
	}
	batcherClient := loadBalancer.NewLoadBalancerClient(conn)

	res, err := batcherClient.ConnectPing(context.Background(), &loadBalancer.ClientConnect{Id: "1"})

	if err != nil || res.Id != "1" {
		log.Fatal().Msg("Ping Unsuccessful")
	} else {
		fmt.Println("Connected to Batch Manager!")
	}

	service := myResolver{
		conn:            batcherClient,
		done:            atomic.Int32{},
		requestId:       atomic.Int64{},
		recvChan:        make(chan int32),
		localRequestID:  atomic.Int64{},
		tracer:          tracer,
		requestsDone:    atomic.Int64{},
		Filters:         make(map[string]*cuckoo.Filter),
		selectIndexKeys: atomic.Int64{},
		selectFetchKeys: atomic.Int64{},
		selectRequests:  atomic.Int64{},
		joinFetchKeys:   atomic.Int64{},
		joinRequests:    atomic.Int64{},
		Created:         atomic.Int64{},
		Inserted:        atomic.Int64{},
	}
	service.readMetaData(metaDataLoc)
	service.readJoinMap(joinMapLoc)
	// service.InitDB(ctx, traceLocation) //Initialize the DB

	return &service
}
