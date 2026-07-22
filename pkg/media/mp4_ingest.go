package media

import (
	"context"
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
		pr, pw := io.Pipe()
		input = io.TeeReader(input, pw)
		go func() {
			err := mm.dumpToFile(ctx, pr, ms.Streamer(), ".rtmp.mp4")
			if err != nil {
				log.Error(ctx, "error dumping to file", "error", err)
			}
		}()
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
func buildMP4IngestPipeline(ctx context.Context, input io.Reader, signerElem *gst.Element) (*gst.Pipeline, error) {
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
