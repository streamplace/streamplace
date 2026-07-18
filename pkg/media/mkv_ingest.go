package media

import (
	"context"
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

// ingest a H264+AAC MKV stream (prolly from an RTMP server)
func (mm *MediaManager) MKVIngest(ctx context.Context, input io.Reader, ms MediaSigner) error {
	shouldRecord, err := mm.shouldRecord(ctx, ms.Streamer())
	if err != nil {
		return err
	}
	if shouldRecord {
		log.Log(ctx, "recording RTMP stream to file", "streamer", ms.Streamer())
		pr, pw := io.Pipe()
		input = io.TeeReader(input, pw)
		go func() {
			err := mm.dumpToFile(ctx, pr, ms.Streamer(), ".rtmp.mkv")
			if err != nil {
				log.Error(ctx, "error dumping to file", "error", err)
			}
		}()
	} else {
		log.Log(ctx, "not recording RTMP stream to file", "streamer", ms.Streamer())
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// The track layout is only known after matroskademux parses the MKV header
	// (N video tracks for eRTMP multitrack pushes), so the signing element is
	// built by the pipeline once it has probed the tracks.
	makeSigner := func(videoTrackCount int) (*gst.Element, <-chan struct{}, error) {
		elem, err := mm.SegmentAndSignElemTracks(ctx, ms, videoTrackCount)
		return elem, nil, err
	}
	pipeline, _, err := buildMKVIngestPipeline(ctx, input, makeSigner)
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

// mkvSignerFactory builds the muxl signing element for the probed MKV track
// layout: videoTrackCount video pads plus the usual single audio pad. It is
// called once matroskademux has exposed the stream's tracks — 1 video track
// for plain pushes, N for eRTMP multitrack (e.g. OBS multitrack via MistServer).
// The returned done channel closes once the signer has drained all segments.
type mkvSignerFactory func(videoTrackCount int) (*gst.Element, <-chan struct{}, error)

// buildMKVIngestPipeline builds the MKV demux graph — one h264parse →
// h264timestamper branch per video track, audio → Opus re-encode — linked into
// a signing element created by makeSigner once probing has established the
// track count. Shared by the in-process MKVIngest and the isolated ingest
// worker, which differ only in where the signing element routes its segments
// (ValidateMP4 vs. a frame writer to the main process).
func buildMKVIngestPipeline(ctx context.Context, input io.Reader, makeSigner mkvSignerFactory) (pipeline *gst.Pipeline, done <-chan struct{}, err error) {
	// Queue sizing: matroskademux feeds both branches from one thread, and the
	// fMP4 muxer downstream is an aggregator — it consumes NOTHING until every
	// pad has data. If the video track goes sparse (e.g. MistServer drops all
	// delta frames when a push falls behind, leaving ~1s-apart keyframes), the
	// audio branch must buffer a full video-frame gap while the demux walks the
	// byte stream to the next video frame. gst's default queue caps at
	// max-size-time=1s, so a ≥1s video gap fills the audio queue, blocks the
	// demux, starves the muxer's video pad, and deadlocks the whole graph with
	// no EOS — a live stream wedges until the watchdog kills it. Use the shared
	// Queue2Big preset (no time/buffer cap, generous byte cap) like the other
	// demux-fed pipelines (transcode, rtmp_push, packetize, media_data_parser).
	//
	// h264timestamper: Matroska blocks carry only presentation timestamps, so
	// for B-frame streams (PTS ≠ DTS) matroskademux emits reordered PTS with
	// dts=none — and h264parse does not reconstruct DTS. The fMP4 muxer needs
	// DTS to mux a reordered stream; without it it treats the jumbled PTS as
	// monotonic timing and stretches the video track (~2.2× on a real capture),
	// which downstream makes qtdemux EOS the audio pad early and every segment
	// fails validation with "no audio in segment". h264timestamper rebuilds
	// DTS from the H264 picture order count.
	pipelineSlice := []string{
		"appsrc name=streamsrc ! matroskademux name=demux",
	}
	pipeline, err = gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, nil, fmt.Errorf("error creating MKVIngest pipeline: %w", err)
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

	// Probe the track layout: in PAUSED the demux parses the Tracks header and
	// exposes one pad per track, then posts no-more-pads. The video track count
	// decides how many video pads the signing element gets. Mid-stream pads
	// (e.g. MistServer's M_JSON metadata track announces late) are left
	// unlinked, which the demuxer's flow combiner tolerates.
	var padsMu sync.Mutex
	padNames := []string{}
	blockProbes := map[*gst.Pad]uint64{}
	padHandle, err := demux.Connect("pad-added", func(self *gst.Element, pad *gst.Pad) {
		padsMu.Lock()
		padNames = append(padNames, pad.GetName())
		// Block media flow on the pad until its branch is linked — otherwise
		// the demux pushes into an unlinked pad during the probe and queues a
		// flow error on the bus. Buffers only: caps events still flow, and
		// no-more-pads still fires once the Tracks header is parsed.
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
		return nil, nil, fmt.Errorf("error probing MKV track layout: %w", err)
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
		return nil, nil, fmt.Errorf("timed out waiting for matroskademux to expose MKV tracks")
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
		return nil, nil, fmt.Errorf("no video track found in MKV stream")
	}
	if len(videoPads) > 1 {
		log.Log(ctx, "multitrack MKV ingest", "videoTracks", len(videoPads))
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
		if err := linkMKVVideoBranch(pipeline, demux, signerElem, padName, i); err != nil {
			return nil, nil, err
		}
	}
	// A single audio branch, as before — extra audio tracks stay unlinked.
	if audioPad != "" {
		if err := linkMKVAudioBranch(pipeline, demux, signerElem, audioPad); err != nil {
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

// newQueue2Big creates a Queue2Big (see buildMKVIngestPipeline) as an element.
func newQueue2Big() (*gst.Element, error) {
	return gst.NewElementWithProperties("queue2", map[string]any{
		"max-size-buffers": constants.QueueMaxSizeBuffers,
		"max-size-bytes":   constants.QueueMaxSizeBytes,
		"max-size-time":    constants.QueueMaxSizeTime,
	})
}

// linkMKVBranch links one matroskademux pad through a queue and a chain of
// processing elements into a signer sink pad. The caller provides the elements
// in link order (queue first, then each processing element); the helper handles
// pipeline.Add, sequential linking, demux pad linking, and state sync.
func linkMKVBranch(pipeline *gst.Pipeline, demux, signer *gst.Element, demuxPadName, signerPadName string, elements ...*gst.Element) error {
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
		return fmt.Errorf("matroskademux pad %s vanished", demuxPadName)
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

// linkMKVVideoBranch links one matroskademux video pad through
// queue2→h264parse→h264timestamper into the signer's video_<index> pad.
func linkMKVVideoBranch(pipeline *gst.Pipeline, demux, signer *gst.Element, padName string, index int) error {
	queue, err := newQueue2Big()
	if err != nil {
		return fmt.Errorf("failed to create queue2: %w", err)
	}
	parse, err := gst.NewElement("h264parse")
	if err != nil {
		return fmt.Errorf("failed to create h264parse: %w", err)
	}
	ts, err := gst.NewElement("h264timestamper")
	if err != nil {
		return fmt.Errorf("failed to create h264timestamper: %w", err)
	}
	return linkMKVBranch(pipeline, demux, signer, padName, fmt.Sprintf("video_%d", index), queue, parse, ts)
}

// linkMKVAudioBranch links the matroskademux audio pad through
// queue2→fdkaacdec→audioresample→opusenc into the signer's audio_0 pad.
func linkMKVAudioBranch(pipeline *gst.Pipeline, demux, signer *gst.Element, padName string) error {
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
	return linkMKVBranch(pipeline, demux, signer, padName, "audio_0", queue, dec, resample, enc)
}

func (mm *MediaManager) dumpToFile(ctx context.Context, r io.Reader, user string, filesuffix string) error {
	now := aqtime.FromTime(time.Now())
	filename := fmt.Sprintf("%s%s", now.FileSafeString(), filesuffix)
	// Streams to S3 when configured (production), else a local file under DataDir
	// (dev). Close finalizes either target — for S3 it commits the upload.
	f, err := mm.cli.DebugRecordingCreate(ctx, []string{"debug-recordings", user, filename}, "video/x-matroska", false)
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
