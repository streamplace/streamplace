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

	// The sender's avatar only shows in the default format; the streamplace
	// format uses the webhook's own avatar, so skip the network fetch.
	var avatarURL string
	var err error
	if !w.StreamplaceFormat {
		avatarURL, err = GetAvatarURL(ctx, did)
		if err != nil {
			log.Warn(ctx, "failed to get avatar URL", "err", err)
		}
	}

	payload := buildChatPayload(w, scm.Author.Handle, msg, avatarURL)

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	log.Debug(ctx, "sending chat to discord", "payload", string(jsonPayload), "for_did", did)

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

// buildChatPayload builds the Discord webhook payload for a chat message.
//
// By default the message is posted under the chatter's handle ("@handle",
// with their avatar). With StreamplaceFormat set, it is posted as
// "[Streamplace]" using the webhook's own avatar, with the handle inline:
// "**@handle**: text". The avatar URL is only used in the default format.
func buildChatPayload(w *discordtypes.Webhook, handle string, msg *placestream.ChatMessage, avatarURL string) discordtypes.Payload {
	payload := discordtypes.Payload{
		Content: fmt.Sprintf("%s%s%s", w.Prefix, msg.Text, w.Suffix),
	}
	if w.StreamplaceFormat {
		payload.Username = "[Streamplace]"
		payload.Content = fmt.Sprintf("**@%s**: %s", handle, payload.Content)
	} else {
		payload.Username = fmt.Sprintf("@%s", handle)
		if avatarURL != "" {
			payload.AvatarURL = avatarURL
		}
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

	return payload
}
