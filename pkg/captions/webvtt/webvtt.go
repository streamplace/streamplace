// Package webvtt encodes and parses caption cue files: WebVTT and SRT for
// download and HLS subtitle segments, and a JSON cue list for programmatic
// consumers.
//
// Cue text is plain text with "\n" line breaks. WebVTT escapes markup and
// decodes entities on import. SRT writes text without entity escaping; imports
// strip its formatting tags, so literal tag-shaped formatting is not retained.
// Line breaking and cue splitting are the caller's business.
package webvtt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Cue is one caption cue on a timeline of offsets. What the offsets are
// relative to is up to the caller: the video start for VOD, the live window
// epoch for live HLS segments.
type Cue struct {
	// ID is an optional cue identifier. Players use it to recognise one cue
	// repeated in several HLS subtitle segments, so identical cues must keep
	// identical IDs.
	ID    string
	Start time.Duration
	End   time.Duration
	Text  string
}

// TimestampMap is the WebVTT X-TIMESTAMP-MAP header (RFC 8216 §3.5): the cue
// time Local corresponds to the 90kHz media timestamp MPEGTS. HLS players
// use it to place a segment's cues on the media timeline.
type TimestampMap struct {
	// MPEGTS is a 33-bit, 90kHz media timestamp.
	MPEGTS uint64
	// Local is the cue time that maps to MPEGTS.
	Local time.Duration
}

// MPEGTSModulus is the wrap point of an MPEG-TS timestamp (33 bits).
const MPEGTSModulus = 1 << 33

// EncodeVTT writes cues as a WebVTT document. A non-nil tm adds an
// X-TIMESTAMP-MAP header. Cues must be in start order; empty cues (no text
// after escaping) are dropped, since a WebVTT cue cannot be empty.
func EncodeVTT(cues []Cue, tm *TimestampMap) []byte {
	var b bytes.Buffer
	b.WriteString("WEBVTT\n")
	if tm != nil {
		fmt.Fprintf(&b, "X-TIMESTAMP-MAP=MPEGTS:%d,LOCAL:%s\n", tm.MPEGTS%MPEGTSModulus, FormatVTTTime(tm.Local))
	}
	b.WriteString("\n")
	for _, c := range cues {
		text := escapeVTT(c.Text)
		if text == "" {
			continue
		}
		if id := sanitizeID(c.ID); id != "" {
			b.WriteString(id)
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s --> %s\n", FormatVTTTime(c.Start), FormatVTTTime(c.End))
		b.WriteString(text)
		b.WriteString("\n\n")
	}
	return b.Bytes()
}

// EncodeSRT writes cues as a SubRip document, numbered from 1, without entity
// escaping. Empty cues are dropped; SRT consumers interpret formatting tags.
func EncodeSRT(cues []Cue) []byte {
	var b bytes.Buffer
	n := 0
	for _, c := range cues {
		text := cleanLines(c.Text)
		if text == "" {
			continue
		}
		n++
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", n, FormatSRTTime(c.Start), FormatSRTTime(c.End), text)
	}
	return b.Bytes()
}

type jsonCue struct {
	ID      string `json:"id,omitempty"`
	StartMs int64  `json:"startMs"`
	EndMs   int64  `json:"endMs"`
	Text    string `json:"text"`
}

type jsonDoc struct {
	Epoch string    `json:"epoch,omitempty"`
	Cues  []jsonCue `json:"cues"`
}

// EncodeJSON writes cues as {"epoch"?, "cues":[{"id"?, "startMs", "endMs",
// "text"}]}. Offsets are milliseconds. A non-zero epoch is the wall-clock
// instant (RFC 3339) the offsets count from, for live captions; VOD cues
// count from the video start and carry no epoch.
func EncodeJSON(cues []Cue, epoch time.Time) ([]byte, error) {
	doc := jsonDoc{Cues: make([]jsonCue, 0, len(cues))}
	if !epoch.IsZero() {
		doc.Epoch = epoch.UTC().Format(time.RFC3339Nano)
	}
	for _, c := range cues {
		doc.Cues = append(doc.Cues, jsonCue{ID: c.ID, StartMs: millis(c.Start), EndMs: millis(c.End), Text: cleanLines(c.Text)})
	}
	return json.Marshal(doc)
}

// FormatVTTTime formats d as a WebVTT timestamp, HH:MM:SS.mmm; hours grow
// past two digits as needed. Negative durations clamp to zero.
func FormatVTTTime(d time.Duration) string {
	return formatTime(d, '.')
}

// FormatSRTTime formats d as a SubRip timestamp, HH:MM:SS,mmm.
func FormatSRTTime(d time.Duration) string {
	return formatTime(d, ',')
}

func formatTime(d time.Duration, sep byte) string {
	ms := millis(d)
	if ms < 0 {
		ms = 0
	}
	h := ms / 3600000
	ms %= 3600000
	m := ms / 60000
	ms %= 60000
	s := ms / 1000
	ms %= 1000
	return fmt.Sprintf("%02d:%02d:%02d%c%03d", h, m, s, sep, ms)
}

// millis rounds d to the nearest millisecond.
func millis(d time.Duration) int64 {
	if d < 0 {
		return -int64((-d + time.Millisecond/2) / time.Millisecond)
	}
	return int64((d + time.Millisecond/2) / time.Millisecond)
}

var vttEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// escapeVTT makes plain text safe as WebVTT cue payload: markup characters
// are escaped (which also keeps "-->" out of the text), and blank lines, which
// would end the cue, are removed.
func escapeVTT(text string) string {
	return vttEscaper.Replace(cleanLines(text))
}

// cleanLines normalises line endings, drops control characters, trims every
// line, and removes blank lines.
func cleanLines(text string) string {
	text = strings.NewReplacer("\r\n", "\n", "\r", "\n", "\u2028", "\n", "\u2029", "\n").Replace(text)
	lines := strings.Split(text, "\n")
	out := lines[:0]
	for _, l := range lines {
		l = strings.Map(func(r rune) rune {
			if r < 0x20 && r != '\t' || r == 0x7f {
				return -1
			}
			return r
		}, l)
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// sanitizeID makes a cue identifier safe for the line above a timing line: no
// line breaks and no "-->".
func sanitizeID(id string) string {
	id = strings.NewReplacer("\r", " ", "\n", " ", "-->", "->").Replace(id)
	return strings.TrimSpace(id)
}
