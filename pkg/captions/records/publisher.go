package records

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/labstack/echo/v4"
	glex "github.com/streamplace/glex/runtime"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

// XRPCClient is the subset of an authenticated PDS client the records need.
// It is the same shape as pkg/vod's and pkg/director's XRPCClient, which are
// what the callers hold.
type XRPCClient interface {
	Do(ctx context.Context, method string, contentType string, path string, queryParams map[string]any, body any, out any) error
}

// ClientFunc returns a client acting as the account with the given DID, with
// a stored OAuth session (or a dev-account login) behind it.
type ClientFunc func(ctx context.Context, did string) (XRPCClient, error)

// SessionClients returns the ClientFunc that uses the OAuth sessions stored in
// state, refreshing them when they are about to expire, as pkg/vod does to
// publish a streamer's records.
func SessionClients(state *statedb.StatefulDB) ClientFunc {
	return func(ctx context.Context, did string) (XRPCClient, error) {
		session, err := state.GetSessionByDID(did)
		if err != nil {
			return nil, fmt.Errorf("get oauth session for %s: %w", did, err)
		}
		if session == nil {
			return nil, fmt.Errorf("no oauth session for %s", did)
		}
		session, err = state.OATProxy.RefreshIfNeeded(session)
		if err != nil {
			return nil, fmt.Errorf("refresh oauth session for %s: %w", did, err)
		}
		client, err := state.OATProxy.GetXrpcClient(session)
		if err != nil {
			return nil, fmt.Errorf("get xrpc client for %s: %w", did, err)
		}
		return client, nil
	}
}

// RepoPublisher writes transcript records to the repo a Target names:
// streamers' repos through their stored sessions, the node's through its
// server repo.
type RepoPublisher struct {
	CLI     *config.CLI
	Clients ClientFunc
}

func (p *RepoPublisher) Publish(ctx context.Context, target Target, rkey string, rec *placestream.CaptionTranscript) (string, error) {
	if target.Node {
		if err := atproto.CommitServerRepoRecord(ctx, p.CLI, constants.PLACE_STREAM_CAPTION_TRANSCRIPT, rkey, rec); err != nil {
			return "", fmt.Errorf("commit to server repo: %w", err)
		}
		return fmt.Sprintf("at://%s/%s/%s", target.Repo, constants.PLACE_STREAM_CAPTION_TRANSCRIPT, rkey), nil
	}
	client, err := p.Clients(ctx, target.Repo)
	if err != nil {
		return "", err
	}
	return putTranscript(ctx, client, target.Repo, rkey, rec)
}

// putTranscript writes a record under a chosen rkey, so a repeat of the same
// write replaces the record instead of adding another.
func putTranscript(ctx context.Context, client XRPCClient, repo, rkey string, rec *placestream.CaptionTranscript) (string, error) {
	in := comatproto.RepoPutRecord_Input{
		Collection: constants.PLACE_STREAM_CAPTION_TRANSCRIPT,
		Record:     &glex.LexiconTypeDecoder{Val: rec},
		Rkey:       rkey,
		Repo:       repo,
	}
	var out comatproto.RepoPutRecord_Output
	if err := client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", map[string]any{}, in, &out); err != nil {
		return "", fmt.Errorf("putRecord %s: %w", constants.PLACE_STREAM_CAPTION_TRANSCRIPT, err)
	}
	return out.Uri, nil
}

// LatestLivestream returns the SubjectResolver that points captions at the
// streamer's latest livestream record, resolved anew for each encoded batch.
func LatestLivestream(m interface {
	GetLatestLivestreamForRepo(repoDID string) (*model.Livestream, error)
}) SubjectResolver {
	return func(ctx context.Context, streamer string, sessionStart time.Time) (comatproto.RepoStrongRef, error) {
		ls, err := m.GetLatestLivestreamForRepo(streamer)
		if err != nil {
			return comatproto.RepoStrongRef{}, err
		}
		if ls == nil {
			return comatproto.RepoStrongRef{}, fmt.Errorf("no livestream record indexed for %s's current session", streamer)
		}
		if ls.Livestream == nil {
			return comatproto.RepoStrongRef{}, errors.New("livestream record is nil")
		}
		var rec placestream.Livestream
		if err := glex.DecodeCBOR(*ls.Livestream, &rec); err != nil {
			return comatproto.RepoStrongRef{}, fmt.Errorf("decode livestream record: %w", err)
		}
		if rec.EndedAt != nil {
			endedAt, err := time.Parse(time.RFC3339, *rec.EndedAt)
			if err != nil {
				return comatproto.RepoStrongRef{}, fmt.Errorf("parse livestream endedAt: %w", err)
			}
			if endedAt.Before(sessionStart) {
				return comatproto.RepoStrongRef{}, fmt.Errorf("latest livestream for %s ended before this session", streamer)
			}
		}
		return comatproto.RepoStrongRef{LexiconTypeID: "com.atproto.repo.strongRef", Uri: ls.URI, Cid: ls.CID}, nil
	}
}

// NewNodeWriter returns the Writer a node runs for its live sessions: records
// are written through the streamers' sessions (clients) and the node's server
// repo, point at the streamer's latest indexed livestream, and are indexed
// locally as soon as they are written. Wire it into the stream session with
//
//	w.StartSession(sessionCtx, streamerDID, firstSegment.StartTime)
//
// on the first segment of a session this node produces captions for; the
// session ends, with its last records flushed, when sessionCtx does.
func NewNodeWriter(cli *config.CLI, hub *captions.Hub, m model.Model, clients ClientFunc) (*Writer, error) {
	return NewWriter(Config{
		Hub:       hub,
		NodeDID:   cli.ServerDID(),
		Subject:   LatestLivestream(m),
		Publisher: &RepoPublisher{CLI: cli, Clients: clients},
		Index: func(ctx context.Context, rec *placestream.CaptionTranscript, uri string) error {
			return m.UpsertCaptionTranscript(ctx, *rec, syntax.ATURI(uri))
		},
	})
}

var resetRe = regexp.MustCompile(`will reset at (\S+?)\)`)

// retryAfter reports how long to wait after a write failed because the PDS
// rate limited the account, and whether that is what happened. oatproxy turns
// a 429 into an echo.HTTPError that carries the reset time in its message, and
// answers the same way without trying while the limit stands; a bare xrpc
// client reports it as an xrpc.Error with the limit headers.
func retryAfter(err error, now time.Time) (time.Duration, bool) {
	var xe *xrpc.Error
	if errors.As(err, &xe) && xe.StatusCode == http.StatusTooManyRequests {
		if xe.Ratelimit != nil && xe.Ratelimit.Reset.After(now) {
			return xe.Ratelimit.Reset.Sub(now), true
		}
		return backoffMin, true
	}
	var he *echo.HTTPError
	if errors.As(err, &he) && he.Code == http.StatusTooManyRequests {
		if m := resetRe.FindStringSubmatch(fmt.Sprint(he.Message)); m != nil {
			if reset, perr := time.Parse(time.RFC3339, m[1]); perr == nil && reset.After(now) {
				return reset.Sub(now), true
			}
		}
		return backoffMin, true
	}
	return 0, false
}
