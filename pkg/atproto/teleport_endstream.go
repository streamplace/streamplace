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

// endLivestreamForTeleport ends the source streamer's current livestream when a
// teleport fires — the same record update that place.stream.live.stopLivestream
// performs (set endedAt on the latest place.stream.livestream record via a
// getRecord/putRecord swap). A teleport sends the source streamer's viewers to
// a target; without this, the source stream stays live and viewers can navigate
// back. Setting endedAt returns the source streamer to "pre-live" and stops the
// manifest from publishing.
//
// The firehose path has no OAuth session, so the streamer's own stored session
// is looked up and an XRPC client is built from it — the same delegated-client
// pattern used by startLivestream's service-auth branch and moderation helpers.
//
// Best-effort: any failure only logs and returns. It never blocks the teleport
// arrival notification (the caller publishes that before invoking this). It is a
// no-op when there is no stored session, no active livestream, or the livestream
// is already ended.
func (atsync *ATProtoSynchronizer) endLivestreamForTeleport(ctx context.Context, repoDID string) {
	ctx = log.WithLogValues(ctx, "func", "endLivestreamForTeleport", "repoDID", repoDID)

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

	if err := atsync.endStreamersLivestream(ctx, repoDID, client); err != nil {
		log.Error(ctx, "failed to end livestream for teleport", "err", err)
		return
	}
	log.Log(ctx, "ended source livestream for teleport", "repoDID", repoDID)
}

// endStreamersLivestream sets endedAt on the streamer's latest un-ended
// place.stream.livestream record. It mirrors the record-ending half of
// stopLivestream / endPriorLivestream: fetch the latest livestream, bail if
// none or already ended, getRecord for a fresh swap CID, set endedAt, putRecord
// with the swap so a concurrent update is rejected rather than clobbered.
//
// Split from endLivestreamForTeleport so the session/client plumbing is
// testable independently of the record update.
func (atsync *ATProtoSynchronizer) endStreamersLivestream(ctx context.Context, repoDID string, client *oatproxy.XrpcClient) error {
	livestream, err := atsync.Model.GetLatestLivestreamForRepo(repoDID)
	if err != nil {
		return fmt.Errorf("get latest livestream: %w", err)
	}
	if livestream == nil || livestream.Livestream == nil {
		// Nothing live to end.
		return nil
	}

	livestreamView, err := livestream.ToLivestreamView()
	if err != nil {
		return fmt.Errorf("convert livestream to view: %w", err)
	}

	rec, ok := livestreamView.Record.Val.(*placestream.Livestream)
	if !ok {
		return fmt.Errorf("livestream is not a streamplace livestream")
	}

	if rec.EndedAt != nil {
		// Already ended (e.g. by stopLivestream or an earlier finalize).
		return nil
	}

	aturi, err := syntax.ParseATURI(livestreamView.Uri)
	if err != nil {
		return fmt.Errorf("parse livestream URI: %w", err)
	}

	// Fetch the current CID to swap on, so we don't clobber a concurrent
	// update (and so the putRecord is rejected if the record changed).
	var swapRecord *string
	getOutput := comatproto.RepoGetRecord_Output{}
	err = client.Do(ctx, xrpc.Query, "application/json", "com.atproto.repo.getRecord", map[string]any{
		"repo":       repoDID,
		"collection": "place.stream.livestream",
		"rkey":       aturi.RecordKey().String(),
	}, nil, &getOutput)
	if err != nil {
		return fmt.Errorf("get livestream record: %w", err)
	}
	swapRecord = getOutput.Cid

	now := time.Now().UTC().Format(util.ISO8601)
	rec.EndedAt = &now

	inp := comatproto.RepoPutRecord_Input{
		Collection: "place.stream.livestream",
		Record:     &glex.LexiconTypeDecoder{Val: rec},
		Rkey:       aturi.RecordKey().String(),
		Repo:       repoDID,
		SwapRecord: swapRecord,
	}
	var out comatproto.RepoPutRecord_Output
	if err := client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", map[string]any{}, inp, &out); err != nil {
		return fmt.Errorf("end livestream record: %w", err)
	}
	return nil
}
