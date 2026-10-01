package media

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/stt"
)

// captionAudioDecoder is ONE long-lived decode of a stream's audio into
// 16 kHz mono float32 for speech recognition. Segments are fed in order
// (init once, then the bare canonical bytes), so the decoder runs
// continuously: no priming, no boundary clicks, one timeline.
//
// Every fed segment records where its audio starts on the media timeline
// (its tfdt) and on the wall clock (its startTime). Decoded buffers carry
// media-timeline PTS, which the mark table turns back into wall-clock
// time, so a sample's time is exact relative to the segment it came from
// whatever the decoder's latency.
type captionAudioDecoder struct {
	ctx    context.Context
	cancel context.CancelFunc
	codec  string // "aac" or "opus"
	onPCM  func(start time.Time, pcm []float32)

	feedW *io.PipeWriter
	done  chan struct{}

	mu       sync.Mutex
	marks    []audioMark
	started  bool
	closed   bool
	err      error
	lastTfdt uint64
	haveTfdt bool
}

type audioMark struct {
	media time.Duration // tfdt on the media timeline
	wall  time.Time     // the segment's startTime
}

// audioMarksKept bounds the mark table; decoded audio lags the fed
// segments by far less.
const audioMarksKept = 32

func newCaptionAudioDecoder(parent context.Context, codec string, onPCM func(time.Time, []float32)) (*captionAudioDecoder, error) {
	pipeline, err := buildCaptionAudioPipeline(codec)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	feedR, feedW := io.Pipe()
	d := &captionAudioDecoder{ctx: ctx, cancel: cancel, codec: codec, onPCM: onPCM, feedW: feedW, done: make(chan struct{})}
	go func() {
		defer close(d.done)
		err := d.run(pipeline, feedR)
		d.mu.Lock()
		if d.err == nil {
			d.err = err
		}
		d.mu.Unlock()
		feedR.CloseWithError(err)
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Error(ctx, "caption audio decoder exited", "codec", codec, "error", err)
		}
	}()
	return d, nil
}

// buildCaptionAudioPipeline demuxes fed fMP4 bytes, decodes the one audio
// track to 16 kHz mono float samples, and discards the video.
func buildCaptionAudioPipeline(codec string) (*gst.Pipeline, error) {
	var decode string
	switch codec {
	case "aac":
		decode = "aacparse ! fdkaacdec"
	case "opus":
		decode = "opusparse ! opusdec"
	default:
		return nil, fmt.Errorf("unsupported caption audio codec %q", codec)
	}
	pipeline, err := gst.NewPipelineFromString(strings.Join([]string{
		"appsrc name=src ! qtdemux name=demux",
		constants.Queue2Big + " name=vq ! fakesink sync=false",
		fmt.Sprintf("%s name=aq ! %s ! audioconvert ! audioresample ! audio/x-raw,format=F32LE,rate=%d,channels=1,layout=interleaved ! appsink name=sink sync=false", constants.Queue2Big, decode, stt.SampleRate),
	}, "\n"))
	if err != nil {
		return nil, fmt.Errorf("create caption audio pipeline: %w", err)
	}
	vq, err := pipeline.GetElementByName("vq")
	if err != nil {
		return nil, err
	}
	aq, err := pipeline.GetElementByName("aq")
	if err != nil {
		return nil, err
	}
	demux, err := pipeline.GetElementByName("demux")
	if err != nil {
		return nil, err
	}
	if _, err := demux.Connect("pad-added", func(self *gst.Element, pad *gst.Pad) {
		name := pad.GetName()
		var dst *gst.Pad
		switch {
		case strings.HasPrefix(name, "video"):
			dst = vq.GetStaticPad("sink")
		case strings.HasPrefix(name, "audio"):
			dst = aq.GetStaticPad("sink")
		default:
			return
		}
		if r := pad.Link(dst); r != gst.PadLinkOK {
			// A second track of the same kind is simply not decoded.
			fmt.Printf("caption audio: failed to link demux pad %s: %v\n", name, r)
		}
	}); err != nil {
		return nil, fmt.Errorf("connect demux pad-added: %w", err)
	}
	return pipeline, nil
}

func (d *captionAudioDecoder) run(pipeline *gst.Pipeline, feedR *io.PipeReader) error {
	ctx := d.ctx
	defer func() {
		if e := pipeline.SetState(gst.StateNull); e != nil {
			log.Error(ctx, "caption audio: set null", "error", e)
		}
	}()
	srcEle, err := pipeline.GetElementByName("src")
	if err != nil {
		return err
	}
	app.SrcFromElement(srcEle).SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedDataIncremental(ctx, feedR),
	})
	sinkEle, err := pipeline.GetElementByName("sink")
	if err != nil {
		return err
	}
	app.SinkFromElement(sinkEle).SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}
			buf := sample.GetBuffer()
			if buf == nil {
				return gst.FlowError
			}
			pts := buf.PresentationTimestamp()
			if pts == gst.ClockTimeNone {
				return gst.FlowOK
			}
			d.deliver(time.Duration(pts), buf.Bytes())
			return gst.FlowOK
		},
	})
	busErr := make(chan error, 1)
	go func() { busErr <- HandleBusMessages(ctx, pipeline) }()
	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		return fmt.Errorf("caption audio: set playing: %w", err)
	}
	return <-busErr
}

// deliver maps a decoded buffer's media PTS to the wall clock through the
// mark of the segment it belongs to, and hands the samples on.
func (d *captionAudioDecoder) deliver(pts time.Duration, raw []byte) {
	n := len(raw) / 4
	if n == 0 {
		return
	}
	d.mu.Lock()
	var mark *audioMark
	for i := len(d.marks) - 1; i >= 0; i-- {
		if d.marks[i].media <= pts+discontinuitySlack {
			mark = &d.marks[i]
			break
		}
	}
	if mark == nil && len(d.marks) > 0 {
		mark = &d.marks[0]
	}
	if mark == nil {
		d.mu.Unlock()
		return
	}
	start := mark.wall.Add(pts - mark.media)
	d.mu.Unlock()

	pcm := make([]float32, n)
	for i := range pcm {
		pcm[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
	}
	d.onPCM(start, pcm)
}

// discontinuitySlack lets a buffer whose PTS sits a hair before its
// segment's tfdt (decoder reordering, rounding) still map to that segment.
const discontinuitySlack = 20 * time.Millisecond

// feed writes one segment's audio into the decoder. seg is the bare
// canonical bytes filtered to the one audio codec (plus video, which is
// discarded); media is the audio track's tfdt as a duration; wall is the
// segment's startTime. init is the synthesized init segment, written once
// before the first feed.
func (d *captionAudioDecoder) feed(init, seg []byte, tfdt uint64, media time.Duration, wall time.Time) error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return errors.New("caption audio decoder closed")
	}
	if d.err != nil {
		err := d.err
		d.mu.Unlock()
		return err
	}
	first := !d.started
	d.started = true
	d.marks = append(d.marks, audioMark{media: media, wall: wall})
	if len(d.marks) > audioMarksKept {
		d.marks = d.marks[len(d.marks)-audioMarksKept:]
	}
	d.lastTfdt, d.haveTfdt = tfdt, true
	d.mu.Unlock()

	if first {
		if _, err := d.feedW.Write(init); err != nil {
			return err
		}
	}
	_, err := d.feedW.Write(seg)
	return err
}

// backwards reports whether a segment's tfdt would step the media timeline
// backwards: a reconnect restarted it, so the decoder must be rebuilt.
func (d *captionAudioDecoder) backwards(tfdt uint64) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.haveTfdt && tfdt < d.lastTfdt
}

func (d *captionAudioDecoder) failed() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.err != nil
}

// close ends the stream (EOS flushes the decoder's tail) and tears the
// pipeline down.
func (d *captionAudioDecoder) close() {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	d.closed = true
	started := d.started
	d.mu.Unlock()
	d.feedW.Close()
	if started {
		select {
		case <-d.done:
		case <-time.After(10 * time.Second):
			d.cancel()
			<-d.done
		}
	} else {
		d.cancel()
		<-d.done
	}
	d.cancel()
}
