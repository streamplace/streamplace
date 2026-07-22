package media

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
)

// ingest a H264+AAC fragmented-MP4 stream (the MistServer live .mp4 output, or
// an fMP4 push to /live)
func (mm *MediaManager) MP4Ingest(ctx context.Context, input io.Reader, ms MediaSigner) error {
	shouldRecord, err := mm.shouldRecord(ctx, ms.Streamer())
	if err != nil {
		return err
	}
	if shouldRecord {
		log.Log(ctx, "recording ingest stream to file", "streamer", ms.Streamer())
		var finalize func()
		input, finalize = mm.recordTee(ctx, input, ms.Streamer(), ".rtmp.mp4")
		defer finalize()
	} else {
		log.Log(ctx, "not recording ingest stream to file", "streamer", ms.Streamer())
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	signer, err := mm.SegmentAndSignElem(ctx, ms)
	if err != nil {
		return err
	}
	pipeline, err := buildMP4IngestPipeline(ctx, input, signer)
	if err != nil {
		return err
	}

	busErr := make(chan error)
	go func() {
		busErr <- HandleBusMessages(ctx, pipeline)
	}()

	go mm.HandleKeyRevocation(ctx, ms, pipeline)

	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		return err
	}
	defer func() {
		if err := pipeline.SetState(gst.StateNull); err != nil {
			log.Error(ctx, "error setting pipeline to null state", "error", err)
		}
	}()

	return <-busErr
}

// buildMP4IngestPipeline builds the H264+AAC fragmented-MP4 demux graph (video
// → h264parse, audio → Opus re-encode) and links both branches into signerElem
// — the muxl signing bin that emits one bare canonical .m4s per GoP. Shared by
// the in-process MP4Ingest and the isolated ingest worker, which differ only in
// where signerElem routes its segments (ValidateMP4 vs. a frame writer to the
// main process).
//
// The source is fMP4, not MKV, very much on purpose: MP4 track fragments carry
// both decode (tfdt/trun) and presentation (ctts) timestamps, so qtdemux hands
// us the encoder's real DTS. Matroska carries only presentation timestamps, so
// the old MKV ingest had to *reconstruct* DTS with h264timestamper — which
// guesses a worst-case full-DPB reorder window for streams whose SPS doesn't
// declare one (notably VideoToolbox), minting a constant spurious PTS−DTS
// offset that pushed every GoP's presentation past its segment's declared
// window and broke WebRTC playback at every keyframe. Real DTS in the
// container means no reconstruction and no guessing.
// matroskaMagic is the EBML header every Matroska/WebM stream opens with.
var matroskaMagic = []byte{0x1A, 0x45, 0xDF, 0xA3}

// rejectMatroska peeks at the ingest stream and fails fast with a diagnosis if
// it's Matroska. MKV was this pipeline's previous bridge format, so the most
// likely stray MKV source is a MistServer still running the legacy MKVExec
// process config (`streamplace live` POSTing MKV to /live on a restart loop) —
// without the sniff that just looks like qtdemux dying instantly, over and
// over, which is a miserable thing to debug. Returns a reader that includes
// the peeked bytes.
func rejectMatroska(input io.Reader) (io.Reader, error) {
	br := bufio.NewReader(input)
	head, err := br.Peek(len(matroskaMagic))
	if err != nil {
		if errors.Is(err, io.EOF) {
			return br, nil // shorter than the magic; let the pipeline EOS/complain
		}
		return nil, fmt.Errorf("peek ingest stream: %w", err)
	}
	if bytes.Equal(head, matroskaMagic) {
		return nil, fmt.Errorf("ingest input is Matroska (MKV), but this node ingests fragmented MP4 — a MistServer running the legacy MKVExec process config is probably still pushing MKV to /live; update its config (see docker/mistserver.json)")
	}
	return br, nil
}

func buildMP4IngestPipeline(ctx context.Context, input io.Reader, signerElem *gst.Element) (*gst.Pipeline, error) {
	input, err := rejectMatroska(input)
	if err != nil {
		return nil, err
	}
	// Queue sizing: qtdemux feeds both branches from one thread, and the fMP4
	// muxer downstream is an aggregator — it consumes NOTHING until every pad
	// has data. If the video track goes sparse (e.g. MistServer drops all delta
	// frames when a push falls behind, leaving ~1s-apart keyframes), the audio
	// branch must buffer a full video-frame gap while the demux walks the byte
	// stream to the next video frame. gst's default queue caps at
	// max-size-time=1s, so a ≥1s video gap fills the audio queue, blocks the
	// demux, starves the muxer's video pad, and deadlocks the whole graph with
	// no EOS — a live stream wedges until the watchdog kills it. Use the shared
	// Queue2Big preset (no time/buffer cap, generous byte cap) like the other
	// demux-fed pipelines (transcode, rtmp_push, packetize, media_data_parser).
	pipelineSlice := []string{
		"appsrc name=streamsrc ! qtdemux name=demux",
		"demux. ! " + constants.Queue2Big + " ! h264parse name=videoout",
		"demux. ! " + constants.Queue2Big + " ! fdkaacdec ! audioresample ! opusenc name=audioenc",
	}
	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, fmt.Errorf("error creating MP4Ingest pipeline: %w", err)
	}
	srcele, err := pipeline.GetElementByName("streamsrc")
	if err != nil {
		return nil, err
	}
	app.SrcFromElement(srcele).SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedDataIncremental(ctx, input),
	})
	parseEle, err := pipeline.GetElementByName("videoout")
	if err != nil {
		return nil, err
	}
	if err := pipeline.Add(signerElem); err != nil {
		return nil, err
	}
	if err := parseEle.Link(signerElem); err != nil {
		return nil, err
	}
	audioenc, err := pipeline.GetElementByName("audioenc")
	if err != nil {
		return nil, err
	}
	if err := audioenc.Link(signerElem); err != nil {
		return nil, err
	}
	return pipeline, nil
}

// debugRecordingFlushTimeout bounds how long ingest teardown waits for a debug
// recording to finalize — for S3 the commit only happens at Close, so an
// unbounded wait could wedge teardown while an unwaited exit loses the object.
// Generous on purpose: at teardown there can be up to ~128 MB of backpressured
// parts still uploading (multipartUploadConcurrency × MultipartPartSize), and a
// slow-but-working uplink deserves the time to land them — a post-stream worker
// lingering is cheap, a lost recording isn't. A genuinely stalled connection is
// bounded separately by the s3 package's per-operation timeouts; past this
// window the recording is abandoned (logged by the dump goroutine when its op
// timeouts fire; bucket lifecycle rules should reap the dangling multipart).
const debugRecordingFlushTimeout = 5 * time.Minute

// recordTee wires up a debug recording: everything read through the returned
// reader is teed into an asynchronous dumpToFile. The returned finalize ends
// the dump (closing the tee's pipe — the dump's io.Copy never sees EOF
// otherwise, since a TeeReader doesn't propagate one) and waits, bounded, for
// it to commit. Callers MUST finalize after ingest ends: on the S3 path the
// object only exists once Close commits the upload, so skipping it (e.g. a
// worker process exiting) silently loses the recording.
func (mm *MediaManager) recordTee(ctx context.Context, r io.Reader, user string, filesuffix string) (io.Reader, func()) {
	pr, pw := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := mm.dumpToFile(ctx, pr, user, filesuffix); err != nil {
			log.Error(ctx, "error dumping to file", "error", err, "streamer", user)
		}
	}()
	finalize := func() {
		pw.Close()
		select {
		case <-done:
		case <-time.After(debugRecordingFlushTimeout):
			log.Error(ctx, "debug recording did not finalize in time", "streamer", user)
		}
	}
	return io.TeeReader(r, pw), finalize
}

func (mm *MediaManager) dumpToFile(ctx context.Context, r io.Reader, user string, filesuffix string) error {
	now := aqtime.FromTime(time.Now())
	filename := fmt.Sprintf("%s%s", now.FileSafeString(), filesuffix)
	// Streams to S3 when configured (production), else a local file under DataDir
	// (dev). Close finalizes either target — for S3 it commits the upload.
	f, err := mm.cli.DebugRecordingCreate(ctx, []string{"debug-recordings", user, filename}, "video/mp4", false)
	if err != nil {
		return fmt.Errorf("failed to create debug recording: %w", err)
	}
	if _, err = io.Copy(f, r); err != nil {
		f.Close()
		return fmt.Errorf("failed to copy to debug recording: %w", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("failed to finalize debug recording: %w", err)
	}
	return nil
}
