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
	pipelineSlice := []string{
		"appsrc name=streamsrc ! matroskademux name=demux",
		"demux. ! queue ! h264parse name=parse",
		"demux. ! queue ! fdkaacdec ! audioresample ! opusenc name=audioenc",
	}
	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating MKVIngest pipeline: %w", err)
	}

	srcele, err := pipeline.GetElementByName("streamsrc")
	if err != nil {
		return err
	}
	// defer runtime.KeepAlive(srcele)
	src := app.SrcFromElement(srcele)
	src.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedDataIncremental(ctx, input),
	})
	parseEle, err := pipeline.GetElementByName("parse")
	if err != nil {
		return err
	}

	signer, err := mm.SegmentAndSignElem(ctx, ms)
	if err != nil {
		return err
	}

	err = pipeline.Add(signer)
	if err != nil {
		return err
	}
	err = parseEle.Link(signer)
	if err != nil {
		return err
	}
	audioenc, err := pipeline.GetElementByName("audioenc")
	if err != nil {
		return err
	}
	err = audioenc.Link(signer)
	if err != nil {
		return err
	}

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

	defer func() {
		err := pipeline.SetState(gst.StateNull)
		if err != nil {
			log.Error(ctx, "error setting pipeline to null state", "error", err)
		}
	}()

	err = <-busErr

	return err
}

func (mm *MediaManager) dumpToFile(ctx context.Context, r io.Reader, user string, filesuffix string) error {
	now := aqtime.FromTime(time.Now())
	filename := fmt.Sprintf("%s%s", now.FileSafeString(), filesuffix)
	f, err := mm.cli.DataFileCreate([]string{"debug-recordings", user, filename}, false)
	if err != nil {
		return fmt.Errorf("failed to create data file: %w", err)
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	if err != nil {
		return fmt.Errorf("failed to copy to file: %w", err)
	}
	return nil
}
