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

func (ss *StreamSession) maybeStartS3Upload(ctx context.Context, repoDID string) {
	if !ss.cli.S3Configured() {
		return
	}
	cfg := s3.Config{
		Endpoint:        ss.cli.S3Endpoint,
		Bucket:          ss.cli.S3Bucket,
		AccessKeyID:     ss.cli.S3AccessKeyID,
		SecretAccessKey: ss.cli.S3SecretAccessKey,
		Region:          ss.cli.S3Region,
	}
	// live-rec/ namespaces the in-progress livestream recordings away from the
	// finalized VOD blobs (blobs/) and anything else in the bucket.
	keyPrefix := liveRecPrefix + repoDID + "/"
	ss.s3Uploader = s3.NewS3Uploader(cfg, repoDID, keyPrefix, s3.DefaultCutoverEvery, ss.statefulDB)
	// Best-effort initial resolve of the livestream URI so the very first
	// object is tagged. The director treats "latest livestream for repo" as
	// the current stream everywhere (notification blast, idle finalize), so we
	// do the same here; NewSegment refreshes it once the stream's own record is
	// indexed, in case a prior stream was momentarily still "latest".
	if ls, err := ss.mod.GetLatestLivestreamForRepo(repoDID); err == nil && ls != nil {
		ss.s3Uploader.SetLivestreamURI(ls.URI)
	}
	log.Log(ctx, "S3 upload enabled", "bucket", ss.cli.S3Bucket, "endpoint", ss.cli.S3Endpoint)
}

func (ss *StreamSession) s3Upload(ctx context.Context, notif *media.NewSegmentNotification) {
	if ss.s3Uploader == nil {
		return
	}
	ss.Go(ctx, func() error {
		// notif.Muxl is the bare canonical segment; it concatenates directly
		// (the S3 uploader synthesizes one init and prepends it per object).
		return ss.s3Uploader.AddSegment(ctx, notif.Muxl)
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
