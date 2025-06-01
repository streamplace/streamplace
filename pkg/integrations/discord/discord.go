package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/bluesky-social/indigo/api/bsky"
	"golang.org/x/net/context/ctxhttp"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/integrations/discord/discordtypes"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/streamplace"
)

func SendLivestream(ctx context.Context, w *discordtypes.Webhook, r *model.Repo, lsv *streamplace.Livestream_LivestreamView, postView *bsky.FeedDefs_PostView) error {
	ctx = log.WithLogValues(ctx, "func", "SendLivestream")
	ls, ok := lsv.Record.Val.(*streamplace.Livestream)
	if !ok {
		return fmt.Errorf("failed to cast livestream view to livestream")
	}
	payload := discordtypes.Payload{
		Username: fmt.Sprintf("@%s", r.Handle),
		Content:  fmt.Sprintf("🔴 LIVE %s", ls.Title),
	}

	if ls.Thumb != nil {
		u, err := url.Parse(fmt.Sprintf("%s/xrpc/com.atproto.sync.getBlob", r.PDS))
		if err != nil {
			return fmt.Errorf("failed to parse base URL: %w", err)
		}
		q := u.Query()
		q.Set("did", r.DID)
		q.Set("cid", ls.Thumb.Ref.String())
		u.RawQuery = q.Encode()
		imageURL := u.String()
		payload.Embeds = []discordtypes.Embed{
			{
				Title: ls.Title,
				URL:   fmt.Sprintf("%s/%s", *ls.Url, r.Handle),
				Image: &discordtypes.Image{
					URL: imageURL,
				},
			},
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
