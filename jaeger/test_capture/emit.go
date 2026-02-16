package main

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"google.golang.org/grpc"
)

func initTracer(ctx context.Context) (*sdktrace.TracerProvider, error) {
	// Connect to Jaeger all-in-one collector on localhost:4317
	conn, err := grpc.DialContext(ctx, "localhost:4317",
		grpc.WithInsecure(),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
	}

	exp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("go-jaeger-test"),
		)),
	)

	otel.SetTracerProvider(tp)
	return tp, nil
}

func main() {
	ctx := context.Background()
	tp, err := initTracer(ctx)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = tp.Shutdown(ctx)
	}()

	tracer := otel.Tracer("go-jaeger-test")

	// Generate a few traces
	for i := 1; i <= 5; i++ {
		func() {
			_, span := tracer.Start(ctx, "test-span")
			defer span.End()

			span.SetAttributes(
				attribute.String("iteration", fmt.Sprint(i)),
				attribute.String("status", "running"),
			)
			span.AddEvent("doing some work")
			time.Sleep(200 * time.Millisecond)
			span.AddEvent("work done")
		}()
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("✅ Sent test traces to Jaeger")
}
