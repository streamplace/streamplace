package media

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/ingestframe"
	"stream.place/streamplace/pkg/llhls"
)

// llhlsIngestOutput is the boundary between an ingest pipeline and its LL-HLS
// destination. The in-process path observes directly into a Window; an isolated
// worker serializes the same events onto its reconnectable frame stream for main
// to observe.
type llhlsIngestOutput struct {
	presentation string
	session      uint64
	window       *llhls.Window
	publish      func(llhls.Event) error
	done         func()
}

func webRTCLLHLSAvailable(cli *config.CLI) bool {
	if cli == nil || !cli.LLHLS {
		return false
	}
	for _, name := range []string{"isofmp4mux", "opusdec", "fdkaacenc"} {
		if gst.Find(name) == nil {
			return false
		}
	}
	return true
}

type llhlsFrameWriter interface {
	LLInit(ingestframe.LLFrame) error
	LLPart(ingestframe.LLFrame) error
	LLSegmentComplete(ingestframe.LLFrame) error
	LLDiscontinuity(ingestframe.LLFrame) error
	LLSessionEnd(ingestframe.LLFrame) error
}

type workerLLHLSOutput struct {
	writer       llhlsFrameWriter
	mu           sync.Mutex
	tracks       map[string]uint64
	presentation string
	session      uint64
}

func newWorkerLLHLSOutput(writer llhlsFrameWriter, presentation string, session uint64) *workerLLHLSOutput {
	return &workerLLHLSOutput{
		writer:       writer,
		tracks:       make(map[string]uint64),
		presentation: presentation,
		session:      session,
	}
}

func (o *workerLLHLSOutput) publish(ev llhls.Event) error {
	frame := llhlsEventToFrame(ev)
	o.mu.Lock()
	if ev.Track != "" && ev.Kind == llhls.Init {
		o.tracks[ev.Track] = ev.Generation
	}
	o.mu.Unlock()

	switch ev.Kind {
	case llhls.Init:
		return o.writer.LLInit(frame)
	case llhls.Part:
		return o.writer.LLPart(frame)
	case llhls.SegmentComplete:
		return o.writer.LLSegmentComplete(frame)
	case llhls.Discontinuity:
		return o.writer.LLDiscontinuity(frame)
	case llhls.SessionEnd:
		return o.writer.LLSessionEnd(frame)
	default:
		return fmt.Errorf("unsupported LL-HLS event kind %d", ev.Kind)
	}
}

func (o *workerLLHLSOutput) done() {
	o.mu.Lock()
	track, generation := "video", o.tracks["video"]
	if generation == 0 {
		for candidate, candidateGeneration := range o.tracks {
			track, generation = candidate, candidateGeneration
			break
		}
	}
	presentation, session := o.presentation, o.session
	o.mu.Unlock()
	if generation == 0 {
		return
	}
	if err := o.publish(llhls.Event{
		Kind:         llhls.SessionEnd,
		Presentation: presentation,
		Session:      session,
		Track:        track,
		Generation:   generation,
	}); err != nil {
		return
	}
}

func llhlsEventToFrame(ev llhls.Event) ingestframe.LLFrame {
	frame := ingestframe.LLFrame{
		Presentation: ev.Presentation,
		Session:      ev.Session,
		Track:        ev.Track,
		Generation:   ev.Generation,
		Timescale:    ev.Timescale,
		MSN:          ev.MSN,
		Part:         ev.Part,
		Start:        durationToLLTicks(ev.Start, ev.Timescale),
		Duration:     durationToLLTicks(ev.Duration, ev.Timescale),
		Independent:  ev.Independent,
		FrameRate:    ev.FrameRate,
		Channels:     ev.AudioChannels,
		Data:         ev.Data,
	}
	if !ev.ProgramDateTime.IsZero() {
		frame.ProgramDateTimeUnixNano = ev.ProgramDateTime.UnixNano()
	}
	return frame
}

func llhlsFrameToEvent(kind ingestframe.Type, frame ingestframe.LLFrame) (llhls.Event, error) {
	var eventKind llhls.EventKind
	switch kind {
	case ingestframe.LLInit:
		eventKind = llhls.Init
	case ingestframe.LLPart:
		eventKind = llhls.Part
	case ingestframe.LLSegmentComplete:
		eventKind = llhls.SegmentComplete
	case ingestframe.LLDiscontinuity:
		eventKind = llhls.Discontinuity
	case ingestframe.LLSessionEnd:
		eventKind = llhls.SessionEnd
	default:
		return llhls.Event{}, fmt.Errorf("unsupported LL-HLS worker frame %s", kind)
	}
	start, err := durationFromLLTicks(frame.Start, frame.Timescale)
	if err != nil {
		return llhls.Event{}, fmt.Errorf("invalid LL-HLS worker start: %w", err)
	}
	duration, err := durationFromLLTicks(frame.Duration, frame.Timescale)
	if err != nil {
		return llhls.Event{}, fmt.Errorf("invalid LL-HLS worker duration: %w", err)
	}
	if frame.Presentation == "" || frame.Track == "" || frame.Generation == 0 {
		return llhls.Event{}, fmt.Errorf("invalid LL-HLS worker frame: missing presentation, track, or generation")
	}
	event := llhls.Event{
		Kind:          eventKind,
		Presentation:  frame.Presentation,
		Session:       frame.Session,
		Track:         frame.Track,
		Generation:    frame.Generation,
		Timescale:     frame.Timescale,
		MSN:           frame.MSN,
		Part:          frame.Part,
		Start:         start,
		Duration:      duration,
		Independent:   frame.Independent,
		FrameRate:     frame.FrameRate,
		AudioChannels: frame.Channels,
		Data:          frame.Data,
	}
	if frame.ProgramDateTimeUnixNano != 0 {
		event.ProgramDateTime = time.Unix(0, frame.ProgramDateTimeUnixNano)
	}
	return event, nil
}

func durationToLLTicks(value time.Duration, timescale uint32) uint64 {
	if value <= 0 {
		return 0
	}
	if timescale == 0 {
		return uint64(value)
	}
	return uint64(value) * uint64(timescale) / uint64(time.Second)
}

func durationFromLLTicks(value uint64, timescale uint32) (time.Duration, error) {
	if value == 0 {
		return 0, nil
	}
	if timescale == 0 {
		if value > maxDurationNanos {
			return 0, fmt.Errorf("tick value %d exceeds duration range", value)
		}
		return time.Duration(value), nil
	}
	scale := uint64(timescale)
	wholeSeconds := value / scale
	if wholeSeconds > maxDurationNanos/uint64(time.Second) {
		return 0, fmt.Errorf("tick value %d exceeds duration range", value)
	}
	nanos := wholeSeconds * uint64(time.Second)
	fractionalNanos := (value % scale) * uint64(time.Second) / scale
	if nanos > maxDurationNanos-fractionalNanos {
		return 0, fmt.Errorf("tick value %d exceeds duration range", value)
	}
	return time.Duration(nanos + fractionalNanos), nil
}

const maxDurationNanos = uint64(1<<63 - 1)

func (mm *MediaManager) observeWorkerLLHLSFrame(streamer string, kind ingestframe.Type, payload []byte) error {
	frame, err := ingestframe.DecodeLLFrame(payload)
	if err != nil {
		return err
	}
	event, err := llhlsFrameToEvent(kind, frame)
	if err != nil {
		return err
	}
	window := mm.GetLLWindow(streamer)
	if window == nil {
		if event.Kind != llhls.Init {
			return fmt.Errorf("LL-HLS worker event arrived before init")
		}
		window = mm.replaceLLWindow(streamer)
	}
	if event.FrameRate > 0 {
		window.SetVideoFrameRate(event.FrameRate)
	}
	if event.AudioChannels > 0 {
		window.SetAudioConfig(llhls.AudioConfig{Channels: event.AudioChannels})
	}
	if err := window.Observe(event); err != nil {
		return err
	}
	if event.Kind == llhls.SessionEnd {
		mm.removeLLWindow(streamer, event.Presentation, window)
	}
	return nil
}
