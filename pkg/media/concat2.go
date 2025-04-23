package media

import (
	"context"
	"fmt"

	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media/segchanman"
)

func ConcatBin(ctx context.Context, segCh <-chan *segchanman.Seg) (*gst.Bin, error) {
	ctx = log.WithLogValues(ctx, "func", "ConcatBin")
	bin := gst.NewBin("concat-bin")

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

	mq, err := gst.NewElementWithProperties("multiqueue", map[string]interface{}{
		"name": "concat-multiqueue",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create multiqueue element: %w", err)
	}
	err = bin.Add(mq)
	if err != nil {
		return nil, fmt.Errorf("failed to add multiqueue to bin: %w", err)
	}

	mqVideoSink := mq.GetRequestPad("sink_%u")
	if mqVideoSink == nil {
		return nil, fmt.Errorf("video sink pad not found")
	}

	mqAudioSink := mq.GetRequestPad("sink_%u")
	if mqAudioSink == nil {
		return nil, fmt.Errorf("audio sink pad not found")
	}

	mqVideoSrc := mq.GetStaticPad("src_0")
	if mqVideoSrc == nil {
		return nil, fmt.Errorf("video source pad not found")
	}

	mqAudioSrc := mq.GetStaticPad("src_1")
	if mqAudioSrc == nil {
		return nil, fmt.Errorf("audio source pad not found")
	}

	linked := syncPadVideoSrc.Link(mqVideoSink)
	if linked != gst.PadLinkOK {
		return nil, fmt.Errorf("failed to link sync video src pad to multiqueue video sink pad: %v", linked)
	}

	linked = syncPadAudioSrc.Link(mqAudioSink)
	if linked != gst.PadLinkOK {
		return nil, fmt.Errorf("failed to link sync audio src pad to multiqueue audio sink pad: %v", linked)
	}

	videoGhost := gst.NewGhostPad("video_0", mqVideoSrc)
	if videoGhost == nil {
		return nil, fmt.Errorf("failed to create video ghost pad")
	}

	audioGhost := gst.NewGhostPad("audio_0", mqAudioSrc)
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

	go func() {
		for {
			select {
			case seg := <-segCh:
				err := addConcatDemuxer(ctx, bin, seg, syncPadVideoSink, syncPadAudioSink)
				if err != nil {
					panic(fmt.Errorf("failed to add concat demuxer: %w", err))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return bin, nil
}

func addConcatDemuxer(ctx context.Context, bin *gst.Bin, seg *segchanman.Seg, syncPadVideoSink *gst.Pad, syncPadAudioSink *gst.Pad) error {
	log.Debug(ctx, "adding concat demuxer", "seg", seg.Filepath)
	demuxBin, err := ConcatDemuxBin(ctx, seg)
	if err != nil {
		return fmt.Errorf("failed to create demux bin: %w", err)
	}

	err = bin.Add(demuxBin.Element)
	if err != nil {
		return fmt.Errorf("failed to add demux bin to bin: %w", err)
	}

	demuxBinPadVideoSrc := demuxBin.GetStaticPad("video_0")
	if demuxBinPadVideoSrc == nil {
		return fmt.Errorf("failed to get demux bin video src pad")
	}

	demuxBinPadAudioSrc := demuxBin.GetStaticPad("audio_0")
	if demuxBinPadAudioSrc == nil {
		return fmt.Errorf("failed to get demux bin audio src pad")
	}

	linked := demuxBinPadVideoSrc.Link(syncPadVideoSink)
	if linked != gst.PadLinkOK {
		return fmt.Errorf("failed to link demux bin video src pad to sync video sink pad: %v", linked)
	}

	linked = demuxBinPadAudioSrc.Link(syncPadAudioSink)
	if linked != gst.PadLinkOK {
		return fmt.Errorf("failed to link demux bin audio src pad to sync audio sink pad: %v", linked)
	}

	bufferCh := make(chan struct{})
	demuxBinPadVideoSrc.AddProbe(gst.PadProbeTypeBuffer, func(pad *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		log.Warn(ctx, "pad-buffer", "type", pad.GetName(), "direction", pad.GetDirection())
		go func() {
			bufferCh <- struct{}{}
		}()
		return gst.PadProbeRemove
	})

	bin.SetState(gst.StatePlaying)

	<-bufferCh
	<-bufferCh

	idleCh := make(chan struct{})
	padIdle := func(pad *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		log.Warn(ctx, "pad-idle", "name", pad.GetName(), "direction", pad.GetDirection())
		go func() {
			idleCh <- struct{}{}
		}()
		return gst.PadProbeRemove
	}
	demuxBinPadVideoSrc.AddProbe(gst.PadProbeTypeIdle, padIdle)
	demuxBinPadAudioSrc.AddProbe(gst.PadProbeTypeIdle, padIdle)

	<-idleCh
	<-idleCh

	ok := demuxBinPadVideoSrc.Unlink(syncPadVideoSink)
	if !ok {
		return fmt.Errorf("failed to unlink demux bin video src pad from sync video sink pad: %v", ok)
	}

	ok = demuxBinPadAudioSrc.Unlink(syncPadAudioSink)
	if !ok {
		return fmt.Errorf("failed to unlink demux bin audio src pad from sync audio sink pad: %v", ok)
	}

	err = bin.Remove(demuxBin.Element)
	if err != nil {
		return fmt.Errorf("failed to remove demux bin from bin: %w", err)
	}

	log.Debug(ctx, "removed concat demuxer", "seg", seg.Filepath)

	return nil
}
