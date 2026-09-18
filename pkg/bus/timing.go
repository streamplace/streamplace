package bus

import "time"

// SegmentTiming is runtime-only correlation metadata for one logical media
// segment. It is deliberately separate from the signed segment metadata so
// operational timestamps never affect segment identity or signatures.
type SegmentTiming struct {
	SegmentID              string
	Ingress                string
	SourceStart            time.Time
	IngestReceived         time.Time
	Signed                 time.Time
	MasteringCompleted     time.Time
	Distributed            time.Time
	PacketizeStarted       time.Time
	PacketizeCompleted     time.Time
	WebRTCQueued           time.Time
	WebRTCFirstSent        time.Time
	WebRTCSegmentFirstSent time.Time
	WebRTCSegmentCompleted time.Time
}

// Clone returns an independent timing record for a downstream consumer.
// Segment notifications can fan out to more than one stream session, so
// downstream stages must not mutate a timing record shared by those sessions.
func (t *SegmentTiming) Clone() *SegmentTiming {
	if t == nil {
		return nil
	}
	clone := *t
	return &clone
}

// SourceAge returns the wall-clock age of the segment at now. Missing or
// future source timestamps are treated as unavailable rather than producing a
// misleading negative age.
func (t *SegmentTiming) SourceAge(now time.Time) time.Duration {
	if t == nil || t.SourceStart.IsZero() || now.Before(t.SourceStart) {
		return 0
	}
	return now.Sub(t.SourceStart)
}
