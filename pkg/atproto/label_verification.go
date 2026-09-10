package atproto

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/xrpc"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

// Label-based verification: a network whose notion of "verified" is a label
// on the account (from its own labeler) rather than an app.bsky.graph.
// verification record. The branding keys labelerDid and verifiedLabels name
// the labeler and the label values that count; matching non-negated account
// labels are mirrored into the verification table as if the labeler had
// issued a verification record, so badges, the chat lock and getStatus all
// work unchanged. Labels are pulled from the labeler's queryLabels with its
// sequence cursor, persisted in statedb, so each poll only fetches what is
// new; a negation removes the mirrored row.

const labelPollInterval = time.Minute

func labelCursorKey(labeler string) string { return "labels-cursor:" + labeler }

// labelVerificationURI is the synthetic record URI of a mirrored label.
func labelVerificationURI(src, subject, val string) string {
	return fmt.Sprintf("label://%s/%s/%s", src, subject, val)
}

// labelMatches reports whether val is one of the configured verified labels.
func labelMatches(val string, patterns []string) bool {
	for _, p := range patterns {
		if strings.HasSuffix(p, "*") {
			if strings.HasPrefix(val, strings.TrimSuffix(p, "*")) {
				return true
			}
		} else if val == p {
			return true
		}
	}
	return false
}

// SeedLabelsForever mirrors the labeler's verified labels at boot and every
// minute after, following its cursor.
func (atsync *ATProtoSynchronizer) SeedLabelsForever(ctx context.Context) {
	for {
		if labeler, patterns := atsync.Labeler(ctx); labeler != "" {
			if err := atsync.seedLabels(ctx, labeler, patterns); err != nil {
				log.Warn(ctx, "failed to mirror labels", "labeler", labeler, "err", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(labelPollInterval):
		}
	}
}

type queryLabelsOut struct {
	Cursor string `json:"cursor"`
	Labels []struct {
		Src string  `json:"src"`
		Uri string  `json:"uri"`
		Cid *string `json:"cid"`
		Val string  `json:"val"`
		Neg bool    `json:"neg"`
		Cts string  `json:"cts"`
	} `json:"labels"`
}

func (atsync *ATProtoSynchronizer) seedLabels(ctx context.Context, labeler string, patterns []string) error {
	ident, err := atsync.resolveIdent(ctx, labeler, true)
	if err != nil {
		return err
	}
	host := ident.GetServiceEndpoint("atproto_labeler")
	if host == "" {
		return fmt.Errorf("%s has no atproto_labeler service", labeler)
	}
	xrpcc := xrpc.Client{Host: host, Client: SyncHTTPClient}
	cursor := ""
	if atsync.StatefulDB != nil {
		if conf, err := atsync.StatefulDB.GetConfig(labelCursorKey(labeler)); err == nil && conf != nil {
			cursor = string(conf.Value)
		}
	}
	added, removed := 0, 0
	for {
		params := map[string]any{
			// Account labels only: DIDs, not records.
			"uriPatterns": []string{"did:*"},
			"limit":       250,
		}
		if cursor != "" {
			params["cursor"] = cursor
		}
		var out queryLabelsOut
		if err := xrpcc.Do(ctx, xrpc.Query, "", "com.atproto.label.queryLabels", params, nil, &out); err != nil {
			return err
		}
		for _, l := range out.Labels {
			if l.Src != labeler || !strings.HasPrefix(l.Uri, "did:") || !labelMatches(l.Val, patterns) {
				continue
			}
			uri := labelVerificationURI(l.Src, l.Uri, l.Val)
			if l.Neg {
				if err := atsync.Model.DeleteVerification(ctx, uri); err != nil {
					log.Warn(ctx, "failed to remove negated label", "uri", uri, "err", err)
					continue
				}
				removed++
				continue
			}
			v := &model.Verification{
				URI:        uri,
				IssuerDID:  l.Src,
				SubjectDID: l.Uri,
			}
			if l.Cid != nil {
				v.CID = *l.Cid
			}
			if created, err := aqtime.FromString(l.Cts); err == nil {
				v.CreatedAt = created.Time()
			} else {
				v.CreatedAt = time.Now()
			}
			if err := atsync.Model.CreateVerification(ctx, v); err != nil && err != model.ErrAlreadyIndexed {
				log.Warn(ctx, "failed to mirror label", "uri", uri, "err", err)
				continue
			}
			added++
		}
		if out.Cursor == "" || out.Cursor == cursor || len(out.Labels) == 0 {
			break
		}
		cursor = out.Cursor
		if atsync.StatefulDB != nil {
			if err := atsync.StatefulDB.PutConfig(labelCursorKey(labeler), []byte(cursor)); err != nil {
				log.Warn(ctx, "failed to store label cursor", "err", err)
			}
		}
	}
	if added > 0 || removed > 0 {
		log.Log(ctx, "mirrored verified labels", "labeler", labeler, "added", added, "removed", removed, "cursor", cursor)
	}
	return nil
}
