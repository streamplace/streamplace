package aigateway

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	// minCueDurationMS is the minimum duration for a VTT cue in milliseconds.
	minCueDurationMS = 200

	// defaultLastCueDurationMS is the default duration for the last cue when no next cue exists.
	defaultLastCueDurationMS = 2000
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
