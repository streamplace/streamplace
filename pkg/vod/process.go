// Package vod implements the post-upload processing pipeline for a VOD
// upload: read the user's raw file from wherever the upload manager
// stored it, run it through gstreamer (parsebin -> mp4mux -> muxl
// concatenator), and write the resulting fMP4 to S3 under a key derived
// from the BLAKE3-based BDASL CID of the final bytes.
//
// The full pipeline is streaming end-to-end — bytes flow from the source
// (file or ranged S3 GETs), through gstreamer's appsrc -> parsebin ->
// fdkaacenc/h264parse -> mp4mux -> appsink, into the muxl wasm
// concatenator, and out to an S3 multipart upload. The bdasl hasher
// runs as a tee on the way out; the final CID is known only when the
// last byte is written.
package vod

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/muxl"
	s3pkg "stream.place/streamplace/pkg/s3"
	"stream.place/streamplace/pkg/spmetrics"
)

var vodTracer = otel.Tracer("vod")

// stage labels for spmetrics.VODProcessErrorsTotal — keep these in sync
// with whatever the dashboard / alert routing keys off.
const (
	stageOpenSource         = "open_source"
	stageStaging            = "start_staging"
	stagePipeline           = "gstreamer_pipeline"
	stageStagingComplete    = "s3_complete"
	stageContentAddressCopy = "content_address_copy"
)

// Input is the per-upload state needed to drive ProcessVOD. It's a
// flattened copy of statedb.VODProcessTask so this package doesn't
// import statedb (which would invite a dependency cycle with the queue
// processor).
type Input struct {
	UploadID string
	RepoDID  string
	MimeType string
	Filename string
	Size     int64
	// Backend is the storage tier the user upload lives on: "file" or "s3".
	Backend string
	// Location is the path (file backend) or s3:// URL (s3 backend) of
	// the user upload.
	Location string
}

// Backend constants mirror pkg/upload.BackendFile / BackendS3 without
// importing the upload package (which would pull statedb in transitively).
const (
	BackendFile = "file"
	BackendS3   = "s3"
)

// StagingPrefix is where in-progress VOD outputs land in S3 before being
// renamed to their content-addressed key. We park them under a dedicated
// prefix so a periodic janitor can sweep abandoned uploads.
const StagingPrefix = "vod-staging/"

// ContentPrefix is the prefix for the final, content-addressed object.
const ContentPrefix = "vod/"

// ProcessVOD runs the streaming pipeline for one VOD upload and returns
// the BDASL CID of the resulting fMP4. The output object is written at
// ContentPrefix+<cid>.fmp4 in the configured S3 bucket; staging objects
// are cleaned up on success or failure.
func ProcessVOD(ctx context.Context, cli *config.CLI, in Input) (string, error) {
	ctx = log.WithLogValues(ctx, "func", "ProcessVOD", "uploadId", in.UploadID, "did", in.RepoDID)
	ctx, span := vodTracer.Start(ctx, "vod.ProcessVOD", trace.WithAttributes(
		attribute.String("upload_id", in.UploadID),
		attribute.String("did", in.RepoDID),
		attribute.String("backend", in.Backend),
		attribute.String("mime_type", in.MimeType),
		attribute.Int64("input_size_bytes", in.Size),
	))
	defer span.End()

	startTime := time.Now()
	spmetrics.VODProcessAttemptsTotal.WithLabelValues(in.Backend).Inc()
	defer func() {
		spmetrics.VODProcessDurationMS.Observe(float64(time.Since(startTime).Milliseconds()))
	}()

	log.Log(ctx, "starting VOD processing", "backend", in.Backend, "size", in.Size, "mimeType", in.MimeType)

	if !cli.S3Configured() {
		err := errors.New("vod processing requires S3 to be configured")
		recordErr(span, "config", err)
		return "", err
	}
	s3client := newS3Client(cli)

	src, size, closer, err := openSource(ctx, cli, s3client, in)
	if err != nil {
		recordErr(span, stageOpenSource, err)
		return "", fmt.Errorf("open source: %w", err)
	}
	defer closer()
	span.SetAttributes(attribute.Int64("source_size_bytes", size))
	spmetrics.VODInputBytes.Observe(float64(size))

	stagingKey := StagingPrefix + in.UploadID + ".fmp4"
	span.SetAttributes(attribute.String("staging_key", stagingKey))
	staging, err := s3pkg.NewMultipartWriter(ctx, s3client, cli.S3Bucket, stagingKey, "video/mp4")
	if err != nil {
		recordErr(span, stageStaging, err)
		return "", fmt.Errorf("start staging upload: %w", err)
	}
	defer staging.Close() // Abort is idempotent; no-op once Complete ran

	hasher := bdasl.NewWriter()
	counter := &countingWriter{}
	final := io.MultiWriter(hasher, counter, staging)

	if _, err := streamThroughMuxl(ctx, src, size, final); err != nil {
		recordErr(span, stagePipeline, err)
		// staging is aborted by the deferred Close
		return "", err
	}
	span.SetAttributes(attribute.Int64("output_size_bytes", counter.n))
	spmetrics.VODOutputBytes.Observe(float64(counter.n))

	if err := completeStaging(ctx, staging, stagingKey); err != nil {
		recordErr(span, stageStagingComplete, err)
		return "", fmt.Errorf("complete staging upload: %w", err)
	}

	finalCID := hasher.CID()
	contentKey := ContentPrefix + finalCID + ".fmp4"
	span.SetAttributes(
		attribute.String("cid", finalCID),
		attribute.String("content_key", contentKey),
	)

	if err := finalizeUpload(ctx, s3client, cli.S3Bucket, stagingKey, contentKey); err != nil {
		recordErr(span, stageContentAddressCopy, err)
		return "", fmt.Errorf("finalize: %w", err)
	}

	spmetrics.VODProcessSuccessesTotal.WithLabelValues(in.Backend).Inc()
	span.SetStatus(codes.Ok, "")
	log.Log(ctx, "VOD processed",
		"cid", finalCID,
		"key", contentKey,
		"input_size", size,
		"output_size", counter.n,
		"duration_ms", time.Since(startTime).Milliseconds(),
	)
	return finalCID, nil
}

// completeStaging wraps the multipart Complete call in its own span so
// we can see how much of total processing time is spent waiting for S3
// to acknowledge the upload.
func completeStaging(ctx context.Context, staging *s3pkg.MultipartWriter, stagingKey string) error {
	_, span := vodTracer.Start(ctx, "vod.completeStaging", trace.WithAttributes(
		attribute.String("staging_key", stagingKey),
	))
	defer span.End()
	if err := staging.Complete(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

// countingWriter is an io.Writer that tallies bytes written. Used to
// observe output size without buffering or hashing it twice.
type countingWriter struct{ n int64 }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}

// recordErr is a small helper to attach the standard error attributes
// to a span + bump the per-stage counter. Both spans and counters need
// to be kept in sync for dashboards to work.
func recordErr(span trace.Span, stage string, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	span.SetAttributes(attribute.String("error_stage", stage))
	spmetrics.VODProcessErrorsTotal.WithLabelValues(stage).Inc()
}

// streamThroughMuxl wires up the goroutines that connect:
//
//	[gstreamer pipeline] -> mp4mux output (io.Pipe)
//	                                      -> muxl concatenator -> dst
//
// gstreamer writes the fMP4 stream to one end of an io.Pipe; a goroutine
// forwards those bytes into the muxl concatenator; another goroutine
// drains the concatenator's init+seg channels and writes them to dst.
//
// The empty string return is a placeholder — the CID is computed by the
// caller from the bdasl.Writer tee'd into dst, since the hash is only
// final once the last byte lands.
func streamThroughMuxl(ctx context.Context, src io.ReaderAt, size int64, dst io.Writer) (string, error) {
	ctx, span := vodTracer.Start(ctx, "vod.streamThroughMuxl", trace.WithAttributes(
		attribute.Int64("source_size_bytes", size),
	))
	defer span.End()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pr, pw := io.Pipe()

	// muxStats: bytes counted at the muxer output (input to muxl), and at
	// the concatenator output (final fMP4). Reported in span attributes
	// at the end so we can see the per-stage byte amplification.
	var mp4muxBytes int64
	var initBytes int64
	var segBytes int64
	var initEmits int64
	var segEmits int64

	// Forwarder: gstreamer mp4mux output -> muxl concatenator stdin.
	// Run as a goroutine because both ends are pipes; both sides need to
	// be live for either to make progress.
	concat := muxl.NewConcatenator(ctx)
	feedDone := make(chan error, 1)
	go func() {
		defer close(feedDone)
		buf := make([]byte, 64*1024)
		for {
			n, err := pr.Read(buf)
			if n > 0 {
				mp4muxBytes += int64(n)
				if werr := concat.Write(buf[:n]); werr != nil {
					feedDone <- werr
					return
				}
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					log.Debug(ctx, "muxl feed: EOF, closing concatenator", "mp4mux_bytes", mp4muxBytes)
					feedDone <- concat.Close()
				} else {
					feedDone <- err
				}
				return
			}
		}
	}()

	// Consumer: muxl concatenator output channels -> dst.
	consumeDone := make(chan error, 1)
	go func() {
		consumeDone <- consumeConcatTraced(ctx, concat, dst, &initBytes, &segBytes, &initEmits, &segEmits)
	}()

	// Run gstreamer on this goroutine; mp4mux output bytes land at pw.
	pipelineErr := media.RunVODPipeline(ctx, src, size, pw)
	// Close pw so the forwarder sees EOF and can close the concat.
	_ = pw.Close()

	// Wait for both downstream goroutines. We collect any error but
	// prefer surfacing the pipeline error since downstream errors are
	// often a consequence of it.
	feedErr := <-feedDone
	consumeErr := <-consumeDone

	span.SetAttributes(
		attribute.Int64("mp4mux_output_bytes", mp4muxBytes),
		attribute.Int64("muxl_init_bytes", initBytes),
		attribute.Int64("muxl_seg_bytes", segBytes),
		attribute.Int64("muxl_init_emits", initEmits),
		attribute.Int64("muxl_seg_emits", segEmits),
	)

	if pipelineErr != nil {
		span.RecordError(pipelineErr)
		span.SetStatus(codes.Error, "pipeline")
		return "", fmt.Errorf("gstreamer pipeline: %w", pipelineErr)
	}
	if feedErr != nil {
		span.RecordError(feedErr)
		span.SetStatus(codes.Error, "feed")
		return "", fmt.Errorf("muxl feed: %w", feedErr)
	}
	if consumeErr != nil {
		span.RecordError(consumeErr)
		span.SetStatus(codes.Error, "consume")
		return "", fmt.Errorf("muxl consume: %w", consumeErr)
	}
	return "", nil
}

// consumeConcatTraced drains the concatenator's two output channels and
// writes their contents (init, then segments) to dst in order of
// arrival. Increments the supplied counters so the parent span can
// report them. If the init segment changes mid-stream (multi-input
// concatenation), the new init is written too — for VOD with a single
// input that doesn't happen, but the loop handles it for free.
func consumeConcatTraced(ctx context.Context, c *muxl.Concatenator, dst io.Writer, initBytes, segBytes, initEmits, segEmits *int64) error {
	initCh, segCh := c.InitCh, c.SegCh
	for initCh != nil || segCh != nil {
		select {
		case init, ok := <-initCh:
			if !ok {
				initCh = nil
				continue
			}
			*initEmits++
			*initBytes += int64(len(init))
			log.Debug(ctx, "muxl init segment", "size", len(init), "emit_n", *initEmits)
			if _, err := dst.Write(init); err != nil {
				return err
			}
		case seg, ok := <-segCh:
			if !ok {
				segCh = nil
				continue
			}
			*segEmits++
			*segBytes += int64(len(seg))
			if _, err := dst.Write(seg); err != nil {
				return err
			}
		}
	}
	log.Debug(ctx, "muxl drain complete", "init_emits", *initEmits, "seg_emits", *segEmits, "init_bytes", *initBytes, "seg_bytes", *segBytes)
	return nil
}

// openSource returns a ReaderAt + size for the upload, regardless of
// backend. The returned closer must be called when the caller is done
// with the source.
func openSource(ctx context.Context, cli *config.CLI, s3client *awss3.Client, in Input) (io.ReaderAt, int64, func(), error) {
	ctx, span := vodTracer.Start(ctx, "vod.openSource", trace.WithAttributes(
		attribute.String("backend", in.Backend),
		attribute.String("location", in.Location),
	))
	defer span.End()
	switch in.Backend {
	case BackendFile:
		f, err := os.Open(in.Location)
		if err != nil {
			span.RecordError(err)
			return nil, 0, nil, fmt.Errorf("open file upload %q: %w", in.Location, err)
		}
		st, err := f.Stat()
		if err != nil {
			span.RecordError(err)
			_ = f.Close()
			return nil, 0, nil, fmt.Errorf("stat file upload %q: %w", in.Location, err)
		}
		span.SetAttributes(attribute.Int64("size_bytes", st.Size()))
		log.Debug(ctx, "opened file upload", "path", in.Location, "size", st.Size())
		return f, st.Size(), func() { _ = f.Close() }, nil
	case BackendS3:
		bucket, key, err := s3pkg.ParseURL(in.Location)
		if err != nil {
			span.RecordError(err)
			return nil, 0, nil, fmt.Errorf("parse s3 location %q: %w", in.Location, err)
		}
		span.SetAttributes(attribute.String("bucket", bucket), attribute.String("key", key))
		ra, err := s3pkg.NewReaderAt(ctx, s3client, bucket, key)
		if err != nil {
			span.RecordError(err)
			return nil, 0, nil, fmt.Errorf("open s3 upload: %w", err)
		}
		span.SetAttributes(attribute.Int64("size_bytes", ra.Size()))
		log.Debug(ctx, "opened s3 upload", "bucket", bucket, "key", key, "size", ra.Size())
		return ra, ra.Size(), func() { _ = ra.Close() }, nil
	default:
		err := fmt.Errorf("unknown upload backend %q", in.Backend)
		span.RecordError(err)
		return nil, 0, nil, err
	}
}

// finalizeUpload renames the staging object to its content-addressed
// key. S3 has no rename: we CopyObject server-side, then DeleteObject
// the staging key. If the content key already exists (duplicate upload
// of identical content), we still proceed — copy is idempotent and we
// still want to drop the staging copy.
func finalizeUpload(ctx context.Context, c *awss3.Client, bucket, stagingKey, contentKey string) error {
	ctx, span := vodTracer.Start(ctx, "vod.finalizeUpload", trace.WithAttributes(
		attribute.String("bucket", bucket),
		attribute.String("staging_key", stagingKey),
		attribute.String("content_key", contentKey),
	))
	defer span.End()

	copyStart := time.Now()
	_, err := c.CopyObject(ctx, &awss3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(contentKey),
		CopySource: aws.String(bucket + "/" + stagingKey),
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "copy")
		return fmt.Errorf("copy staging -> %s: %w", contentKey, err)
	}
	span.SetAttributes(attribute.Int64("copy_duration_ms", time.Since(copyStart).Milliseconds()))
	log.Debug(ctx, "copied staging to content-addressed key", "duration_ms", time.Since(copyStart).Milliseconds())

	if _, err := c.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(stagingKey),
	}); err != nil {
		// Non-fatal: the content key is in place; staging will be swept
		// later. Log + continue, and record it on the span for visibility.
		span.RecordError(err)
		log.Warn(ctx, "failed to delete staging object", "key", stagingKey, "error", err)
	}
	return nil
}

func newS3Client(cli *config.CLI) *awss3.Client {
	return awss3.New(awss3.Options{
		Region: cli.S3Region,
		Credentials: credentials.NewStaticCredentialsProvider(
			cli.S3AccessKeyID,
			cli.S3SecretAccessKey,
			"",
		),
		BaseEndpoint: aws.String(cli.S3Endpoint),
		UsePathStyle: true,
	})
}
