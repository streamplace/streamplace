package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	resp, err := aqhttp.Do(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		log.Error(ctx, "stream.received webhook delivery failed", "webhook_url", w.URL, "status_code", resp.StatusCode, "response_body", string(body))
		return fmt.Errorf("failed to send request (http %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
