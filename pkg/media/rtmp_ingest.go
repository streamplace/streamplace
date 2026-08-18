package media

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/log"
)

type RTMPH264Data struct {
	TrackID uint8
	AU      [][]byte
	PTS     time.Duration
	DTS     time.Duration
}

type RTMPAACData struct {
	AU  []byte
	PTS time.Duration
}

type RTMPSession struct {
	EventChan chan any
	// eRTMP multitrack: one entry per video track, keyed by the session-local
	// track ID assigned in HandleRTMPPublisher (0-based, in wire track order).
	// Keys are always contiguous 0..N-1 — HandleRTMPPublisher assigns them
	// sequentially, and HandleRTMPPlayback iterates them in that order.
	VideoTracks map[uint8]*format.H264
	AudioTrack  *format.MPEG4Audio
	MediaSigner MediaSigner
}

// RTMPIngest pulls the relayed FLV for a publishing session and runs it
// through the segment-signing pipeline. videoTrackCount is the number of video
// tracks on the session (1 for plain RTMP, N for eRTMP multitrack); the signer
// element exposes one video pad per track and flvdemux's pads are linked into
// them dynamically — flvdemux names the relay's legacy track "video" and each
// multitrack track "video_<id>", matching the session's track IDs.
func (mm *MediaManager) RTMPIngest(ctx context.Context, rtmpURL string, ms MediaSigner, videoTrackCount int) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// Mint the source audio: RTMP/FLV audio is already AAC, so pass it through
	// (aacparse) rather than transcoding to Opus. The validate path completes
	// each segment to also carry Opus when a consumer (WebRTC) needs it — so
	// the old RTMP-AAC→Opus→HLS-AAC double-transcode is gone.
	pipelineSlice := []string{
		fmt.Sprintf("rtmp2src location=%s ! flvdemux name=demux", rtmpURL),
	}
	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating RTMPIngest pipeline: %w", err)
	}

	signer, err := mm.SegmentAndSignElemTracks(ctx, ms, videoTrackCount)
	if err != nil {
		return err
	}

	err = pipeline.Add(signer)
	if err != nil {
		return err
	}

	demux, err := pipeline.GetElementByName("demux")
	if err != nil {
		return err
	}

	// Pre-build one queue→parser branch per expected pad while the pipeline is
	// still stopped: no element creation mid-playback, and the pad-added
	// closure only links — importantly it captures only the queues (elements
	// owned by the pipeline), never the pipeline itself, which would pin the
	// whole graph in go-gst's signal registry and leak it.
	queues, err := buildRTMPTrackChains(pipeline, signer, videoTrackCount)
	if err != nil {
		return err
	}

	// linked records which flvdemux pads have linked. Shared between the
	// pad-added handler and the multitrack pad watchdog.
	linkedMu := sync.Mutex{}
	linked := map[string]bool{}

	handle, err := demux.Connect("pad-added", func(self *gst.Element, pad *gst.Pad) {
		name := pad.GetName()
		chain, ok := queues[name]
		if !ok {
			log.Error(ctx, "unexpected flvdemux pad", "pad", name)
			return
		}
		if ret := pad.Link(chain.queue.GetStaticPad("sink")); ret != gst.PadLinkOK {
			log.Error(ctx, "error linking flvdemux pad", "pad", name, "error", ret)
			return
		}
		linkedMu.Lock()
		linked[name] = true
		linkedMu.Unlock()
		// Elements were created before the pipeline started, but a pad can
		// appear while parts of the graph are still mid-rollout; a push into a
		// not-yet-PLAYING branch returns FLUSHING and silently stalls the
		// demuxer. Sync the branch explicitly after linking.
		if !chain.queue.SyncStateWithParent() || !chain.parse.SyncStateWithParent() {
			log.Error(ctx, "error syncing branch state for flvdemux pad", "pad", name)
		}
	})
	if err != nil {
		return fmt.Errorf("error connecting pad-added handler: %w", err)
	}
	// deterministic cleanup of the go-gst closure registry entry: leaving the
	// connection live pins the closure's captures (the queues map) until the
	// demux is disposed, which shows up as leaked GstObjects in the leak tracer
	defer demux.HandlerDisconnect(handle)

	busErr := make(chan error)
	go func() {
		err := HandleBusMessages(ctx, pipeline)
		busErr <- err
	}()

	go mm.HandleKeyRevocation(ctx, ms, pipeline)

	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return err
	}

	// Prevents multitrack ingest from hanging indefinitely when a declared track
	// never links. mp4mux waits for every requested pad before emitting output, so
	// a missing track can otherwise stall the pipeline without producing a bus
	// error. After a grace period from PLAYING, fail the pipeline and report the
	// unlinked pads.
	//
	// Note that this only verifies pad linkage and not continued data flow. A track that links
	// and later goes silent will require a separate no-output watchdog.
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		deadline := time.After(multitrackIngestWatchdog)
		for {
			linkedMu.Lock()
			allLinked := len(linked) == len(queues)
			var missing []string
			if !allLinked {
				for name := range queues {
					if !linked[name] {
						missing = append(missing, name)
					}
				}
			}
			linkedMu.Unlock()
			if allLinked {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-deadline:
				sort.Strings(missing)
				err := fmt.Errorf("multitrack ingest wedged: declared track(s) never linked: %s", strings.Join(missing, ","))
				log.Error(ctx, "multitrack ingest pad watchdog fired", "unlinked", missing)
				pipeline.GetPipelineBus().Post(gst.NewErrorMessage(pipeline, err, "", nil))
				return
			case <-ticker.C:
			}
		}
	}()

	defer func() {
		teardownPipeline(ctx, pipeline)
	}()

	err = <-busErr

	return err
}

// rtmpTrackChain is one pre-built demux→signer branch (queue→parser).
type rtmpTrackChain struct {
	queue *gst.Element
	parse *gst.Element
}

// buildRTMPTrackChains pre-creates one queue→parser branch per expected
// flvdemux pad ("video"/"video_<id>" → h264parse → signer video_<id> pad,
// "audio" → aacparse → signer audio_0 pad) and returns them keyed by demux
// pad name. Elements are added to the pipeline before it starts, so they
// follow the pipeline's state changes — no SyncStateWithParent dance.
func buildRTMPTrackChains(pipeline *gst.Pipeline, signer *gst.Element, videoTrackCount int) (map[string]rtmpTrackChain, error) {
	queues := map[string]rtmpTrackChain{}
	addChain := func(demuxPadName, parser, signerPadName string) error {
		signerPad := signer.GetStaticPad(signerPadName)
		if signerPad == nil {
			return fmt.Errorf("signer has no pad %s (track count mismatch?)", signerPadName)
		}
		queue, err := gst.NewElement("queue")
		if err != nil {
			return fmt.Errorf("failed to create queue: %w", err)
		}
		parse, err := gst.NewElement(parser)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", parser, err)
		}
		if err := pipeline.Add(queue); err != nil {
			return err
		}
		if err := pipeline.Add(parse); err != nil {
			return err
		}
		if err := queue.Link(parse); err != nil {
			return fmt.Errorf("failed to link queue to %s: %w", parser, err)
		}
		if ret := parse.GetStaticPad("src").Link(signerPad); ret != gst.PadLinkOK {
			return fmt.Errorf("failed to link %s to signer pad %s: %s", parser, signerPadName, ret)
		}
		queues[demuxPadName] = rtmpTrackChain{queue: queue, parse: parse}
		return nil
	}

	for i := 0; i < videoTrackCount; i++ {
		// flvdemux exposes the relay's legacy track 0 as "video", multitrack
		// tracks as "video_<id>"
		demuxPadName := "video"
		if i > 0 {
			demuxPadName = fmt.Sprintf("video_%d", i)
		}
		if err := addChain(demuxPadName, "h264parse", fmt.Sprintf("video_%d", i)); err != nil {
			return nil, err
		}
	}
	if err := addChain("audio", "aacparse", "audio_0"); err != nil {
		return nil, err
	}
	return queues, nil
}
