package cmd

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/trace"
)

// setupOTelSDK bootstraps the OpenTelemetry pipeline.
// If it does not return an error, make sure to call shutdown for proper cleanup.
func startTelemetry(ctx context.Context, endpoint string) error {
	tracerProvider, err := newTracerProvider(ctx, endpoint)
	if err != nil {
		return err
	}

	otel.SetTracerProvider(tracerProvider)

	<-ctx.Done()

	return tracerProvider.Shutdown(ctx)
}

func newTracerProvider(ctx context.Context, endpoint string) (*trace.TracerProvider, error) {
	exp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpointURL(endpoint))
	if err != nil {
		return nil, err
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(exp,
			trace.WithBatchTimeout(time.Second)),
	)
	return tracerProvider, nil
}
