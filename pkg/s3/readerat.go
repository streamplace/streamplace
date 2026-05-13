package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spmetrics"
)

var s3Tracer = otel.Tracer("s3")

// ReaderAt is an io.ReaderAt against an S3 object. It re-uses a single
// open GetObject body for sequential reads and transparently closes &
// reopens it on a non-sequential read. This makes it efficient for the
// common case of "seek once, then drain forward" that gstreamer demuxers
// produce in pull mode, while still allowing arbitrary jumps.
//
// Concurrent ReadAt calls are serialized by an internal mutex; this matches
// the gstreamer appsrc usage where need-data and seek-data callbacks fire
// on a single streaming thread.
type ReaderAt struct {
	ctx    context.Context
	client *s3.Client
	bucket string
	key    string
	size   int64

	mu   sync.Mutex
	body io.ReadCloser
	pos  int64
}

// NewReaderAt issues a HEAD against the given S3 object to discover its
// size, then returns a ReaderAt. The provided context is used for both
// the HEAD and any subsequent GetObject requests; cancelling it aborts
// in-flight reads and is the supported way to tear the ReaderAt down.
func NewReaderAt(ctx context.Context, client *s3.Client, bucket, key string) (*ReaderAt, error) {
	ctx, span := s3Tracer.Start(ctx, "s3.NewReaderAt", trace.WithAttributes(
		attribute.String("bucket", bucket),
		attribute.String("key", key),
	))
	defer span.End()
	head, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("head s3://%s/%s: %w", bucket, key, err)
	}
	if head.ContentLength == nil {
		err := fmt.Errorf("s3://%s/%s missing content-length", bucket, key)
		span.RecordError(err)
		return nil, err
	}
	size := *head.ContentLength
	span.SetAttributes(attribute.Int64("size_bytes", size))
	log.Debug(ctx, "opened S3 ReaderAt", "bucket", bucket, "key", key, "size", size)
	return &ReaderAt{
		ctx:    ctx,
		client: client,
		bucket: bucket,
		key:    key,
		size:   size,
	}, nil
}

// Size returns the object size discovered at construction time.
func (r *ReaderAt) Size() int64 { return r.size }

// ReadAt implements io.ReaderAt. Sequential reads (off == previous end)
// drain the open body; non-sequential reads close the body and issue a
// fresh ranged GetObject starting at off.
func (r *ReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, fmt.Errorf("negative offset %d", off)
	}
	if off >= r.size {
		return 0, io.EOF
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	reopened := false
	if r.body == nil || off != r.pos {
		_ = r.closeBodyLocked()
		if err := r.openLocked(off); err != nil {
			spmetrics.S3ReaderAtReadsTotal.WithLabelValues("seek").Inc()
			return 0, err
		}
		reopened = true
	}
	if reopened {
		spmetrics.S3ReaderAtReadsTotal.WithLabelValues("seek").Inc()
	} else {
		spmetrics.S3ReaderAtReadsTotal.WithLabelValues("sequential").Inc()
	}

	want := len(p)
	remaining := r.size - off
	if int64(want) > remaining {
		want = int(remaining)
	}
	n, err := io.ReadFull(r.body, p[:want])
	r.pos += int64(n)

	// Truncated reads at the tail of the object surface as
	// ErrUnexpectedEOF from ReadFull; report EOF to the caller and
	// drop the now-empty body so the next read reopens.
	if errors.Is(err, io.ErrUnexpectedEOF) || (err == nil && r.pos >= r.size) {
		_ = r.closeBodyLocked()
		if r.pos >= r.size {
			err = io.EOF
		}
	}
	return n, err
}

func (r *ReaderAt) openLocked(off int64) error {
	spmetrics.S3ReaderAtOpensTotal.Inc()
	ctx, span := s3Tracer.Start(r.ctx, "s3.ReaderAt.open", trace.WithAttributes(
		attribute.String("bucket", r.bucket),
		attribute.String("key", r.key),
		attribute.Int64("offset", off),
	))
	defer span.End()
	resp, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(r.key),
		Range:  aws.String(fmt.Sprintf("bytes=%d-", off)),
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "get_object")
		return fmt.Errorf("get s3://%s/%s bytes=%d-: %w", r.bucket, r.key, off, err)
	}
	r.body = resp.Body
	r.pos = off
	log.Debug(r.ctx, "S3 ReaderAt ranged GET", "bucket", r.bucket, "key", r.key, "offset", off)
	return nil
}

func (r *ReaderAt) closeBodyLocked() error {
	if r.body == nil {
		return nil
	}
	err := r.body.Close()
	r.body = nil
	return err
}

// Close releases any open GetObject body. Safe to call concurrently with
// ReadAt; in-flight reads observe the closed body as an error.
func (r *ReaderAt) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closeBodyLocked()
}

// ParseURL parses an "s3://bucket/key" URL into its bucket and key parts.
// The key may contain forward slashes.
func ParseURL(u string) (bucket, key string, err error) {
	const prefix = "s3://"
	if !strings.HasPrefix(u, prefix) {
		return "", "", fmt.Errorf("not an s3:// URL: %q", u)
	}
	rest := strings.TrimPrefix(u, prefix)
	slash := strings.IndexByte(rest, '/')
	if slash <= 0 || slash == len(rest)-1 {
		return "", "", fmt.Errorf("invalid s3:// URL: %q", u)
	}
	return rest[:slash], rest[slash+1:], nil
}
