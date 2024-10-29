package resolver

import (
	"sync/atomic"

	loadBalancer "github.com/project/ObliSql/api/loadbalancer"
	"github.com/project/ObliSql/api/resolver"
	cuckoo "github.com/seiflotfy/cuckoofilter"
	"go.opentelemetry.io/otel/trace"
)

type myResolver struct {
	resolver.UnimplementedResolverServer
	conn            loadBalancer.LoadBalancerClient
	requestId       atomic.Int64
	done            atomic.Int32
	recvChan        chan int32
	metaData        map[string]MetaData
	JoinMap         map[string]interface{}
	Filters         map[string]*cuckoo.Filter
	localRequestID  atomic.Int64
	tracer          trace.Tracer
	requestsDone    atomic.Int64
	selectIndexKeys atomic.Int64
	selectFetchKeys atomic.Int64
	selectRequests  atomic.Int64
	joinFetchKeys   atomic.Int64
	joinRequests    atomic.Int64
	Created         atomic.Int64
	Inserted        atomic.Int64
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
