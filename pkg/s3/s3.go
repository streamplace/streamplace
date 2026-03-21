package s3

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
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
// Full fMP4 archives are fed via AddSegment. They are run through a muxl
// Concatenator to strip duplicate init segments, then uploaded as a
// multipart upload. Every cutoverEvery, the current upload is completed
// and a new one begins.
type S3Uploader struct {
	client       *s3.Client
	bucket       string
	cutoverEvery time.Duration
	keyPrefix    string // e.g. "did:plc:abc123/"
	concat       *muxl.Concatenator
	done         chan error
}

// S3 requires each part except the last to be at least 5MB.
const minPartSize = 5 * 1024 * 1024

type activeUpload struct {
	key      string
	uploadID string
	parts    []types.CompletedPart
	partNum  int32
	started  time.Time
	buf      []byte // accumulates segments until we hit minPartSize
}

// NewS3Uploader creates a new S3Uploader. keyPrefix is prepended to every
// object key (typically the streamer DID + "/"). Starts the muxl Concatenator
// and a background goroutine that reads processed segments and uploads them.
func NewS3Uploader(ctx context.Context, cfg Config, keyPrefix string, cutoverEvery time.Duration) *S3Uploader {
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
	concat := muxl.NewConcatenator(ctx)
	u := &S3Uploader{
		client:       client,
		bucket:       cfg.Bucket,
		cutoverEvery: cutoverEvery,
		keyPrefix:    keyPrefix,
		concat:       concat,
		done:         make(chan error, 1),
	}
	go u.uploadLoop(ctx)
	return u
}

// AddSegment feeds a full fMP4 archive (init+segments) to the concatenator
// for processing and upload.
func (u *S3Uploader) AddSegment(ctx context.Context, data []byte) error {
	return u.concat.Write(data)
}

// Close signals that no more segments will be added, waits for all
// in-flight uploads to complete, and returns any error.
func (u *S3Uploader) Close(ctx context.Context) error {
	closeErr := u.concat.Close()
	uploadErr := <-u.done
	if uploadErr != nil {
		return uploadErr
	}
	return closeErr
}

// uploadLoop reads init and segment events from the concatenator and manages
// multipart uploads. Runs until the concatenator's channels are closed.
func (u *S3Uploader) uploadLoop(ctx context.Context) {
	ctx = log.WithLogValues(ctx, "func", "s3.uploadLoop")
	var initSeg []byte
	var current *activeUpload

	// Helper: prepend init to buffer when starting a new upload
	startUpload := func() error {
		now := time.Now()
		key := fmt.Sprintf("%s%s.mp4", u.keyPrefix, now.UTC().Format("2006-01-02T15-04-05"))

		resp, err := u.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
			Bucket:      aws.String(u.bucket),
			Key:         aws.String(key),
			ContentType: aws.String("video/mp4"),
		})
		if err != nil {
			return fmt.Errorf("creating multipart upload for %s: %w", key, err)
		}

		current = &activeUpload{
			key:      key,
			uploadID: *resp.UploadId,
			started:  now,
		}
		// Prepend init segment to the buffer so the file starts valid
		if initSeg != nil {
			current.buf = append(current.buf, initSeg...)
		}
		log.Log(ctx, "started S3 multipart upload", "key", key)
		return nil
	}

	flushBuffer := func() error {
		if current == nil || len(current.buf) == 0 {
			return nil
		}
		current.partNum++
		partNum := current.partNum

		resp, err := u.client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(u.bucket),
			Key:        aws.String(current.key),
			UploadId:   aws.String(current.uploadID),
			PartNumber: aws.Int32(partNum),
			Body:       bytes.NewReader(current.buf),
		})
		if err != nil {
			return fmt.Errorf("uploading part %d: %w", partNum, err)
		}
		log.Debug(ctx, "uploaded S3 part", "key", current.key, "part", partNum, "size", len(current.buf))
		current.parts = append(current.parts, types.CompletedPart{
			ETag:       resp.ETag,
			PartNumber: aws.Int32(partNum),
		})
		current.buf = current.buf[:0]
		return nil
	}

	completeUpload := func() error {
		if current == nil {
			return nil
		}
		if err := flushBuffer(); err != nil {
			return err
		}
		if len(current.parts) == 0 {
			_, err := u.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(u.bucket),
				Key:      aws.String(current.key),
				UploadId: aws.String(current.uploadID),
			})
			if err != nil {
				log.Error(ctx, "aborting empty multipart upload", "key", current.key, "error", err)
			}
			current = nil
			return nil
		}
		_, err := u.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
			Bucket:   aws.String(u.bucket),
			Key:      aws.String(current.key),
			UploadId: aws.String(current.uploadID),
			MultipartUpload: &types.CompletedMultipartUpload{
				Parts: current.parts,
			},
		})
		if err != nil {
			return fmt.Errorf("completing multipart upload %s: %w", current.key, err)
		}
		log.Log(ctx, "completed S3 multipart upload", "key", current.key, "parts", len(current.parts))
		current = nil
		return nil
	}

	handleSegment := func(seg []byte) error {
		now := time.Now()

		// Cut over if needed
		if current != nil && now.Sub(current.started) >= u.cutoverEvery {
			if err := completeUpload(); err != nil {
				return err
			}
		}

		// Start a new upload if needed
		if current == nil {
			if err := startUpload(); err != nil {
				return err
			}
		}

		// Append segment data to buffer
		current.buf = append(current.buf, seg...)

		// Flush if buffer is large enough for a part
		if len(current.buf) >= minPartSize {
			if err := flushBuffer(); err != nil {
				return err
			}
		}

		return nil
	}

	var err error
	for err == nil {
		select {
		case init, ok := <-u.concat.InitCh:
			if !ok {
				u.concat.InitCh = nil
				continue
			}
			initSeg = init
			log.Log(ctx, "received init segment for S3 upload", "size", len(init))

		case seg, ok := <-u.concat.SegCh:
			log.Log(ctx, "received segment for S3 upload", "size", len(seg))
			if !ok {
				// Concatenator is done, complete any in-progress upload
				err = completeUpload()
				u.done <- err
				return
			}
			if err = handleSegment(seg); err != nil {
				log.Error(ctx, "error handling segment", "error", err)
			}

		case <-ctx.Done():
			_ = completeUpload()
			u.done <- ctx.Err()
			return
		}
	}

	u.done <- err
}
