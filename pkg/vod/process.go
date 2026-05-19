// Package vod implements the post-upload processing pipeline for a VOD
// upload: read the user's raw file from wherever the upload manager
// stored it, run it through gstreamer (parsebin -> mp4mux -> muxl
// concatenator), and write the resulting fMP4 to a content-addressed
// key under a blob.Store.
//
// The pipeline is streaming end-to-end — bytes flow from the source
// (file or ranged S3 GETs), through gstreamer's appsrc -> parsebin ->
// fdkaacenc/h264parse -> mp4mux -> appsink, into the muxl wasm
// concatenator, and out to a blob.Store writer. A bdasl.Writer runs
// as a tee on the way out; the final CID is known only when the last
// byte is written, at which point we Move the staging blob to the
// content-addressed key.
package vod

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/pkg/spmetrics"
	"stream.place/streamplace/pkg/statedb"
)

var vodTracer = otel.Tracer("vod")

// vodProgressLogInterval is how often consumeConcatTraced emits an
// info-level heartbeat while segments flow, so a long-running mux is
// visible in the logs rather than silent between start and finish.
const vodProgressLogInterval = 5 * time.Second

// stage labels for spmetrics.VODProcessErrorsTotal — keep these in sync
// with whatever the dashboard / alert routing keys off.
const (
	stageOpenSource         = "open_source"
	stageStaging            = "start_staging"
	stageSigner             = "create_signer"
	stagePipeline           = "gstreamer_pipeline"
	stageEmptyOutput        = "empty_muxl_output"
	stageStagingComplete    = "store_complete"
	stageContentAddressCopy = "content_address_move"
	stageMetafile           = "write_metafile"
	stagePublish            = "publish_records"
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
	// Backend describes the upload source backend ("file" or "s3"),
	// matching what the upload manager stored on the row. Used only
	// for metrics labeling; the actual reading goes through the
	// blob.Store passed to ProcessVOD.
	Backend string
	// Location is the backend-specific URL/path (a file path or
	// "s3://bucket/key" URL) the upload landed at. Translated to a
	// Store key via Store.ParseLocation.
	Location string
}

// StagingPrefix is where in-progress VOD outputs land before being
// renamed to their content-addressed key. We park them under a
// dedicated prefix so a periodic janitor can sweep abandoned uploads.
const StagingPrefix = "vod-staging/"

// BlobsPrefix is where finalized, content-addressed blobs land in the
// Store: `<BlobsPrefix><cid>.mp4` for content + init segments,
// `<BlobsPrefix><cid>.json` for metafiles. The prefix is intentionally
// content-agnostic — the blob doesn't know what kind of video it's for
// or whether it's a primary fmp4, a per-track init, or a metafile.
// CDN deployments map `<vod-cdn-url>/blobs/...` straight at this layout.
const BlobsPrefix = "blobs/"

// ProcessVOD runs the streaming pipeline for one VOD upload and returns
// the BDASL CID of the resulting fMP4. Reads come from `in` via the
// supplied Store; the output blob lands at BlobsPrefix+<cid>.mp4 in
// the same Store; staging blobs are cleaned up on success or failure.
//
// The Store is the storage layer; it can be either a FileStore (single-
// node deployments) or an S3Store (production). Future Stores can mix
// caches and archives behind the same interface.
func ProcessVOD(ctx context.Context, cli *config.CLI, state *statedb.StatefulDB, store blob.Store, in Input) (string, error) {
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

	log.Log(ctx, "starting VOD processing",
		"backend", in.Backend,
		"size", in.Size,
		"mimeType", in.MimeType,
		"location", in.Location,
	)

	src, size, closer, err := openSource(ctx, store, in)
	if err != nil {
		recordErr(span, stageOpenSource, err)
		return "", fmt.Errorf("open source: %w", err)
	}
	defer closer()
	span.SetAttributes(attribute.Int64("source_size_bytes", size))
	spmetrics.VODInputBytes.Observe(float64(size))

	stagingKey := StagingPrefix + in.UploadID + ".mp4"
	span.SetAttributes(attribute.String("staging_key", stagingKey))
	staging, err := store.NewWriter(ctx, stagingKey, "video/mp4")
	if err != nil {
		recordErr(span, stageStaging, err)
		return "", fmt.Errorf("start staging upload: %w", err)
	}
	defer staging.Close() // Abort is idempotent; no-op once Complete ran

	hasher := bdasl.NewWriter()
	counter := &countingWriter{}
	final := io.MultiWriter(hasher, counter, staging)

	// Ephemeral per-upload signing key. Generated here as muxing starts,
	// used to C2PA-sign every segment, and dropped when this function
	// returns — so once the upload is processed nobody can mint more
	// signatures under its did:key.
	signer, err := newUploadSigner(startTime)
	if err != nil {
		recordErr(span, stageSigner, err)
		return "", fmt.Errorf("create upload signer: %w", err)
	}
	span.SetAttributes(attribute.String("signing_did", signer.DIDKey))

	metaBuilder := newMetafileBuilder(ctx, store)
	probe, err := streamThroughMuxl(ctx, src, size, final, metaBuilder, signer.SignerInput)
	if err != nil {
		recordErr(span, stagePipeline, err)
		// staging is aborted by the deferred Close
		return "", err
	}
	span.SetAttributes(
		attribute.Int64("output_size_bytes", counter.n),
		attribute.Int64("probe_duration_ms", probe.DurationMS),
	)
	if probe.Video != nil {
		span.SetAttributes(
			attribute.String("probe_video_codec", probe.Video.Codec),
			attribute.Int("probe_video_width", probe.Video.Width),
			attribute.Int("probe_video_height", probe.Video.Height),
		)
	}
	if probe.Audio != nil {
		span.SetAttributes(
			attribute.String("probe_audio_codec", probe.Audio.Codec),
			attribute.Int("probe_audio_rate", probe.Audio.Rate),
			attribute.Int("probe_audio_channels", probe.Audio.Channels),
		)
	}
	spmetrics.VODOutputBytes.Observe(float64(counter.n))

	if counter.n == 0 {
		err := errors.New("muxl produced zero bytes")
		recordErr(span, stageEmptyOutput, err)
		return "", err
	}

	if err := completeStaging(ctx, staging, stagingKey); err != nil {
		recordErr(span, stageStagingComplete, err)
		return "", fmt.Errorf("complete staging upload: %w", err)
	}

	finalCID := hasher.CID()
	contentKey := BlobsPrefix + finalCID + ".mp4"
	span.SetAttributes(
		attribute.String("cid", finalCID),
		attribute.String("content_key", contentKey),
		attribute.String("content_url", store.URL(contentKey)),
	)

	if err := finalizeMove(ctx, store, stagingKey, contentKey); err != nil {
		recordErr(span, stageContentAddressCopy, err)
		return "", fmt.Errorf("finalize: %w", err)
	}

	metafile := metaBuilder.Finalize(finalCID, counter.n)
	if err := writeMetafile(ctx, store, finalCID, metafile); err != nil {
		recordErr(span, stageMetafile, err)
		return "", fmt.Errorf("write metafile: %w", err)
	}

	// A thumbnail is nice-to-have, not load-bearing: a failure here
	// (codec quirk, odd segment) shouldn't sink an otherwise-good
	// upload, so log and publish without it.
	thumbnail, err := generateThumbnail(ctx, store, finalCID, metafile)
	if err != nil {
		log.Warn(ctx, "VOD thumbnail generation failed; publishing without thumbnail", "error", err)
	}
	span.SetAttributes(attribute.Bool("thumbnail_generated", len(thumbnail) > 0))

	if err := publishRecords(ctx, publishParams{
		cli:        cli,
		state:      state,
		in:         in,
		cid:        finalCID,
		size:       counter.n,
		mimeType:   "video/mp4",
		probe:      probe,
		signingKey: signer.DIDKey,
		thumbnail:  thumbnail,
	}); err != nil {
		recordErr(span, stagePublish, err)
		return "", fmt.Errorf("publish records: %w", err)
	}

	spmetrics.VODProcessSuccessesTotal.WithLabelValues(in.Backend).Inc()
	span.SetStatus(codes.Ok, "")
	log.Log(ctx, "VOD processed",
		"cid", finalCID,
		"url", store.URL(contentKey),
		"input_size", size,
		"output_size", counter.n,
		"duration_ms", time.Since(startTime).Milliseconds(),
	)
	return finalCID, nil
}

// completeStaging wraps the writer Complete call in its own span so
// we can see how much of total processing time is spent waiting for
// the storage layer to acknowledge the upload.
func completeStaging(ctx context.Context, staging blob.Writer, stagingKey string) error {
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

// finalizeMove atomically promotes the staged blob at stagingKey to the
// content-addressed contentKey. For S3Store this is a CopyObject +
// DeleteObject; for FileStore it's an os.Rename.
func finalizeMove(ctx context.Context, store blob.Store, stagingKey, contentKey string) error {
	ctx, span := vodTracer.Start(ctx, "vod.finalizeMove", trace.WithAttributes(
		attribute.String("staging_key", stagingKey),
		attribute.String("content_key", contentKey),
	))
	defer span.End()
	moveStart := time.Now()
	if err := store.Move(ctx, stagingKey, contentKey); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "move")
		return err
	}
	span.SetAttributes(attribute.Int64("move_duration_ms", time.Since(moveStart).Milliseconds()))
	log.Debug(ctx, "moved staging to content-addressed key",
		"duration_ms", time.Since(moveStart).Milliseconds(),
	)
	return nil
}

// countingWriter is an io.Writer that tallies bytes written. Used to
// observe output size without buffering or hashing it twice.
type countingWriter struct{ n int64 }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}

// ProcessToDiscard runs the full VOD media path — gstreamer remux, muxl
// sign-segment, and metafile assembly — exactly as ProcessVOD does, but
// throws the bytes away instead of staging, finalizing, and publishing
// them. It needs no statedb, CLI, or PDS, which makes it the engine
// behind `streamplace vod-test`: a way to run the parts of the pipeline
// that have crashed or stalled in production against an arbitrary local
// file. Returns the probe metadata and the number of fMP4 bytes produced.
func ProcessToDiscard(ctx context.Context, src io.ReaderAt, size int64) (media.VODResult, int64, error) {
	signer, err := newUploadSigner(time.Now())
	if err != nil {
		return media.VODResult{}, 0, fmt.Errorf("create upload signer: %w", err)
	}
	// The metafile builder writes per-track init blobs into a Store; a
	// throwaway temp dir stands in for the real one so we exercise the
	// same wasm-parsing/metafile code production runs.
	dir, err := os.MkdirTemp("", "vod-test-")
	if err != nil {
		return media.VODResult{}, 0, fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(dir)
	store, err := blob.NewFileStore(dir)
	if err != nil {
		return media.VODResult{}, 0, fmt.Errorf("file store: %w", err)
	}
	mb := newMetafileBuilder(ctx, store)
	counter := &countingWriter{}
	result, err := streamThroughMuxl(ctx, src, size, counter, mb, signer.SignerInput)
	return result, counter.n, err
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
//	                                      -> muxl sign-segment -> dst
//
// gstreamer writes the fMP4 stream to one end of an io.Pipe; a goroutine
// forwards those bytes into muxl-sign's sign-segment subcommand, which
// segments per-GoP and C2PA-signs each canonical segment; another
// goroutine drains the init+seg channels and writes them to dst. The
// segment bytes that land in dst are therefore signed
// ([c2pa-uuid][muxl-uuid][moof][mdat] per track).
//
// If metaBuilder is non-nil, a third goroutine consumes the rich event
// channel and feeds events to it — used to build the per-blob metafile
// sidecar (segment offsets/sizes/durations and per-track init blobs)
// without re-parsing the bytes.
//
// The returned media.VODResult is the probe metadata gathered during
// the pipeline run (codec/dimensions/duration). The CID is computed
// by the caller from the bdasl.Writer tee'd into dst, since the hash
// is only final once the last byte lands.
func streamThroughMuxl(ctx context.Context, src io.ReaderAt, size int64, dst io.Writer, metaBuilder *metafileBuilder, signerInput muxl.SignerInput) (media.VODResult, error) {
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

	// Forwarder: gstreamer mp4mux output -> muxl sign-segment stdin.
	// Run as a goroutine because both ends are pipes; both sides need to
	// be live for either to make progress.
	concat := muxl.NewSigningSegmenter(ctx, signerInput)
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

	// Optional metafile builder: parallel consumer on the event channel.
	// Buffered enough that a stall in the metafile path doesn't backpressure
	// the byte consumer. EventCh always exists on the concatenator; if no
	// builder was provided we still need to drain it so the wasm parser
	// doesn't block.
	metaDone := make(chan error, 1)
	go func() {
		var err error
		for ev := range concat.EventCh {
			if metaBuilder == nil {
				continue
			}
			if oerr := metaBuilder.Observe(ev); oerr != nil {
				err = oerr
				// Keep draining the channel so upstream doesn't block;
				// the first error wins.
			}
		}
		metaDone <- err
	}()

	// Run gstreamer on this goroutine; mp4mux output bytes land at pw.
	result, pipelineErr := media.RunVODPipeline(ctx, src, size, pw)
	// Close pw so the forwarder sees EOF and can close the concat.
	_ = pw.Close()

	// Wait for all downstream goroutines. We collect any error but
	// prefer surfacing the pipeline error since downstream errors are
	// often a consequence of it.
	feedErr := <-feedDone
	consumeErr := <-consumeDone
	metaErr := <-metaDone

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
		return result, fmt.Errorf("gstreamer pipeline: %w", pipelineErr)
	}
	if feedErr != nil {
		span.RecordError(feedErr)
		span.SetStatus(codes.Error, "feed")
		return result, fmt.Errorf("muxl feed: %w", feedErr)
	}
	if consumeErr != nil {
		span.RecordError(consumeErr)
		span.SetStatus(codes.Error, "consume")
		return result, fmt.Errorf("muxl consume: %w", consumeErr)
	}
	if metaErr != nil {
		span.RecordError(metaErr)
		span.SetStatus(codes.Error, "metafile")
		return result, fmt.Errorf("muxl metafile builder: %w", metaErr)
	}
	return result, nil
}

// consumeConcatTraced drains the concatenator's two output channels and
// writes their contents (init, then segments) to dst in order of
// arrival. Increments the supplied counters so the parent span can
// report them. If the init segment changes mid-stream (multi-input
// concatenation), the new init is written too — for VOD with a single
// input that doesn't happen, but the loop handles it for free.
func consumeConcatTraced(ctx context.Context, c *muxl.Concatenator, dst io.Writer, initBytes, segBytes, initEmits, segEmits *int64) error {
	initCh, segCh := c.InitCh, c.SegCh
	start := time.Now()
	ticker := time.NewTicker(vodProgressLogInterval)
	defer ticker.Stop()
	for initCh != nil || segCh != nil {
		select {
		case <-ticker.C:
			log.Log(ctx, "muxl progress",
				"seg_emits", *segEmits, "seg_bytes", *segBytes,
				"elapsed_seconds", time.Since(start).Seconds())
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

// openSource returns a Reader + size for the upload via the supplied
// Store. The returned closer must be called when the caller is done
// with the source.
//
// The upload's Location field (a backend-specific URL/path) is
// translated to a Store-relative key via store.ParseLocation. If the
// configured Store can't claim the Location — e.g. the upload was
// stored in S3 but the deployment is now serving file-only — the open
// fails with a clear error.
func openSource(ctx context.Context, store blob.Store, in Input) (io.ReaderAt, int64, func(), error) {
	ctx, span := vodTracer.Start(ctx, "vod.openSource", trace.WithAttributes(
		attribute.String("backend", in.Backend),
		attribute.String("location", in.Location),
	))
	defer span.End()
	key, ok := store.ParseLocation(in.Location)
	if !ok {
		err := fmt.Errorf("store does not own upload location %q (backend %s)", in.Location, in.Backend)
		span.RecordError(err)
		return nil, 0, nil, err
	}
	span.SetAttributes(attribute.String("key", key))
	rdr, err := store.Open(ctx, key)
	if err != nil {
		span.RecordError(err)
		return nil, 0, nil, fmt.Errorf("open upload: %w", err)
	}
	size := rdr.Size()
	span.SetAttributes(attribute.Int64("size_bytes", size))
	log.Debug(ctx, "opened upload via Store", "url", store.URL(key), "size", size)
	return rdr, size, func() { _ = rdr.Close() }, nil
}
