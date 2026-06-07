package s3

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
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

// Recorder is an optional persistence hook for S3Uploader. RecordStart is
// called when a new multipart upload begins; the returned id is passed back
// to RecordComplete when the upload is finalized. Implementations should
// tolerate nil contexts being passed. livestreamURI ties the object to the
// livestream it belongs to (may be empty if not yet known) so the live-to-VOD
// finalize can later enumerate exactly the objects for one stream.
type Recorder interface {
	RecordStart(ctx context.Context, userDID, bucket, key, livestreamURI string, started time.Time) (id string, err error)
	RecordComplete(ctx context.Context, id string, parts int32, size int64) error
}

// S3Uploader manages streaming multipart uploads to an S3-compatible endpoint.
// Bare canonical MUXL segments are fed via AddSegment and written verbatim —
// no per-object init header — so the objects of one stream concatenate
// directly into a single MUXL byte stream. The live-to-VOD finalize prepends
// one synthesized init to the whole concatenation; keeping the objects bare is
// what lets it "concat fearlessly". Every cutoverEvery, the current upload is
// completed and a new one begins.
type S3Uploader struct {
	client       *s3.Client
	bucket       string
	cutoverEvery time.Duration
	keyPrefix    string // e.g. "did:plc:abc123/"
	userDID      string
	segCh        chan []byte // bare canonical MUXL segments awaiting upload
	done         chan error
	recorder     Recorder

	mu            sync.Mutex
	livestreamURI string // guarded by mu; stamped on each S3Segment row

	closeOnce sync.Once
	closeErr  error
	closed    atomic.Bool
}

// SetLivestreamURI records the livestream this stream's objects belong to. It
// may be called once the URI is resolved (it can be unknown when the uploader
// starts); subsequent multipart objects are stamped with it via RecordStart.
func (u *S3Uploader) SetLivestreamURI(uri string) {
	u.mu.Lock()
	u.livestreamURI = uri
	u.mu.Unlock()
}

func (u *S3Uploader) getLivestreamURI() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.livestreamURI
}

// S3 requires each part except the last to be at least 5MB.
const minPartSize = 5 * 1024 * 1024

type activeUpload struct {
	key       string
	uploadID  string
	recordID  string // set by Recorder.RecordStart, used for RecordComplete
	parts     []types.CompletedPart
	partNum   int32
	started   time.Time
	buf       []byte // accumulates segments until we hit minPartSize
	totalSize int64  // running total of bytes flushed across all parts
}

var DefaultCutoverEvery = 10 * time.Minute

// NewS3Uploader creates a new S3Uploader. keyPrefix is prepended to every
// object key (typically the streamer DID + "/"). userDID is passed through
// to the Recorder so uploads can be attributed to a user. recorder may be
// nil to disable persistence. Starts the muxl Concatenator and a background
// goroutine that reads processed segments and uploads them.
func NewS3Uploader(cfg Config, userDID, keyPrefix string, cutoverEvery time.Duration, recorder Recorder) *S3Uploader {
	ctx := context.Background()
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
		cutoverEvery = DefaultCutoverEvery
	}
	u := &S3Uploader{
		client:       client,
		bucket:       cfg.Bucket,
		cutoverEvery: cutoverEvery,
		keyPrefix:    keyPrefix,
		userDID:      userDID,
		segCh:        make(chan []byte, 16),
		done:         make(chan error, 1),
		recorder:     recorder,
	}
	go u.uploadLoop(ctx)
	return u
}

// AddSegment feeds one bare canonical MUXL segment (uuid+moof+mdat per track)
// for upload. The bytes are copied, so the caller may reuse its buffer. It is
// the caller's responsibility not to call AddSegment concurrently with Close
// (the StreamSession guarantees this by draining its goroutines before closing,
// see director.Start); the closed guard here is a best-effort backstop that
// turns a late call into an error rather than a send-on-closed-channel panic.
func (u *S3Uploader) AddSegment(ctx context.Context, data []byte) error {
	if u.closed.Load() {
		return fmt.Errorf("s3 uploader closed")
	}
	seg := append([]byte(nil), data...)
	select {
	case u.segCh <- seg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close signals that no more segments will be added, waits for all in-flight
// uploads to complete, and returns any error. It is idempotent: repeated calls
// return the same result without re-closing the channel. The supplied ctx is
// unused for the wait (uploadLoop runs on its own context so it can flush the
// final object even after the session context is cancelled) but kept for API
// symmetry.
func (u *S3Uploader) Close(ctx context.Context) error {
	u.closeOnce.Do(func() {
		u.closed.Store(true)
		close(u.segCh)
		if uploadErr := <-u.done; uploadErr != nil {
			u.closeErr = fmt.Errorf("error uploading: %w", uploadErr)
		}
	})
	return u.closeErr
}

// uploadLoop reads bare segments off segCh and manages multipart uploads.
// Runs until segCh is closed (Close) or ctx is canceled. ctx is intentionally
// independent of the session context so a final object can still be completed
// after the stream tears down.
func (u *S3Uploader) uploadLoop(ctx context.Context) {
	ctx = log.WithLogValues(ctx, "func", "s3.uploadLoop")
	var current *activeUpload

	startUpload := func() error {
		now := time.Now()
		key := fmt.Sprintf("%s%s.m4s", u.keyPrefix, now.UTC().Format("2006-01-02T15-04-05"))

		resp, err := u.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
			Bucket:      aws.String(u.bucket),
			Key:         aws.String(key),
			ContentType: aws.String("video/iso.segment"),
		})
		if err != nil {
			return fmt.Errorf("creating multipart upload for %s: %w", key, err)
		}

		current = &activeUpload{
			key:      key,
			uploadID: *resp.UploadId,
			started:  now,
		}
		if u.recorder != nil {
			id, recErr := u.recorder.RecordStart(ctx, u.userDID, u.bucket, key, u.getLivestreamURI(), now)
			if recErr != nil {
				log.Error(ctx, "recording S3 upload start", "key", key, "error", recErr)
			}
			current.recordID = id
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
		current.totalSize += int64(len(current.buf))
		current.buf = current.buf[:0]
		return nil
	}

	completeUpload := func() error {
		if current == nil {
			return nil
		}
		if err := flushBuffer(); err != nil {
			return fmt.Errorf("error flushing buffer: %w", err)
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
		log.Log(ctx, "completed S3 multipart upload", "key", current.key, "parts", len(current.parts), "size", current.totalSize)
		if u.recorder != nil && current.recordID != "" {
			if recErr := u.recorder.RecordComplete(ctx, current.recordID, int32(len(current.parts)), current.totalSize); recErr != nil {
				log.Error(ctx, "recording S3 upload completion", "key", current.key, "error", recErr)
			}
		}
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
		case seg, ok := <-u.segCh:
			if !ok {
				// No more segments; complete any in-progress upload.
				err = completeUpload()
				if err != nil {
					err = fmt.Errorf("error completing upload: %w", err)
				}
				u.done <- err
				return
			}
			log.Debug(ctx, "received segment for S3 upload", "size", len(seg))
			if err = handleSegment(seg); err != nil {
				log.Error(ctx, "error handling segment", "error", err)
			}

		case <-ctx.Done():
			err = completeUpload()
			if err != nil {
				err = fmt.Errorf("error completing upload: %w", err)
			}
			u.done <- err
			return
		}
	}

	u.done <- err
}
