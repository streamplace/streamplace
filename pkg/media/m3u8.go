package media

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/renditions"
	"stream.place/streamplace/pkg/streamplace"
)

// how many segments are served in the live playlist?
const LIVE_PLAYLIST_SIZE = 8

// how long should we keep old segments around?
const RETAIN_SEGMENT_SIZE = LIVE_PLAYLIST_SIZE * 3

const INDEX_M3U8 = "index.m3u8"
const STREAM_M3U8 = "stream.m3u8"

type Segment struct {
	MSN      uint64 // media sequence number
	Duration time.Duration
	Buf      *bytes.Buffer
	Time     time.Time
}

type M3U8 struct {
	curSeg          uint64
	pendingSegments []*Segment
	waits           []chan struct{}
	renditions      []*M3U8Rendition
}

type M3U8Rendition struct {
	Rendition   renditions.Rendition
	Segments    []*Segment
	SegmentLock sync.RWMutex
	MSN         uint64
}

func NewM3U8(renditions renditions.Renditions) *M3U8 {
	rends := []*M3U8Rendition{}
	for _, r := range renditions {
		rends = append(rends, &M3U8Rendition{
			Rendition: r,
		})
	}
	return &M3U8{
		curSeg:     0,
		renditions: rends,
	}
}

// func (m *M3U8) GetNextSegment(ctx context.Context) (*Segment, error) {
// 	log.Debug(ctx, "next segment")
// 	msn := m.curSeg
// 	m.curSeg += 1
// 	seg := &Segment{
// 		MSN: msn,
// 		Buf: &bytes.Buffer{},
// 	}
// 	m.pendingSegments = append(m.pendingSegments, seg)
// 	return seg, nil
// }

// func (m *M3U8) CloseSegment(ctx context.Context, seg *Segment) {
// 	log.Debug(ctx, "close segment", "MSN", seg.MSN)
// 	seg.Closed = true
// 	m.checkSegments(ctx)
// }

// func (m *M3U8) FragmentOpened(ctx context.Context, t uint64) error {
// 	log.Debug(ctx, "fragment opened", "time", t)
// 	if len(m.pendingSegments) == 0 {
// 		return fmt.Errorf("no pending segments")
// 	}
// 	for _, seg := range m.pendingSegments {
// 		if seg.StartTime == nil {
// 			seg.StartTime = &t
// 			break
// 		}
// 	}
// 	m.checkSegments(ctx)
// 	return nil
// }

// func (m *M3U8) FragmentClosed(ctx context.Context, t uint64) error {
// 	log.Debug(ctx, "fragment closed", "time", t)
// 	if len(m.pendingSegments) == 0 {
// 		return fmt.Errorf("no pending segments")
// 	}
// 	for _, seg := range m.pendingSegments {
// 		if seg.EndTime == nil {
// 			seg.EndTime = &t
// 			if m.Bitrate == 0 {
// 				dur := seg.Duration()
// 				m.Bitrate = uint64(float64(seg.Buf.Len())/dur.Seconds()) * 8
// 			}
// 			break
// 		}
// 	}
// 	m.checkSegments(ctx)
// 	return nil
// }

// the tricky piece of the design here is that we need to expect GetNextSegment,
// CloseSegment, FragmentOpened, and FragmentClosed to be called in any order. So
// all of those functions call this one, and it checks if we have the necessary information
// to finalize a segment and add it to our playlist.
// func (m *M3U8) checkSegments(ctx context.Context) {
// 	pending := m.pendingSegments[0]
// 	if pending.StartTime != nil && pending.EndTime != nil && pending.Closed {
// 		m.segments = append(m.segments, pending)
// 		m.pendingSegments = m.pendingSegments[1:]
// 		log.Debug(ctx, "finalizing segment", "MSN", pending.MSN)
// 		for _, wait := range m.waits {
// 			go func(wait chan struct{}) {
// 				wait <- struct{}{}
// 			}(wait)
// 		}
// 		m.waits = []chan struct{}{}
// 	}
// 	if len(m.segments) > RETAIN_SEGMENT_SIZE {
// 		startWith := len(m.segments) - RETAIN_SEGMENT_SIZE
// 		m.segments = m.segments[startWith:]
// 	}
// }

// func (m *M3U8) waitForStart() {
// 	if len(m.segments) == 0 {
// 		// todo: fix concurrent access here
// 		wait := make(chan struct{})
// 		m.waits = append(m.waits, wait)
// 		<-wait
// 	}
// }

func (r *M3U8Rendition) GetMediaLine(session string) string {
	// m.waitForStart()
	lines := []string{}
	lines = append(lines, "#EXTM3U")
	lines = append(lines, fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d", r.Rendition.Bitrate, r.Rendition.Width, r.Rendition.Height))
	lines = append(lines, fmt.Sprintf("%s/stream.m3u8?session=%s", r.Rendition.Name, session))
	return strings.Join(lines, "\n")
}

func (r *M3U8Rendition) GetPlaylist(session string) []byte {
	r.SegmentLock.RLock()
	defer r.SegmentLock.RUnlock()
	// m.waitForStart()
	lines := []string{}
	lines = append(lines, "#EXTM3U")
	lines = append(lines, "#EXT-X-VERSION:3")
	startWith := len(r.Segments) - LIVE_PLAYLIST_SIZE
	if startWith < 0 {
		startWith = 0
	}
	firstSeg := r.Segments[startWith]
	lastSeg := r.Segments[len(r.Segments)-1]
	targetDuration := int64(math.Round(lastSeg.Duration.Seconds()))
	lines = append(lines, fmt.Sprintf("#EXT-X-MEDIA-SEQUENCE:%d", firstSeg.MSN))
	lines = append(lines, fmt.Sprintf("#EXT-X-DISCONTINUITY-SEQUENCE:%d", firstSeg.MSN))
	lines = append(lines, fmt.Sprintf("#EXT-X-TARGETDURATION:%d", targetDuration+1))
	lines = append(lines, "#EXT-X-INDEPENDENT-SEGMENTS")
	lines = append(lines, "")
	lastSegments := r.Segments[startWith:]
	for _, seg := range lastSegments {
		dur := seg.Duration
		lines = append(lines, "#EXT-X-DISCONTINUITY")
		lines = append(lines, fmt.Sprintf("#EXT-X-PROGRAM-DATE-TIME:%s", seg.Time.Format(time.RFC3339)))
		lines = append(lines, fmt.Sprintf("#EXTINF:%f,", dur.Seconds()))
		lines = append(lines, fmt.Sprintf("segment%05d.ts?session=%s", seg.MSN, session))
	}
	lines = append(lines, "")
	return []byte(strings.Join(lines, "\n"))
}

func (r *M3U8Rendition) GetSegment(session string, filename string) []byte {
	r.SegmentLock.RLock()
	defer r.SegmentLock.RUnlock()
	for _, seg := range r.Segments {
		if fmt.Sprintf("segment%05d.ts", seg.MSN) == filename {
			return seg.Buf.Bytes()
		}
	}
	return nil
}

func (m *M3U8) GetMultivariantPlaylist() []byte {
	uu, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	// m.waitForStart()
	lines := []string{}
	lines = append(lines, "#EXTM3U")
	for _, r := range m.renditions {
		lines = append(lines, r.GetMediaLine(uu.String()))
	}
	return []byte(strings.Join(lines, "\n"))
}

// needs to handle:
// - index.m3u8
// - 720p/stream.m3u8
// - 720p/segment00015.ts
func (m *M3U8) GetFile(str string, session string) ([]byte, error) {
	str = strings.TrimPrefix(str, "/")
	if str == INDEX_M3U8 {
		return m.GetMultivariantPlaylist(), nil
	}
	parts := strings.Split(str, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid path")
	}
	rStr := parts[0]
	fStr := parts[1]
	rend := m.GetRendition(rStr)
	log.Warn(context.Background(), "get file", "str", str, "session", session, "rend", rStr, "file", fStr)
	if rend == nil {
		return nil, fmt.Errorf("rendition not found")
	}
	if fStr == STREAM_M3U8 {
		return rend.GetPlaylist(session), nil
	}
	seg := rend.GetSegment(session, fStr)
	if seg == nil {
		return nil, fmt.Errorf("segment not found")
	}
	return seg, nil
}

func (m *M3U8) NewSegment(seg *streamplace.Segment, rendition string, data []byte) error {
	m3u8Rendition := m.GetRendition(rendition)
	if m3u8Rendition == nil {
		return fmt.Errorf("rendition not found")
	}
	return m3u8Rendition.NewSegment(seg, data)
}

func (r *M3U8Rendition) NewSegment(seg *streamplace.Segment, data []byte) error {
	r.SegmentLock.Lock()
	defer r.SegmentLock.Unlock()
	t, err := time.Parse(time.RFC3339, seg.StartTime)
	if err != nil {
		return fmt.Errorf("invalid time")
	}
	r.Segments = append(r.Segments, &Segment{
		Buf:      bytes.NewBuffer(data),
		Duration: time.Duration(*seg.Duration),
		MSN:      r.MSN,
		Time:     t,
	})
	r.MSN += 1
	return nil
}

func (m *M3U8) GetRendition(rendition string) *M3U8Rendition {
	for _, r := range m.renditions {
		if r.Rendition.Name == rendition {
			return r
		}
	}
	return nil
}
