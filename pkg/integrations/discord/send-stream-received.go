package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/integrations/discord/discordtypes"
	"stream.place/streamplace/pkg/log"
)

func SendStreamReceived(ctx context.Context, w *discordtypes.Webhook, streamerDID string) error {
	content := fmt.Sprintf("stream.received %s", streamerDID)
	for _, rewrite := range w.Rewrite {
		content = strings.ReplaceAll(content, rewrite.From, rewrite.To)
	}

	payload := discordtypes.Payload{
		Content: fmt.Sprintf("%s%s%s", w.Prefix, content, w.Suffix),
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	log.Log(ctx, "sending stream.received to discord", "streamerDID", streamerDID, "webhook_url", w.URL)

	req, err := http.NewRequestWithContext(ctx, "POST", w.URL, bytes.NewReader(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return sendWebhookRequest(ctx, req)
}

type webhookDispatchKey struct{}
type webhookDispatch func(context.Context, func(context.Context) error) error

// WithWebhookDispatch wraps the actual POST, after all request preparation.
func WithWebhookDispatch(ctx context.Context, dispatch func(context.Context, func(context.Context) error) error) context.Context {
	return context.WithValue(ctx, webhookDispatchKey{}, webhookDispatch(dispatch))
}

// ErrWebhookRateLimited indicates the service rejected the POST before delivery.
var ErrWebhookRateLimited = errors.New("webhook rate limited")

func sendWebhookRequest(ctx context.Context, req *http.Request) error {
	post := func(ctx context.Context) error {
		resp, err := aqhttp.Do(ctx, req)
		if err != nil {
			return fmt.Errorf("send webhook: %w", err)
		}
		defer resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusNoContent:
			return nil
		case http.StatusTooManyRequests:
			return ErrWebhookRateLimited
		default:
			body, err := readResponseBody(resp.Body)
			if err != nil {
				return fmt.Errorf("read webhook response: %w", err)
			}
			return fmt.Errorf("send webhook (http %d): %s", resp.StatusCode, body)
		}
	}
	if dispatch, ok := ctx.Value(webhookDispatchKey{}).(webhookDispatch); ok {
		return dispatch(ctx, post)
	}
	return post(ctx)
}
