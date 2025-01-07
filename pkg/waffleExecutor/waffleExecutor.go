package waffleExecutor

import (
	"context"
	"fmt"

	"github.com/apache/thrift/lib/go/thrift"
	waffle "github.com/project/ObliSql/api/gen-go/waffle"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type ProxyClient struct {
	client   *waffle.WaffleThriftClient
	response *waffle.WaffleThriftResponseClient
	tracer   trace.Tracer
}

func (p *ProxyClient) Init(host string, port int, traceObj trace.Tracer) error {
	// Create a configuration for the socket
	conf := &thrift.TConfiguration{
		// Set your configuration options here (if needed)
		// MaxFrameSize:   int32(600 * 1024 * 1024),
		// MaxMessageSize: int32(600 * 1024 * 1024),
	}

	// Use NewTSocketConf instead of NewTSocket
	socket := thrift.NewTSocketConf(fmt.Sprintf("%s:%d", host, port), conf)

	transport := thrift.NewTFramedTransportConf(socket, conf)
	protocol := thrift.NewTBinaryProtocolFactoryConf(conf)
	client := waffle.NewWaffleThriftClientFactory(transport, protocol)
	response := waffle.NewWaffleThriftResponseClientFactory(transport, protocol)

	if err := transport.Open(); err != nil {
		return fmt.Errorf("error opening transport: %v", err)
	}

	p.client = client
	p.response = response
	p.tracer = traceObj
	return nil
}

func (p *ProxyClient) GetClientID() (int64, error) {
	ctx := context.Background()
	clientID, err := p.client.GetClientID(ctx)
	if err != nil {
		return 0, fmt.Errorf("error calling GetClientID: %v", err)
	}
	return clientID, nil
}

func (p *ProxyClient) RegisterClientID(blockID int32, clientID int64) error {
	ctx := context.Background()
	err := p.client.RegisterClientID(ctx, blockID, clientID)
	if err != nil {
		return fmt.Errorf("error calling RegisterClientID: %v", err)
	}
	return nil
}

func (p *ProxyClient) Get(key string) (string, error) {
	ctx := context.Background()
	result, err := p.client.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("error calling Get: %v", err)
	}
	return result, nil
}

func (p *ProxyClient) Put(key, value string) error {
	ctx := context.Background()
	err := p.client.Put(ctx, key, value)
	if err != nil {
		return fmt.Errorf("error calling Put: %v", err)
	}
	return nil
}

func (p *ProxyClient) GetBatch(keys []string) ([]string, error) {
	ctx := context.Background()
	results, err := p.client.GetBatch(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("error calling GetBatch: %v", err)
	}
	return results, nil
}

func (p *ProxyClient) PutBatch(keys, values []string) error {
	ctx := context.Background()
	err := p.client.PutBatch(ctx, keys, values)
	if err != nil {
		return fmt.Errorf("error calling PutBatch: %v", err)
	}
	return nil
}

func (p *ProxyClient) MixBatch(keys, values []string, batchID int64) ([]string, error) {
	ctx := context.Background()

	ctx, span := p.tracer.Start(ctx, "Thrift MixBatch")
	defer span.End()
	span.SetAttributes(
		attribute.Int("keyLen", len(keys)),
		attribute.Int64("batch_id", batchID),
	)
	span.AddEvent("Sending Batch to Waffle")
	results, err := p.client.MixBatch(ctx, keys, values)
	span.AddEvent("Got Batch from Waffle")

	if err != nil {
		return nil, fmt.Errorf("error calling PutBatch: %v", err)
	}
	return results, nil
}

// func (p *ProxyClient) MixBatch(keys, values []string, batchID int64) ([]string, error) {
// 	// Start tracing
// 	ctx := context.Background()
// 	ctx, span := p.tracer.Start(ctx, "Thrift MixBatch")
// 	defer span.End()

// 	// Record attributes in the span
// 	span.SetAttributes(
// 		attribute.Int("keyLen", len(keys)),
// 		attribute.Int64("batch_id", batchID),
// 	)

// 	// Hard limit
// 	const maxBatchSize = 10000 //Can be some factor of B_R

// 	if len(keys) <= maxBatchSize {
// 		// Send one batch if within the limit
// 		span.AddEvent("Sending single batch to Waffle")

// 		results, err := p.client.MixBatch(ctx, keys, values)
// 		span.AddEvent("Got single batch response from Waffle")

// 		if err != nil {
// 			return nil, fmt.Errorf("error calling MixBatch: %v", err)
// 		}
// 		return results, nil

// 	} else {
// 		// Split into multiple batches
// 		span.AddEvent("Splitting into multiple batches for Waffle")

// 		var allResults []string
// 		for start := 0; start < len(keys); start += maxBatchSize {
// 			end := start + maxBatchSize
// 			if end > len(keys) {
// 				end = len(keys)
// 			}

// 			partKeys := keys[start:end]
// 			partValues := values[start:end]

// 			// We can optionally create sub-spans for each batch if desired.
// 			subCtx, subSpan := p.tracer.Start(ctx, "Thrift MixBatch - subBatch")
// 			subSpan.SetAttributes(
// 				attribute.Int("subBatch_keyLen", len(partKeys)),
// 			)
// 			subSpan.AddEvent("Sending sub-batch to Waffle")

// 			partResults, err := p.client.MixBatch(subCtx, partKeys, partValues)

// 			subSpan.AddEvent("Got sub-batch response from Waffle")
// 			subSpan.End()

// 			if err != nil {
// 				return nil, fmt.Errorf("error calling MixBatch (sub-batch): %v", err)
// 			}

// 			// Accumulate results from this sub-batch
// 			allResults = append(allResults, partResults...)
// 		}

// 		return allResults, nil
// 	}
// }

func (p *ProxyClient) InitDB(keys, values []string) {
	ctx := context.Background()
	ctx, span := p.tracer.Start(ctx, "Thrift InitDB")
	defer span.End()
	span.AddEvent("Initializing DB on Waffle")
	err := p.client.InitDb(ctx, keys, values)
	span.AddEvent("Finished Initialization")

	if err != nil {
		log.Fatal().Msgf("Error Initializing DB: %v", err)
	}
}

func (p *ProxyClient) InitArgs(B, R, F, D, C, N int64) {
	ctx := context.Background()
	err := p.client.InitArgs_(ctx, B, R, F, D, C, N)
	if err != nil {
		log.Fatal().Msgf("Error Initializing Arguments: %v", err)
	}
}
