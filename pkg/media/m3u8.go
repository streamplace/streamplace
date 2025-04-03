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
)

// how many segments are served in the live playlist?
const LIVE_PLAYLIST_SIZE = 8

// how long should we keep old segments around?
const RETAIN_SEGMENT_SIZE = LIVE_PLAYLIST_SIZE * 3

const INDEX_M3U8 = "index.m3u8"

type Segment struct {
	MSN      uint64 // media sequence number
	Duration time.Duration
	Buf      *bytes.Buffer
	Time     time.Time
	Closed   bool
	StartTS  *uint64
	EndTS    *uint64
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
		mr := &M3U8Rendition{
			Rendition: r,
		}
		rends = append(rends, mr)
	}
	return &M3U8{
		curSeg:     0,
		renditions: rends,
	}
}

func (r *M3U8Rendition) GetMediaLine(session string) string {
	// m.waitForStart()
	lines := []string{}
	lines = append(lines, "#EXTM3U")
	lines = append(lines, fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d", r.Rendition.Bitrate, r.Rendition.Width, r.Rendition.Height))
	lines = append(lines, fmt.Sprintf("%s/%s?session=%s", r.Rendition.Name, INDEX_M3U8, session))
	return strings.Join(lines, "\n")
}

func (r *M3U8Rendition) GetPlaylist(session string) []byte {
	if session == "" {
		uu, err := uuid.NewV7()
		if err != nil {
			panic(err)
		}
		session = uu.String()
	}
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
	if len(r.Segments) == 0 {
		return []byte{}
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
		lines = append(lines, fmt.Sprintf("#EXT-X-PROGRAM-DATE-TIME:%s", seg.Time.Format(time.RFC3339Nano)))
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

func (m *M3U8) GetMultivariantPlaylist(rendition string) []byte {
	uu, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	// m.waitForStart()
	lines := []string{}
	lines = append(lines, "#EXTM3U")
	for _, r := range m.renditions {
		if rendition == "" || r.Rendition.Name == rendition {
			lines = append(lines, r.GetMediaLine(uu.String()))
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// needs to handle:
// - index.m3u8
// - 720p/stream.m3u8
// - 720p/segment00015.ts
func (m *M3U8) GetFile(str string, session string, rendition string) ([]byte, error) {
	str = strings.TrimPrefix(str, "/")
	if str == INDEX_M3U8 {
		return m.GetMultivariantPlaylist(rendition), nil
	}
	parts := strings.Split(str, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid path")
	}
	rStr := parts[0]
	fStr := parts[1]
	rend := m.GetRendition(rStr)
	log.Debug(context.Background(), "m3u8 get file", "str", str, "session", session, "rend", rStr, "file", fStr)
	if rend == nil {
		return nil, fmt.Errorf("rendition not found")
	}
	if fStr == INDEX_M3U8 {
		return rend.GetPlaylist(session), nil
	}
	seg := rend.GetSegment(session, fStr)
	if seg == nil {
		return nil, fmt.Errorf("segment not found")
	}
	return seg, nil
}

func (r *M3U8Rendition) NewSegment(seg *Segment) error {
	r.SegmentLock.Lock()
	defer r.SegmentLock.Unlock()
	seg.MSN = r.MSN
	r.MSN += 1
	r.Segments = append(r.Segments, seg)
	if len(r.Segments) > RETAIN_SEGMENT_SIZE {
		// Calculate how many segments to remove
		removeCount := len(r.Segments) - RETAIN_SEGMENT_SIZE
		// Remove the oldest segments (from the front of the slice)
		r.Segments = r.Segments[removeCount:]
	}
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
