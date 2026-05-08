package director

import (
	"context"
	"time"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/s3"
)

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
	keyPrefix := repoDID + "/"
	ss.s3Uploader = s3.NewS3Uploader(cfg, keyPrefix, time.Minute)
	log.Log(ctx, "S3 upload enabled", "bucket", ss.cli.S3Bucket, "endpoint", ss.cli.S3Endpoint)
}

func (ss *StreamSession) s3Upload(ctx context.Context, notif *media.NewSegmentNotification) {
	if ss.s3Uploader == nil {
		return
	}
	ss.Go(ctx, func() error {
		return ss.s3Uploader.AddSegment(ctx, notif.Data)
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
