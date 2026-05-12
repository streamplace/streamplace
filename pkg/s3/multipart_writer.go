package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
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
	in := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	resp, err := client.CreateMultipartUpload(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("create multipart upload s3://%s/%s: %w", bucket, key, err)
	}
	return &MultipartWriter{
		ctx:      ctx,
		client:   client,
		bucket:   bucket,
		key:      key,
		uploadID: aws.ToString(resp.UploadId),
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
	resp, err := w.client.UploadPart(w.ctx, &s3.UploadPartInput{
		Bucket:     aws.String(w.bucket),
		Key:        aws.String(w.key),
		UploadId:   aws.String(w.uploadID),
		PartNumber: aws.Int32(w.partNum),
		Body:       bytes.NewReader(w.buf[:size]),
	})
	if err != nil {
		return fmt.Errorf("upload part %d to s3://%s/%s: %w", w.partNum, w.bucket, w.key, err)
	}
	w.parts = append(w.parts, types.CompletedPart{
		ETag:       resp.ETag,
		PartNumber: aws.Int32(w.partNum),
	})
	w.buf = w.buf[size:]
	return nil
}

// Complete finalizes the multipart upload, flushing any remaining
// buffered bytes as a final part. After Complete returns, the object
// exists at the configured key. The writer cannot be reused.
func (w *MultipartWriter) Complete() error {
	if w.finalized {
		return fmt.Errorf("Complete called twice on s3://%s/%s", w.bucket, w.key)
	}
	if len(w.buf) > 0 {
		if err := w.flushPart(len(w.buf)); err != nil {
			return err
		}
	}
	if len(w.parts) == 0 {
		// Zero-byte upload: S3 won't accept an empty CompletedMultipartUpload,
		// so abort and create an empty object via PutObject.
		_, _ = w.client.AbortMultipartUpload(w.ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(w.bucket),
			Key:      aws.String(w.key),
			UploadId: aws.String(w.uploadID),
		})
		if _, err := w.client.PutObject(w.ctx, &s3.PutObjectInput{
			Bucket: aws.String(w.bucket),
			Key:    aws.String(w.key),
			Body:   bytes.NewReader(nil),
		}); err != nil {
			return fmt.Errorf("put zero-byte object s3://%s/%s: %w", w.bucket, w.key, err)
		}
		w.finalized = true
		return nil
	}
	_, err := w.client.CompleteMultipartUpload(w.ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(w.bucket),
		Key:      aws.String(w.key),
		UploadId: aws.String(w.uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: w.parts,
		},
	})
	if err != nil {
		return fmt.Errorf("complete multipart s3://%s/%s: %w", w.bucket, w.key, err)
	}
	w.finalized = true
	return nil
}

// Abort cancels the upload, releasing any storage S3 has allocated for
// the parts so far. Safe to call multiple times.
func (w *MultipartWriter) Abort() error {
	if w.finalized {
		return nil
	}
	w.finalized = true
	_, err := w.client.AbortMultipartUpload(w.ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(w.bucket),
		Key:      aws.String(w.key),
		UploadId: aws.String(w.uploadID),
	})
	if err != nil {
		return fmt.Errorf("abort multipart s3://%s/%s: %w", w.bucket, w.key, err)
	}
	return nil
}

// Close runs Abort if Complete hasn't been called; idempotent.
func (w *MultipartWriter) Close() error { return w.Abort() }

var _ io.WriteCloser = (*MultipartWriter)(nil)
