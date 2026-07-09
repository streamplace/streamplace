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
	"stream.place/streamplace/pkg/spmetrics"
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

// uploadAPI is the subset of *s3.Client the upload loop uses. Pulled out so
// tests can inject a fake; *s3.Client satisfies it.
type uploadAPI interface {
	CreateMultipartUpload(context.Context, *s3.CreateMultipartUploadInput, ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error)
	UploadPart(context.Context, *s3.UploadPartInput, ...func(*s3.Options)) (*s3.UploadPartOutput, error)
	CompleteMultipartUpload(context.Context, *s3.CompleteMultipartUploadInput, ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error)
	AbortMultipartUpload(context.Context, *s3.AbortMultipartUploadInput, ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error)
}

// S3Uploader manages streaming multipart uploads to an S3-compatible endpoint.
// Bare canonical MUXL segments are fed via AddSegment and written verbatim —
// no per-object init header — so the objects of one stream concatenate
// directly into a single MUXL byte stream. The live-to-VOD finalize prepends
// one synthesized init to the whole concatenation; keeping the objects bare is
// what lets it "concat fearlessly". Every cutoverEvery, the current upload is
// completed and a new one begins.
type S3Uploader struct {
	client       uploadAPI
	bucket       string
	cutoverEvery time.Duration
	keyPrefix    string // e.g. "did:plc:abc123/"
	userDID      string
	segCh        chan uploadCmd // bare canonical MUXL segments / cutover requests
	done         chan error
	recorder     Recorder

	mu            sync.Mutex
	livestreamURI string // guarded by mu; stamped on each S3Segment row

	closeOnce sync.Once
	closeErr  error
	closed    atomic.Bool
}

// SetLivestreamURI records the livestream this stream's objects belong to. A
// single continuous ingest can move through several place.stream.livestream
// records (each "update livestream" mints a new one — chapter markers), so when
// this changes the upload loop rolls over to a fresh object tagged with the new
// URI. That keeps every object within a single livestream, which is what lets
// finalize select exactly one livestream's objects (and, across nodes, coalesce
// them by the shared URI). It may be called before the URI is first resolved.
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

// liveUploadPartSize is the exact size of every non-final part the live
// uploader flushes. R2 — unlike AWS/minio — rejects CompleteMultipartUpload
// with "All non-trailing parts must have the same length" unless parts are
// uniform, so the buffer is sliced at fixed boundaries instead of flushing
// whatever segments accumulated past the 5MB minimum. The S3 minimum keeps
// flushes prompt for a live stream; at 5MB a 10000-part object still spans
// ~48GB, far beyond one 10-minute cutover object.
const liveUploadPartSize = minPartSize

type activeUpload struct {
	key           string
	uploadID      string
	recordID      string // set by Recorder.RecordStart, used for RecordComplete
	livestreamURI string // the livestream this object belongs to; a change rolls it over
	parts         []types.CompletedPart
	partNum       int32
	started       time.Time
	buf           []byte // accumulates segments until we hit minPartSize
	totalSize     int64  // running total of bytes flushed across all parts
}

var DefaultCutoverEvery = 10 * time.Minute

// NewS3Uploader creates a new S3Uploader. keyPrefix is prepended to every
// object key (typically the streamer DID + "/"). userDID is passed through
// to the Recorder so uploads can be attributed to a user. recorder may be
// nil to disable persistence. Starts the muxl Concatenator and a background
// goroutine that reads processed segments and uploads them.
func NewS3Uploader(cfg Config, userDID, keyPrefix string, cutoverEvery time.Duration, recorder Recorder) *S3Uploader {
	return newS3Uploader(NewClient(cfg), cfg.Bucket, userDID, keyPrefix, cutoverEvery, recorder)
}

// NewClient builds an *s3.Client for an S3-compatible endpoint from cfg. Uses
// static credentials, an explicit BaseEndpoint, and path-style addressing (so it
// works against MinIO / R2 / plain-IP endpoints, not just AWS virtual-host style).
func NewClient(cfg Config) *s3.Client {
	return s3.New(s3.Options{
		Region: cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		),
		BaseEndpoint: aws.String(cfg.Endpoint),
		UsePathStyle: true,
	})
}

// newS3Uploader is the client-injectable constructor behind NewS3Uploader; the
// fake-client tests use it directly.
func newS3Uploader(client uploadAPI, bucket, userDID, keyPrefix string, cutoverEvery time.Duration, recorder Recorder) *S3Uploader {
	if cutoverEvery == 0 {
		cutoverEvery = DefaultCutoverEvery
	}
	u := &S3Uploader{
		client:       client,
		bucket:       bucket,
		cutoverEvery: cutoverEvery,
		keyPrefix:    keyPrefix,
		userDID:      userDID,
		segCh:        make(chan uploadCmd, 16),
		done:         make(chan error, 1),
		recorder:     recorder,
	}
	go u.uploadLoop(context.Background())
	return u
}

// uploadCmd is one item on segCh: either a segment to append (seg != nil) or a
// request to complete the current object now (cutover). Both travel the same
// channel so a cutover stays FIFO-ordered behind the segments queued before it.
type uploadCmd struct {
	seg     []byte // bare canonical MUXL segment to append; nil for a cutover
	cutover bool   // complete the current object now (see Cutover)
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
	case u.segCh <- uploadCmd{seg: seg}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Cutover completes the current in-progress object (if any) so it becomes a
// finalize-able, completed S3 segment, without tearing the uploader down — the
// next AddSegment simply starts a fresh object. It's used when a livestream
// ends (or the stream goes unpublished): the recording is closed out promptly
// instead of waiting for the cutoverEvery timer or stream teardown, so finalize
// can find the completed objects right away. A no-op if there's no current
// object, and a no-op after Close.
func (u *S3Uploader) Cutover(ctx context.Context) error {
	if u.closed.Load() {
		return nil
	}
	select {
	case u.segCh <- uploadCmd{cutover: true}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close signals that no more segments will be added, waits for all in-flight
// uploads to complete, and returns any error completing the final object.
// Mid-stream upload failures don't surface here — the upload loop recovers
// from those by abandoning the broken object (logged loudly at the time) and
// continuing with a fresh one. It is idempotent: repeated calls return the
// same result without re-closing the channel. The supplied ctx is unused for
// the wait (uploadLoop runs on its own context so it can flush the final
// object even after the session context is cancelled) but kept for API
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
	objSeq := 0 // disambiguates keys when two objects roll over within one second

	startUpload := func() error {
		objSeq++
		now := time.Now()
		key := fmt.Sprintf("%s%s-%d.m4s", u.keyPrefix, now.UTC().Format("2006-01-02T15-04-05"), objSeq)

		resp, err := u.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
			Bucket:      aws.String(u.bucket),
			Key:         aws.String(key),
			ContentType: aws.String("video/iso.segment"),
		})
		if err != nil {
			return fmt.Errorf("creating multipart upload for %s: %w", key, err)
		}

		uri := u.getLivestreamURI()
		current = &activeUpload{
			key:           key,
			uploadID:      *resp.UploadId,
			started:       now,
			livestreamURI: uri,
		}
		if u.recorder != nil {
			id, recErr := u.recorder.RecordStart(ctx, u.userDID, u.bucket, key, uri, now)
			if recErr != nil {
				log.Error(ctx, "recording S3 upload start", "key", key, "error", recErr)
			}
			current.recordID = id
		}
		spmetrics.S3LiveRecObjectsStartedTotal.Inc()
		log.Log(ctx, "started S3 multipart upload", "key", key)
		return nil
	}

	// flushPart uploads exactly the first n buffered bytes as the next part,
	// keeping the remainder buffered. Callers pass liveUploadPartSize for every
	// part except the object's final flush (completeUpload passes whatever is
	// left) — R2 requires all non-trailing parts to have the same length.
	flushPart := func(n int) error {
		if current == nil || n == 0 {
			return nil
		}
		current.partNum++
		partNum := current.partNum

		resp, err := u.client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(u.bucket),
			Key:        aws.String(current.key),
			UploadId:   aws.String(current.uploadID),
			PartNumber: aws.Int32(partNum),
			Body:       bytes.NewReader(current.buf[:n]),
		})
		if err != nil {
			return fmt.Errorf("uploading part %d: %w", partNum, err)
		}
		log.Debug(ctx, "uploaded S3 part", "key", current.key, "part", partNum, "size", n)
		current.parts = append(current.parts, types.CompletedPart{
			ETag:       resp.ETag,
			PartNumber: aws.Int32(partNum),
		})
		spmetrics.S3LiveRecBytesUploadedTotal.Add(float64(n))
		current.totalSize += int64(n)
		current.buf = append(current.buf[:0], current.buf[n:]...)
		return nil
	}

	completeUpload := func() error {
		if current == nil {
			return nil
		}
		if err := flushPart(len(current.buf)); err != nil {
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
		spmetrics.S3LiveRecObjectsCompletedTotal.Inc()
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

		// Roll over to a new object when the current one has run for cutoverEvery,
		// or when the livestream changed (a new place.stream.livestream "chapter"
		// record). Cutting over on the livestream change keeps each object within
		// a single livestream so finalize can select exactly one livestream's
		// objects without one straddling two chapters.
		if current != nil && (now.Sub(current.started) >= u.cutoverEvery || current.livestreamURI != u.getLivestreamURI()) {
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

		// Flush full-size parts; a sub-part remainder stays buffered until the
		// next segment or the object's completing flush.
		for len(current.buf) >= liveUploadPartSize {
			if err := flushPart(liveUploadPartSize); err != nil {
				return err
			}
		}

		return nil
	}

	// abandonCurrent is the failure recovery: abort the broken object (so the
	// backend doesn't hold its parts) and drop its un-completed bytes, loudly.
	// The next segment starts a fresh object, so one bad object costs a gap in
	// the recording instead of wedging the uploader for the rest of the stream
	// (the abandoned object's recorder row never completes, so finalize skips
	// it). Before this existed, the first error killed the loop: segments
	// backed up silently and the stream never recorded another byte.
	abandonCurrent := func(reason error) {
		if current == nil {
			// Nothing in flight (e.g. CreateMultipartUpload itself failed); the
			// segment is still dropped, so say so.
			spmetrics.S3LiveRecSegmentsDroppedTotal.Inc()
			log.Error(ctx, "error in live-rec S3 upload; segment dropped", "error", reason)
			return
		}
		spmetrics.S3LiveRecObjectsAbandonedTotal.Inc()
		spmetrics.S3LiveRecBytesDroppedTotal.Add(float64(current.totalSize + int64(len(current.buf))))
		log.Error(ctx, "abandoning live-rec S3 object; its bytes will be missing from the recording",
			"key", current.key,
			"uploadedBytes", current.totalSize,
			"droppedBufferedBytes", len(current.buf),
			"error", reason,
		)
		if _, err := u.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(u.bucket),
			Key:      aws.String(current.key),
			UploadId: aws.String(current.uploadID),
		}); err != nil {
			log.Error(ctx, "aborting abandoned S3 upload", "key", current.key, "error", err)
		}
		current = nil
	}

	for {
		select {
		case cmd, ok := <-u.segCh:
			if !ok {
				// No more segments; complete any in-progress upload. This is the
				// one error that still surfaces through done/Close — there are no
				// more segments coming to recover with.
				err := completeUpload()
				if err != nil {
					err = fmt.Errorf("error completing upload: %w", err)
					abandonCurrent(err)
				}
				u.done <- err
				return
			}
			if cmd.cutover {
				// Close out the current object so it's immediately finalize-able
				// (e.g. the livestream just ended). No-op if nothing is in flight.
				if err := completeUpload(); err != nil {
					abandonCurrent(fmt.Errorf("error completing upload on cutover: %w", err))
				}
				continue
			}
			log.Debug(ctx, "received segment for S3 upload", "size", len(cmd.seg))
			if err := handleSegment(cmd.seg); err != nil {
				// The triggering segment is dropped along with the object: it may
				// already be partially flushed into it, so it can't be salvaged.
				abandonCurrent(fmt.Errorf("error handling segment: %w", err))
			}

		case <-ctx.Done():
			err := completeUpload()
			if err != nil {
				err = fmt.Errorf("error completing upload: %w", err)
				abandonCurrent(err)
			}
			u.done <- err
			return
		}
	}
}
