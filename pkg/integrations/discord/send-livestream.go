package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/bluesky-social/indigo/api/bsky"
	"golang.org/x/net/context/ctxhttp"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/integrations/discord/discordtypes"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/streamplace"
)

func SendLivestream(ctx context.Context, w *discordtypes.Webhook, pdsURL string, lsv *streamplace.Livestream_LivestreamView, postView *bsky.FeedDefs_PostView, spcp *streamplace.ChatProfile) error {
	ctx = log.WithLogValues(ctx, "func", "SendLivestream")
	ls, ok := lsv.Record.Val.(*streamplace.Livestream)
	if !ok {
		return fmt.Errorf("failed to cast livestream view to livestream")
	}
	content := fmt.Sprintf("🔴 LIVE %s", ls.Title)

	for _, rewrite := range w.Rewrite {
		content = strings.ReplaceAll(content, rewrite.From, rewrite.To)
	}

	payload := discordtypes.Payload{
		Username: fmt.Sprintf("@%s", lsv.Author.Handle),
		Content:  fmt.Sprintf("%s%s%s", w.Prefix, content, w.Suffix),
	}

	avatarURL, err := getAvatarURL(ctx, lsv.Author.Did)
	if err != nil {
		log.Warn(ctx, "failed to get avatar URL", "err", err)
	}
	if avatarURL != "" {
		payload.AvatarURL = avatarURL
	}

	color := "f8baca"
	if spcp != nil && spcp.Color != nil {
		color = strings.TrimPrefix(model.ColorToHex(spcp.Color), "#")
	}

	colorInt, err := strconv.ParseInt(color, 16, 64)
	if err != nil {
		log.Warn(ctx, "failed to parse color", "err", err)
	}
	payload.Embeds = []discordtypes.Embed{
		{
			Color: int(colorInt),
		},
	}

	suffix := "!"
	if ls.Url != nil {
		u, err := url.Parse(*ls.Url)
		if err != nil {
			log.Warn(ctx, "failed to parse URL", "err", err)
		} else {
			suffix = fmt.Sprintf(" on %s!", u.Host)
			payload.Embeds[0].URL = fmt.Sprintf("%s/%s", *ls.Url, lsv.Author.Handle)
		}
	}

	payload.Embeds[0].Title = fmt.Sprintf("@%s is LIVE%s", lsv.Author.Handle, suffix)

	if ls.Thumb != nil {
		u, err := url.Parse(fmt.Sprintf("%s/xrpc/com.atproto.sync.getBlob", pdsURL))
		if err != nil {
			return fmt.Errorf("failed to parse base URL: %w", err)
		}
		q := u.Query()
		q.Set("did", lsv.Author.Did)
		q.Set("cid", ls.Thumb.Ref.String())
		u.RawQuery = q.Encode()
		imageURL := u.String()
		payload.Embeds[0].Image = &discordtypes.Image{
			URL: imageURL,
		}
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	log.Warn(ctx, "sending livestream to discord", "payload", string(jsonPayload))

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

	if resp.StatusCode != http.StatusNoContent {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		return fmt.Errorf("failed to send request (http %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
