package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"stream.place/streamplace/pkg/log"
)

// ErrConcatPartTooSmall is returned by ConcatWithHeader when a non-final source
// object is smaller than S3's 5 MB minimum part size and therefore can't be a
// server-side UploadPartCopy part. The caller is expected to fall back to a
// download-and-rewrite assembly. In production (10-minute cutover objects) this
// never triggers; it only guards against pathologically short streams.
var ErrConcatPartTooSmall = errors.New("s3 concat: non-final source object below 5MB minimum part size")

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
// many-GB) stream: the synthesized init header rides in part 1, and the bulk of
// the bytes are copied with UploadPartCopy and never pass through this process.
//
// Part 1 (uploaded) is header plus just enough leading source bytes to clear
// S3's 5 MB minimum, so it costs ~5 MB of memory + one upload. Every remaining
// source object becomes one server-side UploadPartCopy part. S3 requires every
// part except the last to be ≥5 MB; a non-final source object below that bound
// can't be copied as its own part, so ConcatWithHeader returns
// ErrConcatPartTooSmall and the caller falls back to a full rewrite.
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

	var parts []types.CompletedPart
	var partNum int32

	// --- Part 1: header + leading source bytes until >= minPartSize. ---
	// We pull only enough source bytes to clear the 5 MB floor (bounding memory
	// to ~minPartSize), tracking which object we stopped in so the copy phase
	// resumes from exactly there.
	buf := bytes.NewBuffer(make([]byte, 0, 2*minPartSize+len(header)))
	buf.Write(header)
	idx := 0                // index of the source object the copy phase resumes at
	var consumedInIdx int64 // bytes of srcKeys[idx] already pulled into part 1
	for idx < len(srcKeys) && int64(buf.Len()) < minPartSize {
		remaining := int64(minPartSize) - int64(buf.Len())
		avail := sizes[idx] - consumedInIdx
		take := remaining
		if take > avail {
			take = avail
		}
		// Look-ahead: a partial take that leaves this (non-final) object with a
		// sub-5MB remainder would make that remainder an illegal small middle
		// copy part. Absorb the whole object into part 1 instead — costing a
		// little extra upload but keeping every copy part legal.
		if take < avail && (avail-take) < int64(minPartSize) && idx != len(srcKeys)-1 {
			take = avail
		}
		if take > 0 {
			body, err := getRange(ctx, client, bucket, srcKeys[idx], consumedInIdx, consumedInIdx+take-1)
			if err != nil {
				abort()
				span.RecordError(err)
				return err
			}
			buf.Write(body)
			consumedInIdx += take
		}
		if consumedInIdx >= sizes[idx] {
			idx++
			consumedInIdx = 0
		}
	}

	partNum++
	if err := uploadPart(ctx, client, bucket, dstKey, uploadID, partNum, buf.Bytes(), &parts); err != nil {
		abort()
		span.RecordError(err)
		span.SetStatus(codes.Error, "upload_part_1")
		return err
	}

	// --- Remaining objects: one server-side UploadPartCopy part each. ---
	for i := idx; i < len(srcKeys); i++ {
		start := int64(0)
		if i == idx {
			start = consumedInIdx // resume mid-object if part 1 stopped here
		}
		if start >= sizes[i] {
			continue // wholly absorbed into part 1
		}
		partSize := sizes[i] - start
		isLast := i == len(srcKeys)-1
		if !isLast && partSize < minPartSize {
			abort()
			err := fmt.Errorf("%w (object %s contributes %d bytes)", ErrConcatPartTooSmall, srcKeys[i], partSize)
			span.RecordError(err)
			return err
		}
		if partSize > maxCopyObjectSize {
			abort()
			err := fmt.Errorf("s3 concat: object %s range %d bytes exceeds single-part copy limit", srcKeys[i], partSize)
			span.RecordError(err)
			return err
		}
		partNum++
		res, err := client.UploadPartCopy(ctx, &s3.UploadPartCopyInput{
			Bucket:          aws.String(bucket),
			Key:             aws.String(dstKey),
			UploadId:        aws.String(uploadID),
			PartNumber:      aws.Int32(partNum),
			CopySource:      aws.String(bucket + "/" + srcKeys[i]),
			CopySourceRange: aws.String(fmt.Sprintf("bytes=%d-%d", start, sizes[i]-1)),
		})
		if err != nil {
			abort()
			span.RecordError(err)
			span.SetStatus(codes.Error, "upload_part_copy")
			return fmt.Errorf("upload part copy %d s3://%s/%s: %w", partNum, bucket, srcKeys[i], err)
		}
		if res.CopyPartResult == nil {
			abort()
			return fmt.Errorf("upload part copy %d s3://%s/%s: missing CopyPartResult", partNum, bucket, srcKeys[i])
		}
		parts = append(parts, types.CompletedPart{
			ETag:       res.CopyPartResult.ETag,
			PartNumber: aws.Int32(partNum),
		})
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

func uploadPart(ctx context.Context, client concatAPI, bucket, dstKey, uploadID string, partNum int32, body []byte, parts *[]types.CompletedPart) error {
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
	*parts = append(*parts, types.CompletedPart{
		ETag:       res.ETag,
		PartNumber: aws.Int32(partNum),
	})
	return nil
}
