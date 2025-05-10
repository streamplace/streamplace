package atproto

import (
	"context"

	"stream.place/streamplace/pkg/model"
)

// singleton value to identify our logging metadata in context
var OAuthContextKey = oauthContextKeyType{}

// unique type to prevent assignment.
type oauthContextKeyType struct{}

func GetOAuthSession(ctx context.Context) *model.OAuthSession {
	session, ok := ctx.Value(OAuthContextKey).(*model.OAuthSession)
	if !ok {
		return nil
	}
	return session
}
