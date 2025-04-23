package media

import (
	"context"
	"fmt"

	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/media/segchanman"
)

func ConcatBin(ctx context.Context, segCh <-chan *segchanman.Seg) (*gst.Bin, error) {
	bin := gst.NewBin("concat")

	streamsynchronizer, err := gst.NewElementWithProperties("streamsynchronizer", map[string]any{
		"name": "concat-streamsynchronizer",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create streamsynchronizer element: %w", err)
	}

	err = bin.Add(streamsynchronizer)
	if err != nil {
		return nil, fmt.Errorf("failed to add streamsynchronizer to pipeline: %w", err)
	}

	syncPadVideoSink := streamsynchronizer.GetRequestPad("sink_%u")
	if syncPadVideoSink == nil {
		return nil, fmt.Errorf("failed to get sync video sink pad")
	}

	syncPadAudioSink := streamsynchronizer.GetRequestPad("sink_%u")
	if syncPadAudioSink == nil {
		return nil, fmt.Errorf("failed to get sync audio sink pad")
	}

	syncPadVideoSrc := streamsynchronizer.GetStaticPad("src_0")
	if syncPadVideoSrc == nil {
		return nil, fmt.Errorf("failed to get sync video src pad")
	}

	syncPadAudioSrc := streamsynchronizer.GetStaticPad("src_1")
	if syncPadAudioSrc == nil {
		return nil, fmt.Errorf("failed to get sync audio src pad")
	}

	videoGhost := gst.NewGhostPad("video_0", syncPadVideoSrc)
	if videoGhost == nil {
		return nil, fmt.Errorf("failed to create video ghost pad")
	}

	audioGhost := gst.NewGhostPad("audio_0", syncPadAudioSrc)
	if audioGhost == nil {
		return nil, fmt.Errorf("failed to create audio ghost pad")
	}

	ok := bin.AddPad(videoGhost.Pad)
	if !ok {
		return nil, fmt.Errorf("failed to add video ghost pad to bin")
	}

	ok = bin.AddPad(audioGhost.Pad)
	if !ok {
		return nil, fmt.Errorf("failed to add audio ghost pad to bin")
	}

	demuxBin, err := SegDemuxBin(ctx, segCh)
	if err != nil {
		return nil, fmt.Errorf("failed to create demux bin: %w", err)
	}

	err = bin.Add(demuxBin.Element)
	if err != nil {
		return nil, fmt.Errorf("failed to add demux bin to bin: %w", err)
	}

	demuxBinPadVideoSrc := demuxBin.GetStaticPad("video_0")
	if demuxBinPadVideoSrc == nil {
		return nil, fmt.Errorf("failed to get demux bin video src pad")
	}

	demuxBinPadAudioSrc := demuxBin.GetStaticPad("audio_0")
	if demuxBinPadAudioSrc == nil {
		return nil, fmt.Errorf("failed to get demux bin audio src pad")
	}

	linked := demuxBinPadVideoSrc.Link(syncPadVideoSink)
	if linked != gst.PadLinkOK {
		return nil, fmt.Errorf("failed to link demux bin video src pad to sync video sink pad: %v", ok)
	}

	linked = demuxBinPadAudioSrc.Link(syncPadAudioSink)
	if linked != gst.PadLinkOK {
		return nil, fmt.Errorf("failed to link demux bin audio src pad to sync audio sink pad: %v", ok)
	}

	return bin, nil
}
