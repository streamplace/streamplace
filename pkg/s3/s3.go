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
	spool        *Spool // non-nil = disk-spool mode (segments durable until uploaded)

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
// nil to disable persistence. Starts a background goroutine that reads
// segments and uploads them.
//
// When spoolDir is non-empty and spoolMaxBytes > 0, segments are written
// through a bounded disk spool and deleted only once uploaded, so upload
// failures cost lag instead of losing recorded bytes; otherwise segments
// buffer in memory only. A spool that fails to open logs and falls back to
// memory mode — recording must not die because a directory couldn't be made.
func NewS3Uploader(cfg Config, userDID, keyPrefix string, cutoverEvery time.Duration, recorder Recorder, spoolDir string, spoolMaxBytes int64) *S3Uploader {
	var spool *Spool
	if spoolDir != "" && spoolMaxBytes > 0 {
		var err error
		spool, err = OpenSpool(spoolDir, spoolMaxBytes)
		if err != nil {
			log.Error(context.Background(), "opening live-rec spool; falling back to in-memory upload buffering", "dir", spoolDir, "error", err)
			spool = nil
		}
	}
	return newS3Uploader(NewClient(cfg), cfg.Bucket, userDID, keyPrefix, cutoverEvery, recorder, spool)
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
func newS3Uploader(client uploadAPI, bucket, userDID, keyPrefix string, cutoverEvery time.Duration, recorder Recorder, spool *Spool) *S3Uploader {
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
		spool:        spool,
	}
	if spool != nil {
		go u.uploadLoopSpool(context.Background())
	} else {
		go u.uploadLoop(context.Background())
	}
	return u
}

// uploadCmd is one item on segCh: a segment to append (memory mode, seg !=
// nil), a kick meaning "new spool data is available" (spool mode), or a
// request to complete the current object now (cutover). All travel the same
// channel so a cutover stays FIFO-ordered behind the segments queued before it.
type uploadCmd struct {
	seg     []byte // bare canonical MUXL segment to append (memory mode)
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
	if u.spool != nil {
		// Durably spool the segment, then kick the consumer. The kick is only
		// a hint — the loop drains everything available per wakeup — so a full
		// channel just means a wakeup is already pending, and this never
		// blocks behind a slow or backing-off upload.
		if _, err := u.spool.Append(ctx, data, u.getLivestreamURI()); err != nil {
			spmetrics.S3LiveRecSegmentsDroppedTotal.Inc()
			return fmt.Errorf("spooling segment: %w", err)
		}
		select {
		case u.segCh <- uploadCmd{}:
		default:
		}
		return nil
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

// objectWriter owns one in-flight live-rec object at a time: starting it,
// slicing buffered segments into uniform parts, completing or aborting it.
// Shared by the in-memory upload loop and the spool-consuming loop, which
// differ only in where segments come from and what a failure costs.
type objectWriter struct {
	u       *S3Uploader
	current *activeUpload
	objSeq  int // disambiguates keys when two objects roll over within one second

	// keySuffix is appended to every object key before ".m4s". The salvage
	// path sets it to the spool session name so salvaged keys can never
	// collide with each other (objSeq resets per salvaged spool and mtimes
	// have second granularity) or with keys the crashed run already wrote.
	keySuffix string

	// strictRecorder makes Recorder failures upload failures. The spool loop
	// and salvage set this: their segments survive on disk, so failing and
	// retrying beats completing an object that no s3_segments row points at —
	// such an object is invisible to finalize, and once the spool is acked
	// the bytes are unrecoverable. The memory loop stays lenient: without a
	// spool the segments are gone either way, and an untracked object in the
	// bucket at least preserves the bytes.
	strictRecorder bool
}

// start opens a new multipart object. started is the moment the object's
// first byte was recorded — wall clock for live uploads, the segment's spool
// mtime for catch-up uploads — and lands in the recorder row, which is what
// finalize orders objects by; keeping it honest is what lets a late retry
// still sort correctly in the assembled VOD.
func (w *objectWriter) start(ctx context.Context, uri string, started time.Time) error {
	w.objSeq++
	key := fmt.Sprintf("%s%s-%d%s.m4s", w.u.keyPrefix, started.UTC().Format("2006-01-02T15-04-05"), w.objSeq, w.keySuffix)

	resp, err := w.u.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(w.u.bucket),
		Key:         aws.String(key),
		ContentType: aws.String("video/iso.segment"),
	})
	if err != nil {
		return fmt.Errorf("creating multipart upload for %s: %w", key, err)
	}

	w.current = &activeUpload{
		key:           key,
		uploadID:      *resp.UploadId,
		started:       started,
		livestreamURI: uri,
	}
	if w.u.recorder != nil {
		id, recErr := w.u.recorder.RecordStart(ctx, w.u.userDID, w.u.bucket, key, uri, started)
		if recErr != nil {
			log.Error(ctx, "recording S3 upload start", "key", key, "error", recErr)
			if w.strictRecorder {
				// An object without a row is invisible to finalize; fail now
				// (the segments are still spooled) rather than upload into a
				// black hole.
				w.abortMultipart(ctx)
				w.current = nil
				return fmt.Errorf("recording S3 upload start for %s: %w", key, recErr)
			}
		}
		w.current.recordID = id
	}
	spmetrics.S3LiveRecObjectsStartedTotal.Inc()
	log.Log(ctx, "started S3 multipart upload", "key", key)
	return nil
}

// flushPart uploads exactly the first n buffered bytes as the next part,
// keeping the remainder buffered. Callers pass liveUploadPartSize for every
// part except the object's final flush (complete passes whatever is left) —
// R2 requires all non-trailing parts to have the same length.
func (w *objectWriter) flushPart(ctx context.Context, n int) error {
	if w.current == nil || n == 0 {
		return nil
	}
	w.current.partNum++
	partNum := w.current.partNum

	resp, err := w.u.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(w.u.bucket),
		Key:        aws.String(w.current.key),
		UploadId:   aws.String(w.current.uploadID),
		PartNumber: aws.Int32(partNum),
		Body:       bytes.NewReader(w.current.buf[:n]),
	})
	if err != nil {
		return fmt.Errorf("uploading part %d: %w", partNum, err)
	}
	log.Debug(ctx, "uploaded S3 part", "key", w.current.key, "part", partNum, "size", n)
	w.current.parts = append(w.current.parts, types.CompletedPart{
		ETag:       resp.ETag,
		PartNumber: aws.Int32(partNum),
	})
	spmetrics.S3LiveRecBytesUploadedTotal.Add(float64(n))
	w.current.totalSize += int64(n)
	w.current.buf = append(w.current.buf[:0], w.current.buf[n:]...)
	return nil
}

// appendAndFlush buffers one segment into the current object and flushes any
// full-size parts; a sub-part remainder stays buffered until the next segment
// or the object's completing flush.
func (w *objectWriter) appendAndFlush(ctx context.Context, seg []byte) error {
	w.current.buf = append(w.current.buf, seg...)
	for len(w.current.buf) >= liveUploadPartSize {
		if err := w.flushPart(ctx, liveUploadPartSize); err != nil {
			return err
		}
	}
	return nil
}

// complete flushes the trailing part and finalizes the current object. An
// object that never accumulated any bytes is aborted instead (S3 rejects an
// empty part list).
func (w *objectWriter) complete(ctx context.Context) error {
	if w.current == nil {
		return nil
	}
	if err := w.flushPart(ctx, len(w.current.buf)); err != nil {
		return fmt.Errorf("error flushing buffer: %w", err)
	}
	if len(w.current.parts) == 0 {
		w.abortMultipart(ctx)
		w.current = nil
		return nil
	}
	_, err := w.u.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(w.u.bucket),
		Key:      aws.String(w.current.key),
		UploadId: aws.String(w.current.uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: w.current.parts,
		},
	})
	if err != nil {
		return fmt.Errorf("completing multipart upload %s: %w", w.current.key, err)
	}
	spmetrics.S3LiveRecObjectsCompletedTotal.Inc()
	log.Log(ctx, "completed S3 multipart upload", "key", w.current.key, "parts", len(w.current.parts), "size", w.current.totalSize)
	if w.u.recorder != nil && w.current.recordID != "" {
		if recErr := w.u.recorder.RecordComplete(ctx, w.current.recordID, int32(len(w.current.parts)), w.current.totalSize); recErr != nil {
			log.Error(ctx, "recording S3 upload completion", "key", w.current.key, "error", recErr)
			if w.strictRecorder {
				// The object completed in S3 but finalize will never see it
				// (its row stays incomplete). Surface the failure so the spool
				// segments are retried into a fresh, properly-recorded object;
				// the completed-but-unrecorded one is orphaned storage, not
				// lost footage.
				return fmt.Errorf("recording S3 upload completion for %s: %w", w.current.key, recErr)
			}
		}
	}
	w.current = nil
	return nil
}

func (w *objectWriter) abortMultipart(ctx context.Context) {
	if _, err := w.u.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(w.u.bucket),
		Key:      aws.String(w.current.key),
		UploadId: aws.String(w.current.uploadID),
	}); err != nil {
		log.Error(ctx, "aborting S3 multipart upload", "key", w.current.key, "error", err)
	}
}

// abandon is the in-memory loop's failure recovery: abort the broken object
// (so the backend doesn't hold its parts) and drop its un-completed bytes,
// loudly. The next segment starts a fresh object, so one bad object costs a
// gap in the recording instead of wedging the uploader for the rest of the
// stream (the abandoned object's recorder row never completes, so finalize
// skips it). Before this existed, the first error killed the loop: segments
// backed up silently and the stream never recorded another byte.
func (w *objectWriter) abandon(ctx context.Context, reason error) {
	if w.current == nil {
		// Nothing in flight (e.g. CreateMultipartUpload itself failed); the
		// segment is still dropped, so say so.
		spmetrics.S3LiveRecSegmentsDroppedTotal.Inc()
		log.Error(ctx, "error in live-rec S3 upload; segment dropped", "error", reason)
		return
	}
	spmetrics.S3LiveRecObjectsAbandonedTotal.Inc()
	spmetrics.S3LiveRecBytesDroppedTotal.Add(float64(w.current.totalSize + int64(len(w.current.buf))))
	log.Error(ctx, "abandoning live-rec S3 object; its bytes will be missing from the recording",
		"key", w.current.key,
		"uploadedBytes", w.current.totalSize,
		"droppedBufferedBytes", len(w.current.buf),
		"error", reason,
	)
	w.abortMultipart(ctx)
	w.current = nil
}

// abort is the spool loop's failure recovery: abort the broken object but keep
// nothing-lost semantics — the segments are still on disk and will be retried,
// so this logs a retry, not a loss.
func (w *objectWriter) abort(ctx context.Context, reason error) {
	if w.current == nil {
		return
	}
	spmetrics.S3LiveRecUploadRetriesTotal.Inc()
	log.Error(ctx, "aborting live-rec S3 object; its segments are retained in the spool and will be retried",
		"key", w.current.key, "error", reason)
	w.abortMultipart(ctx)
	w.current = nil
}

// uploadLoop reads bare segments off segCh and manages multipart uploads,
// buffering purely in memory — the no-spool fallback. Runs until segCh is
// closed (Close) or ctx is canceled. ctx is intentionally independent of the
// session context so a final object can still be completed after the stream
// tears down.
func (u *S3Uploader) uploadLoop(ctx context.Context) {
	ctx = log.WithLogValues(ctx, "func", "s3.uploadLoop")
	w := &objectWriter{u: u}

	handleSegment := func(seg []byte) error {
		now := time.Now()

		// Roll over to a new object when the current one has run for cutoverEvery,
		// or when the livestream changed (a new place.stream.livestream "chapter"
		// record). Cutting over on the livestream change keeps each object within
		// a single livestream so finalize can select exactly one livestream's
		// objects without one straddling two chapters.
		if w.current != nil && (now.Sub(w.current.started) >= u.cutoverEvery || w.current.livestreamURI != u.getLivestreamURI()) {
			if err := w.complete(ctx); err != nil {
				return err
			}
		}
		if w.current == nil {
			if err := w.start(ctx, u.getLivestreamURI(), now); err != nil {
				return err
			}
		}
		return w.appendAndFlush(ctx, seg)
	}

	for {
		select {
		case cmd, ok := <-u.segCh:
			if !ok {
				// No more segments; complete any in-progress upload. This is the
				// one error that still surfaces through done/Close — there are no
				// more segments coming to recover with.
				err := w.complete(ctx)
				if err != nil {
					err = fmt.Errorf("error completing upload: %w", err)
					w.abandon(ctx, err)
				}
				u.done <- err
				return
			}
			if cmd.cutover {
				// Close out the current object so it's immediately finalize-able
				// (e.g. the livestream just ended). No-op if nothing is in flight.
				if err := w.complete(ctx); err != nil {
					w.abandon(ctx, fmt.Errorf("error completing upload on cutover: %w", err))
				}
				continue
			}
			log.Debug(ctx, "received segment for S3 upload", "size", len(cmd.seg))
			if err := handleSegment(cmd.seg); err != nil {
				// The triggering segment is dropped along with the object: it may
				// already be partially flushed into it, so it can't be salvaged.
				w.abandon(ctx, fmt.Errorf("error handling segment: %w", err))
			}

		case <-ctx.Done():
			err := w.complete(ctx)
			if err != nil {
				err = fmt.Errorf("error completing upload: %w", err)
				w.abandon(ctx, err)
			}
			u.done <- err
			return
		}
	}
}

// Spool-mode retry pacing: first failure retries quickly (most S3 hiccups are
// momentary), backing off exponentially for persistent failures. During
// backoff every retry re-uploads the in-flight object's parts from scratch,
// so the cap balances catch-up latency against wasted upload bandwidth when
// the bucket is deterministically broken.
var (
	spoolRetryMin = 15 * time.Second
	spoolRetryMax = 30 * time.Minute
)

// uploadLoopSpool consumes segments from the disk spool instead of the
// channel; segCh carries only kicks (a new segment landed), cutovers, and the
// close signal. Segments are deleted from the spool only after the object
// containing them completes, so any upload failure rewinds to the failed
// object's first segment and retries with backoff — recorded bytes survive
// provider outages, incompatibilities, and (via startup salvage) node
// crashes, costing only lag.
func (u *S3Uploader) uploadLoopSpool(ctx context.Context) {
	ctx = log.WithLogValues(ctx, "func", "s3.uploadLoopSpool")
	w := &objectWriter{u: u, strictRecorder: true}
	var nextSeq int64 = 1     // next spool seq to consume
	var objFirstSeq int64     // first seq of the in-flight object (rewind point)
	var lastConsumed int64    // last seq appended into the in-flight object
	var backoff time.Duration // 0 = healthy
	var retryAt time.Time

	fail := func(err error) {
		if w.current != nil {
			nextSeq = objFirstSeq // re-consume the whole object on retry
			w.abort(ctx, err)
		}
		if backoff == 0 {
			backoff = spoolRetryMin
		} else if backoff = backoff * 2; backoff > spoolRetryMax {
			backoff = spoolRetryMax
		}
		retryAt = time.Now().Add(backoff)
		log.Error(ctx, "live-rec upload failed; segments retained in spool for retry",
			"retryIn", backoff.String(), "spoolBytes", u.spool.Bytes(), "error", err)
	}

	// completeAndAck finalizes the in-flight object and only then deletes its
	// segments from the spool — the ack is what makes an upload failure cost
	// lag instead of data.
	completeAndAck := func() error {
		if w.current == nil {
			return nil
		}
		if err := w.complete(ctx); err != nil {
			return err
		}
		u.spool.Ack(lastConsumed)
		backoff = 0
		retryAt = time.Time{}
		return nil
	}

	// process consumes every spool segment that's ready, building objects with
	// the same rollover rules as the live loop but keyed to segment arrival
	// times (spool mtimes), so objects assembled during catch-up still map to
	// real stream time and sort honestly in finalize.
	process := func() {
		if !retryAt.IsZero() {
			if time.Now().Before(retryAt) {
				return
			}
			// The retry is due: clear it so the wake timer stops re-arming
			// with an in-the-past deadline (a ~100ms busy-wake otherwise).
			// backoff itself only resets when an object completes, so a
			// still-failing bucket keeps escalating rather than restarting
			// at spoolRetryMin every part success.
			retryAt = time.Time{}
		}
		for {
			seq, ok := u.spool.NextFrom(nextSeq)
			if !ok {
				return
			}
			data, arrived, uri, err := u.spool.Get(seq)
			if err != nil {
				// Evicted or unreadable; already accounted when it was dropped.
				log.Error(ctx, "skipping unreadable spool segment", "seq", seq, "error", err)
				nextSeq = seq + 1
				continue
			}
			if w.current != nil && (arrived.Sub(w.current.started) >= u.cutoverEvery || uri != w.current.livestreamURI) {
				if err := completeAndAck(); err != nil {
					fail(fmt.Errorf("completing on rollover: %w", err))
					return
				}
			}
			if w.current == nil {
				if err := w.start(ctx, uri, arrived); err != nil {
					fail(fmt.Errorf("starting object: %w", err))
					return
				}
				objFirstSeq = seq
			}
			if err := w.appendAndFlush(ctx, data); err != nil {
				fail(fmt.Errorf("uploading part: %w", err))
				return
			}
			lastConsumed = seq
			nextSeq = seq + 1
		}
	}

	// finish is the common drain for close/cancel: one immediate attempt at
	// the backlog regardless of backoff, then either a clean spool teardown or
	// a loud handoff to the next startup's salvage pass.
	finish := func() {
		retryAt = time.Time{}
		process()
		err := completeAndAck()
		if err != nil {
			err = fmt.Errorf("error completing upload: %w", err)
			w.abort(ctx, err)
		}
		if u.spool.Len() > 0 {
			log.Error(ctx, "leaving un-uploaded segments in live-rec spool for salvage at next startup",
				"segments", u.spool.Len(), "bytes", u.spool.Bytes())
			if err == nil {
				err = fmt.Errorf("%d segments (%d bytes) left in spool for salvage", u.spool.Len(), u.spool.Bytes())
			}
		} else {
			u.spool.Destroy()
		}
		u.done <- err
	}

	for {
		// While backing off, wake up when the retry is due; otherwise sleep
		// until something happens.
		var retryTimer <-chan time.Time
		if !retryAt.IsZero() {
			retryTimer = time.After(time.Until(retryAt) + 100*time.Millisecond)
		}
		select {
		case cmd, ok := <-u.segCh:
			if !ok {
				finish()
				return
			}
			if cmd.cutover {
				// Close out the current object so it's immediately finalize-able
				// (e.g. the livestream just ended). A cutover overrides any
				// backoff: the stream is ending, so make one immediate attempt
				// at the backlog — if the bucket is still down it fails fast and
				// re-arms the backoff, and the tail completes on a later retry
				// or next-startup salvage (a late tail can merge across the cut;
				// finalize orders by started-at, so the recording stays correct,
				// just chunked differently).
				retryAt = time.Time{}
				process()
				if err := completeAndAck(); err != nil {
					fail(fmt.Errorf("error completing upload on cutover: %w", err))
				}
				continue
			}
			process()
		case <-retryTimer:
			process()
		case <-ctx.Done():
			finish()
			return
		}
	}
}
