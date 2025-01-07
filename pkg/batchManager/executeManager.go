package batcher

import (
	"fmt"
	"strings"

	"github.com/project/ObliSql/pkg/oramClient"
	ptClient "github.com/project/ObliSql/pkg/plainTextClient"
	"github.com/project/ObliSql/pkg/waffleExecutor"
	"go.opentelemetry.io/otel/trace"
)

// ExecutorClient defines the methods that any executor client should implement.
type ExecutorClient interface {
	MixBatch(keys []string, values []string, batchID int64) ([]string, error)
}

// WaffleExecutorAdapter adapts waffleExecutor.ProxyClient to ExecutorClient interface.
type WaffleExecutorAdapter struct {
	client *waffleExecutor.ProxyClient
	tracer trace.Tracer
}

func NewWaffleExecutorAdapter(host string, port int, tracer trace.Tracer) (*WaffleExecutorAdapter, error) {
	client := &waffleExecutor.ProxyClient{}
	if err := client.Init(host, port, tracer); err != nil {
		return nil, err
	}
	client.GetClientID()
	return &WaffleExecutorAdapter{client: client, tracer: tracer}, nil
}

func (w *WaffleExecutorAdapter) MixBatch(keys []string, values []string, batchID int64) ([]string, error) {
	return w.client.MixBatch(keys, values, batchID)
}

// ORAMClientAdapter adapts oramClient.OramClient to ExecutorClient interface.
type ORAMClientAdapter struct {
	client *oramClient.OramClient
	tracer trace.Tracer
}

func NewORAMClientAdapter(host string, port int, tracer trace.Tracer) (*ORAMClientAdapter, error) {
	client := &oramClient.OramClient{}
	if err := client.CreateClient(host, port); err != nil {
		return nil, err
	}
	return &ORAMClientAdapter{client: client, tracer: tracer}, nil
}

func (o *ORAMClientAdapter) MixBatch(keys []string, values []string, batchID int64) ([]string, error) {
	return o.client.MixBatch(keys, values, batchID)
}

type plainTextClientAdapter struct {
	client *ptClient.PlainTextClient
	tracer trace.Tracer
}

func NewPlaintextClientAdapter(host string, port int, tracer trace.Tracer) (*plainTextClientAdapter, error) {
	client := &ptClient.PlainTextClient{}
	if err := client.CreateClient(host, port); err != nil {
		return nil, err
	}
	return &plainTextClientAdapter{client: client, tracer: tracer}, nil
}

func (p *plainTextClientAdapter) MixBatch(keys []string, values []string, batchID int64) ([]string, error) {
	return p.client.MixBatch(keys, values, batchID)
}

// Factory function to create ExecutorClient based on executorType
const (
	ExecutorTypeWaffle    = "waffle"
	ExecutorTypeORAM      = "oram"
	ExecutorTypePlaintext = "plaintext"
)

func NewExecutorClient(executorType, host string, port int, tracer trace.Tracer) (ExecutorClient, error) {
	switch strings.ToLower(executorType) {
	case ExecutorTypeWaffle:
		return NewWaffleExecutorAdapter(host, port, tracer)
	case ExecutorTypeORAM:
		return NewORAMClientAdapter(host, port, tracer)
	case ExecutorTypePlaintext:
		return NewPlaintextClientAdapter(host, port, tracer)
	default:
		return nil, fmt.Errorf("unsupported executor type: %s", executorType)
	}
}
