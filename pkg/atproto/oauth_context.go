package atproto

import (
	"context"

	"stream.place/streamplace/pkg/model"
)

// singleton value to identify our logging metadata in context
var OAuthContextKey = oauthContextKeyType{}
var ModelContextKey = modelContextKeyType{}

// unique type to prevent assignment.
type oauthContextKeyType struct{}
type modelContextKeyType struct{}

func GetOAuthSession(ctx context.Context) (*model.OAuthSession, *XrpcClient) {
	session, ok := ctx.Value(OAuthContextKey).(*model.OAuthSession)
	if !ok {
		return nil, nil
	}
	model, ok := ctx.Value(ModelContextKey).(model.Model)
	if !ok {
		panic("model not found in context (but session was found)")
	}
	return session, GetXrpcClient(ctx, model, session)
}
