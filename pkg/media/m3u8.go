package media

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"aquareum.tv/aquareum/pkg/log"
)

// how many segments are served in the live playlist?
const LIVE_PLAYLIST_SIZE = 8

// how long should we keep old segments around?
const RETAIN_SEGMENT_SIZE = LIVE_PLAYLIST_SIZE * 3

type Segment struct {
	MSN       uint64 // media sequence number
	Buf       *bytes.Buffer
	StartTime *uint64
	EndTime   *uint64
	Closed    bool
}

func (s *Segment) Duration() time.Duration {
	return time.Duration(*s.EndTime - *s.StartTime)
}

type M3U8 struct {
	curSeg          uint64
	segments        []*Segment
	pendingSegments []*Segment
	waits           []chan struct{}
}

func NewM3U8() *M3U8 {
	return &M3U8{
		curSeg: 0,
	}
}

func (m *M3U8) GetNextSegment(ctx context.Context) (*Segment, error) {
	log.Debug(ctx, "next segment")
	msn := m.curSeg
	m.curSeg += 1
	seg := &Segment{
		MSN: msn,
		Buf: &bytes.Buffer{},
	}
	m.pendingSegments = append(m.pendingSegments, seg)
	return seg, nil
}

func (m *M3U8) CloseSegment(ctx context.Context, seg *Segment) {
	log.Debug(ctx, "close segment", "MSN", seg.MSN)
	seg.Closed = true
	m.checkSegments(ctx)
}

func (m *M3U8) FragmentOpened(ctx context.Context, t uint64) error {
	log.Debug(ctx, "fragment opened", "time", t)
	if len(m.pendingSegments) == 0 {
		return fmt.Errorf("no pending segments")
	}
	for _, seg := range m.pendingSegments {
		if seg.StartTime == nil {
			seg.StartTime = &t
			break
		}
	}
	m.checkSegments(ctx)
	return nil
}

func (m *M3U8) FragmentClosed(ctx context.Context, t uint64) error {
	log.Debug(ctx, "fragment closed", "time", t)
	if len(m.pendingSegments) == 0 {
		return fmt.Errorf("no pending segments")
	}
	for _, seg := range m.pendingSegments {
		if seg.EndTime == nil {
			seg.EndTime = &t
			break
		}
	}
	m.checkSegments(ctx)
	return nil
}

// the tricky piece of the design here is that we need to expect GetNextSegment,
// CloseSegment, FragmentOpened, and FragmentClosed to be called in any order. So
// all of those functions call this one, and it checks if we have the necessary information
// to finalize a segment and add it to our playlist.
func (m *M3U8) checkSegments(ctx context.Context) {
	pending := m.pendingSegments[0]
	if pending.StartTime != nil && pending.EndTime != nil && pending.Closed {
		m.segments = append(m.segments, pending)
		m.pendingSegments = m.pendingSegments[1:]
		log.Debug(ctx, "finalizing segment", "MSN", pending.MSN)
		for _, wait := range m.waits {
			go func(wait chan struct{}) {
				wait <- struct{}{}
			}(wait)
		}
		m.waits = []chan struct{}{}
	}
	if len(m.segments) > RETAIN_SEGMENT_SIZE {
		startWith := len(m.segments) - RETAIN_SEGMENT_SIZE
		m.segments = m.segments[startWith:]
	}
}

func (m *M3U8) GetPlaylist() []byte {
	if len(m.segments) == 0 {
		// todo: fix concurrent access here
		wait := make(chan struct{})
		m.waits = append(m.waits, wait)
		<-wait
	}
	lines := []string{}
	lines = append(lines, "#EXTM3U")
	lines = append(lines, "#EXT-X-VERSION:3")
	startWith := len(m.segments) - LIVE_PLAYLIST_SIZE
	if startWith < 0 {
		startWith = 0
	}
	firstSeg := m.segments[startWith]
	lastSeg := m.segments[len(m.segments)-1]
	targetDuration := int64(math.Round(lastSeg.Duration().Seconds()))
	lines = append(lines, fmt.Sprintf("#EXT-X-MEDIA-SEQUENCE:%d", firstSeg.MSN))
	lines = append(lines, fmt.Sprintf("#EXT-X-TARGETDURATION:%d", targetDuration))
	lines = append(lines, "")
	lastSegments := m.segments[startWith:]
	for _, seg := range lastSegments {
		dur := seg.Duration()
		lines = append(lines, fmt.Sprintf("#EXTINF:%f,", dur.Seconds()))
		lines = append(lines, fmt.Sprintf("segment%05d.ts", seg.MSN))
	}
	lines = append(lines, "")
	return []byte(strings.Join(lines, "\n"))
}

// takes segment00015.ts and returns the corresponding segment
func (m *M3U8) GetSegment(str string) ([]byte, error) {
	if str == "stream.m3u8" {
		return m.GetPlaylist(), nil
	}
	for _, seg := range m.segments {
		if fmt.Sprintf("segment%05d.ts", seg.MSN) == str {
			return seg.Buf.Bytes(), nil
		}
	}
	return nil, fmt.Errorf("segment not found")
}

// #EXTM3U
// #EXT-X-VERSION:3
// #EXT-X-MEDIA-SEQUENCE:15
// #EXT-X-TARGETDURATION:1

// #EXTINF:1,
// segment00015.ts
// #EXTINF:1.0089999437332153,
// segment00016.ts
// #EXTINF:1,
// segment00017.ts
// #EXTINF:1.0060000419616699,
// segment00018.ts
// #EXTINF:1,
// segment00019.ts
