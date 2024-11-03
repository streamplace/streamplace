package media

import (
	"bytes"
	"context"
	"fmt"

	"aquareum.tv/aquareum/pkg/log"
)

type Segment struct {
	Path      string
	Buf       *bytes.Buffer
	StartTime *uint64
	EndTime   *uint64
	Closed    bool
}

type M3U8 struct {
	curSeg          int
	segments        []*Segment
	pendingSegments []*Segment
}

func NewM3U8() *M3U8 {
	return &M3U8{
		curSeg: 0,
	}
}

func (m *M3U8) GetNextSegment(ctx context.Context) (*Segment, error) {
	log.Warn(ctx, "next segment")
	ret := fmt.Sprintf("segment%05d.ts", m.curSeg)
	m.curSeg += 1
	seg := &Segment{
		Path: ret,
		Buf:  &bytes.Buffer{},
	}
	m.pendingSegments = append(m.pendingSegments, seg)
	return seg, nil
}

func (m *M3U8) CloseSegment(ctx context.Context, seg *Segment) {
	log.Warn(ctx, "close segment", "path", seg.Path)
	seg.Closed = true
	m.checkSegments(ctx)
}

func (m *M3U8) FragmentOpened(ctx context.Context, t uint64) error {
	log.Warn(ctx, "fragment opened", "time", t)
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
	log.Warn(ctx, "fragment closed", "time", t)
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
		log.Warn(ctx, "finalizing segment", "path", pending.Path)
	}
}

func (m *M3U8) GetPlaylist() []byte {
	return []byte{}
}
