package webvtt

import (
	"errors"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Format is a caption file format.
type Format string

const (
	FormatVTT Format = "vtt"
	FormatSRT Format = "srt"
)

var (
	// ErrNotWebVTT is returned by ParseVTT when the file has no WEBVTT header.
	ErrNotWebVTT = errors.New("webvtt: missing WEBVTT header")
	// ErrNoCues is returned when a file holds no usable cue.
	ErrNoCues = errors.New("webvtt: no cues found")
)

// Parse parses a caption file in either format: WebVTT when it starts with a
// WEBVTT header, SubRip otherwise.
func Parse(data []byte) ([]Cue, Format, error) {
	if hasVTTHeader(normalize(data)) {
		cues, err := ParseVTT(data)
		return cues, FormatVTT, err
	}
	cues, err := ParseSRT(data)
	return cues, FormatSRT, err
}

// ParseVTT parses a WebVTT document. It skips the header block (including
// X-TIMESTAMP-MAP and metadata) and NOTE, STYLE, and REGION blocks, and keeps
// cue identifiers and settings. Cue timestamps may omit hours. Blocks that
// are not well-formed cues are skipped rather than failing the file, so an
// import survives a few bad cues; cues without text or with an end before
// their start are dropped. Markup in cue text is stripped and entities are
// decoded. The result is sorted by start time.
func ParseVTT(data []byte) ([]Cue, error) {
	text := normalize(data)
	if !hasVTTHeader(text) {
		return nil, ErrNotWebVTT
	}
	var cues []Cue
	for i, block := range splitBlocks(text) {
		if i == 0 {
			continue // header block
		}
		first := block[0]
		switch {
		case isBlockKeyword(first, "NOTE"), isBlockKeyword(first, "STYLE"), isBlockKeyword(first, "REGION"):
			continue
		}
		var id string
		timing := 0
		if !strings.Contains(first, "-->") {
			// A cue identifier line, but only when a timing line follows.
			if len(block) < 2 || !strings.Contains(block[1], "-->") {
				continue
			}
			id = strings.TrimSpace(first)
			timing = 1
		}
		start, end, settings, ok := parseTimingLine(block[timing])
		if !ok {
			continue
		}
		if c, ok := newCue(id, start, end, settings, block[timing+1:]); ok {
			cues = append(cues, c)
		}
	}
	return finish(cues)
}

// ParseSRT parses a SubRip document, including the sloppy files found in
// the wild: a missing blank line between cues, missing or non-numeric cue
// numbers, "." instead of "," before the milliseconds, one- and two-digit
// fields, "->" arrows, and WebVTT-style settings after the timing. A cue's
// text runs from its timing line to the next blank line or timing line.
// Styling tags ("<i>", "{\an8}") are stripped and entities decoded. Cues
// without text or with an end before their start are dropped. The result is
// sorted by start time.
func ParseSRT(data []byte) ([]Cue, error) {
	lines := strings.Split(normalize(data), "\n")
	var cues []Cue
	for i := 0; i < len(lines); i++ {
		start, end, settings, ok := parseTimingLine(lines[i])
		if !ok {
			continue
		}
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) != "" {
			if _, _, _, isTiming := parseTimingLine(lines[j]); isTiming {
				break
			}
			j++
		}
		body := lines[i+1 : j]
		// The cue number of the next cue sits right above its timing line
		// when the blank line between the cues is missing.
		if j < len(lines) && strings.TrimSpace(lines[j]) != "" && len(body) > 0 && isIndexLine(body[len(body)-1]) {
			body = body[:len(body)-1]
		}
		if c, ok := newCue("", start, end, settings, body); ok {
			cues = append(cues, c)
		}
		i = j - 1
	}
	return finish(cues)
}

func finish(cues []Cue) ([]Cue, error) {
	if len(cues) == 0 {
		return nil, ErrNoCues
	}
	sort.SliceStable(cues, func(i, j int) bool { return cues[i].Start < cues[j].Start })
	return cues, nil
}

func newCue(id string, start, end time.Duration, settings string, body []string) (Cue, bool) {
	if end <= start {
		return Cue{}, false
	}
	text := cleanText(strings.Join(body, "\n"))
	if text == "" {
		return Cue{}, false
	}
	return Cue{ID: id, Start: start, End: end, Text: text, Settings: settings}, true
}

// normalize strips a UTF-8 byte order mark and NULs, replaces invalid UTF-8,
// and converts every line ending to "\n".
func normalize(data []byte) string {
	s := strings.TrimPrefix(string(data), "\ufeff")
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "\ufffd")
	}
	s = strings.ReplaceAll(s, "\x00", "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

func hasVTTHeader(text string) bool {
	if !strings.HasPrefix(text, "WEBVTT") {
		return false
	}
	rest := text[len("WEBVTT"):]
	return rest == "" || rest[0] == ' ' || rest[0] == '\t' || rest[0] == '\n'
}

// splitBlocks splits text into blocks of consecutive non-blank lines.
func splitBlocks(text string) [][]string {
	var blocks [][]string
	var cur []string
	for _, l := range strings.Split(text, "\n") {
		if strings.TrimSpace(l) == "" {
			if len(cur) > 0 {
				blocks = append(blocks, cur)
				cur = nil
			}
			continue
		}
		cur = append(cur, l)
	}
	if len(cur) > 0 {
		blocks = append(blocks, cur)
	}
	return blocks
}

func isBlockKeyword(line, kw string) bool {
	if !strings.HasPrefix(line, kw) {
		return false
	}
	rest := line[len(kw):]
	return rest == "" || rest[0] == ' ' || rest[0] == '\t'
}

var (
	// timing line: start --> end [settings]. Arrows are matched loosely
	// ("->", "--->") because SRT files in the wild use them.
	timingRe = regexp.MustCompile(`^\s*(\d+(?::\d{1,2}){1,2}[.,]\d+|\d+(?::\d{1,2}){1,2})\s*-{1,3}>\s*(\d+(?::\d{1,2}){1,2}[.,]\d+|\d+(?::\d{1,2}){1,2})(?:\s+(.*?))?\s*$`)
	indexRe  = regexp.MustCompile(`^\s*\d+\s*$`)
	// markup: HTML-ish tags (a "<" followed by a space is text, not a tag),
	// WebVTT karaoke timestamps, and SRT/ASS override blocks.
	tagRe = regexp.MustCompile(`</?[A-Za-z][^>\n]*>|<\d+:\d+(?::\d+)?[.,]\d+>|\{\\[^}\n]*\}`)
)

func isIndexLine(l string) bool { return indexRe.MatchString(l) }

func parseTimingLine(line string) (start, end time.Duration, settings string, ok bool) {
	m := timingRe.FindStringSubmatch(line)
	if m == nil {
		return 0, 0, "", false
	}
	start, ok1 := parseTimestamp(m[1])
	end, ok2 := parseTimestamp(m[2])
	if !ok1 || !ok2 {
		return 0, 0, "", false
	}
	return start, end, strings.TrimSpace(m[3]), true
}

// parseTimestamp parses [h+:]mm:ss[.,fff] where the fraction is a decimal
// fraction of a second of any length ("5" is 500ms, "050" is 50ms).
func parseTimestamp(s string) (time.Duration, bool) {
	frac := ""
	if i := strings.IndexAny(s, ".,"); i >= 0 {
		frac = s[i+1:]
		s = s[:i]
	}
	parts := strings.Split(s, ":")
	var h, m, sec int64
	var err error
	switch len(parts) {
	case 2:
		m, err = parseInt(parts[0])
		if err == nil {
			sec, err = parseInt(parts[1])
		}
	case 3:
		h, err = parseInt(parts[0])
		if err == nil {
			m, err = parseInt(parts[1])
		}
		if err == nil {
			sec, err = parseInt(parts[2])
		}
	default:
		return 0, false
	}
	if err != nil || m > 59 || sec > 59 || h > 1_000_000 {
		return 0, false
	}
	var ms int64
	for i := range 3 {
		ms *= 10
		if i < len(frac) {
			ms += int64(frac[i] - '0')
		}
	}
	return time.Duration(((h*60+m)*60+sec)*1000+ms) * time.Millisecond, true
}

func parseInt(s string) (int64, error) {
	var n int64
	if s == "" {
		return 0, fmt.Errorf("empty number")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("bad number %q", s)
		}
		n = n*10 + int64(r-'0')
		if n > 1<<40 {
			return 0, fmt.Errorf("number too large")
		}
	}
	return n, nil
}

// cleanText turns cue payload into plain text: markup removed, entities
// decoded, lines trimmed, blank lines dropped.
func cleanText(s string) string {
	s = tagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return cleanLines(s)
}
