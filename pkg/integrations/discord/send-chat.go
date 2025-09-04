package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/context/ctxhttp"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/integrations/discord/discordtypes"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/streamplace"
)

func SendChat(ctx context.Context, w *discordtypes.Webhook, did string, scm *streamplace.ChatDefs_MessageView) error {

	msg, ok := scm.Record.Val.(*streamplace.ChatMessage)
	if !ok {
		return fmt.Errorf("failed to cast chat message to streamplace chat message")
	}

	avatarURL, err := getAvatarURL(ctx, did)
	if err != nil {
		log.Warn(ctx, "failed to get avatar URL", "err", err)
	}

	payload := discordtypes.Payload{
		Username: fmt.Sprintf("@%s", scm.Author.Handle),
		Content:  fmt.Sprintf("%s%s%s", w.Prefix, msg.Text, w.Suffix),
	}
	if avatarURL != "" {
		payload.AvatarURL = avatarURL
	}

	for _, rewrite := range w.Rewrite {
		payload.Content = strings.ReplaceAll(payload.Content, rewrite.From, rewrite.To)
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	log.Warn(ctx, "sending chat to discord", "payload", string(jsonPayload))

	req, err := http.NewRequestWithContext(ctx, "POST", w.URL, bytes.NewReader(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ctxhttp.Do(ctx, &aqhttp.Client, req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != 204 {
		return fmt.Errorf("failed to send chat to discord: %s", string(body))
	}

	log.Warn(ctx, "chat sent to discord", "payload", string(body))

	return nil
}
