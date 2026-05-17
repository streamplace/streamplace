package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

// MultipartWriter is an io.WriteCloser that streams writes into an
// in-progress S3 multipart upload. Writes accumulate in an internal
// buffer; each time the buffer hits MultipartPartSize it's flushed as
// the next part. Call Complete to finalize the upload (returns the
// final object metadata) or Abort to discard everything written.
//
// Close runs Abort if Complete hasn't been called yet — useful as a
// `defer w.Close()` guard around the upload that releases resources on
// any error path.
type MultipartWriter struct {
	ctx      context.Context
	client   *s3.Client
	bucket   string
	key      string
	uploadID string

	buf     []byte
	parts   []types.CompletedPart
	partNum int32

	finalized bool
}

// NewMultipartWriter starts a fresh multipart upload at the given S3
// object key and returns a writer that streams into it.
func NewMultipartWriter(ctx context.Context, client *s3.Client, bucket, key, contentType string) (*MultipartWriter, error) {
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
	}, nil
}

// Write buffers data and flushes parts of MultipartPartSize each as soon
// as enough has accumulated. Never returns a short write.
func (w *MultipartWriter) Write(p []byte) (int, error) {
	if w.finalized {
		return 0, fmt.Errorf("write after Complete/Abort on s3://%s/%s", w.bucket, w.key)
	}
	w.buf = append(w.buf, p...)
	for len(w.buf) >= MultipartPartSize {
		if err := w.flushPart(MultipartPartSize); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (w *MultipartWriter) flushPart(size int) error {
	w.partNum++
	partStart := time.Now()
	ctx, span := s3Tracer.Start(w.ctx, "s3.MultipartWriter.flushPart", trace.WithAttributes(
		attribute.String("bucket", w.bucket),
		attribute.String("key", w.key),
		attribute.Int("part_number", int(w.partNum)),
		attribute.Int("part_size_bytes", size),
	))
	defer span.End()
	resp, err := w.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(w.bucket),
		Key:        aws.String(w.key),
		UploadId:   aws.String(w.uploadID),
		PartNumber: aws.Int32(w.partNum),
		Body:       bytes.NewReader(w.buf[:size]),
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "upload_part")
		return fmt.Errorf("upload part %d to s3://%s/%s: %w", w.partNum, w.bucket, w.key, err)
	}
	w.parts = append(w.parts, types.CompletedPart{
		ETag:       resp.ETag,
		PartNumber: aws.Int32(w.partNum),
	})
	w.buf = w.buf[size:]
	spmetrics.S3MultipartPartsUploadedTotal.Inc()
	spmetrics.S3MultipartBytesUploadedTotal.Add(float64(size))
	log.Debug(w.ctx, "S3 multipart part uploaded",
		"key", w.key,
		"part", w.partNum,
		"size", size,
		"duration_ms", time.Since(partStart).Milliseconds(),
	)
	return nil
}

// Complete finalizes the multipart upload, flushing any remaining
// buffered bytes as a final part. After Complete returns, the object
// exists at the configured key. The writer cannot be reused.
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
		if err := w.flushPart(len(w.buf)); err != nil {
			span.RecordError(err)
			return err
		}
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
		log.Debug(w.ctx, "completed empty S3 multipart upload via PutObject", "bucket", w.bucket, "key", w.key)
		return nil
	}
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
// the parts so far. Safe to call multiple times.
func (w *MultipartWriter) Abort() error {
	if w.finalized {
		return nil
	}
	w.finalized = true
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
	log.Debug(w.ctx, "aborted S3 multipart upload", "bucket", w.bucket, "key", w.key, "parts_pending", len(w.parts))
	return nil
}

// Close runs Abort if Complete hasn't been called; idempotent.
func (w *MultipartWriter) Close() error { return w.Abort() }

var _ io.WriteCloser = (*MultipartWriter)(nil)
