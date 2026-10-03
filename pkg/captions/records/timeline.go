package records

import (
	"context"
	"time"

	"stream.place/streamplace/pkg/statedb"
)

// Span is one stretch of wall-clock time that went into a video: the time a
// recording object of a livestream was being written. A live-derived VOD is
// the recording objects of its livestream(s), concatenated in order.
type Span struct {
	Start, End time.Time
}

// Recording says which stretches of a livestream's wall-clock time ended up in
// a VOD of it.
type Recording interface {
	// Spans lists the recording objects of the given livestream records, in
	// the order they were recorded and concatenated into the VOD.
	Spans(ctx context.Context, livestreamURIs []string) ([]Span, error)
}

// StatedbRecording is the Recording of the S3 objects this node recorded: the
// ones live-to-VOD finalize concatenates.
type StatedbRecording struct {
	State *statedb.StatefulDB
}

func (r StatedbRecording) Spans(ctx context.Context, livestreamURIs []string) ([]Span, error) {
	segs, err := r.State.ListS3SegmentsForLivestreams(ctx, livestreamURIs)
	if err != nil {
		return nil, err
	}
	spans := make([]Span, 0, len(segs))
	for _, s := range segs {
		if s.CompletedAt == nil {
			continue
		}
		spans = append(spans, Span{Start: s.StartedAt, End: *s.CompletedAt})
	}
	return spans, nil
}

// timeline places instants of a livestream's wall clock on the timeline of a
// VOD recorded from it. The VOD is the spans back to back, so time between
// spans (a disconnect, a pause in recording) is not on it.
type timeline struct {
	spans    []Span
	vodStart []time.Duration // where each span begins on the VOD
}

// newTimeline builds the mapping for spans in recording order. The first span
// is pinned to anchor, the start time of the live session's first segment,
// which is what the first VOD frame is: a recording object's own start time is
// when this node began writing it, a little after that. Every later span is
// shifted by the same difference, so the clock skew between segment start
// times and this node's cancels out.
func newTimeline(spans []Span, anchor time.Time) *timeline {
	t := &timeline{spans: make([]Span, len(spans)), vodStart: make([]time.Duration, len(spans))}
	bias := anchor.Sub(spans[0].Start)
	var total time.Duration
	for i, s := range spans {
		t.spans[i] = Span{Start: s.Start.Add(bias), End: s.End.Add(bias)}
		t.vodStart[i] = total
		total += max(s.End.Sub(s.Start), 0)
	}
	return t
}

// offset returns where at falls on the VOD. An instant in a gap between spans
// lands on the start of the next span; one before the first span comes out
// negative, and the caller decides what to do with it.
func (t *timeline) offset(at time.Time) time.Duration {
	for i, s := range t.spans {
		if at.Before(s.Start) {
			if i == 0 {
				return at.Sub(s.Start)
			}
			return t.vodStart[i]
		}
		if at.Before(s.End) {
			return t.vodStart[i] + at.Sub(s.Start)
		}
	}
	last := len(t.spans) - 1
	return t.vodStart[last] + at.Sub(t.spans[last].Start)
}
