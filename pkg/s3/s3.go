package s3

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"stream.place/streamplace/pkg/log"
)

// Config holds the configuration for an S3-compatible upload target.
type Config struct {
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	Region          string
}

// S3Uploader manages streaming multipart uploads to an S3-compatible endpoint.
// Segments are appended as parts to the current upload. Every CutoverInterval,
// the current upload is completed and a new one begins.
type S3Uploader struct {
	client         *s3.Client
	bucket         string
	cutoverEvery   time.Duration
	keyPrefix      string // e.g. "did:plc:abc123/"
	mu             sync.Mutex
	currentUpload  *activeUpload
	uploadCount    int
}

type activeUpload struct {
	key      string
	uploadID string
	parts    []types.CompletedPart
	partNum  int32
	started  time.Time
}

// NewS3Uploader creates a new S3Uploader. keyPrefix is prepended to every
// object key (typically the streamer DID + "/").
func NewS3Uploader(cfg Config, keyPrefix string, cutoverEvery time.Duration) *S3Uploader {
	client := s3.New(s3.Options{
		Region: cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		),
		BaseEndpoint: aws.String(cfg.Endpoint),
		UsePathStyle: true,
	})
	if cutoverEvery == 0 {
		cutoverEvery = time.Minute
	}
	return &S3Uploader{
		client:       client,
		bucket:       cfg.Bucket,
		cutoverEvery: cutoverEvery,
		keyPrefix:    keyPrefix,
	}
}

// AddSegment appends a segment to the current multipart upload. If no upload
// is active, or the cutover interval has elapsed, a new upload is started
// (completing the previous one first).
func (u *S3Uploader) AddSegment(ctx context.Context, data []byte) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	now := time.Now()

	// Cut over if needed
	if u.currentUpload != nil && now.Sub(u.currentUpload.started) >= u.cutoverEvery {
		if err := u.completeUploadLocked(ctx); err != nil {
			return fmt.Errorf("completing upload before cutover: %w", err)
		}
		u.currentUpload = nil
	}

	// Start a new upload if needed
	if u.currentUpload == nil {
		if err := u.startUploadLocked(ctx, now); err != nil {
			return fmt.Errorf("starting new upload: %w", err)
		}
	}

	// Upload this segment as the next part
	u.currentUpload.partNum++
	partNum := u.currentUpload.partNum

	resp, err := u.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(u.bucket),
		Key:        aws.String(u.currentUpload.key),
		UploadId:   aws.String(u.currentUpload.uploadID),
		PartNumber: aws.Int32(partNum),
		Body:       bytes.NewReader(data),
	})
	if err != nil {
		return fmt.Errorf("uploading part %d: %w", partNum, err)
	}

	u.currentUpload.parts = append(u.currentUpload.parts, types.CompletedPart{
		ETag:       resp.ETag,
		PartNumber: aws.Int32(partNum),
	})

	log.Debug(ctx, "uploaded S3 part", "key", u.currentUpload.key, "part", partNum, "size", len(data))
	return nil
}

// Close completes any in-progress upload. Call this when the stream ends.
func (u *S3Uploader) Close(ctx context.Context) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.currentUpload == nil {
		return nil
	}
	err := u.completeUploadLocked(ctx)
	u.currentUpload = nil
	return err
}

func (u *S3Uploader) startUploadLocked(ctx context.Context, now time.Time) error {
	key := fmt.Sprintf("%s%s.mp4", u.keyPrefix, now.UTC().Format("2006-01-02T15-04-05"))
	u.uploadCount++

	resp, err := u.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(u.bucket),
		Key:         aws.String(key),
		ContentType: aws.String("video/mp4"),
	})
	if err != nil {
		return fmt.Errorf("creating multipart upload for %s: %w", key, err)
	}

	u.currentUpload = &activeUpload{
		key:      key,
		uploadID: *resp.UploadId,
		started:  now,
	}

	log.Log(ctx, "started S3 multipart upload", "key", key, "uploadID", *resp.UploadId)
	return nil
}

func (u *S3Uploader) completeUploadLocked(ctx context.Context) error {
	up := u.currentUpload
	if len(up.parts) == 0 {
		// Nothing was uploaded, abort instead
		_, err := u.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(u.bucket),
			Key:      aws.String(up.key),
			UploadId: aws.String(up.uploadID),
		})
		if err != nil {
			log.Error(ctx, "aborting empty multipart upload", "key", up.key, "error", err)
		}
		return nil
	}

	_, err := u.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(u.bucket),
		Key:      aws.String(up.key),
		UploadId: aws.String(up.uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: up.parts,
		},
	})
	if err != nil {
		return fmt.Errorf("completing multipart upload %s: %w", up.key, err)
	}

	log.Log(ctx, "completed S3 multipart upload", "key", up.key, "parts", len(up.parts))
	return nil
}

