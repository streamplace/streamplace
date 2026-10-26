package director

import (
	"context"
	"fmt"

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
// mid-stream settings change takes effect on the streamer's next session. A
// lookup error is returned, not taken as a no: the record role hands the job
// to a peer and asks again later (see contend).
func (ss *StreamSession) shouldRecordLivestream(ctx context.Context, repoDID string) (bool, error) {
	inBeta := false
	if ss.cli.BetaInviteDID != "" {
		has, err := ss.mod.HasBetaInvite(ctx, ss.cli.BetaInviteDID, repoDID, vodInviteFeature)
		if err != nil {
			return false, fmt.Errorf("live recording: failed to check VOD beta invite: %w", err)
		}
		inBeta = has
	} else {
		inBeta = ss.cli.StreamIsAllowed(repoDID) == nil
	}
	if !inBeta {
		return false, nil
	}

	settings, err := ss.mod.GetServerSettings(ctx, ss.cli.BroadcasterHost, repoDID)
	if err != nil {
		return false, fmt.Errorf("live recording: failed to load server settings: %w", err)
	}
	if settings == nil {
		// No settings record at all: default on for beta users.
		return true, nil
	}
	spsettings, err := settings.ToStreamplaceServerSettings()
	if err != nil {
		return false, fmt.Errorf("live recording: failed to decode server settings: %w", err)
	}
	// Default on: record unless the streamer explicitly set the flag to false.
	return spsettings.LivestreamRecording == nil || *spsettings.LivestreamRecording, nil
}

// startS3Upload takes up recording the stream: the record role's start. It
// installs the uploader last, so on an error there is nothing to put down.
func (ss *StreamSession) startS3Upload(ctx context.Context, repoDID string) error {
	record, err := ss.shouldRecordLivestream(ctx, repoDID)
	if err != nil {
		return err
	}
	if !record {
		// Info, not Debug: this fires once per stream session, and it's the
		// answer to "why does this streamer have no recording?" — a question
		// that once cost a production debugging session at Debug level.
		log.Log(ctx, "live recording disabled for streamer (not in VOD beta, or recording turned off in settings)", "repoDID", repoDID)
		return errRoleDeclined
	}
	cfg := ss.cli.S3Config()
	// live-rec/ namespaces the in-progress livestream recordings away from the
	// finalized VOD blobs (blobs/) and anything else in the bucket.
	keyPrefix := liveRecPrefix + repoDID + "/"
	u := s3.NewS3Uploader(cfg, repoDID, keyPrefix, s3.DefaultCutoverEvery, ss.statefulDB)
	// The object being uploaded can't be read back until it completes, so
	// hold the stream's segments in memory that long for clips to draw on.
	ss.mm.RetainSegments(repoDID, s3.MaxOpenObjectAge)
	// Best-effort initial resolve of the livestream URI so the very first
	// object is tagged. The director treats "latest livestream for repo" as
	// the current stream everywhere (notification blast, idle finalize), so we
	// do the same here; NewSegment refreshes it once the stream's own record is
	// indexed, in case a prior stream was momentarily still "latest".
	if ls, err := ss.mod.GetLatestLivestreamForRepo(repoDID); err != nil {
		log.Warn(ctx, "live recording: failed to resolve initial livestream URI; first object starts untagged", "error", err, "repoDID", repoDID)
	} else if ls != nil {
		u.SetLivestreamURI(ls.URI)
	}
	ss.s3Mu.Lock()
	ss.s3Uploader = u
	ss.s3Mu.Unlock()
	log.Log(ctx, "S3 upload enabled", "bucket", ss.cli.S3Bucket, "endpoint", ss.cli.S3Endpoint, "repoDID", repoDID)
	return nil
}

// uploader is the live recording, or nil while this node is not recording.
func (ss *StreamSession) uploader() *s3.S3Uploader {
	ss.s3Mu.Lock()
	defer ss.s3Mu.Unlock()
	return ss.s3Uploader
}

// withUploader runs f on the live recording, if there is one, as a session
// goroutine s3Close lets finish before it closes that recording.
func (ss *StreamSession) withUploader(ctx context.Context, f func(u *s3.S3Uploader) error) {
	ss.s3Mu.Lock()
	u := ss.s3Uploader
	if u != nil {
		ss.s3Pending.Add(1)
	}
	ss.s3Mu.Unlock()
	if u == nil {
		return
	}
	ss.Go(ctx, func() error {
		defer ss.s3Pending.Done()
		return f(u)
	})
}

func (ss *StreamSession) s3Upload(ctx context.Context, notif *media.NewSegmentNotification) {
	ss.withUploader(ctx, func(u *s3.S3Uploader) error {
		// notif.Muxl is the bare canonical segment; it concatenates directly
		// (the S3 uploader synthesizes one init and prepends it per object).
		return u.AddSegment(ctx, notif.Muxl, notif.Segment.StartTime)
	})
}

// s3Cutover completes the current live-recording object so it's immediately
// finalize-able. Called when a segment arrives that is not part of a live
// (published) stream — i.e. the livestream just ended, or hasn't started — so
// the recording is closed out promptly rather than lingering un-completed until
// stream teardown.
func (ss *StreamSession) s3Cutover(ctx context.Context) {
	ss.withUploader(ctx, func(u *s3.S3Uploader) error {
		return u.Cutover(ctx)
	})
}

// s3Close puts recording down, flushing the current object so finalize can
// find it: the record role's stop, whether the session ended or another
// node took the job over. Segments already handed to the recording go in
// first.
func (ss *StreamSession) s3Close(ctx context.Context) {
	ss.s3Mu.Lock()
	u := ss.s3Uploader
	ss.s3Uploader = nil
	ss.s3Mu.Unlock()
	if u == nil {
		return
	}
	ss.s3Pending.Wait()
	// the session context may already be canceled, so we need a new one
	err := u.Close(context.Background())
	if err != nil {
		log.Error(ctx, "error closing S3 upload", "error", err)
	}
}
