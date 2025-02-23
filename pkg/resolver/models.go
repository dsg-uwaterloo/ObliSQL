package resolver

import (
	"sync"
	"sync/atomic"

	blobloom "github.com/greatroar/blobloom"
	loadBalancer "github.com/project/ObliSql/api/loadbalancer"
	"github.com/project/ObliSql/api/resolver"
	"go.opentelemetry.io/otel/trace"
)

type myResolver struct {
	resolver.UnimplementedResolverServer
	connPool           map[string]loadBalancer.LoadBalancerClient
	connPoolMutex      sync.RWMutex
	requestId          atomic.Int64
	done               atomic.Int32
	recvChan           chan int32
	metaData           map[string]MetaData
	JoinMap            []string
	Filters            map[string]*blobloom.Filter
	PartitionMap       map[string]int
	localRequestID     atomic.Int64
	tracer             trace.Tracer
	requestsDone       atomic.Int64
	selectIndexKeys    atomic.Int64
	selectFetchKeys    atomic.Int64
	selectRequests     atomic.Int64
	JoinFetchKeys      atomic.Int64
	joinRequests       atomic.Int64
	Created            atomic.Int64
	Inserted           atomic.Int64
	UseBloom           bool
	JoinBloomOptimized bool
}

type parsedQuery struct {
	client_id  string
	queryType  string
	tableName  string
	colToGet   []string
	searchCol  []string
	searchVal  []string
	searchType []string
}

type queryResponse struct {
	Keys   []string
	Values []string
}

type RangeIndex struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type MetaData struct {
	ColNames       []string              `json:"colNames"`
	IndexOn        []string              `json:"indexOn"`
	RangeIndexInfo map[string]RangeIndex `json:"rangeIndexInfo"`
	PkEnd          int                   `json:"pkEnd"`
	PkStart        int                   `json:"pkStart"`
	TableName      string                `json:"tableName"`
	ColTypes       map[string]string     `json:"colTypes"`
}
