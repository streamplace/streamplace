package atproto

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/label"
	"github.com/bluesky-social/indigo/events"
	"github.com/bluesky-social/indigo/events/schedulers/parallel"
	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

func (atsync *ATProtoSynchronizer) StartLabelerFirehose(ctx context.Context, did string) error {
	retryCount := 0
	retryWindow := time.Now()

	for {
		if ctx.Err() != nil {
			return nil
		}
		err := atsync.StartLabelerFirehoseRetry(ctx, did)
		if err != nil {
			log.Error(ctx, "firehose error", "err", err)

			// Check if we're within the 1-minute window
			now := time.Now()
			if now.Sub(retryWindow) > time.Minute {
				// Reset the counter if more than a minute has passed
				retryCount = 1
				retryWindow = now
			} else {
				// Increment retry count if within the window
				retryCount++
				if retryCount >= 3 {
					log.Error(ctx, "firehose failed 3 times within a minute, crashing", "err", err)
					return fmt.Errorf("firehose failed 3 times within a minute: %w", err)
				}
			}
		}
	}
}

func (atsync *ATProtoSynchronizer) StartLabelerFirehoseRetry(ctx context.Context, did string) error {
	ctx = log.WithLogValues(ctx, "func", "StartLabelerFirehose")

	ident, err := ResolveIdent(ctx, did)
	if err != nil {
		return fmt.Errorf("failed to resolve DID %s: %w", did, err)
	}

	ctx = log.WithLogValues(ctx, "labelerDID", ident.DID.String(), "labelerHandle", ident.Handle.String())

	pub, err := ident.GetPublicKey("atproto_label")
	if err != nil {
		return fmt.Errorf("failed to get public key for labeler %s: %w", did, err)
	}

	labeler, ok := ident.Services["atproto_labeler"]
	if !ok {
		return fmt.Errorf("labeler %s does not have a atproto_labeler service", did)
	}

	ctx = log.WithLogValues(ctx, "func", "StartFirehose")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	dialer := websocket.DefaultDialer
	u, err := url.Parse(labeler.URL)
	if err != nil {
		return fmt.Errorf("invalid labeler URI: %w", err)
	}
	u.Path = "xrpc/com.atproto.label.subscribeLabels"
	if u.Scheme == "http" {
		u.Scheme = "ws"
	} else if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		return fmt.Errorf("invalid labeler URI scheme: %s", labeler.URL)
	}
	dbLabeler, err := atsync.Model.GetLabeler(did)
	if err != nil {
		return fmt.Errorf("failed to get labeler %s: %w", did, err)
	}
	if dbLabeler == nil {
		dbLabeler, err = atsync.Model.CreateLabeler(did)
		if err != nil {
			return fmt.Errorf("failed to create labeler %s: %w", did, err)
		}
	}
	query := u.Query()
	query.Set("cursor", fmt.Sprintf("%d", dbLabeler.Cursor))
	u.RawQuery = query.Encode()

	con, _, err := dialer.Dial(u.String(), http.Header{
		"User-Agent": []string{aqhttp.UserAgent},
	})
	if err != nil {
		return fmt.Errorf("subscribing to firehose failed (dialing): %w", err)
	}

	rsc := &events.RepoStreamCallbacks{
		LabelLabels: func(evt *comatproto.LabelSubscribeLabels_Labels) error {
			log.Log(ctx, "labeler labels", "labels", evt.Labels, "seq", evt.Seq)
			err = atsync.Model.UpdateLabelerCursor(did, evt.Seq)
			if err != nil {
				log.Error(ctx, "failed to update labeler cursor", "err", err)
			}
			for _, labelLex := range evt.Labels {
				l := label.FromLexicon(labelLex)
				err = l.VerifySignature(pub)
				if err != nil {
					log.Error(ctx, "failed to verify label signature", "err", err)
					continue
				}
				err = l.VerifySyntax()
				if err != nil {
					log.Error(ctx, "failed to verify label syntax", "err", err)
					continue
				}
				bs := bytes.Buffer{}
				err = labelLex.MarshalCBOR(&bs)
				if err != nil {
					log.Error(ctx, "failed to marshal label", "err", err)
					continue
				}
				err = atsync.Model.CreateLabel(&model.Label{
					Cid:    l.CID,
					Cts:    l.CreatedAt,
					Exp:    l.ExpiresAt,
					Neg:    l.Negated,
					Sig:    l.Sig,
					Src:    l.SourceDID,
					Uri:    l.URI,
					Val:    l.Val,
					Ver:    &l.Version,
					Record: bs.Bytes(),
				})
				if err != nil {
					log.Error(ctx, "failed to create label", "err", err)
					continue
				}
			}
			return nil
		},
		LabelInfo: func(evt *comatproto.LabelSubscribeLabels_Info) error {
			log.Log(ctx, "labeler info", "name", evt.Name, "message", evt.Message)
			return nil
		},
		Error: func(evt *events.ErrorFrame) error {
			log.Error(ctx, "firehose error", "err", evt.Error, "message", evt.Message)
			cancel()
			return fmt.Errorf("firehose error: %s", evt.Error)
		},
	}

	scheduler := parallel.NewScheduler(
		10,
		100,
		did,
		rsc.EventHandler,
	)

	log.Log(ctx, "starting labeler firehose consumer", "labelerDID", did)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return events.HandleRepoStream(ctx, con, scheduler, nil)
	})

	<-ctx.Done()

	return nil

}
