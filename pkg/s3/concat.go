package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/log"
)

// concatPartSize is the uniform length of every non-final part the concat
// emits. R2 — unlike AWS/minio — rejects CompleteMultipartUpload with "All
// non-trailing parts must have the same length" unless parts are uniform, so
// the destination byte stream is sliced into fixed windows rather than one
// part per source object. 16MB (matching MultipartPartSize) keeps the
// boundary-window downloads small while allowing a 10000-part object to
// reach ~156GB.
const concatPartSize = MultipartPartSize

// s3MaxParts is the multipart part-count ceiling shared by AWS and R2.
const s3MaxParts = 10000

// concatAPI is the subset of *s3.Client that ConcatWithHeader uses. Pulled out
// so tests can inject a fake; *s3.Client satisfies it.
type concatAPI interface {
	copyAPI
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	UploadPart(context.Context, *s3.UploadPartInput, ...func(*s3.Options)) (*s3.UploadPartOutput, error)
}

// ConcatWithHeader assembles destKey = header ++ srcKeys[0] ++ srcKeys[1] ++ …
// (the source objects in order) as a single multipart object, transferring
// almost everything server-side.
//
// The layout exists to give live-to-VOD finalize a content blob shaped exactly
// like an uploaded VOD ([init][segments…]) without re-uploading the (possibly
// many-GB) stream: the destination byte stream is sliced into concatPartSize
// windows (uniform non-trailing parts, which R2 requires); a window that falls
// entirely inside one source object becomes a server-side UploadPartCopy, and
// only the windows containing the header or an object boundary are downloaded
// and re-uploaded. That bounds the through-process traffic to roughly one
// window per source object regardless of stream length.
func ConcatWithHeader(ctx context.Context, client *s3.Client, bucket string, header []byte, srcKeys []string, dstKey, contentType string) error {
	return concatWithHeader(ctx, client, bucket, header, srcKeys, dstKey, contentType)
}

func concatWithHeader(ctx context.Context, client concatAPI, bucket string, header []byte, srcKeys []string, dstKey, contentType string) error {
	ctx = log.WithLogValues(ctx, "func", "s3.ConcatWithHeader")
	ctx, span := s3Tracer.Start(ctx, "s3.ConcatWithHeader", trace.WithAttributes(
		attribute.String("bucket", bucket),
		attribute.String("dst_key", dstKey),
		attribute.Int("src_count", len(srcKeys)),
		attribute.Int("header_bytes", len(header)),
	))
	defer span.End()

	if len(srcKeys) == 0 {
		return fmt.Errorf("s3 concat: no source objects")
	}

	// Object sizes drive both the part-1 fill and the per-object copy parts.
	sizes := make([]int64, len(srcKeys))
	for i, key := range srcKeys {
		head, err := client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("head s3://%s/%s: %w", bucket, key, err)
		}
		sizes[i] = aws.ToInt64(head.ContentLength)
	}

	// Destination layout: header at [0, headerLen), then each source object at
	// offsets[i]. Every part is exactly concatPartSize except the trailing one.
	headerLen := int64(len(header))
	offsets := make([]int64, len(srcKeys))
	total := headerLen
	for i := range srcKeys {
		offsets[i] = total
		total += sizes[i]
	}
	if total == 0 {
		return fmt.Errorf("s3 concat: nothing to assemble (empty header and sources)")
	}
	numParts := (total + concatPartSize - 1) / concatPartSize
	if numParts > s3MaxParts {
		return fmt.Errorf("s3 concat: %d bytes needs %d parts, exceeding the %d-part limit", total, numParts, s3MaxParts)
	}

	create := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(dstKey),
	}
	if contentType != "" {
		create.ContentType = aws.String(contentType)
	}
	resp, err := client.CreateMultipartUpload(ctx, create)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("create multipart upload s3://%s/%s: %w", bucket, dstKey, err)
	}
	uploadID := aws.ToString(resp.UploadId)

	abort := func() {
		// Use ctx (not a cancelled child) so cleanup runs; best-effort.
		_, _ = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(bucket),
			Key:      aws.String(dstKey),
			UploadId: aws.String(uploadID),
		})
	}

	// Parts land at fixed indices, so the completed list is already ordered.
	// Downloaded (boundary) windows hold up to concatPartSize bytes each, but
	// there's at most ~one per source object; copyConcurrency bounds how many
	// are in memory at once.
	parts := make([]types.CompletedPart, numParts)
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(copyConcurrency)
	for p := int64(0); p < numParts; p++ {
		partNum := int32(p + 1)
		wStart := p * concatPartSize
		wEnd := min(wStart+concatPartSize, total) // exclusive
		g.Go(func() error {
			// A window entirely inside one source object copies server-side.
			for i := range srcKeys {
				if offsets[i] <= wStart && wEnd <= offsets[i]+sizes[i] {
					res, err := client.UploadPartCopy(gctx, &s3.UploadPartCopyInput{
						Bucket:          aws.String(bucket),
						Key:             aws.String(dstKey),
						UploadId:        aws.String(uploadID),
						PartNumber:      aws.Int32(partNum),
						CopySource:      aws.String(bucket + "/" + srcKeys[i]),
						CopySourceRange: aws.String(fmt.Sprintf("bytes=%d-%d", wStart-offsets[i], wEnd-1-offsets[i])),
					})
					if err != nil {
						return fmt.Errorf("upload part copy %d s3://%s/%s: %w", partNum, bucket, srcKeys[i], err)
					}
					if res.CopyPartResult == nil {
						return fmt.Errorf("upload part copy %d s3://%s/%s: missing CopyPartResult", partNum, bucket, srcKeys[i])
					}
					parts[p] = types.CompletedPart{
						ETag:       res.CopyPartResult.ETag,
						PartNumber: aws.Int32(partNum),
					}
					return nil
				}
			}
			// Mixed window (contains the header and/or spans object boundaries):
			// assemble it in memory and upload it as a regular part.
			buf := make([]byte, 0, wEnd-wStart)
			if wStart < headerLen {
				buf = append(buf, header[wStart:min(wEnd, headerLen)]...)
			}
			for i := range srcKeys {
				s := max(wStart, offsets[i])
				e := min(wEnd, offsets[i]+sizes[i])
				if s >= e {
					continue
				}
				body, err := getRange(gctx, client, bucket, srcKeys[i], s-offsets[i], e-1-offsets[i])
				if err != nil {
					return err
				}
				buf = append(buf, body...)
			}
			return uploadPartAt(gctx, client, bucket, dstKey, uploadID, partNum, buf, &parts[p])
		})
	}
	if err := g.Wait(); err != nil {
		abort()
		span.RecordError(err)
		span.SetStatus(codes.Error, "upload_parts")
		return err
	}

	if _, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(bucket),
		Key:             aws.String(dstKey),
		UploadId:        aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: parts},
	}); err != nil {
		abort()
		span.RecordError(err)
		span.SetStatus(codes.Error, "complete")
		return fmt.Errorf("complete multipart upload s3://%s/%s: %w", bucket, dstKey, err)
	}
	log.Log(ctx, "completed S3 header concat", "bucket", bucket, "key", dstKey, "parts", len(parts))
	return nil
}

// getRange reads [start,end] (inclusive) of an object into memory.
func getRange(ctx context.Context, client concatAPI, bucket, key string, start, end int64) ([]byte, error) {
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", start, end)),
	})
	if err != nil {
		return nil, fmt.Errorf("get s3://%s/%s bytes=%d-%d: %w", bucket, key, start, end, err)
	}
	defer out.Body.Close()
	body, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("read s3://%s/%s body: %w", bucket, key, err)
	}
	return body, nil
}

func uploadPartAt(ctx context.Context, client concatAPI, bucket, dstKey, uploadID string, partNum int32, body []byte, out *types.CompletedPart) error {
	res, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(dstKey),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(partNum),
		Body:       bytes.NewReader(body),
	})
	if err != nil {
		return fmt.Errorf("upload part %d s3://%s/%s: %w", partNum, bucket, dstKey, err)
	}
	*out = types.CompletedPart{
		ETag:       res.ETag,
		PartNumber: aws.Int32(partNum),
	}
	return nil
}
