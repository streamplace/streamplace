package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"golang.org/x/net/context/ctxhttp"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/integrations/discord/discordtypes"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/streamplace"
)

func SendChat(ctx context.Context, w *discordtypes.Webhook, did string, scm *streamplace.ChatDefs_MessageView) error {

	resolv := aqhttp.NewDoHResolver("")
	targetIP, parsedURL, err := resolv.ValidateAndGetIP(w.URL)
	if err != nil {
		return fmt.Errorf("webhook URL validation failed: %w", err)
	}

	msg, ok := scm.Record.Val.(*streamplace.ChatMessage)
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

	log.Warn(ctx, "sending chat to discord", "payload", string(jsonPayload))

	port := parsedURL.Port()
	if port == "" {
		// Use default port based on scheme
		if parsedURL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	requestURL := fmt.Sprintf("%s://%s%s", parsedURL.Scheme, net.JoinHostPort(targetIP, port), parsedURL.RequestURI())

	req, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewReader(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// set Host header to the original hostname
	req.Host = parsedURL.Host

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
