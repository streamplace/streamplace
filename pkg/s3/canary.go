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
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spmetrics"
)

// The canary exists because of a production incident where R2 accepted every
// CreateMultipartUpload and UploadPart, then rejected CompleteMultipartUpload
// ("All non-trailing parts must have the same length") — so live recordings
// died ten minutes into each stream, and nothing failed at deploy time. This
// probe performs a real multipart upload exercising the exact call shapes the
// live recorder and live-to-VOD finalize depend on, so a provider that can't
// support them fails loudly at startup (and keeps failing every interval)
// instead of eating user livestreams.

// canaryPartSize is the uniform part size the probe uses: S3's 5MB minimum,
// the same floor the live uploader flushes at.
const canaryPartSize = minPartSize

// canaryPrefix namespaces probe objects away from real data. Objects are
// deleted after each probe; anything lingering here is debris from a probe
// that died mid-run and is safe to remove.
const canaryPrefix = ".canary/"

// canaryAPI is the subset of *s3.Client the canary drives. Pulled out so
// tests can inject a fake; *s3.Client satisfies it.
type canaryAPI interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	CreateMultipartUpload(context.Context, *s3.CreateMultipartUploadInput, ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error)
	UploadPart(context.Context, *s3.UploadPartInput, ...func(*s3.Options)) (*s3.UploadPartOutput, error)
	UploadPartCopy(context.Context, *s3.UploadPartCopyInput, ...func(*s3.Options)) (*s3.UploadPartCopyOutput, error)
	CompleteMultipartUpload(context.Context, *s3.CompleteMultipartUploadInput, ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error)
	AbortMultipartUpload(context.Context, *s3.AbortMultipartUploadInput, ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error)
}

// RunCanary performs one end-to-end probe against the bucket:
//
//  1. PutObject a 5MB source object (finalize reads real objects this way).
//  2. Multipart-upload a destination whose parts are: an uploaded 5MB part,
//     a server-side UploadPartCopy of the source (uniform with part 1 — the
//     rule R2 enforces), and a short trailing uploaded part — the exact shape
//     of a live-rec object rollover and a finalize concat window.
//  3. Read the result back and verify it byte-for-byte.
//  4. Delete both objects.
//
// ~10MB up / ~10MB down per run. Any error means the bucket cannot support
// live recording as implemented and is worth an alert.
func RunCanary(ctx context.Context, client *s3.Client, bucket string) error {
	return runCanary(ctx, client, bucket)
}

func runCanary(ctx context.Context, client canaryAPI, bucket string) error {
	ts := time.Now().UTC().Format("2006-01-02T15-04-05.000")
	srcKey := canaryPrefix + ts + "-src"
	dstKey := canaryPrefix + ts + "-dst"

	// Deterministic, position-dependent patterns so a shuffled or truncated
	// reassembly can't accidentally verify.
	pattern := func(b byte, n int) []byte {
		out := make([]byte, n)
		for i := range out {
			out[i] = b ^ byte(i*7)
		}
		return out
	}
	srcBody := pattern('s', canaryPartSize)
	part1 := pattern('1', canaryPartSize)
	trailer := pattern('t', 1024)

	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(srcKey),
		Body:   bytes.NewReader(srcBody),
	}); err != nil {
		return fmt.Errorf("canary: put source object: %w", err)
	}
	defer func() {
		_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(srcKey)})
	}()

	create, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(dstKey),
	})
	if err != nil {
		return fmt.Errorf("canary: create multipart upload: %w", err)
	}
	uploadID := aws.ToString(create.UploadId)
	completed := false
	defer func() {
		if !completed {
			_, _ = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket: aws.String(bucket), Key: aws.String(dstKey), UploadId: aws.String(uploadID),
			})
		}
	}()

	var parts []types.CompletedPart

	up1, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket: aws.String(bucket), Key: aws.String(dstKey), UploadId: aws.String(uploadID),
		PartNumber: aws.Int32(1), Body: bytes.NewReader(part1),
	})
	if err != nil {
		return fmt.Errorf("canary: upload part 1: %w", err)
	}
	parts = append(parts, types.CompletedPart{ETag: up1.ETag, PartNumber: aws.Int32(1)})

	cp, err := client.UploadPartCopy(ctx, &s3.UploadPartCopyInput{
		Bucket: aws.String(bucket), Key: aws.String(dstKey), UploadId: aws.String(uploadID),
		PartNumber:      aws.Int32(2),
		CopySource:      aws.String(bucket + "/" + srcKey),
		CopySourceRange: aws.String(fmt.Sprintf("bytes=0-%d", canaryPartSize-1)),
	})
	if err != nil {
		return fmt.Errorf("canary: upload part copy: %w", err)
	}
	if cp.CopyPartResult == nil {
		return fmt.Errorf("canary: upload part copy: missing CopyPartResult")
	}
	parts = append(parts, types.CompletedPart{ETag: cp.CopyPartResult.ETag, PartNumber: aws.Int32(2)})

	up3, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket: aws.String(bucket), Key: aws.String(dstKey), UploadId: aws.String(uploadID),
		PartNumber: aws.Int32(3), Body: bytes.NewReader(trailer),
	})
	if err != nil {
		return fmt.Errorf("canary: upload trailing part: %w", err)
	}
	parts = append(parts, types.CompletedPart{ETag: up3.ETag, PartNumber: aws.Int32(3)})

	if _, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket: aws.String(bucket), Key: aws.String(dstKey), UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: parts},
	}); err != nil {
		return fmt.Errorf("canary: complete multipart upload: %w", err)
	}
	completed = true
	defer func() {
		_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(dstKey)})
	}()

	got, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket), Key: aws.String(dstKey),
	})
	if err != nil {
		return fmt.Errorf("canary: get assembled object: %w", err)
	}
	body, err := io.ReadAll(got.Body)
	_ = got.Body.Close()
	if err != nil {
		return fmt.Errorf("canary: read assembled object: %w", err)
	}
	want := make([]byte, 0, len(part1)+len(srcBody)+len(trailer))
	want = append(want, part1...)
	want = append(want, srcBody...)
	want = append(want, trailer...)
	if !bytes.Equal(body, want) {
		return fmt.Errorf("canary: assembled object mismatch: got %d bytes, want %d", len(body), len(want))
	}
	return nil
}

// StartCanary probes the bucket immediately and then every interval until ctx
// is done. Results land in metrics (streamplace_s3_canary_*) and logs; a
// failure is an error-level line because it means live recording against this
// bucket is likely broken. Always returns nil at shutdown so it never tears
// down the caller's errgroup.
func StartCanary(ctx context.Context, cfg Config, interval time.Duration) error {
	ctx = log.WithLogValues(ctx, "func", "s3.StartCanary")
	client := NewClient(cfg)
	probe := func() {
		start := time.Now()
		if err := RunCanary(ctx, client, cfg.Bucket); err != nil {
			spmetrics.S3CanaryChecksTotal.WithLabelValues("fail").Inc()
			log.Error(ctx, "S3 CANARY FAILED — live recording against this bucket will likely fail",
				"bucket", cfg.Bucket, "endpoint", cfg.Endpoint, "error", err)
			return
		}
		spmetrics.S3CanaryChecksTotal.WithLabelValues("ok").Inc()
		spmetrics.S3CanaryLastSuccessTimestamp.Set(float64(time.Now().Unix()))
		log.Log(ctx, "S3 canary ok", "bucket", cfg.Bucket, "duration_ms", time.Since(start).Milliseconds())
	}
	probe()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			probe()
		}
	}
}
