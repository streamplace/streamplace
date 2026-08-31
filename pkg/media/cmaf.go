package media

import (
	"bytes"
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/llhls"
	"stream.place/streamplace/pkg/log"
)

// cmafTrackSink translates the buffer-list contract of cmafmux into the
// application-level LL-HLS events. cmafmux emits one list per parent fragment;
// the first list also carries the initialization segment.
type cmafTrackSink struct {
	presentation string
	track        string
	window       *llhls.Window
	generation   uint64
	nextMSN      uint64
	initialized  bool
	partDuration time.Duration
	parent       bytes.Buffer
	parentStart  time.Duration
	parentLength time.Duration
	partIndex    uint32
	hasParent    bool
	samples      atomic.Uint64
}

func (s *cmafTrackSink) sample(sample *gst.Sample) error {
	list := sample.GetBufferList()
	if list == nil || list.Length() == 0 {
		return nil
	}
	var buffers [][]byte
	list.ForEach(func(buffer *gst.Buffer, _ uint) bool {
		buffers = append(buffers, append([]byte(nil), buffer.Bytes()...))
		return true
	})
	if len(buffers) == 0 {
		return nil
	}

	initIndex := -1
	if !s.initialized {
		for i, data := range buffers {
			buffer := list.GetBufferAt(uint(i))
			if isCMAFInit(data) || buffer.HasFlags(gst.BufferFlagHeader|gst.BufferFlagDiscont) {
				initIndex = i
				break
			}
		}
	}
	if initIndex >= 0 {
		if err := s.window.Observe(llhls.Event{
			Kind:         llhls.Init,
			Presentation: s.presentation,
			Track:        s.track,
			Generation:   s.generation,
			Data:         buffers[initIndex],
		}); err != nil {
			return fmt.Errorf("publish CMAF init: %w", err)
		}
		s.initialized = true
	}

	mediaStart := initIndex + 1
	if initIndex < 0 {
		mediaStart = 0
	}
	if mediaStart >= len(buffers) {
		return nil
	}
	first := list.GetBufferAt(uint(mediaStart))
	if first.HasFlags(gst.BufferFlagHeader) && !first.HasFlags(gst.BufferFlagDeltaUnit) && s.hasParent {
		if err := s.completeParent(); err != nil {
			return err
		}
	}

	var fragment bytes.Buffer
	for i, data := range buffers {
		if i < mediaStart {
			continue
		}
		fragment.Write(data)
	}
	partStart := clockDuration(first.PresentationTimestamp())
	chunkDuration := clockDuration(first.Duration())
	if chunkDuration <= 0 {
		chunkDuration = s.partDuration
	}
	if !s.hasParent {
		s.parentStart = partStart
		s.hasParent = true
	}
	s.parentLength += chunkDuration
	s.parent.Write(fragment.Bytes())
	if err := s.window.Observe(llhls.Event{
		Kind:         llhls.Part,
		Presentation: s.presentation,
		Track:        s.track,
		Generation:   s.generation,
		MSN:          s.nextMSN,
		Part:         s.partIndex,
		Start:        partStart,
		Duration:     chunkDuration,
		Independent:  !first.HasFlags(gst.BufferFlagDeltaUnit),
		Data:         fragment.Bytes(),
	}); err != nil {
		return fmt.Errorf("publish CMAF part: %w", err)
	}
	s.partIndex++
	return nil
}

func (s *cmafTrackSink) completeParent() error {
	if !s.hasParent {
		return nil
	}
	if err := s.window.Observe(llhls.Event{
		Kind:         llhls.SegmentComplete,
		Presentation: s.presentation,
		Track:        s.track,
		Generation:   s.generation,
		MSN:          s.nextMSN,
		Start:        s.parentStart,
		Duration:     s.parentLength,
		Data:         s.parent.Bytes(),
	}); err != nil {
		return fmt.Errorf("publish CMAF segment: %w", err)
	}
	s.nextMSN++
	s.parent.Reset()
	s.parentStart = 0
	s.parentLength = 0
	s.partIndex = 0
	s.hasParent = false
	return nil
}

func isCMAFInit(data []byte) bool {
	return len(data) >= 8 && string(data[4:8]) == "ftyp"
}

func clockDuration(value gst.ClockTime) time.Duration {
	if d := value.AsDuration(); d != nil {
		return *d
	}
	return 0
}

func installCMAFSink(ctx context.Context, sink *app.Sink, state *cmafTrackSink) {
	sink.SetBufferListSupport(true)
	sink.SetCallbacks(&app.SinkCallbacks{NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
		sample := sink.PullSample()
		if sample == nil {
			return gst.FlowEOS
		}
		if n := state.samples.Add(1); n <= 3 {
			log.Log(ctx, "received CMAF buffer list", "presentation", state.presentation, "track", state.track, "sample", n)
		}
		if err := state.sample(sample); err != nil {
			log.Error(ctx, "LL-HLS CMAF output failed", "presentation", state.presentation, "track", state.track, "error", err)
			return gst.FlowError
		}
		return gst.FlowOK
	}})
}
