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

	signer, err := mm.SegmentAndSignElem(ctx, ms)
	if err != nil {
		return err
	}
	pipeline, err := buildMKVIngestPipeline(ctx, input, signer)
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

// buildMKVIngestPipeline builds the H264+AAC MKV demux graph (video → h264parse,
// audio → Opus re-encode) and links both branches into signerElem — the muxl
// signing bin that emits one bare canonical .m4s per GoP. Shared by the
// in-process MKVIngest and the isolated ingest worker, which differ only in
// where signerElem routes its segments (ValidateMP4 vs. a frame writer to the
// main process).
func buildMKVIngestPipeline(ctx context.Context, input io.Reader, signerElem *gst.Element) (*gst.Pipeline, error) {
	pipelineSlice := []string{
		"appsrc name=streamsrc ! matroskademux name=demux",
		"demux. ! queue ! h264parse name=parse",
		"demux. ! queue ! fdkaacdec ! audioresample ! opusenc name=audioenc",
	}
	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return nil, fmt.Errorf("error creating MKVIngest pipeline: %w", err)
	}
	srcele, err := pipeline.GetElementByName("streamsrc")
	if err != nil {
		return nil, err
	}
	app.SrcFromElement(srcele).SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedDataIncremental(ctx, input),
	})
	parseEle, err := pipeline.GetElementByName("parse")
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
