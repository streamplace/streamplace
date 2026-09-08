package atproto

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/bluesky-social/indigo/xrpc"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/branding"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
)

// Verification on this node: the operator names trusted verifiers in the
// branding key verifierDids; any app.bsky.graph.verification record one of
// them holds for a user makes that user "verified" here. The records are
// indexed from the firehose (see sync.go) and, because a verifier's history
// predates this node, also pulled from the verifier's repo on a schedule.

const verifierCacheTTL = 30 * time.Second

var (
	verifierMu     sync.Mutex
	verifierCached []string
	verifierAt     time.Time
	verifiedOnlyC  bool
)

// Verifiers returns the trusted verifier DIDs, cached briefly.
func (atsync *ATProtoSynchronizer) Verifiers(ctx context.Context) []string {
	verifierMu.Lock()
	defer verifierMu.Unlock()
	if time.Since(verifierAt) < verifierCacheTTL {
		return verifierCached
	}
	verifierCached = nil
	verifiedOnlyC = false
	if atsync.CLI != nil && atsync.StatefulDB != nil {
		raw := branding.Text(atsync.StatefulDB, atsync.CLI.BroadcasterHost, "verifierDids")
		if raw != "" {
			var dids []string
			if err := json.Unmarshal([]byte(raw), &dids); err == nil {
				for _, d := range dids {
					d = strings.TrimSpace(d)
					if strings.HasPrefix(d, "did:") {
						verifierCached = append(verifierCached, d)
					}
				}
			}
		}
		verifiedOnlyC = branding.Text(atsync.StatefulDB, atsync.CLI.BroadcasterHost, "chatVerifiedOnly") == "on"
	}
	verifierAt = time.Now()
	return verifierCached
}

// ChatVerifiedOnly reports whether chat is restricted to verified users.
func (atsync *ATProtoSynchronizer) ChatVerifiedOnly(ctx context.Context) bool {
	atsync.Verifiers(ctx)
	verifierMu.Lock()
	defer verifierMu.Unlock()
	return verifiedOnlyC && len(verifierCached) > 0
}

// verificationsOf returns the trusted verifications for did, or nil.
func (atsync *ATProtoSynchronizer) verificationsOf(ctx context.Context, did string) []model.Verification {
	verifiers := atsync.Verifiers(ctx)
	if len(verifiers) == 0 || did == "" {
		return nil
	}
	found, err := atsync.Model.VerificationsFor(ctx, []string{did}, verifiers)
	if err != nil {
		log.Error(ctx, "failed to look up verifications", "did", did, "err", err)
		return nil
	}
	return found[did]
}

// IsVerified reports whether a trusted verifier vouches for did.
func (atsync *ATProtoSynchronizer) IsVerified(ctx context.Context, did string) bool {
	return len(atsync.verificationsOf(ctx, did)) > 0
}

// DecorateVerification fills the author's verification state on a chat
// message view from this node's trusted verifiers, in the shape the app view
// uses, so the app renders the same badge either way.
func (atsync *ATProtoSynchronizer) DecorateVerification(ctx context.Context, message *placestream.ChatDefs_MessageView) {
	if message == nil {
		return
	}
	vs := atsync.verificationsOf(ctx, message.Author.Did)
	if len(vs) == 0 {
		return
	}
	state := &appbsky.ActorDefs_VerificationState{
		VerifiedStatus:        "valid",
		TrustedVerifierStatus: "none",
	}
	for _, v := range vs {
		state.Verifications = append(state.Verifications, appbsky.ActorDefs_VerificationView{
			CreatedAt: aqtime.FromTime(v.CreatedAt).String(),
			IsValid:   true,
			Issuer:    v.IssuerDID,
			Uri:       v.URI,
		})
	}
	message.Author.Verification = state
}

// ChatAllowed reports whether a message from did may be shown, given the
// chat lock: everyone when the lock is off, verified users when it is on.
func (atsync *ATProtoSynchronizer) ChatAllowed(ctx context.Context, did string) bool {
	if !atsync.ChatVerifiedOnly(ctx) {
		return true
	}
	return atsync.IsVerified(ctx, did)
}

// SeedVerificationsForever pulls every verification record from each trusted
// verifier's repo at boot and hourly after, so verifications issued before
// this node existed (or while it was down) count too. The firehose keeps
// the table current in between.
func (atsync *ATProtoSynchronizer) SeedVerificationsForever(ctx context.Context) {
	for {
		for _, did := range atsync.Verifiers(ctx) {
			if err := atsync.seedVerifier(ctx, did); err != nil {
				log.Warn(ctx, "failed to seed verifications", "verifier", did, "err", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Hour):
		}
	}
}

type listRecordsOut struct {
	Cursor  string `json:"cursor"`
	Records []struct {
		Uri   string          `json:"uri"`
		Cid   string          `json:"cid"`
		Value json.RawMessage `json:"value"`
	} `json:"records"`
}

func (atsync *ATProtoSynchronizer) seedVerifier(ctx context.Context, did string) error {
	ident, err := atsync.resolveIdent(ctx, did, true)
	if err != nil {
		return err
	}
	xrpcc := xrpc.Client{Host: ident.PDSEndpoint(), Client: SyncHTTPClient}
	if xrpcc.Host == "" {
		return nil
	}
	cursor := ""
	total := 0
	for {
		params := map[string]any{
			"repo":       did,
			"collection": constants.APP_BSKY_GRAPH_VERIFICATION,
			"limit":      100,
		}
		if cursor != "" {
			params["cursor"] = cursor
		}
		var out listRecordsOut
		if err := xrpcc.Do(ctx, xrpc.Query, "", "com.atproto.repo.listRecords", params, nil, &out); err != nil {
			return err
		}
		for _, r := range out.Records {
			var rec appbsky.GraphVerification
			if err := json.Unmarshal(r.Value, &rec); err != nil || rec.Subject == "" {
				continue
			}
			v := &model.Verification{
				URI:         r.Uri,
				CID:         r.Cid,
				IssuerDID:   did,
				SubjectDID:  rec.Subject,
				Handle:      rec.Handle,
				DisplayName: rec.DisplayName,
			}
			if created, err := aqtime.FromString(rec.CreatedAt); err == nil {
				v.CreatedAt = created.Time()
			}
			if err := atsync.Model.CreateVerification(ctx, v); err != nil && err != model.ErrAlreadyIndexed {
				log.Warn(ctx, "failed to index seeded verification", "uri", r.Uri, "err", err)
				continue
			}
			total++
		}
		if out.Cursor == "" || len(out.Records) == 0 {
			break
		}
		cursor = out.Cursor
	}
	log.Log(ctx, "seeded verifications", "verifier", did, "records", total)
	return nil
}
