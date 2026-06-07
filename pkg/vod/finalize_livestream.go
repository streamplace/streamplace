package vod

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/muxl"
	s3pkg "stream.place/streamplace/pkg/s3"
	"stream.place/streamplace/pkg/statedb"
)

// FinalizeInput drives FinalizeLivestreamVOD. It's resolved by the bootstrap
// from the FinalizeLivestreamVODTask payload (plus a model lookup for the
// signing key), so this package needn't import pkg/model.
type FinalizeInput struct {
	// UploadID is the synthetic Upload row created by the finalizeLivestream
	// procedure; the getUploadStatus/publishVideo client contract keys off it.
	UploadID string
	// RepoDID is the streamer whose repo the media.track records land in.
	RepoDID string
	// LivestreamURI selects which S3Segment objects belong to this stream.
	LivestreamURI string
	// SigningKey is the did:key whose private half C2PA-signed the live
	// segments (the streamer's place.stream.key). Recorded on each
	// place.stream.media.track so playback can verify signatures.
	SigningKey string
}

// FinalizeLivestreamVOD turns a finished livestream's recorded MUXL objects
// into a VOD, reusing the upload pipeline's tail half. The live segments are
// already C2PA-signed canonical MUXL, so — unlike ProcessVOD — there is no
// gstreamer remux and no re-signing: we concatenate the objects under one
// synthesized init header into a content-addressed blob, derive the playback
// sidecars from it with `muxl unwrap` (exactly as VOD transfer does), and
// publish the same origin + track records.
//
// The content blob is shaped [init][segments…] — byte-structurally identical
// to an uploaded VOD — so the metafile offset math is the proven path and a
// live-derived VOD is indistinguishable from an uploaded one downstream
// (playback, transfer, view-count). The bulk of the bytes are assembled
// server-side via S3 UploadPartCopy; only the ~5 MB header part is uploaded.
func FinalizeLivestreamVOD(ctx context.Context, cli *config.CLI, state *statedb.StatefulDB, store blob.Store, in FinalizeInput) (string, error) {
	ctx = log.WithLogValues(ctx, "func", "FinalizeLivestreamVOD", "uploadId", in.UploadID, "did", in.RepoDID, "livestream", in.LivestreamURI)
	ctx, span := vodTracer.Start(ctx, "vod.FinalizeLivestreamVOD", trace.WithAttributes(
		attribute.String("upload_id", in.UploadID),
		attribute.String("did", in.RepoDID),
		attribute.String("livestream", in.LivestreamURI),
	))
	defer span.End()

	segs, err := state.ListS3SegmentsForLivestream(ctx, in.LivestreamURI)
	if err != nil {
		recordErr(span, "list_segments", err)
		return "", fmt.Errorf("list s3 segments: %w", err)
	}
	if len(segs) == 0 {
		err := fmt.Errorf("no recorded S3 segments for livestream %s", in.LivestreamURI)
		recordErr(span, "list_segments", err)
		return "", err
	}
	keys := make([]string, len(segs))
	for i, s := range segs {
		keys[i] = s.Key
	}
	span.SetAttributes(attribute.Int("object_count", len(keys)))
	log.Log(ctx, "finalizing livestream VOD", "objects", len(keys))

	// 1. Capture the canonical init header from the first object. We use the
	// init `muxl unwrap` itself emits (rather than synthesizing one) so its
	// length is exactly what the metafile builder assumes for a leading init —
	// keeping the prepended-header blob's offsets self-consistent regardless of
	// whether unwrap later passes the header through verbatim or re-derives it.
	header, err := captureInitHeader(ctx, store, keys[0])
	if err != nil {
		recordErr(span, "capture_init", err)
		return "", fmt.Errorf("capture init header: %w", err)
	}
	span.SetAttributes(attribute.Int("header_bytes", len(header)))

	// 2. One pass over [header]+objects: hash for the CID + build the metafile
	// (and per-track init blobs) via unwrap. The content blob is shaped exactly
	// like an uploaded VOD, so the metafile offsets are the proven path.
	cid, size, metafile, err := hashAndBuildMetafile(ctx, store, header, keys)
	if err != nil {
		recordErr(span, "build_metafile", err)
		return "", err
	}
	contentKey := BlobsPrefix + cid + ".mp4"
	span.SetAttributes(
		attribute.String("cid", cid),
		attribute.Int64("size_bytes", size),
		attribute.String("content_key", contentKey),
	)

	// 3. Assemble the content blob, mostly server-side.
	if err := runVODStage(ctx, "concat_assemble", func(ctx context.Context) error {
		return assembleContentBlob(ctx, store, header, keys, contentKey)
	}); err != nil {
		recordErr(span, "concat_assemble", err)
		return "", fmt.Errorf("assemble content blob: %w", err)
	}

	if err := runVODStage(ctx, stageMetafile, func(ctx context.Context) error {
		return writeMetafile(ctx, store, cid, metafile)
	}); err != nil {
		recordErr(span, stageMetafile, err)
		return "", fmt.Errorf("write metafile: %w", err)
	}

	// 4. Publish origin + track records and store TrackURIs on the Upload row,
	// reusing the upload pipeline's publish path unchanged.
	probe := metafileToVODResult(metafile)
	if err := runVODStage(ctx, stagePublish, func(ctx context.Context) error {
		return publishRecords(ctx, publishParams{
			cli:        cli,
			state:      state,
			in:         Input{UploadID: in.UploadID, RepoDID: in.RepoDID},
			cid:        cid,
			size:       size,
			mimeType:   "video/mp4",
			probe:      probe,
			signingKey: in.SigningKey,
		})
	}); err != nil {
		recordErr(span, stagePublish, err)
		return "", fmt.Errorf("publish records: %w", err)
	}

	span.SetStatus(codes.Ok, "")
	log.Log(ctx, "livestream VOD finalized", "cid", cid, "size", size, "objects", len(keys), "duration_ms", probe.DurationMS)
	return cid, nil
}

// captureInitHeader synthesizes the per-stream init segment (ftyp+moov) from
// the first recorded object via `muxl wrap --init-only`. The live objects are
// bare canonical segments whose embedded catalogs carry everything needed to
// derive the init; prepending this header to their concatenation yields the
// proven [init][segments…] VOD blob shape.
//
// RunMuxlWrapInit writes straight to a buffer (no event channel), so — unlike
// an unwrap pass — there's no stdout pipe to back up and no mid-stream cancel,
// which is what previously deadlocked here.
func captureInitHeader(ctx context.Context, store blob.Store, key string) ([]byte, error) {
	r, err := store.Open(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("open first object %s: %w", key, err)
	}
	defer r.Close()

	var buf bytes.Buffer
	if err := muxl.RunMuxlWrapInit(ctx, io.NewSectionReader(r, 0, r.Size()), &buf); err != nil {
		return nil, fmt.Errorf("synthesize init header: %w", err)
	}
	if buf.Len() == 0 {
		return nil, errors.New("muxl produced an empty init header")
	}
	return buf.Bytes(), nil
}

// hashAndBuildMetafile streams [header]+objects once, teeing into a bdasl hasher
// (for the content CID) and `muxl unwrap` → metafileBuilder (for the metafile +
// per-track init blobs). Returns the CID, the total blob size, and the
// finalized metafile.
func hashAndBuildMetafile(ctx context.Context, store blob.Store, header []byte, keys []string) (string, int64, *Metafile, error) {
	readers := make([]io.Reader, 0, len(keys)+1)
	closers := make([]io.Closer, 0, len(keys))
	defer func() {
		for _, c := range closers {
			_ = c.Close()
		}
	}()
	readers = append(readers, bytes.NewReader(header))
	for _, key := range keys {
		r, err := store.Open(ctx, key)
		if err != nil {
			return "", 0, nil, fmt.Errorf("open object %s: %w", key, err)
		}
		closers = append(closers, r)
		readers = append(readers, io.NewSectionReader(r, 0, r.Size()))
	}

	hasher := bdasl.NewWriter()
	counter := &countingWriter{}
	tee := io.TeeReader(io.MultiReader(readers...), io.MultiWriter(hasher, counter))

	mb := newMetafileBuilder(ctx, store)
	eventCh := make(chan *muxl.MuxlEvent, 16)
	producerErr := make(chan error, 1)
	go func() {
		producerErr <- muxl.RunMuxlUnwrapEvents(ctx, tee, eventCh)
		close(eventCh)
	}()

	var obsErr error
	for ev := range eventCh {
		if e := mb.Observe(ev); e != nil && obsErr == nil {
			obsErr = e
		}
	}
	if perr := <-producerErr; perr != nil {
		return "", 0, nil, fmt.Errorf("muxl unwrap: %w", perr)
	}
	if obsErr != nil {
		return "", 0, nil, fmt.Errorf("metafile build: %w", obsErr)
	}
	// Unwrap returns nil on context cancellation, which would otherwise let a
	// truncated stream yield a partial metafile. Refuse to publish that.
	if err := ctx.Err(); err != nil {
		return "", 0, nil, err
	}

	cid := hasher.CID()
	size := counter.load()
	return cid, size, mb.Finalize(cid, size), nil
}

// assembleContentBlob writes header ++ objects to contentKey. For an S3 store
// this is a near-entirely server-side concat (UploadPartCopy); a non-final
// object below S3's 5 MB part floor (only short dev streams) falls back to a
// full rewrite, as does any non-S3 store.
func assembleContentBlob(ctx context.Context, store blob.Store, header []byte, keys []string, contentKey string) error {
	if s3store, ok := store.(*blob.S3Store); ok {
		err := s3pkg.ConcatWithHeader(ctx, s3store.Client(), s3store.Bucket(), header, keys, contentKey, "video/mp4")
		if err == nil {
			return nil
		}
		if !errors.Is(err, s3pkg.ErrConcatPartTooSmall) {
			return err
		}
		log.Warn(ctx, "s3 concat fell back to rewrite (object below 5MB part floor)", "key", contentKey)
	}
	return assembleViaWriter(ctx, store, header, keys, contentKey)
}

// assembleViaWriter streams header ++ objects through a store writer. The
// universal fallback: correct for any blob.Store, but re-uploads every byte.
func assembleViaWriter(ctx context.Context, store blob.Store, header []byte, keys []string, contentKey string) error {
	w, err := store.NewWriter(ctx, contentKey, "video/mp4")
	if err != nil {
		return fmt.Errorf("open content writer: %w", err)
	}
	defer w.Close()
	if _, err := w.Write(header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	for _, key := range keys {
		r, err := store.Open(ctx, key)
		if err != nil {
			return fmt.Errorf("open object %s: %w", key, err)
		}
		_, err = io.Copy(w, io.NewSectionReader(r, 0, r.Size()))
		_ = r.Close()
		if err != nil {
			return fmt.Errorf("copy object %s: %w", key, err)
		}
	}
	return w.Complete()
}

// metafileToVODResult derives the probe metadata publishTrack needs from the
// regenerated metafile, standing in for the gstreamer probe that an upload
// would have produced. publishTrack hardcodes the video codec to h264 and only
// uses width/height (+ optional framerate), so we leave framerate unset.
func metafileToVODResult(meta *Metafile) media.VODResult {
	var res media.VODResult
	for _, t := range meta.Tracks {
		switch t.Type {
		case "video":
			res.Video = &media.VODVideoTrack{
				Codec:  t.Codec,
				Width:  int(t.Width),
				Height: int(t.Height),
			}
		case "audio":
			res.Audio = &media.VODAudioTrack{
				Codec:    audioProbeCodec(t.Codec),
				Rate:     int(t.SampleRate),
				Channels: int(t.Channels),
			}
		}
		if d := metafileTrackDurationMS(t); d > res.DurationMS {
			res.DurationMS = d
		}
	}
	return res
}

// metafileTrackDurationMS sums a track's segment durations (in its timescale)
// and converts to milliseconds.
func metafileTrackDurationMS(t MetafileTrack) int64 {
	if t.Timescale == 0 {
		return 0
	}
	var ticks uint64
	for _, s := range t.Segments {
		ticks += s.DurationTicks
	}
	return int64(ticks * 1000 / uint64(t.Timescale))
}

// audioProbeCodec maps a metafile (CMAF catalog) audio codec name to the
// gstreamer-caps-style value audioCodecForLexicon (publish.go) expects, so the
// lexicon enum comes out right ("opus" vs "aac").
func audioProbeCodec(metafileCodec string) string {
	if strings.Contains(strings.ToLower(metafileCodec), "opus") {
		return "x-opus"
	}
	return "mpeg" // audioCodecForLexicon maps "mpeg" -> "aac"
}
