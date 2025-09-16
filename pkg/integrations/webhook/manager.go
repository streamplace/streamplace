package webhook

import (
	"context"
	"fmt"

	"github.com/bluesky-social/indigo/api/bsky"
	"stream.place/streamplace/pkg/integrations/discord"
	"stream.place/streamplace/pkg/integrations/discord/discordtypes"
	"stream.place/streamplace/pkg/streamplace"
)

// SendChatWebhook sends chat message to a specific webhook
func SendChatWebhook(ctx context.Context, webhook *streamplace.ServerDefs_Webhook, authorDID string, scm *streamplace.ChatDefs_MessageView) error {
	discordWebhook, err := webhookToDiscordWebhook(webhook)
	if err != nil {
		return fmt.Errorf("failed to convert webhook: %w", err)
	}

	return discord.SendChat(ctx, discordWebhook, authorDID, scm)
}

// SendLivestreamWebhook sends livestream notification to a specific webhook
func SendLivestreamWebhook(ctx context.Context, webhook *streamplace.ServerDefs_Webhook, pdsURL string, lsv *streamplace.Livestream_LivestreamView, postView *bsky.FeedDefs_PostView, spcp *streamplace.ChatProfile) error {
	discordWebhook, err := webhookToDiscordWebhook(webhook)
	if err != nil {
		return fmt.Errorf("failed to convert webhook: %w", err)
	}

	return discord.SendLivestream(ctx, discordWebhook, pdsURL, lsv, postView, spcp)
}

// webhookToDiscordWebhook converts streamplace.ServerDefs_Webhook to discordtypes.Webhook
func webhookToDiscordWebhook(webhook *streamplace.ServerDefs_Webhook) (*discordtypes.Webhook, error) {
	var rewriteRules []*discordtypes.WebhookRewrite
	for _, rule := range webhook.Rewrite {
		rewriteRules = append(rewriteRules, &discordtypes.WebhookRewrite{
			From: rule.From,
			To:   rule.To,
		})
	}

	var prefix, suffix string
	if webhook.Prefix != nil {
		prefix = *webhook.Prefix
	}
	if webhook.Suffix != nil {
		suffix = *webhook.Suffix
	}

	return &discordtypes.Webhook{
		URL:     webhook.Url,
		Prefix:  prefix,
		Suffix:  suffix,
		Rewrite: rewriteRules,
	}, nil
}
