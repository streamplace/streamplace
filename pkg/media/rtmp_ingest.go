package media

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bluenviron/gortmplib"
	"github.com/bluenviron/gortmplib/pkg/codecs"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/log"
)

type RTMPH264Data struct {
	AU  [][]byte
	PTS time.Duration
	DTS time.Duration
}

type RTMPAACData struct {
	AU  []byte
	PTS time.Duration
}

type RTMPSession struct {
	EventChan   chan any
	VideoTrack  *gortmplib.Track
	AudioTrack  *gortmplib.Track
	MediaSigner MediaSigner
}

// func (mm *MediaManager) RTMPIngest(ctx context.Context, rtmpURL string, ms MediaSigner) error {
// 	ctx, cancel := context.WithCancel(ctx)
// 	defer cancel()
// 	pipelineSlice := []string{
// 		fmt.Sprintf("rtmp2src location=%s ! flvdemux name=demux", rtmpURL),
// 		"demux.audio ! queue ! fdkaacdec ! audioresample ! opusenc ! queue name=audioenc",
// 		"demux.video ! queue ! h264parse ! queue name=parse",
// 	}
// 	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
// 	if err != nil {
// 		return fmt.Errorf("error creating RTMPIngest pipeline: %w", err)
// 	}

// 	signer, err := mm.SegmentAndSignElem(ctx, ms)
// 	if err != nil {
// 		return err
// 	}

// 	parseEle, err := pipeline.GetElementByName("parse")
// 	if err != nil {
// 		return err
// 	}

// 	err = pipeline.Add(signer)
// 	if err != nil {
// 		return err
// 	}
// 	err = parseEle.Link(signer)
// 	if err != nil {
// 		return err
// 	}
// 	audioenc, err := pipeline.GetElementByName("audioenc")
// 	if err != nil {
// 		return err
// 	}
// 	err = audioenc.Link(signer)
// 	if err != nil {
// 		return err
// 	}

// 	busErr := make(chan error)
// 	go func() {
// 		err := HandleBusMessages(ctx, pipeline)
// 		busErr <- err
// 	}()

// 	go mm.HandleKeyRevocation(ctx, ms, pipeline)

// 	err = pipeline.SetState(gst.StatePlaying)
// 	if err != nil {
// 		return err
// 	}

// 	defer func() {
// 		err := pipeline.SetState(gst.StateNull)
// 		if err != nil {
// 			log.Error(ctx, "error setting pipeline to null state", "error", err)
// 		}
// 	}()

// 	err = <-busErr

// 	return err
// }

// }
// }
// }
// }
// }

// // ingest a H264+AAC RTMP stream
func (mm *MediaManager) RTMPIngest(ctx context.Context, eventStream chan any, audioCodec *codecs.MPEG4Audio, ms MediaSigner) error {
	videoInput := make(chan *RTMPH264Data)
	audioInput := make(chan *RTMPAACData)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-eventStream:
				if event == nil {
					return
				}
				switch event := event.(type) {
				case *RTMPH264Data:
					videoInput <- event
				case *RTMPAACData:
					audioInput <- event
				}
			}
		}
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	pipelineSlice := []string{
		"appsrc name=videosrc ! queue ! h264parse name=parse",
		"appsrc name=audiosrc ! queue ! aacparse ! fdkaacdec ! audioresample ! opusenc name=audioenc",
	}
	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating RTMPIngest pipeline: %w", err)
	}

	videosrcEle, err := pipeline.GetElementByName("videosrc")
	if err != nil {
		return err
	}
	// first := true
	// defer runtime.KeepAlive(srcele)
	videosrc := app.SrcFromElement(videosrcEle)
	videosrc.SetCaps(gst.NewCapsFromString("video/x-h264,stream-format=byte-stream"))
	videosrc.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: func(self *app.Source, length uint) {
			if ctx.Err() != nil {
				self.EndStream()
				return
			}

			packet := <-videoInput
			if packet == nil {
				log.Debug(ctx, "video input closed, ending stream")
				self.EndStream()
				return
			}

			// allBytes := bytes.Buffer{}
			// for _, au := range packet.AU {
			// 	allBytes.Write(au)
			// }

			var avc []byte
			// if first {
			// 	c := h264conf.Conf{
			// 		SPS: packet.AU[0],
			// 		PPS: packet.AU[1],
			// 	}
			// 	avc, err = c.Marshal()
			// 	if err != nil {
			// 		log.Error(ctx, "failed to marshal H264 config", "error", err)
			// 		self.Error("failed to marshal H264 config", fmt.Errorf("failed to marshal H264 config: %w", err))
			// 		return
			// 	}
			// 	first = false
			// } else {
			avc, err = h264.AnnexB(packet.AU).Marshal()
			if err != nil {
				log.Error(ctx, "failed to marshal AnnexB", "error", err)
				self.Error("failed to marshal AnnexB", fmt.Errorf("failed to marshal AnnexB: %w", err))
				return
			}
			// }

			buf := gst.NewBufferFromBytes(avc)
			pts := gst.ClockTime(uint64(packet.PTS.Nanoseconds()))
			buf.SetPresentationTimestamp(pts)
			ret := self.PushBuffer(buf)
			if ret != gst.FlowOK {
				log.Error(ctx, "failed to push video buffer", "error", ret.String())
				self.Error("failed to push video buffer", fmt.Errorf("failed to push video buffer: %s", ret.String()))
				return
			}
		},
	})

	audiosrcEle, err := pipeline.GetElementByName("audiosrc")
	if err != nil {
		return err
	}

	// audio/mpeg:
	//   mpegversion: { (int)2, (int)4 }
	// stream-format: { (string)adts, (string)adif, (string)raw }
	//      channels: [ 1, 8 ]
	// defer runtime.KeepAlive(srcele)
	audiosrc := app.SrcFromElement(audiosrcEle)
	audiosrc.SetCaps(gst.NewCapsFromString("audio/mpeg,mpegversion=4,stream-format=adts,channels=2"))
	audiosrc.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: func(self *app.Source, length uint) {
			if ctx.Err() != nil {
				self.EndStream()
				return
			}
			aacData := <-audioInput
			if aacData == nil {
				log.Debug(ctx, "audio input closed, ending stream")
				self.EndStream()
				return
			}
			// mpeg4audio.ADTSPacket(packet.AU)

			pkt := &mpeg4audio.ADTSPacket{
				Type:         audioCodec.Config.Type,
				SampleRate:   audioCodec.Config.SampleRate,
				ChannelCount: audioCodec.Config.ChannelCount,
				AU:           aacData.AU,
			}
			pkts := mpeg4audio.ADTSPackets{pkt}

			bs, err := pkts.Marshal()
			if err != nil {
				log.Error(ctx, "failed to marshal ADTS packets", "error", err)
				self.Error("failed to marshal ADTS packets", fmt.Errorf("failed to marshal ADTS packets: %w", err))
				return
			}

			buf := gst.NewBufferFromBytes(bs)
			pts := gst.ClockTime(uint64(aacData.PTS.Nanoseconds()))
			buf.SetPresentationTimestamp(pts)
			ret := self.PushBuffer(buf)
			if ret != gst.FlowOK {
				log.Error(ctx, "failed to push audio buffer", "error", ret.String())
				self.Error("failed to push audio buffer", fmt.Errorf("failed to push audio buffer: %s", ret.String()))
				return
			}
		},
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

	log.Log(ctx, "starting pipeline")

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
