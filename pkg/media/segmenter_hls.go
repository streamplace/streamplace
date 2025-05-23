package media

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/log"
)

func (mm *MediaManager) ToHLS(ctx context.Context, user string, rendition string, m3u8 *M3U8) error {
	ctx = log.WithLogValues(ctx, "GStreamerFunc", "ToHLS", "rendition", rendition)

	pipelineSlice := []string{
		"h264parse name=videoparse",
		"opusdec use-inband-fec=true name=audioparse ! audioresample ! audiorate ! fdkaacenc name=audioenc",
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join(pipelineSlice, "\n"))
	if err != nil {
		return fmt.Errorf("error creating ToHLS pipeline: %w", err)
	}

	outputQueue, done, err := ConcatStream(ctx, pipeline, user, rendition, mm)
	if err != nil {
		return fmt.Errorf("failed to get output queue: %w", err)
	}

	videoParse, err := pipeline.GetElementByName("videoparse")
	if err != nil {
		return fmt.Errorf("failed to get video sink element from pipeline: %w", err)
	}
	err = outputQueue.Link(videoParse)
	if err != nil {
		return fmt.Errorf("failed to link output queue to video parse: %w", err)
	}

	audioParse, err := pipeline.GetElementByName("audioparse")
	if err != nil {
		return fmt.Errorf("failed to get audio parse element from pipeline: %w", err)
	}
	err = outputQueue.Link(audioParse)
	if err != nil {
		return fmt.Errorf("failed to link output queue to audio parse: %w", err)
	}

	splitmuxsink, err := gst.NewElementWithProperties("splitmuxsink", map[string]any{
		"name":           "mux",
		"async-finalize": true,
		"sink-factory":   "appsink",
		"muxer-factory":  "mpegtsmux",
		"max-size-bytes": 1,
	})
	if err != nil {
		return err
	}

	r := m3u8.GetRendition(rendition)
	defer func() { r = nil }()
	ps := NewPendingSegments(r)
	defer func() { ps = nil }()

	p := splitmuxsink.GetRequestPad("video")
	if p == nil {
		return fmt.Errorf("failed to get video pad")
	}
	p = splitmuxsink.GetRequestPad("audio_%u")
	if p == nil {
		return fmt.Errorf("failed to get audio pad")
	}

	err = pipeline.Add(splitmuxsink)
	if err != nil {
		return fmt.Errorf("error adding splitmuxsink to ToHLS pipeline: %w", err)
	}

	videoparse, err := pipeline.GetElementByName("videoparse")
	if err != nil {
		return fmt.Errorf("error getting videoparse from ToHLS pipeline: %w", err)
	}
	err = videoparse.Link(splitmuxsink)
	if err != nil {
		return fmt.Errorf("error linking videoparse to splitmuxsink: %w", err)
	}

	audioenc, err := pipeline.GetElementByName("audioenc")
	if err != nil {
		return fmt.Errorf("error getting audioenc from ToHLS pipeline: %w", err)
	}
	err = audioenc.Link(splitmuxsink)
	if err != nil {
		return fmt.Errorf("error linking audioenc to splitmuxsink: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-done:
			cancel()
		}
	}()

	_, err = splitmuxsink.Connect("sink-added", func(split, sinkEle *gst.Element) {
		log.Debug(ctx, "hls-check sink-added")
		vf, err := ps.GetNextSegment(ctx)
		if err != nil {
			panic(err)
		}
		appsink := app.SinkFromElement(sinkEle)
		appsink.SetCallbacks(&app.SinkCallbacks{
			NewSampleFunc: WriterNewSample(ctx, vf.Buf),
			EOSFunc: func(sink *app.Sink) {
				log.Debug(ctx, "hls-check Segment EOS", "buf", vf.Buf.Len())
				ps.CloseSegment(ctx, vf)
			},
		})
	})
	if err != nil {
		return fmt.Errorf("failed to add hls-check to sink: %w", err)
	}

	onPadAdded := func(element *gst.Element, pad *gst.Pad) {
		caps := pad.GetCurrentCaps()
		if caps == nil {
			fmt.Println("Unable to get pad caps")
			return
		}

		log.Debug(ctx, "New pad added", "pad", pad.GetName(), "caps", caps.String())

		structure := caps.GetStructureAt(0)
		if structure == nil {
			fmt.Println("Unable to get structure from caps")
			return
		}

		name := structure.Name()
		fmt.Printf("Structure Name: %s\n", name)
	}

	_, err = splitmuxsink.Connect("pad-added", onPadAdded)
	if err != nil {
		return fmt.Errorf("failed to add pad: %w", err)
	}

	defer cancel()
	go func() {
		err := HandleBusMessagesCustom(ctx, pipeline, func(msg *gst.Message) {
			switch msg.Type() {
			case gst.MessageElement:
				structure := msg.GetStructure()
				name := structure.Name()
				if name == "splitmuxsink-fragment-opened" {
					runningTime, err := structure.GetValue("running-time")
					if err != nil {
						log.Debug(ctx, "splitmuxsink-fragment-opened error", "error", err)
						cancel()
					}
					runningTimeInt, ok := runningTime.(uint64)
					if !ok {
						log.Warn(ctx, "splitmuxsink-fragment-opened not a uint64")
						cancel()
					}
					log.Debug(ctx, "hls-check splitmuxsink-fragment-opened", "runningTime", runningTimeInt)
					if err := ps.FragmentOpened(ctx, runningTimeInt); err != nil {
						log.Debug(ctx, "fragment open error", "error", err)
						cancel()
					}
				}
				if name == "splitmuxsink-fragment-closed" {
					runningTime, err := structure.GetValue("running-time")
					if err != nil {
						log.Debug(ctx, "splitmuxsink-fragment-closed error", "error", err)
						cancel()
					}
					runningTimeInt, ok := runningTime.(uint64)
					if !ok {
						log.Warn(ctx, "splitmuxsink-fragment-closed not a uint64")
						cancel()
					}
					log.Debug(ctx, "hls-check splitmuxsink-fragment-closed", "runningTime", runningTimeInt)
					if err := ps.FragmentClosed(ctx, runningTimeInt); err != nil {
						log.Debug(ctx, "fragment close error", "error", err)
						cancel()
					}
				}
			}
		})
		if err != nil {
			log.Log(ctx, "pipeline error", "error", err)
		}
		cancel()
	}()

	// Start the pipeline
	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		return fmt.Errorf("error setting pipeline state: %w", err)
	}

	<-ctx.Done()

	if err := pipeline.BlockSetState(gst.StateNull); err != nil {
		return fmt.Errorf("error setting pipeline state: %w", err)
	}

	return nil
}

type PendingSegments struct {
	segments  []*Segment
	lock      sync.Mutex
	rendition *M3U8Rendition
}

func NewPendingSegments(rendition *M3U8Rendition) *PendingSegments {
	return &PendingSegments{
		segments:  []*Segment{},
		lock:      sync.Mutex{},
		rendition: rendition,
	}
}

func (ps *PendingSegments) GetNextSegment(ctx context.Context) (*Segment, error) {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	log.Debug(ctx, "next segment")
	seg := &Segment{
		Buf:    &bytes.Buffer{},
		Time:   time.Now(),
		Closed: false,
	}
	ps.segments = append(ps.segments, seg)
	return seg, nil
}

func (ps *PendingSegments) CloseSegment(ctx context.Context, seg *Segment) {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	log.Debug(ctx, "close segment", "MSN", seg.MSN)
	seg.Closed = true
	if err := ps.checkSegments(ctx); err != nil {
		log.Debug(ctx, "faile to check segments segment")
	}
}

func (ps *PendingSegments) FragmentOpened(ctx context.Context, t uint64) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	log.Debug(ctx, "fragment opened", "time", t)
	if len(ps.segments) == 0 {
		return fmt.Errorf("no pending segments")
	}
	for _, seg := range ps.segments {
		if seg.StartTS == nil {
			seg.StartTS = &t
			break
		}
	}
	if err := ps.checkSegments(ctx); err != nil {
		return fmt.Errorf("failed to check segments: %w", err)
	}
	return nil
}

func (ps *PendingSegments) FragmentClosed(ctx context.Context, t uint64) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	log.Debug(ctx, "fragment closed", "time", t)
	if len(ps.segments) == 0 {
		return fmt.Errorf("no pending segments")
	}
	for _, seg := range ps.segments {
		if seg.EndTS == nil {
			seg.EndTS = &t
			dur := *seg.EndTS - *seg.StartTS
			seg.Duration = time.Duration(dur)
			break
		}
	}
	if err := ps.checkSegments(ctx); err != nil {
		return fmt.Errorf("failed to check segments: %w", err)
	}
	return nil
}

// the tricky piece of the design here is that we need to expect GetNextSegment,
// CloseSegment, FragmentOpened, and FragmentClosed to be called in any order. So
// all of those functions call this one, and it checks if we have the necessary information
// to finalize a segment and add it to our playlist.
// only call if you're holding ps.lock!
func (ps *PendingSegments) checkSegments(ctx context.Context) error {
	pending := ps.segments[0]
	if pending.StartTS != nil && pending.EndTS != nil && pending.Closed {
		if err := ps.rendition.NewSegment(pending); err != nil {
			return fmt.Errorf("failed to add new segment: %w", err)
		}
		log.Debug(ctx, "finalizing segment", "MSN", pending.MSN)
		ps.segments = ps.segments[1:]
	}
	return nil
}
