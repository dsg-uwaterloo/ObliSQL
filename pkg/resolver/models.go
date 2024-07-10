package main

import (
	"sync/atomic"

	"github.com/Haseeb1399/WorkingThesis/api/loadbalancer"
	"github.com/Haseeb1399/WorkingThesis/api/resolver"
)

type myResolver struct {
	resolver.UnimplementedResolverServer
	conn      loadbalancer.LoadBalancerClient
	requestId atomic.Int64
	done      atomic.Int32
	recvChan  chan int32
	metaData  metaData
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

type metaData struct {
	colNames  []string
	indexOn   []string
	pkEnd     int
	pkStart   int
	tableName string
}
