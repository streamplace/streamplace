package cmd

import (
	"context"
	"encoding/base64"
	"net/url"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
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
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	opts := []otlptracehttp.Option{otlptracehttp.WithEndpointURL(endpoint)}

	if parsedURL.User != nil {
		username := parsedURL.User.Username()
		password, _ := parsedURL.User.Password()
		encoded := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		opts = append(opts, otlptracehttp.WithHeaders(map[string]string{
			"Authorization": "Basic " + encoded,
		}))
	}

	exp, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(exp,
			trace.WithBatchTimeout(time.Second)),
	)
	return tracerProvider, nil
}
