package media

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/log"
)

var ErrConcatDone = errors.New("concat done")

func ConcatBin(ctx context.Context, segCh <-chan *bus.Seg) (*gst.Bin, error) {
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
				if seg == nil {
					ok := syncPadVideoSrc.PushEvent(gst.NewEOSEvent())
					if !ok {
						log.Error(ctx, "failed to post EOS message", "error", ok)
					}
					ok = syncPadAudioSrc.PushEvent(gst.NewEOSEvent())
					if !ok {
						log.Error(ctx, "failed to post EOS message", "error", ok)
					}
					log.Debug(ctx, "concat completed")
					return
				}
				err := addConcatDemuxer(ctx, bin, seg, syncPadVideoSink, syncPadAudioSink)
				if err != nil {
					log.Error(ctx, "failed to add concat demuxer", "error", err)
					bin.Error(err.Error(), err)
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return bin, nil
}

func addConcatDemuxer(ctx context.Context, bin *gst.Bin, seg *bus.Seg, syncPadVideoSink *gst.Pad, syncPadAudioSink *gst.Pad) error {

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

	eosCh := make(chan struct{})
	eos := func(pad *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		if pad.GetDirection() != gst.PadDirectionSource {
			return gst.PadProbeOK
		}
		if info.GetEvent().Type() != gst.EventTypeEOS {
			return gst.PadProbeOK
		}
		log.Debug(ctx, "demux EOS", "name", pad.GetName(), "direction", pad.GetDirection())
		downstreamPad := pad.GetPeer()
		unlinked := pad.Unlink(downstreamPad)
		if !unlinked {
			log.Error(ctx, "failed to unlink pad", "name", pad.GetName(), "direction", pad.GetDirection(), "error", unlinked)
		}
		go func() {
			eosCh <- struct{}{}
		}()
		return gst.PadProbeRemove
	}
	demuxBinPadVideoSrc.AddProbe(gst.PadProbeTypeEventBoth, eos)
	demuxBinPadAudioSrc.AddProbe(gst.PadProbeTypeEventBoth, eos)

	if err := bin.SetState(gst.StatePlaying); err != nil {
		return fmt.Errorf("failed to set state: %w", err)
	}

	<-eosCh
	<-eosCh

	err = bin.Remove(demuxBin.Element)
	if err != nil {
		return fmt.Errorf("failed to remove demux bin from bin: %w", err)
	}

	err = demuxBin.SetState(gst.StateNull)
	if err != nil {
		return fmt.Errorf("failed to set demux bin to null state: %w", err)
	}

	log.Debug(ctx, "removed concat demuxer", "seg", seg.Filepath)

	return nil
}
