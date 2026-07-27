package atproto

import (
	"context"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/util"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"

	glex "github.com/streamplace/glex/runtime"

	comatproto "stream.place/streamplace/pkg/comatproto"
)

// endLivestreamForTeleport ends the source streamer's livestream when a teleport
// fires — the same record update that place.stream.live.stopLivestream performs
// (set endedAt on the place.stream.livestream record via a getRecord/putRecord
// swap). A teleport sends the source streamer's viewers to a target; without
// this, the source stream stays live and viewers can navigate back. Setting
// endedAt returns the source streamer to "pre-live" and stops the manifest from
// publishing.
//
// The livestream to end is identified by the teleport record's `livestream`
// strongRef (uri + cid), NOT by "the streamer's latest livestream". The latest
// record is a race: if the streamer started a new stream before this delayed
// arrival callback runs, GetLatestLivestreamForRepo would select the wrong
// (new) record and terminate an unrelated broadcast. The strongRef pins the
// exact origin stream, so we always end the right one. Teleports whose record
// predates the `livestream` field (or otherwise lacks an origin stream) are
// treated as a no-op.
//
// The firehose path has no OAuth session, so the streamer's own stored session
// is looked up and an XRPC client is built from it — the same delegated-client
// pattern used by startLivestream's service-auth branch and moderation helpers.
//
// Best-effort: any failure only logs and returns. It never blocks the teleport
// arrival notification (the caller publishes that before invoking this). It is a
// no-op when there is no stored session, no origin livestream strongRef, the
// referenced record is gone, or the livestream is already ended.
func (atsync *ATProtoSynchronizer) endLivestreamForTeleport(ctx context.Context, repoDID string, livestreamRef comatproto.RepoStrongRef) {
	ctx = log.WithLogValues(ctx, "func", "endLivestreamForTeleport", "repoDID", repoDID, "livestreamUri", livestreamRef.Uri)

	// A teleport without an origin livestream strongRef (e.g. a record from
	// before the field existed) cannot be tied to a specific stream. Rather
	// than guess via "latest livestream" — which can terminate an unrelated,
	// newer broadcast — treat it as a no-op.
	if livestreamRef.Uri == "" {
		log.Debug(ctx, "teleport has no origin livestream strongRef, skipping stream-end")
		return
	}

	// A teleport record can arrive before the streamer has ever logged in to
	// this node (e.g. multi-node setups). Without a stored session we have no
	// credentials to write the record update, so there is nothing to do.
	session, err := atsync.StatefulDB.GetSessionByDID(repoDID)
	if err != nil {
		log.Error(ctx, "failed to get streamer session for teleport stream-end", "err", err)
		return
	}
	if session == nil {
		log.Debug(ctx, "no stored session for streamer, cannot end livestream for teleport")
		return
	}

	// Refresh the session if its tokens are stale, then build a client that
	// signs requests as the streamer.
	session, err = atsync.OATProxy.RefreshIfNeeded(session)
	if err != nil {
		log.Error(ctx, "failed to refresh streamer session for teleport stream-end", "err", err)
		return
	}
	if session == nil {
		log.Debug(ctx, "streamer session missing after refresh, cannot end livestream for teleport")
		return
	}
	client, err := atsync.OATProxy.GetXrpcClient(session)
	if err != nil {
		log.Error(ctx, "failed to get xrpc client for teleport stream-end", "err", err)
		return
	}

	if err := atsync.endReferencedLivestream(ctx, repoDID, livestreamRef, client); err != nil {
		log.Error(ctx, "failed to end livestream for teleport", "err", err)
		return
	}
	log.Log(ctx, "ended source livestream for teleport", "repoDID", repoDID, "livestreamUri", livestreamRef.Uri)
}

// endReferencedLivestream sets endedAt on the specific place.stream.livestream
// record named by livestreamRef. It mirrors the record-ending half of
// stopLivestream / endPriorLivestream, but targets the strongRef's record rather
// than "the latest livestream": parse the strongRef URI for its rkey, getRecord
// for a fresh swap CID (the record may have changed since the teleport was
// created, e.g. a title update), set endedAt, putRecord with the swap so a
// concurrent update is rejected rather than clobbered.
//
// Split from endLivestreamForTeleport so the session/client plumbing is
// testable independently of the record update.
func (atsync *ATProtoSynchronizer) endReferencedLivestream(ctx context.Context, repoDID string, livestreamRef comatproto.RepoStrongRef, client *oatproxy.XrpcClient) error {
	aturi, err := syntax.ParseATURI(livestreamRef.Uri)
	if err != nil {
		return fmt.Errorf("parse livestream strongRef URI: %w", err)
	}
	rkey := aturi.RecordKey().String()

	// Fetch the current record (and its current CID to swap on) so we don't
	// clobber a concurrent update, and so the putRecord is rejected if the
	// record changed since the teleport was created.
	getOutput := comatproto.RepoGetRecord_Output{}
	err = client.Do(ctx, xrpc.Query, "application/json", "com.atproto.repo.getRecord", map[string]any{
		"repo":       repoDID,
		"collection": "place.stream.livestream",
		"rkey":       rkey,
	}, nil, &getOutput)
	if err != nil {
		return fmt.Errorf("get livestream record: %w", err)
	}
	if getOutput.Value == nil {
		return fmt.Errorf("getRecord returned no value for livestream %s", livestreamRef.Uri)
	}

	rec, ok := getOutput.Value.Val.(*placestream.Livestream)
	if !ok {
		return fmt.Errorf("referenced record is not a streamplace livestream")
	}

	if rec.EndedAt != nil {
		// Already ended (e.g. by stopLivestream or an earlier finalize).
		return nil
	}

	now := time.Now().UTC().Format(util.ISO8601)
	rec.EndedAt = &now

	inp := comatproto.RepoPutRecord_Input{
		Collection: "place.stream.livestream",
		Record:     &glex.LexiconTypeDecoder{Val: rec},
		Rkey:       rkey,
		Repo:       repoDID,
		SwapRecord: getOutput.Cid,
	}
	var out comatproto.RepoPutRecord_Output
	if err := client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", map[string]any{}, inp, &out); err != nil {
		return fmt.Errorf("end livestream record: %w", err)
	}
	return nil
}
