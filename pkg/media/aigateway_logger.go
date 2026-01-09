package media

import (
	"context"

	"stream.place/streamplace/pkg/log"
)

// aiGatewayLogger adapts streamplace logging to the SDK Logger interface.
type aiGatewayLogger struct{}

func (aiGatewayLogger) Debug(ctx context.Context, msg string, args ...any) {
	log.Debug(ctx, msg, args...)
}

func (aiGatewayLogger) Log(ctx context.Context, msg string, args ...any) {
	log.Log(ctx, msg, args...)
}

func (aiGatewayLogger) Warn(ctx context.Context, msg string, args ...any) {
	log.Warn(ctx, msg, args...)
}

func (aiGatewayLogger) Error(ctx context.Context, msg string, args ...any) {
	log.Error(ctx, msg, args...)
}

func newAIGatewayLogger() aiGatewayLogger {
	return aiGatewayLogger{}
}
