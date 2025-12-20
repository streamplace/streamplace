package aigateway

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	// minCueDurationMS is the minimum duration for a VTT cue in milliseconds.
	minCueDurationMS = 200

	// defaultLastCueDurationMS is the default duration for the last cue when no next cue exists.
	defaultLastCueDurationMS = 2000

	// deltaFallbackWordCount is the number of trailing words to show when delta extraction fails.
	deltaFallbackWordCount = 12
)

// FormatVTTTime formats a millisecond timestamp as a VTT time string (HH:MM:SS.mmm).
func FormatVTTTime(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	millis := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, millis)
}

func eventTimeMS(e TranscriptEvent) int64 {
	if e.Timing != nil {
		if e.Timing.MediaWindowStartMS > 0 && e.Timing.MediaWindowEndMS > 0 {
			return (e.Timing.MediaWindowStartMS + e.Timing.MediaWindowEndMS) / 2
		}
		if e.Timing.MediaWindowEndMS > 0 {
			// For windowed ASR, placing the event at the middle of the window usually
			// aligns better than anchoring at the end.
			if e.Stats != nil && e.Stats.AudioDurationMS > 0 {
				return e.Timing.MediaWindowEndMS - int64(e.Stats.AudioDurationMS)/2
			}
			return e.Timing.MediaWindowEndMS
		}
		if e.Timing.MediaWindowStartMS > 0 {
			return e.Timing.MediaWindowStartMS
		}
	}
	return e.TimestampMS
}

func clampInt64(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// GenerateVTT creates a WebVTT subtitle file from transcript events.
// It normalizes timestamps relative to the first event and extracts delta text
// to avoid showing repeated content from rolling transcript snapshots.
func GenerateVTT(events []TranscriptEvent) []byte {
	if len(events) == 0 {
		return []byte("WEBVTT\n\n")
	}

	// Find the minimum timestamp to use as a base for relative timing
	baseMS := int64(math.MaxInt64)
	for _, e := range events {
		ts := eventTimeMS(e)
		if ts > 0 && ts < baseMS {
			baseMS = ts
		}
	}
	if baseMS == int64(math.MaxInt64) {
		baseMS = 0
	}

	var sb strings.Builder
	sb.WriteString("WEBVTT\n\n")

	prevFullText := ""
	for i, event := range events {
		fullText := strings.TrimSpace(event.Text)
		if fullText == "" {
			continue
		}

		// Extract delta text (new content since last event)
		deltaText := extractDeltaText(fullText, prevFullText)
		if strings.TrimSpace(deltaText) == "" {
			prevFullText = fullText
			continue
		}

		// Calculate cue timing
		startMS := eventTimeMS(event) - baseMS
		if startMS < 0 {
			startMS = 0
		}

		nextStartMS := int64(-1)
		if i+1 < len(events) {
			nextStartMS = eventTimeMS(events[i+1]) - baseMS
			if nextStartMS < 0 {
				nextStartMS = 0
			}
		}

		endMS := calculateCueEndTime(startMS, nextStartMS, event.Stats)

		sb.WriteString(fmt.Sprintf("%s --> %s\n", FormatVTTTime(startMS), FormatVTTTime(endMS)))
		sb.WriteString(deltaText)
		sb.WriteString("\n\n")

		prevFullText = fullText
	}

	return []byte(sb.String())
}


// GenerateVTTForSegment generates a WebVTT file suitable for use as an HLS subtitle
// segment. Cue times are relative to the segment start (i.e., within the segment
// duration window), and only transcript segments that overlap the segment time
// window are included.
func GenerateVTTForSegment(segs []TranscriptSegment, segmentStartMS, segmentEndMS int64) []byte {
	if len(segs) == 0 {
		return []byte("WEBVTT\n\n")
	}
	if segmentEndMS <= segmentStartMS {
		return []byte("WEBVTT\n\n")
	}
	segmentDurMS := segmentEndMS - segmentStartMS

	// Ensure stable ordering even if producers resend or arrive out of order.
	ordered := make([]TranscriptSegment, 0, len(segs))
	for _, s := range segs {
		if strings.TrimSpace(s.Text) == "" {
			continue
		}
		if s.EndMS <= s.StartMS {
			continue
		}
		ordered = append(ordered, s)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].StartMS == ordered[j].StartMS {
			return ordered[i].EndMS < ordered[j].EndMS
		}
		return ordered[i].StartMS < ordered[j].StartMS
	})

	var sb strings.Builder
	sb.WriteString("WEBVTT\n\n")

	for _, seg := range ordered {
		// Include if overlapping [segmentStartMS, segmentEndMS)
		if seg.EndMS <= segmentStartMS || seg.StartMS >= segmentEndMS {
			continue
		}

		startAbs := maxInt64(seg.StartMS, segmentStartMS)
		endAbs := minInt64(seg.EndMS, segmentEndMS)

		startMS := clampInt64(startAbs-segmentStartMS, 0, segmentDurMS)
		endMS := clampInt64(endAbs-segmentStartMS, 0, segmentDurMS)
		if endMS < startMS+minCueDurationMS {
			endMS = clampInt64(startMS+minCueDurationMS, 0, segmentDurMS)
		}
		if endMS <= startMS {
			continue
		}

		cueText := strings.TrimSpace(seg.Text)
		if cueText == "" {
			continue
		}

		sb.WriteString(fmt.Sprintf("%s --> %s\n", FormatVTTTime(startMS), FormatVTTTime(endMS)))
		sb.WriteString(cueText)
		sb.WriteString("\n\n")
	}

	return []byte(sb.String())
}

// extractDeltaText extracts new text from fullText that wasn't in prevFullText.
// If fullText is a prefix-extension of prevFullText, returns only the new suffix.
// Otherwise, falls back to the last sentence or last N words.
func extractDeltaText(fullText, prevFullText string) string {
	if prevFullText == "" {
		return fullText
	}

	// If current text extends previous text, return just the new part
	if strings.HasPrefix(fullText, prevFullText) {
		return strings.TrimSpace(fullText[len(prevFullText):])
	}

	// Fallback: try to get the last sentence
	trimmed := strings.TrimSpace(fullText)
	lastPunct := strings.LastIndexAny(trimmed, ".!?")
	if lastPunct >= 0 && lastPunct+1 < len(trimmed) {
		return strings.TrimSpace(trimmed[lastPunct+1:])
	}

	// Fallback: get the last N words
	words := strings.Fields(trimmed)
	if len(words) > deltaFallbackWordCount {
		return strings.Join(words[len(words)-deltaFallbackWordCount:], " ")
	}
	return trimmed
}

// calculateCueEndTime determines the end time for a VTT cue.
// It uses audio duration if available, otherwise the next cue's start time,
// with a fallback default duration for the last cue.
func calculateCueEndTime(startMS, nextStartMS int64, stats *Stats) int64 {
	var endMS int64

	if stats != nil && stats.AudioDurationMS > 0 {
		endMS = startMS + int64(stats.AudioDurationMS)
	} else if nextStartMS >= 0 {
		endMS = nextStartMS
	} else {
		endMS = startMS + defaultLastCueDurationMS
	}

	// Prevent overlap with next cue
	if nextStartMS >= 0 && endMS >= nextStartMS {
		endMS = nextStartMS - 1
	}

	// Enforce minimum duration
	if endMS < startMS+minCueDurationMS {
		endMS = startMS + minCueDurationMS
	}

	return endMS
}

// GenerateSubtitlesPlaylist creates an HLS playlist for subtitle segments.
// It generates segment entries for the specified count starting at mediaSequence.
func GenerateSubtitlesPlaylist(targetDuration int, mediaSequence int, segmentCount int) []byte {
	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")
	sb.WriteString("#EXT-X-VERSION:3\n")
	sb.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", targetDuration))
	sb.WriteString(fmt.Sprintf("#EXT-X-MEDIA-SEQUENCE:%d\n", mediaSequence))
	sb.WriteString("\n")

	for i := 0; i < segmentCount; i++ {
		sb.WriteString(fmt.Sprintf("#EXTINF:%d.000,\n", targetDuration))
		sb.WriteString(fmt.Sprintf("segment%05d.vtt\n", mediaSequence+i))
	}

	return []byte(sb.String())
}
