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
	metaData  map[string]MetaData
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
