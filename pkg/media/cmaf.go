package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/llhls"
	"stream.place/streamplace/pkg/log"
)

var errUnsupportedCMAFPartLayout = errors.New("unsupported CMAF part layout")

const (
	llhlsAudioParentMinimum   = llhlsParentDuration - 100*time.Millisecond
	llhlsAudioParentTolerance = 50 * time.Millisecond
)

// cmafTrackSink translates the buffer-list contract of the GStreamer fMP4
// muxer into application-level LL-HLS events. The first list also carries the
// initialization segment.
type cmafTrackSink struct {
	ctx           context.Context
	presentation  string
	track         string
	window        *llhls.Window
	generation    uint64
	nextMSN       uint64
	initialized   bool
	partDuration  time.Duration
	parent        bytes.Buffer
	parentStart   time.Duration
	parentLength  time.Duration
	timelineEnd   time.Duration
	partIndex     uint32
	lastTiming    map[uint32]cmafFragmentTiming
	videoTrackIDs map[uint32]bool
	hasParent     bool
	partTarget    time.Duration
	pendingPart   cmafPendingPart
	samples       atomic.Uint64
	// audioOnly marks a track whose samples are all independently decodable
	// (AAC) and carry no video, so independence inspection is skipped.
	audioOnly bool
	// programDateTimeBase anchors the playlist PDT grid. All sinks of one
	// presentation share the same base so rendition timelines map to the same
	// wall clock; per-parent offsets come from the fragment decode timeline.
	programDateTimeBase time.Time
	timescale           uint32
}

type cmafPendingPart struct {
	data            []byte
	start           time.Duration
	duration        time.Duration
	independent     bool
	hasVideo        bool
	programDateTime time.Time
	set             bool
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
		if s.track == "video" {
			videoTrackIDs, err := cmafVideoTrackIDs(buffers[initIndex])
			if err != nil && s.ctx != nil {
				log.Error(s.ctx, "LL-HLS CMAF video track mapping failed", "presentation", s.presentation, "track", s.track, "error", err)
			} else if err == nil {
				s.videoTrackIDs = videoTrackIDs
			}
		}
		if s.track == "audio" {
			if channels, err := cmafAudioChannels(buffers[initIndex]); err != nil {
				if s.ctx != nil {
					log.Error(s.ctx, "LL-HLS CMAF audio channel mapping failed", "presentation", s.presentation, "track", s.track, "error", err)
				}
			} else {
				s.window.SetAudioConfig(llhls.AudioConfig{Channels: channels})
			}
		}
		if timescale, err := cmafTrackTimescale(buffers[initIndex]); err != nil {
			if s.ctx != nil {
				log.Error(s.ctx, "LL-HLS CMAF track timescale mapping failed", "presentation", s.presentation, "track", s.track, "error", err)
			}
		} else {
			s.timescale = timescale
		}
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
	chunkDuration := clockDuration(first.Duration())
	if chunkDuration <= 0 {
		chunkDuration = s.partDuration
	}
	// Audio-only muxers can emit a short fragment around a scheduled split.
	// Keep that fragment in the current parent when it is close enough to the
	// target, but close before accepting another full part that would make the
	// parent materially too long.
	if s.audioOnly && s.hasParent && s.parentLength >= llhlsAudioParentMinimum && s.parentLength+chunkDuration > llhlsParentDuration+llhlsAudioParentTolerance {
		if err := s.completeParent(); err != nil {
			return err
		}
	}
	if first.HasFlags(gst.BufferFlagHeader) && !first.HasFlags(gst.BufferFlagDeltaUnit) && s.hasParent && !s.audioOnly {
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
	fragmentTiming := cmafFragmentTiming{}
	if !s.hasParent && s.timescale > 0 {
		timings, err := inspectCMAFFragment(fragment.Bytes())
		if err != nil {
			if s.ctx != nil {
				log.Error(s.ctx, "LL-HLS CMAF fragment timing unavailable for program date time", "presentation", s.presentation, "track", s.track, "msn", s.nextMSN, "part", s.partIndex, "error", err)
			}
		} else if len(timings) == 1 {
			fragmentTiming = timings[0]
		}
	}
	partStart := s.timelineEnd
	programDateTime := time.Time{}
	if !s.hasParent {
		s.parentStart = partStart
		s.hasParent = true
		if s.programDateTimeBase.IsZero() {
			s.programDateTimeBase = time.Now().UTC()
		}
		programDateTime = s.fragmentProgramDateTime(fragmentTiming, partStart)
	}
	s.parentLength += chunkDuration
	s.timelineEnd += chunkDuration
	s.parent.Write(fragment.Bytes())
	s.inspectTiming(fragment.Bytes())
	independent := false
	hasVideo := false
	if s.audioOnly {
		// Every sample of the supported audio codecs decodes on its own.
		independent = true
	} else if len(s.videoTrackIDs) > 0 {
		var err error
		independent, err = inspectCMAFFragmentIndependence(fragment.Bytes(), s.videoTrackIDs)
		if err != nil && s.ctx != nil {
			log.Error(s.ctx, "LL-HLS CMAF independence inspection failed", "presentation", s.presentation, "track", s.track, "msn", s.nextMSN, "part", s.partIndex, "error", err)
			independent = false
		}
		hasVideo, err = inspectCMAFFragmentHasVideo(fragment.Bytes(), s.videoTrackIDs)
		if err != nil && s.ctx != nil {
			log.Error(s.ctx, "LL-HLS CMAF video-track inspection failed", "presentation", s.presentation, "track", s.track, "msn", s.nextMSN, "part", s.partIndex, "error", err)
			hasVideo = false
		}
	}
	if err := s.queuePart(cmafPendingPart{
		data:            append([]byte(nil), fragment.Bytes()...),
		start:           partStart,
		duration:        chunkDuration,
		independent:     independent,
		hasVideo:        hasVideo,
		programDateTime: programDateTime,
		set:             true,
	}); err != nil {
		return err
	}
	if s.audioOnly && s.parentLength >= llhlsParentDuration {
		if err := s.completeParent(); err != nil {
			return err
		}
	}
	return nil
}

func (s *cmafTrackSink) fragmentProgramDateTime(timing cmafFragmentTiming, fallback time.Duration) time.Time {
	if s.programDateTimeBase.IsZero() {
		s.programDateTimeBase = time.Now().UTC()
	}
	start := fallback
	if s.timescale > 0 {
		start = cmafDecodeTimeDuration(timing.DecodeTime, s.timescale)
	}
	return s.programDateTimeBase.Add(start)
}

// queuePart keeps the current part unpublished until the following muxer
// chunk proves whether it is terminal. Short audio-only prefixes may be
// merged with the following chunk, but the merged part must remain within the
// advertised PART-TARGET.
func (s *cmafTrackSink) queuePart(next cmafPendingPart) error {
	if !next.set || s.partDuration <= 0 || s.partTarget <= 0 {
		return s.publishPart(next)
	}
	if !s.pendingPart.set {
		if next.duration >= s.partTargetDuration()*85/100 {
			return s.publishPart(next)
		}
		s.pendingPart = next
		return nil
	}
	target := s.partTargetDuration()
	minimum := target * 85 / 100
	if s.pendingPart.duration >= minimum {
		if err := s.publishPart(s.pendingPart); err != nil {
			return err
		}
		s.pendingPart = next
		return nil
	}
	if s.pendingPart.duration+next.duration > target {
		return fmt.Errorf("%w: short CMAF parts cannot be coalesced within PART-TARGET: prefix=%s next=%s target=%s", errUnsupportedCMAFPartLayout, s.pendingPart.duration, next.duration, target)
	}
	if !s.pendingPart.hasVideo && next.hasVideo {
		s.pendingPart.independent = next.independent
	}
	s.pendingPart.data = append(s.pendingPart.data, next.data...)
	s.pendingPart.duration += next.duration
	s.pendingPart.hasVideo = s.pendingPart.hasVideo || next.hasVideo
	return nil
}

func (s *cmafTrackSink) partTargetDuration() time.Duration {
	if s.partTarget > 0 {
		return s.partTarget
	}
	return s.partDuration
}

func (s *cmafTrackSink) publishPart(part cmafPendingPart) error {
	if !part.set {
		return nil
	}
	if s.partTarget > 0 && part.duration > s.partTarget {
		return fmt.Errorf("CMAF part duration %s exceeds PART-TARGET %s", part.duration, s.partTarget)
	}
	if err := s.window.Observe(llhls.Event{
		Kind:            llhls.Part,
		Presentation:    s.presentation,
		Track:           s.track,
		Generation:      s.generation,
		MSN:             s.nextMSN,
		Part:            s.partIndex,
		Start:           part.start,
		Duration:        part.duration,
		Independent:     part.independent,
		ProgramDateTime: part.programDateTime,
		Data:            append([]byte(nil), part.data...),
	}); err != nil {
		return fmt.Errorf("publish CMAF part: %w", err)
	}
	s.partIndex++
	return nil
}

func (s *cmafTrackSink) inspectTiming(data []byte) {
	if s.ctx == nil {
		return
	}
	timings, err := inspectCMAFFragment(data)
	if err != nil {
		log.Error(s.ctx, "LL-HLS CMAF timing inspection failed", "presentation", s.presentation, "track", s.track, "msn", s.nextMSN, "part", s.partIndex, "error", err)
		return
	}
	if s.lastTiming == nil {
		s.lastTiming = make(map[uint32]cmafFragmentTiming)
	}
	for _, timing := range timings {
		if previous, ok := s.lastTiming[timing.TrackID]; ok {
			expected := previous.DecodeTime + previous.Duration
			if timing.DecodeTime != expected {
				log.Error(s.ctx, "LL-HLS CMAF decode timeline discontinuity", "presentation", s.presentation, "track", s.track, "track_id", timing.TrackID, "msn", s.nextMSN, "part", s.partIndex, "previous_decode_time", previous.DecodeTime, "previous_duration", previous.Duration, "expected_decode_time", expected, "actual_decode_time", timing.DecodeTime, "sample_count", timing.SampleCount, "duration", timing.Duration)
			}
		}
		log.Debug(s.ctx, "LL-HLS CMAF fragment timing", "presentation", s.presentation, "track", s.track, "track_id", timing.TrackID, "msn", s.nextMSN, "part", s.partIndex, "decode_time", timing.DecodeTime, "duration", timing.Duration, "sample_count", timing.SampleCount)
		s.lastTiming[timing.TrackID] = timing
	}
}

func (s *cmafTrackSink) completeParent() error {
	if !s.hasParent {
		return nil
	}
	if err := s.publishPart(s.pendingPart); err != nil {
		return err
	}
	s.pendingPart = cmafPendingPart{}
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
	state.ctx = ctx
	sink.SetBufferListSupport(true)
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
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
		},
		EOSFunc: func(*app.Sink) {
			if err := state.completeParent(); err != nil {
				log.Error(ctx, "LL-HLS CMAF EOS flush failed", "presentation", state.presentation, "track", state.track, "error", err)
			}
		},
	})
}
