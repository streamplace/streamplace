package livehls

import "time"

// VODEpoch is the Epoch of a VOD subtitle window: cue offsets are video
// offsets, so segment times are offsets from the Unix epoch's instant.
var VODEpoch = time.Unix(0, 0).UTC()

// VODSubtitleWindow builds the subtitle playlist window of a video, aligned to
// the segments [first, last] of its reference track (the same segments the
// video's media playlist lists).
//
// durations holds every segment's duration in ticks of timescale, so the
// segment's offset on the video timeline is the sum of those before it, and
// discontinuity marks the segments that start a new timeline (a recording
// that joined several ingest sessions, each restarting its tfdt near zero).
// clipStart is subtracted from video offsets, for a clip whose own timeline
// starts there; discSeq is the number of discontinuities cut off the front.
// tfdt reads the baseMediaDecodeTime of segment i's first sample; it is asked
// for the first segment and each discontinuity only, since the segments
// between those advance with the video clock. When it cannot tell, the
// timeline is taken to start at media time zero.
//
// Cue offsets are video (clip) offsets, so a cue has the same timestamps in
// every segment document it overlaps. Segment times are offsets from
// VODEpoch, and each segment's MPEGTS maps its first sample's tfdt to its
// offset exactly as the live rendition does (see TimestampMapMPEGTS).
// Sequence numbers count from zero at the first segment, as the video
// playlist's do.
func VODSubtitleWindow(timescale uint32, durations []uint64, discontinuity []bool, first, last int, clipStart time.Duration, discSeq int, tfdt func(i int) (uint64, bool)) SubtitleWindow {
	win := SubtitleWindow{Epoch: VODEpoch, Ended: true, DiscontinuitySequence: discSeq}
	if first < 0 || last >= len(durations) || first > last {
		return win
	}
	var cursor uint64
	for _, d := range durations[:first] {
		cursor += d
	}
	var mpegts uint64
	for i := first; i <= last; i++ {
		start := ticksDuration(cursor, timescale) - clipStart
		dur := ticksDuration(durations[i], timescale)
		disc := i < len(discontinuity) && discontinuity[i]
		if i == first || disc {
			v, ok := tfdt(i)
			if !ok {
				v = cursor
			}
			mpegts = TimestampMapMPEGTS(v, timescale, VODEpoch.Add(start), VODEpoch)
		}
		win.Segments = append(win.Segments, SubtitleSegment{
			Seq:           uint64(i - first),
			Start:         VODEpoch.Add(start),
			End:           VODEpoch.Add(start + dur),
			Duration:      dur,
			MPEGTS:        mpegts,
			Discontinuity: disc,
		})
		cursor += durations[i]
	}
	return win
}
