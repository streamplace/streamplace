package media

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
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

	// The track layout is only known after qtdemux parses the init segment
	// (N video tracks for eRTMP multitrack pushes), so the signing element is
	// built by the pipeline once it has probed the tracks.
	makeSigner := func(videoTrackCount int) (*gst.Element, <-chan struct{}, error) {
		elem, err := mm.SegmentAndSignElemTracks(ctx, ms, videoTrackCount)
		return elem, nil, err
	}
	pipeline, _, err := buildMP4IngestPipeline(ctx, input, makeSigner)
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

// mp4SignerFactory builds the muxl signing element for the probed MP4 track
// layout: videoTrackCount video pads plus the usual single audio pad. It is
// called once qtdemux has exposed the stream's tracks — 1 video track for
// plain pushes, N for eRTMP multitrack (e.g. OBS multitrack via MistServer).
// The returned done channel closes once the signer has drained all segments.
type mp4SignerFactory func(videoTrackCount int) (*gst.Element, <-chan struct{}, error)

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

// buildMP4IngestPipeline builds the fMP4 demux graph — one queue2 → h264parse
// branch per video track, audio → Opus re-encode — linked into a signing
// element created by makeSigner once probing has established the track count.
// Shared by the in-process MP4Ingest and the isolated ingest worker, which
// differ only in where the signing element routes its segments (ValidateMP4
// vs. a frame writer to the main process).
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
func buildMP4IngestPipeline(ctx context.Context, input io.Reader, makeSigner mp4SignerFactory) (pipeline *gst.Pipeline, done <-chan struct{}, err error) {
	input, err = rejectMatroska(input)
	if err != nil {
		return nil, nil, err
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
	}
	pipeline, err = gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, nil, fmt.Errorf("error creating MP4Ingest pipeline: %w", err)
	}
	srcele, err := pipeline.GetElementByName("streamsrc")
	if err != nil {
		return nil, nil, err
	}
	app.SrcFromElement(srcele).SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedDataIncremental(ctx, input),
	})
	demux, err := pipeline.GetElementByName("demux")
	if err != nil {
		return nil, nil, err
	}

	// Probe the track layout: in PAUSED the demux parses the init segment's
	// moov and exposes one pad per track, then posts no-more-pads. The video
	// track count decides how many video pads the signing element gets.
	// Mid-stream pads are left unlinked, which the demuxer's flow combiner
	// tolerates.
	var padsMu sync.Mutex
	padNames := []string{}
	blockProbes := map[*gst.Pad]uint64{}
	padHandle, err := demux.Connect("pad-added", func(self *gst.Element, pad *gst.Pad) {
		padsMu.Lock()
		padNames = append(padNames, pad.GetName())
		// Block media flow on the pad until its branch is linked — otherwise
		// the demux pushes into an unlinked pad during the probe and queues a
		// flow error on the bus. Buffers only: caps events still flow, and
		// no-more-pads still fires once the moov is parsed.
		blockProbes[pad] = pad.AddProbe(gst.PadProbeTypeBlock|gst.PadProbeTypeBuffer, func(p *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
			return gst.PadProbeOK
		})
		padsMu.Unlock()
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error connecting pad-added handler: %w", err)
	}
	defer demux.HandlerDisconnect(padHandle)
	noMorePads := make(chan struct{})
	var nmpOnce sync.Once
	nmpHandle, err := demux.Connect("no-more-pads", func(self *gst.Element) {
		nmpOnce.Do(func() { close(noMorePads) })
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error connecting no-more-pads handler: %w", err)
	}
	defer demux.HandlerDisconnect(nmpHandle)

	if err := pipeline.SetState(gst.StatePaused); err != nil {
		return nil, nil, fmt.Errorf("error probing MP4 track layout: %w", err)
	}
	// On any error after PAUSED, tear the pipeline down so GStreamer doesn't
	// hold a live (but stalled) graph in memory. Capture the pipeline in a
	// local so error returns (which set the named return to nil) don't
	// prevent cleanup.
	pausedPipeline := pipeline
	defer func() {
		if err != nil {
			_ = pausedPipeline.SetState(gst.StateNull)
		}
	}()
	select {
	case <-noMorePads:
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-time.After(10 * time.Second):
		return nil, nil, fmt.Errorf("timed out waiting for qtdemux to expose MP4 tracks")
	}

	padsMu.Lock()
	videoPads := []string{}
	audioPad := ""
	for _, name := range padNames {
		switch {
		case strings.HasPrefix(name, "video_"):
			videoPads = append(videoPads, name)
		case strings.HasPrefix(name, "audio_") && audioPad == "":
			audioPad = name
		}
	}
	padsMu.Unlock()
	sort.Slice(videoPads, func(i, j int) bool {
		return numericLess(strings.TrimPrefix(videoPads[i], "video_"), strings.TrimPrefix(videoPads[j], "video_"))
	})
	if len(videoPads) == 0 {
		return nil, nil, fmt.Errorf("no video track found in MP4 stream")
	}
	if len(videoPads) > 1 {
		log.Log(ctx, "multitrack MP4 ingest", "videoTracks", len(videoPads))
	}

	signerElem, signerDone, err := makeSigner(len(videoPads))
	if err != nil {
		return nil, nil, err
	}
	if err := pipeline.Add(signerElem); err != nil {
		return nil, nil, err
	}

	// One demux→signer branch per video track, in track order: demux pad
	// video_<i> lands on signer pad video_<i>, mirroring the RTMP multitrack
	// ingest layout.
	for i, padName := range videoPads {
		if err := linkMP4VideoBranch(pipeline, demux, signerElem, padName, i); err != nil {
			return nil, nil, err
		}
	}
	// A single audio branch, as before — extra audio tracks stay unlinked.
	if audioPad != "" {
		if err := linkMP4AudioBranch(pipeline, demux, signerElem, audioPad); err != nil {
			return nil, nil, err
		}
	}
	// Every branch is linked; lift the probe-time blocks and let media flow.
	padsMu.Lock()
	for pad, id := range blockProbes {
		pad.RemoveProbe(id)
	}
	padsMu.Unlock()
	return pipeline, signerDone, nil
}

// newQueue2Big creates a Queue2Big (see buildMP4IngestPipeline) as an element.
func newQueue2Big() (*gst.Element, error) {
	return gst.NewElementWithProperties("queue2", map[string]any{
		"max-size-buffers": constants.QueueMaxSizeBuffers,
		"max-size-bytes":   constants.QueueMaxSizeBytes,
		"max-size-time":    constants.QueueMaxSizeTime,
	})
}

// linkMP4Branch links one qtdemux pad through a queue and a chain of
// processing elements into a signer sink pad. The caller provides the elements
// in link order (queue first, then each processing element); the helper handles
// pipeline.Add, sequential linking, demux pad linking, and state sync.
func linkMP4Branch(pipeline *gst.Pipeline, demux, signer *gst.Element, demuxPadName, signerPadName string, elements ...*gst.Element) error {
	signerPad := signer.GetStaticPad(signerPadName)
	if signerPad == nil {
		return fmt.Errorf("signer has no pad %s", signerPadName)
	}
	for _, elem := range elements {
		if err := pipeline.Add(elem); err != nil {
			return err
		}
	}
	// Link queue → elem1 → elem2 → ... → signer
	for i := 0; i < len(elements)-1; i++ {
		if err := elements[i].Link(elements[i+1]); err != nil {
			return fmt.Errorf("failed to link %s to %s: %w", elements[i].GetName(), elements[i+1].GetName(), err)
		}
	}
	last := elements[len(elements)-1]
	if ret := last.GetStaticPad("src").Link(signerPad); ret != gst.PadLinkOK {
		return fmt.Errorf("failed to link %s to signer pad %s: %s", last.GetName(), signerPadName, ret)
	}
	srcPad := demux.GetStaticPad(demuxPadName)
	if srcPad == nil {
		return fmt.Errorf("qtdemux pad %s vanished", demuxPadName)
	}
	if ret := srcPad.Link(elements[0].GetStaticPad("sink")); ret != gst.PadLinkOK {
		return fmt.Errorf("failed to link %s to %s: %s", demuxPadName, elements[0].GetName(), ret)
	}
	// The branch was built while the pipeline sat paused mid-probe; sync it so
	// it can't lag behind when playback resumes.
	for _, elem := range elements {
		if !elem.SyncStateWithParent() {
			return fmt.Errorf("failed to sync %s state for pad %s", elem.GetName(), demuxPadName)
		}
	}
	return nil
}

// linkMP4VideoBranch links one qtdemux video pad through queue2 → h264parse
// into the signer's video_<index> pad. No h264timestamper: fMP4 carries real
// decode timestamps (see buildMP4IngestPipeline).
func linkMP4VideoBranch(pipeline *gst.Pipeline, demux, signer *gst.Element, padName string, index int) error {
	queue, err := newQueue2Big()
	if err != nil {
		return fmt.Errorf("failed to create queue2: %w", err)
	}
	parse, err := gst.NewElement("h264parse")
	if err != nil {
		return fmt.Errorf("failed to create h264parse: %w", err)
	}
	return linkMP4Branch(pipeline, demux, signer, padName, fmt.Sprintf("video_%d", index), queue, parse)
}

// linkMP4AudioBranch links the qtdemux audio pad through
// queue2 → fdkaacdec → audioresample → opusenc into the signer's audio_0 pad.
func linkMP4AudioBranch(pipeline *gst.Pipeline, demux, signer *gst.Element, padName string) error {
	queue, err := newQueue2Big()
	if err != nil {
		return fmt.Errorf("failed to create queue2: %w", err)
	}
	dec, err := gst.NewElement("fdkaacdec")
	if err != nil {
		return fmt.Errorf("failed to create fdkaacdec: %w", err)
	}
	resample, err := gst.NewElement("audioresample")
	if err != nil {
		return fmt.Errorf("failed to create audioresample: %w", err)
	}
	enc, err := gst.NewElement("opusenc")
	if err != nil {
		return fmt.Errorf("failed to create opusenc: %w", err)
	}
	return linkMP4Branch(pipeline, demux, signer, padName, "audio_0", queue, dec, resample, enc)
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
