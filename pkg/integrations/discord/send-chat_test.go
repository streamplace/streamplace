package discord

import (
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/integrations/discord/discordtypes"
	"stream.place/streamplace/pkg/placestream"
)

func testPayload(w *discordtypes.Webhook, text, avatarURL string) discordtypes.Payload {
	return buildChatPayload(w, "natalie", &placestream.ChatMessage{Text: text}, avatarURL)
}

func TestBuildChatPayloadDefaultFormat(t *testing.T) {
	w := &discordtypes.Webhook{Prefix: "[Streamplace] ", Suffix: "!"}

	payload := testPayload(w, "hello world", "https://example.com/avatar.png")

	require.Equal(t, "@natalie", payload.Username)
	require.Equal(t, "[Streamplace] hello world!", payload.Content)
	require.Equal(t, "https://example.com/avatar.png", payload.AvatarURL)
}

func TestBuildChatPayloadDefaultFormatNoAvatar(t *testing.T) {
	w := &discordtypes.Webhook{}

	payload := testPayload(w, "hello", "")

	require.Equal(t, "@natalie", payload.Username)
	require.Equal(t, "hello", payload.Content)
	require.Empty(t, payload.AvatarURL)
}

func TestBuildChatPayloadStreamplaceFormat(t *testing.T) {
	w := &discordtypes.Webhook{StreamplaceFormat: true, Prefix: "p ", Suffix: " s"}

	payload := testPayload(w, "hello world", "https://example.com/avatar.png")

	require.Equal(t, "[Streamplace]", payload.Username)
	require.Equal(t, "**@natalie**: p hello world s", payload.Content)
	// The avatar is dropped in streamplace format so the webhook's own
	// avatar shows instead of a random chatter's.
	require.Empty(t, payload.AvatarURL)
}

func TestBuildChatPayloadAntiPing(t *testing.T) {
	w := &discordtypes.Webhook{StreamplaceFormat: true}

	payload := testPayload(w, "hi @everyone and @here and <@1234>", "")

	require.Equal(t, "**@natalie**: hi @\u200Beveryone and @\u200Bhere and <@\u200B1234>", payload.Content)
}

func TestBuildChatPayloadCustomRewritesApplyInBothFormats(t *testing.T) {
	defaultPayload := testPayload(&discordtypes.Webhook{
		Rewrite: []*discordtypes.WebhookRewrite{{From: "foo", To: "bar"}},
	}, "foo baz", "")
	require.Equal(t, "@natalie", defaultPayload.Username)
	require.Equal(t, "bar baz", defaultPayload.Content)

	streamplacePayload := testPayload(&discordtypes.Webhook{
		StreamplaceFormat: true,
		Rewrite:           []*discordtypes.WebhookRewrite{{From: "foo", To: "bar"}},
	}, "foo baz", "")
	require.Equal(t, "[Streamplace]", streamplacePayload.Username)
	require.Equal(t, "**@natalie**: bar baz", streamplacePayload.Content)
}
