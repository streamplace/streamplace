package director

import (
	"context"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/s3"
)

// liveRecPrefix namespaces in-progress livestream recordings in the S3 bucket
// (keys are liveRecPrefix + <did>/ + <timestamp>.m4s).
const liveRecPrefix = "live-rec/"

// vodInviteFeature is the place.stream.beta.invite `feature` value that grants
// VOD access. Mirrors spxrpc's vodInviteFeature (that const is unexported in
// another package); the two must stay in sync.
const vodInviteFeature = "vod"

// shouldRecordLivestream reports whether this node should archive repoDID's
// livestream into a VOD. Both conditions are required:
//
//   - The user is in the VOD beta — an indexed place.stream.beta.invite with
//     feature=vod from the trusted issuer (--beta-invite-did). This mirrors the
//     upload gate (spxrpc.betaFeatureGranted): when no issuer is configured
//     (self-hosted / dev) we fall back to cli.StreamIsAllowed, the same
//     allowlist livestreaming itself uses.
//   - The user has not turned recording off via the livestreamRecording server
//     setting. This defaults ON for beta users (most want their streams kept) —
//     the opposite of debugRecording — so only an explicit `false` opts out.
//
// Evaluated once at stream start (like debugRecording in media.shouldRecord); a
// mid-stream settings change takes effect on the streamer's next session. On
// any lookup error we fail closed (do not record).
func (ss *StreamSession) shouldRecordLivestream(ctx context.Context, repoDID string) bool {
	inBeta := false
	if ss.cli.BetaInviteDID != "" {
		has, err := ss.mod.HasBetaInvite(ctx, ss.cli.BetaInviteDID, repoDID, vodInviteFeature)
		if err != nil {
			log.Error(ctx, "live recording: failed to check VOD beta invite", "error", err, "repoDID", repoDID)
			return false
		}
		inBeta = has
	} else {
		inBeta = ss.cli.StreamIsAllowed(repoDID) == nil
	}
	if !inBeta {
		return false
	}

	settings, err := ss.mod.GetServerSettings(ctx, ss.cli.BroadcasterHost, repoDID)
	if err != nil {
		log.Error(ctx, "live recording: failed to load server settings", "error", err, "repoDID", repoDID)
		return false
	}
	if settings == nil {
		// No settings record at all: default on for beta users.
		return true
	}
	spsettings, err := settings.ToStreamplaceServerSettings()
	if err != nil {
		log.Error(ctx, "live recording: failed to decode server settings", "error", err, "repoDID", repoDID)
		return false
	}
	// Default on: record unless the streamer explicitly set the flag to false.
	return spsettings.LivestreamRecording == nil || *spsettings.LivestreamRecording
}

func (ss *StreamSession) maybeStartS3Upload(ctx context.Context, repoDID string) {
	if !ss.cli.S3Configured() {
		// Debug: the no-S3 dev default, so one line per stream start would be
		// pure noise there — but it's flippable at runtime when an operator is
		// asking "why isn't this node recording anything?".
		log.Debug(ctx, "live recording skipped: S3 not configured", "repoDID", repoDID)
		return
	}
	if !ss.shouldRecordLivestream(ctx, repoDID) {
		// Info, not Debug: this fires once per stream session, and it's the
		// answer to "why does this streamer have no recording?" — a question
		// that once cost a production debugging session at Debug level.
		log.Log(ctx, "live recording disabled for streamer (not in VOD beta, or recording turned off in settings)", "repoDID", repoDID)
		return
	}
	cfg := ss.cli.S3Config()
	// live-rec/ namespaces the in-progress livestream recordings away from the
	// finalized VOD blobs (blobs/) and anything else in the bucket.
	keyPrefix := liveRecPrefix + repoDID + "/"
	ss.s3Uploader = s3.NewS3Uploader(cfg, repoDID, keyPrefix, s3.DefaultCutoverEvery, ss.statefulDB)
	// Best-effort initial resolve of the livestream URI so the very first
	// object is tagged. The director treats "latest livestream for repo" as
	// the current stream everywhere (notification blast, idle finalize), so we
	// do the same here; NewSegment refreshes it once the stream's own record is
	// indexed, in case a prior stream was momentarily still "latest".
	if ls, err := ss.mod.GetLatestLivestreamForRepo(repoDID); err != nil {
		log.Warn(ctx, "live recording: failed to resolve initial livestream URI; first object starts untagged", "error", err, "repoDID", repoDID)
	} else if ls != nil {
		ss.s3Uploader.SetLivestreamURI(ls.URI)
	}
	log.Log(ctx, "S3 upload enabled", "bucket", ss.cli.S3Bucket, "endpoint", ss.cli.S3Endpoint, "repoDID", repoDID)
}

func (ss *StreamSession) s3Upload(ctx context.Context, notif *media.NewSegmentNotification) {
	if ss.s3Uploader == nil {
		return
	}
	// notif.ArchiveCopy is the bare canonical segment (with its captions laid
	// out again for the recording, which can take seconds); it concatenates
	// directly (the S3 uploader synthesizes one init and prepends it per object).
	ss.s3InOrder(ctx, notif.ArchiveCopy, ss.s3Uploader.AddSegment)
}

// s3Cutover completes the current live-recording object so it's immediately
// finalize-able. Called when a segment arrives that is not part of a live
// (published) stream — i.e. the livestream just ended, or hasn't started — so
// the recording is closed out promptly rather than lingering un-completed until
// stream teardown.
func (ss *StreamSession) s3Cutover(ctx context.Context) {
	if ss.s3Uploader == nil {
		return
	}
	ss.s3InOrder(ctx, nil, func(ctx context.Context, _ []byte) error {
		return ss.s3Uploader.Cutover(ctx)
	})
}

// s3InOrder applies op to the recording after every earlier S3 operation of
// the session. prepare runs right away, so archive copies waiting on their
// captions overlap instead of adding up. Unlike a lane, nothing is skipped: a
// recording must not drop a slow segment. NewSegment, the only caller, runs on
// the director's dispatch goroutine, which orders s3Prev.
func (ss *StreamSession) s3InOrder(ctx context.Context, prepare func(context.Context) []byte, op func(context.Context, []byte) error) {
	prev, done := ss.s3Prev, make(chan struct{})
	ss.s3Prev = done
	ss.Go(ctx, func() error {
		defer close(done)
		var data []byte
		if prepare != nil {
			data = prepare(ctx)
		}
		if prev != nil {
			<-prev
		}
		return op(ctx, data)
	})
}

func (ss *StreamSession) s3Close(ctx context.Context) {
	if ss.s3Uploader == nil {
		return
	}
	// this context is already canceled, so we need a new one
	err := ss.s3Uploader.Close(context.Background())
	if err != nil {
		log.Error(ctx, "error closing S3 upload", "error", err)
	}
}
