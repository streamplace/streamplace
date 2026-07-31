package webhook

import (
	"context"
	"fmt"
	"strings"

	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/integrations/discord"
	"stream.place/streamplace/pkg/integrations/discord/discordtypes"
	"stream.place/streamplace/pkg/placestream"
)

// SendChatWebhook sends chat message to a specific webhook
func SendChatWebhook(ctx context.Context, webhook *placestream.ServerDefs_Webhook, authorDID string, scm *placestream.ChatDefs_MessageView) error {
	// Check if message should be muted
	if msg, ok := scm.Record.Val.(*placestream.ChatMessage); ok {
		if len(webhook.MuteWords) > 0 {
			messageText := strings.ToLower(msg.Text)
			for _, muteWord := range webhook.MuteWords {
				if strings.Contains(messageText, strings.ToLower(muteWord)) {
					// Message contains a mute word, skip forwarding
					return nil
				}
			}
		}
	}

	discordWebhook, err := webhookToDiscordWebhook(webhook)
	if err != nil {
		return fmt.Errorf("failed to convert webhook: %w", err)
	}

	return discord.SendChat(ctx, discordWebhook, authorDID, scm)
}

// SendLivestreamWebhook sends livestream notification to a specific webhook
func SendLivestreamWebhook(ctx context.Context, webhook *placestream.ServerDefs_Webhook, pdsURL string, lsv *placestream.Livestream_LivestreamView, postView *appbsky.FeedDefs_PostView, spcp *placestream.ChatProfile) error {
	discordWebhook, err := webhookToDiscordWebhook(webhook)
	if err != nil {
		return fmt.Errorf("failed to convert webhook: %w", err)
	}

	return discord.SendLivestream(ctx, discordWebhook, pdsURL, lsv, postView, spcp)
}

// SendStreamReceivedWebhook sends a stream.received event to a specific webhook.
func SendStreamReceivedWebhook(ctx context.Context, webhook *placestream.ServerDefs_Webhook, streamerDID string) error {
	discordWebhook, err := webhookToDiscordWebhook(webhook)
	if err != nil {
		return fmt.Errorf("failed to convert webhook: %w", err)
	}

	return discord.SendStreamReceived(ctx, discordWebhook, streamerDID)
}

// webhookToDiscordWebhook converts placestream.ServerDefs_Webhook to discordtypes.Webhook
func webhookToDiscordWebhook(webhook *placestream.ServerDefs_Webhook) (*discordtypes.Webhook, error) {
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

// Sender adapts the package-level senders to statedb.WebhookSender so the
// queue processor can invoke them through an injected interface without
// pkg/statedb importing this package. Stateless; the zero value is ready
// to use. (The unqualified calls resolve to the package-level functions,
// not to these methods.)
type Sender struct{}

func (Sender) SendChatWebhook(ctx context.Context, wh *placestream.ServerDefs_Webhook, authorDID string, scm *placestream.ChatDefs_MessageView) error {
	return SendChatWebhook(ctx, wh, authorDID, scm)
}

func (Sender) SendLivestreamWebhook(ctx context.Context, wh *placestream.ServerDefs_Webhook, pdsURL string, lsv *placestream.Livestream_LivestreamView, postView *appbsky.FeedDefs_PostView, spcp *placestream.ChatProfile) error {
	return SendLivestreamWebhook(ctx, wh, pdsURL, lsv, postView, spcp)
}

func (Sender) SendStreamReceivedWebhook(ctx context.Context, wh *placestream.ServerDefs_Webhook, streamerDID string) error {
	return SendStreamReceivedWebhook(ctx, wh, streamerDID)
}
