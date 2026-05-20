package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spmetrics"
)

// MultipartPartSize is the part size used by MultipartWriter. S3 requires
// each part (except the last) to be at least 5 MB; the 16 MB choice
// gives headroom for a 10000-part upload to exceed 150 GB without
// hitting the part-count limit.
const MultipartPartSize = 16 * 1024 * 1024

// multipartUploadConcurrency bounds how many parts upload at once. Each
// in-flight part holds its own MultipartPartSize buffer, so peak memory
// per writer is roughly multipartUploadConcurrency * MultipartPartSize.
//
// Parts upload independently and are reordered by part number before the
// upload is finalized, so concurrency never changes the resulting object
// — it only stops a slow or high-latency backend from serializing the
// whole upload. With serial uploads, a VOD's mux loop was throttled to
// one ~16 MB UploadPart at a time (~30s/part even against a local store),
// so a 1.5 GB upload dragged on for tens of minutes.
const multipartUploadConcurrency = 8

// multipartAPI is the subset of *s3.Client that MultipartWriter calls.
// Pulled out so tests can inject a fake; *s3.Client satisfies it.
type multipartAPI interface {
	CreateMultipartUpload(context.Context, *s3.CreateMultipartUploadInput, ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error)
	UploadPart(context.Context, *s3.UploadPartInput, ...func(*s3.Options)) (*s3.UploadPartOutput, error)
	CompleteMultipartUpload(context.Context, *s3.CompleteMultipartUploadInput, ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error)
	AbortMultipartUpload(context.Context, *s3.AbortMultipartUploadInput, ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// MultipartWriter is an io.WriteCloser that streams writes into an
// in-progress S3 multipart upload. Writes accumulate in an internal
// buffer; each time the buffer hits MultipartPartSize a part is handed
// off to a bounded pool of upload goroutines (see
// multipartUploadConcurrency), so the caller isn't blocked behind one
// part upload at a time. Call Complete to finalize the upload or Abort to
// discard everything written.
//
// Write must be called from a single goroutine; the concurrency is
// internal to the part uploads. Close runs Abort if Complete hasn't been
// called yet — useful as a `defer w.Close()` guard around the upload that
// releases resources on any error path.
type MultipartWriter struct {
	ctx      context.Context
	client   multipartAPI
	bucket   string
	key      string
	uploadID string

	// buf and partNum are touched only by the single Write/Complete
	// goroutine; the upload goroutines never read them.
	buf     []byte
	partNum int32

	sem chan struct{}  // bounds in-flight part uploads (backpressure)
	wg  sync.WaitGroup // tracks in-flight part uploads

	mu        sync.Mutex // guards parts + uploadErr (touched by upload goroutines)
	parts     []types.CompletedPart
	uploadErr error

	finalized bool
}

// NewMultipartWriter starts a fresh multipart upload at the given S3
// object key and returns a writer that streams into it.
func NewMultipartWriter(ctx context.Context, client *s3.Client, bucket, key, contentType string) (*MultipartWriter, error) {
	return newMultipartWriter(ctx, client, bucket, key, contentType)
}

func newMultipartWriter(ctx context.Context, client multipartAPI, bucket, key, contentType string) (*MultipartWriter, error) {
	// Tag the writer's logs so the per-part debug lines (off by default,
	// since there's one per 16 MB) can be turned back on at runtime with
	// --debug=func=MultipartWriter:N (or SP_DEBUG) — no recompile needed.
	ctx = log.WithLogValues(ctx, "func", "MultipartWriter")
	ctx, span := s3Tracer.Start(ctx, "s3.NewMultipartWriter", trace.WithAttributes(
		attribute.String("bucket", bucket),
		attribute.String("key", key),
		attribute.String("content_type", contentType),
	))
	defer span.End()
	in := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	resp, err := client.CreateMultipartUpload(ctx, in)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("create multipart upload s3://%s/%s: %w", bucket, key, err)
	}
	uploadID := aws.ToString(resp.UploadId)
	span.SetAttributes(attribute.String("upload_id", uploadID))
	log.Debug(ctx, "started S3 multipart upload", "bucket", bucket, "key", key, "uploadId", uploadID)
	return &MultipartWriter{
		ctx:      ctx,
		client:   client,
		bucket:   bucket,
		key:      key,
		uploadID: uploadID,
		sem:      make(chan struct{}, multipartUploadConcurrency),
	}, nil
}

// Write buffers data and dispatches parts of MultipartPartSize each (to
// the bounded upload pool) as soon as enough has accumulated. Never
// returns a short write. If an earlier part upload failed, the error
// surfaces here.
func (w *MultipartWriter) Write(p []byte) (int, error) {
	if w.finalized {
		return 0, fmt.Errorf("write after Complete/Abort on s3://%s/%s", w.bucket, w.key)
	}
	w.buf = append(w.buf, p...)
	for len(w.buf) >= MultipartPartSize {
		if err := w.uploadError(); err != nil {
			return 0, err
		}
		w.dispatchPart(MultipartPartSize)
	}
	if err := w.uploadError(); err != nil {
		return 0, err
	}
	return len(p), nil
}

// dispatchPart copies the next `size` bytes off the buffer, assigns the
// next part number, and uploads the part on a bounded worker goroutine.
// The copy is required: w.buf is resliced and reused for later writes,
// but the upload goroutine reads its part asynchronously. The semaphore
// acquire blocks (and so backpressures the caller) once
// multipartUploadConcurrency parts are in flight.
func (w *MultipartWriter) dispatchPart(size int) {
	w.partNum++
	num := w.partNum
	body := make([]byte, size)
	copy(body, w.buf[:size])
	w.buf = w.buf[size:]

	w.sem <- struct{}{}
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer func() { <-w.sem }()
		w.uploadPart(num, body)
	}()
}

func (w *MultipartWriter) uploadPart(num int32, body []byte) {
	partStart := time.Now()
	ctx, span := s3Tracer.Start(w.ctx, "s3.MultipartWriter.flushPart", trace.WithAttributes(
		attribute.String("bucket", w.bucket),
		attribute.String("key", w.key),
		attribute.Int("part_number", int(num)),
		attribute.Int("part_size_bytes", len(body)),
	))
	defer span.End()
	resp, err := w.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(w.bucket),
		Key:        aws.String(w.key),
		UploadId:   aws.String(w.uploadID),
		PartNumber: aws.Int32(num),
		Body:       bytes.NewReader(body),
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "upload_part")
		w.setUploadError(fmt.Errorf("upload part %d to s3://%s/%s: %w", num, w.bucket, w.key, err))
		return
	}
	w.mu.Lock()
	w.parts = append(w.parts, types.CompletedPart{
		ETag:       resp.ETag,
		PartNumber: aws.Int32(num),
	})
	w.mu.Unlock()
	spmetrics.S3MultipartPartsUploadedTotal.Inc()
	spmetrics.S3MultipartBytesUploadedTotal.Add(float64(len(body)))
	log.Debug(w.ctx, "S3 multipart part uploaded",
		"key", w.key,
		"part", num,
		"size", len(body),
		"duration_ms", time.Since(partStart).Milliseconds(),
	)
}

func (w *MultipartWriter) uploadError() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.uploadErr
}

func (w *MultipartWriter) setUploadError(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.uploadErr == nil {
		w.uploadErr = err
	}
}

// Complete finalizes the multipart upload, flushing any remaining
// buffered bytes as a final part and waiting for all in-flight parts.
// After Complete returns, the object exists at the configured key. The
// writer cannot be reused.
func (w *MultipartWriter) Complete() error {
	if w.finalized {
		return fmt.Errorf("Complete called twice on s3://%s/%s", w.bucket, w.key)
	}
	ctx, span := s3Tracer.Start(w.ctx, "s3.MultipartWriter.Complete", trace.WithAttributes(
		attribute.String("bucket", w.bucket),
		attribute.String("key", w.key),
		attribute.String("upload_id", w.uploadID),
	))
	defer span.End()
	if len(w.buf) > 0 {
		w.dispatchPart(len(w.buf))
	}
	// All part uploads have been dispatched; wait for them before reading
	// w.parts (after this, no upload goroutine touches shared state).
	w.wg.Wait()
	if err := w.uploadError(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "upload_part")
		return err
	}
	span.SetAttributes(attribute.Int("part_count", len(w.parts)))
	if len(w.parts) == 0 {
		// Zero-byte upload: S3 won't accept an empty CompletedMultipartUpload,
		// so abort and create an empty object via PutObject.
		_, _ = w.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(w.bucket),
			Key:      aws.String(w.key),
			UploadId: aws.String(w.uploadID),
		})
		if _, err := w.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(w.bucket),
			Key:    aws.String(w.key),
			Body:   bytes.NewReader(nil),
		}); err != nil {
			span.RecordError(err)
			return fmt.Errorf("put zero-byte object s3://%s/%s: %w", w.bucket, w.key, err)
		}
		w.finalized = true
		log.Log(w.ctx, "completed empty S3 multipart upload via PutObject", "bucket", w.bucket, "key", w.key)
		return nil
	}
	// Concurrent uploads finish out of order, but CompleteMultipartUpload
	// requires parts in ascending PartNumber order.
	sort.Slice(w.parts, func(i, j int) bool {
		return aws.ToInt32(w.parts[i].PartNumber) < aws.ToInt32(w.parts[j].PartNumber)
	})
	completeStart := time.Now()
	_, err := w.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(w.bucket),
		Key:      aws.String(w.key),
		UploadId: aws.String(w.uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: w.parts,
		},
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "complete")
		return fmt.Errorf("complete multipart s3://%s/%s: %w", w.bucket, w.key, err)
	}
	w.finalized = true
	log.Log(w.ctx, "completed S3 multipart upload",
		"bucket", w.bucket,
		"key", w.key,
		"parts", len(w.parts),
		"complete_duration_ms", time.Since(completeStart).Milliseconds(),
	)
	return nil
}

// Abort cancels the upload, releasing any storage S3 has allocated for
// the parts so far. It waits for in-flight part uploads to settle first.
// Safe to call multiple times.
func (w *MultipartWriter) Abort() error {
	if w.finalized {
		return nil
	}
	w.finalized = true
	w.wg.Wait()
	ctx, span := s3Tracer.Start(w.ctx, "s3.MultipartWriter.Abort", trace.WithAttributes(
		attribute.String("bucket", w.bucket),
		attribute.String("key", w.key),
		attribute.Int("parts_pending", len(w.parts)),
	))
	defer span.End()
	_, err := w.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(w.bucket),
		Key:      aws.String(w.key),
		UploadId: aws.String(w.uploadID),
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("abort multipart s3://%s/%s: %w", w.bucket, w.key, err)
	}
	log.Log(w.ctx, "aborted S3 multipart upload", "bucket", w.bucket, "key", w.key, "parts_pending", len(w.parts))
	return nil
}

// Close runs Abort if Complete hasn't been called; idempotent.
func (w *MultipartWriter) Close() error { return w.Abort() }

var _ io.WriteCloser = (*MultipartWriter)(nil)
