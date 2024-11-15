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
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/exp/rand"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	loadBalancer "github.com/project/ObliSql/api/loadbalancer"
	"github.com/project/ObliSql/api/resolver"
	cuckoo "github.com/seiflotfy/cuckoofilter"
	"go.opentelemetry.io/otel/trace"
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
	case "update":
		resp, err = c.doUpdate(q, requestID)
		if err != nil {
			log.Info().Msgf("Update failed because: %s", err)
		}
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
	conn, err := r.GetBatchClient()
	if err != nil {
		log.Fatal().Msgf("Failed to get Batch Client!")
	}
	success, err := conn.InitDB(ctx, &req)
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

func (r *myResolver) readFilters(filePath string, joinName string) {
	r.Filters[joinName] = cuckoo.NewFilter(1000000)

	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal().Msgf("Failed to open file: %v", err)
	}
	defer file.Close()

	// Step 2: Read the file's content
	byteValue, err := io.ReadAll(file)
	if err != nil {
		log.Fatal().Msgf("Failed to read file: %v", err)
	}

	// Step 3: Declare a variable to hold the parsed data
	var pairs [][]int

	// Step 4: Parse the JSON data
	err = json.Unmarshal(byteValue, &pairs)
	if err != nil {
		log.Fatal().Msgf("Error parsing JSON: %v", err)
	}
	for _, pair := range pairs {
		if len(pair) == 2 {
			reviewPK := pair[0]
			trustPK := pair[1]
			key := fmt.Sprintf("%d/%d", reviewPK, trustPK)
			res := r.Filters[joinName].Insert([]byte(key))
			if !res {
				log.Fatal().Msgf("Failed to insert into Filter")
			}
		} else {
			log.Fatal().Msgf("Invalid pair: %v", pair)
		}
	}

}

func (r *myResolver) connectToBatchers(lbHosts []string, lbPorts []string) {
	for i, _ := range lbHosts {

		lbAddr := lbHosts[i] + ":" + lbPorts[i]
		conn, err := grpc.NewClient(lbAddr, grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(
				grpc.MaxCallRecvMsgSize(600*1024*1024),
				grpc.MaxCallSendMsgSize(600*1024*1024)))
		if err != nil {
			log.Fatal().Msgf("Could not connect to load balancer at %s: %v", lbAddr, err)
		}

		client := loadBalancer.NewLoadBalancerClient(conn)
		res, err := client.ConnectPing(context.Background(), &loadBalancer.ClientConnect{Id: "1"})
		if err != nil || res.Id != "1" {
			log.Fatal().Msgf("Ping unsuccessful for load balancer at %s", lbAddr)
		} else {
			log.Info().Msgf("Connected to load balancer at %s", lbAddr)
		}

		r.connPoolMutex.Lock()
		r.connPool[lbAddr] = client
		r.connPoolMutex.Unlock()
	}
}

func (r *myResolver) GetBatchClient() (loadBalancer.LoadBalancerClient, error) {
	r.connPoolMutex.RLock()
	defer r.connPoolMutex.RUnlock()

	// Get all keys (addresses) from the connPool map
	keys := make([]string, 0, len(r.connPool))
	for addr := range r.connPool {
		keys = append(keys, addr)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no available batch clients")
	}

	// Select a random key
	randomKey := keys[rand.Intn(len(keys))]

	// Return the corresponding client
	return r.connPool[randomKey], nil
}

func NewResolver(ctx context.Context, lbAddr []string, lbPort []string, traceLocation string, metaDataLoc string, joinMapLoc string, tracer trace.Tracer) *myResolver {

	// Seed the random generator (ideally, do this once in an init function)
	rand.Seed(uint64(time.Now().UnixNano()))

	service := myResolver{
		connPool:        make(map[string]loadBalancer.LoadBalancerClient),
		connPoolMutex:   sync.RWMutex{},
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

	service.connectToBatchers(lbAddr, lbPort)

	service.readMetaData(metaDataLoc)
	service.readJoinMap(joinMapLoc)
	// service.InitDB(ctx, traceLocation) //Initialize the DB

	service.readFilters("../../metaData/JoinMaps/pairList/pairs_review_trust.json", "review,trust")
	service.readFilters("../../metaData/JoinMaps/pairList/pairs_item_review.json", "review,item")

	return &service
}
