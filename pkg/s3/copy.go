package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/log"
)

// maxCopyObjectSize is the largest object S3 will copy in a single
// CopyObject call (5 GiB). Anything larger fails with EntityTooLarge and
// must be copied part-by-part with a multipart upload driven by
// UploadPartCopy.
const maxCopyObjectSize = 5 * 1024 * 1024 * 1024

// copyPartSize is the byte range each UploadPartCopy transfers. The copy
// happens server-side inside S3, so — unlike MultipartPartSize, which sizes
// an in-process buffer — this never costs us memory; it's sized large to
// keep the part (and request) count low while staying under S3's 5 GiB
// per-part ceiling. At 1 GiB/part the 10000-part limit allows objects up to
// ~10 TiB, far beyond any VOD.
const copyPartSize = 1024 * 1024 * 1024

// copyConcurrency bounds how many UploadPartCopy requests run at once.
// Mirrors multipartUploadConcurrency: server-side range copies serialize
// badly otherwise, dragging a large VOD's finalize out for minutes.
const copyConcurrency = 8

// copyAPI is the subset of *s3.Client that Copy uses. Pulled out so tests
// can inject a fake; *s3.Client satisfies it.
type copyAPI interface {
	HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	CopyObject(context.Context, *s3.CopyObjectInput, ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
	CreateMultipartUpload(context.Context, *s3.CreateMultipartUploadInput, ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error)
	UploadPartCopy(context.Context, *s3.UploadPartCopyInput, ...func(*s3.Options)) (*s3.UploadPartCopyOutput, error)
	CompleteMultipartUpload(context.Context, *s3.CompleteMultipartUploadInput, ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error)
	AbortMultipartUpload(context.Context, *s3.AbortMultipartUploadInput, ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error)
}

// Copy copies an object within bucket from srcKey to dstKey, preserving the
// source's content type. Objects at or below maxCopyObjectSize use a single
// CopyObject; larger objects exceed S3's single-copy limit, so they're
// copied with a multipart upload whose parts are server-side UploadPartCopy
// range copies. A HeadObject against the source missing key surfaces as a
// NotFound error the caller can sniff for idempotency.
func Copy(ctx context.Context, client *s3.Client, bucket, srcKey, dstKey string) error {
	return copyObject(ctx, client, bucket, srcKey, dstKey)
}

func copyObject(ctx context.Context, client copyAPI, bucket, srcKey, dstKey string) error {
	ctx = log.WithLogValues(ctx, "func", "s3.Copy")
	ctx, span := s3Tracer.Start(ctx, "s3.Copy", trace.WithAttributes(
		attribute.String("bucket", bucket),
		attribute.String("src_key", srcKey),
		attribute.String("dst_key", dstKey),
	))
	defer span.End()

	head, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(srcKey),
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("head s3://%s/%s: %w", bucket, srcKey, err)
	}
	size := aws.ToInt64(head.ContentLength)
	contentType := aws.ToString(head.ContentType)
	span.SetAttributes(attribute.Int64("size_bytes", size))

	// CopySource is "bucket/key"; keep it unescaped to match what the rest
	// of the codebase already sends (our keys are content hashes, UUIDs and
	// DIDs that the endpoint accepts raw).
	copySource := bucket + "/" + srcKey

	if size <= maxCopyObjectSize {
		if _, err := client.CopyObject(ctx, &s3.CopyObjectInput{
			Bucket:     aws.String(bucket),
			Key:        aws.String(dstKey),
			CopySource: aws.String(copySource),
		}); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "copy_object")
			return fmt.Errorf("copy s3://%s/%s -> %s: %w", bucket, srcKey, dstKey, err)
		}
		return nil
	}

	if err := multipartCopy(ctx, client, bucket, dstKey, copySource, contentType, size); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "multipart_copy")
		return err
	}
	return nil
}

// multipartCopy copies an over-5-GiB object by opening a multipart upload at
// dstKey and filling it with server-side UploadPartCopy range copies of the
// source. Parts run concurrently (bounded by copyConcurrency) but land at
// their fixed part numbers, so the completed-parts list is already ordered.
// Any failure aborts the upload so S3 doesn't retain orphaned parts.
func multipartCopy(ctx context.Context, client copyAPI, bucket, dstKey, copySource, contentType string, size int64) error {
	create := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(dstKey),
	}
	if contentType != "" {
		create.ContentType = aws.String(contentType)
	}
	resp, err := client.CreateMultipartUpload(ctx, create)
	if err != nil {
		return fmt.Errorf("create multipart copy s3://%s/%s: %w", bucket, dstKey, err)
	}
	uploadID := aws.ToString(resp.UploadId)

	// Pre-compute the (start,end) byte range of every part. Part numbers are
	// 1-based; CopySourceRange's end offset is inclusive.
	type byteRange struct{ start, end int64 }
	var ranges []byteRange
	for off := int64(0); off < size; off += copyPartSize {
		end := off + copyPartSize - 1
		if end > size-1 {
			end = size - 1
		}
		ranges = append(ranges, byteRange{start: off, end: end})
	}

	parts := make([]types.CompletedPart, len(ranges))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(copyConcurrency)
	for i, r := range ranges {
		partNum := int32(i + 1)
		g.Go(func() error {
			res, err := client.UploadPartCopy(gctx, &s3.UploadPartCopyInput{
				Bucket:          aws.String(bucket),
				Key:             aws.String(dstKey),
				UploadId:        aws.String(uploadID),
				PartNumber:      aws.Int32(partNum),
				CopySource:      aws.String(copySource),
				CopySourceRange: aws.String(fmt.Sprintf("bytes=%d-%d", r.start, r.end)),
			})
			if err != nil {
				return fmt.Errorf("upload part copy %d (bytes %d-%d) s3://%s/%s: %w", partNum, r.start, r.end, bucket, dstKey, err)
			}
			if res.CopyPartResult == nil {
				return fmt.Errorf("upload part copy %d s3://%s/%s: missing CopyPartResult", partNum, bucket, dstKey)
			}
			parts[i] = types.CompletedPart{
				ETag:       res.CopyPartResult.ETag,
				PartNumber: aws.Int32(partNum),
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		// Best-effort cleanup so a failed copy doesn't leave parts S3 keeps
		// billing for. Use ctx (not the cancelled gctx) so the abort runs.
		_, _ = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(bucket),
			Key:      aws.String(dstKey),
			UploadId: aws.String(uploadID),
		})
		return err
	}

	if _, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(bucket),
		Key:             aws.String(dstKey),
		UploadId:        aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: parts},
	}); err != nil {
		_, _ = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(bucket),
			Key:      aws.String(dstKey),
			UploadId: aws.String(uploadID),
		})
		return fmt.Errorf("complete multipart copy s3://%s/%s: %w", bucket, dstKey, err)
	}
	log.Log(ctx, "completed S3 multipart copy", "bucket", bucket, "key", dstKey, "parts", len(parts), "size", size)
	return nil
}
