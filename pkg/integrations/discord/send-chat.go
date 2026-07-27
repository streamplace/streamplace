package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/integrations/discord/discordtypes"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

func SendChat(ctx context.Context, w *discordtypes.Webhook, did string, scm *placestream.ChatDefs_MessageView) error {

	msg, ok := scm.Record.Val.(*placestream.ChatMessage)
	if !ok {
		return fmt.Errorf("failed to cast chat message to streamplace chat message")
	}

	avatarURL, err := GetAvatarURL(ctx, did)
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

	// apply default anti-ping rewrites
	payload.Content = strings.ReplaceAll(payload.Content, "@here", "@\u200Bhere")
	payload.Content = strings.ReplaceAll(payload.Content, "@everyone", "@\u200Beveryone")
	// and for <@{userid/roleid}>
	payload.Content = strings.ReplaceAll(payload.Content, "<@", "<@\u200B")

	// then apply custom rewrites, in case user wants to undo the above or do something else
	for _, rewrite := range w.Rewrite {
		payload.Content = strings.ReplaceAll(payload.Content, rewrite.From, rewrite.To)
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	log.Warn(ctx, "sending chat to discord", "payload", string(jsonPayload), "for_did", w.DID)

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

	body, err := readResponseBody(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != 204 {
		log.Error(ctx, "chat webhook delivery failed", "webhook_url", w.URL, "status_code", resp.StatusCode, "response_body", body)
		return fmt.Errorf("failed to send chat to discord: %s", body)
	}

	return nil
}
