package captions

import (
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// CueLayout bounds how words are grouped into readable caption cues.
type CueLayout struct {
	MaxLineChars int           // characters per line before wrapping
	MaxLines     int           // lines per cue
	MinDuration  time.Duration // a cue stays up at least this long
	MaxDuration  time.Duration // a cue spans at most this much speech
	MaxGap       time.Duration // a pause longer than this starts a new cue
}

// DefaultCueLayout is the conventional broadcast caption shape: two lines of
// up to 37 characters, on screen for one to seven seconds.
func DefaultCueLayout() CueLayout {
	return CueLayout{MaxLineChars: 37, MaxLines: 2, MinDuration: time.Second, MaxDuration: 7 * time.Second, MaxGap: time.Second}
}

func (l CueLayout) maxChars() int { return l.MaxLineChars * l.MaxLines }

// Grouper folds a stream of committed words into cues that fit a CueLayout.
// Cue IDs are the prefix plus a counter, unique per Grouper.
type Grouper struct {
	layout CueLayout
	prefix string
	seq    int
	words  []Word
}

func NewGrouper(layout CueLayout, idPrefix string) *Grouper {
	g := &Grouper{layout: layout, prefix: idPrefix}
	return g
}

func (g *Grouper) id() string { return g.prefix + strconv.Itoa(g.seq) }

// Add appends a committed word and returns the cues it closed: the open cue
// when the word does not fit it, and the cue the word completes when it ends
// a sentence at a full line.
func (g *Grouper) Add(w Word) []Cue {
	w.Text = strings.TrimSpace(w.Text)
	if w.Text == "" {
		return nil
	}
	var closed []Cue
	if len(g.words) > 0 && !g.fits(w) {
		closed = append(closed, g.close())
	}
	g.words = append(g.words, w)
	if endsSentence(w.Text) && wordChars(g.words) >= g.layout.MaxLineChars {
		closed = append(closed, g.close())
	}
	return closed
}

func (g *Grouper) fits(w Word) bool {
	last := g.words[len(g.words)-1]
	if w.Start.Sub(last.End) > g.layout.MaxGap {
		return false
	}
	if w.End.Sub(g.words[0].Start) > g.layout.MaxDuration {
		return false
	}
	return wordChars(g.words)+1+utf8.RuneCountInString(w.Text) <= g.layout.maxChars()
}

// Open reports whether a cue is being built.
func (g *Grouper) Open() bool { return len(g.words) > 0 }

// Current is the open cue extended with interim words, for publishing as an
// interim cue; ok is false when there is nothing to show. The interim words
// are not kept.
func (g *Grouper) Current(interim []Word) (Cue, bool) {
	words := make([]Word, 0, len(g.words)+len(interim))
	words = append(words, g.words...)
	for _, w := range interim {
		w.Text = strings.TrimSpace(w.Text)
		if w.Text != "" {
			words = append(words, w)
		}
	}
	if len(words) == 0 {
		return Cue{}, false
	}
	// With nothing committed yet the interim cue already carries the next
	// id, so the final that follows replaces it.
	return g.build(words, false), true
}

// Flush closes the open cue, if any.
func (g *Grouper) Flush() []Cue {
	if len(g.words) == 0 {
		return nil
	}
	return []Cue{g.close()}
}

func (g *Grouper) close() Cue {
	c := g.build(g.words, true)
	g.words = nil
	g.seq++
	return c
}

func (g *Grouper) build(words []Word, final bool) Cue {
	start := words[0].Start
	end := words[len(words)-1].End
	if end.Sub(start) < g.layout.MinDuration {
		end = start.Add(g.layout.MinDuration)
	}
	texts := make([]string, len(words))
	for i, w := range words {
		texts[i] = w.Text
	}
	return Cue{
		ID:    g.id(),
		Start: start,
		End:   end,
		Text:  WrapLines(strings.Join(texts, " "), g.layout.MaxLineChars, g.layout.MaxLines),
		Words: append([]Word(nil), words...),
		Final: final,
	}
}

func wordChars(words []Word) int {
	n := 0
	for i, w := range words {
		if i > 0 {
			n++
		}
		n += utf8.RuneCountInString(w.Text)
	}
	return n
}

func endsSentence(s string) bool {
	r, _ := utf8.DecodeLastRuneInString(strings.TrimRight(s, "\"'”’)"))
	return r == '.' || r == '!' || r == '?' || r == '。' || r == '！' || r == '？'
}

// WrapLines breaks text into at most maxLines lines of up to maxChars
// characters at word boundaries. Two-line text is balanced (the break that
// keeps the longer line shortest), so a cue reads as a pair rather than a
// long line over a short tail. Text that cannot fit keeps its last line
// long rather than dropping words.
func WrapLines(text string, maxChars, maxLines int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	if maxLines <= 1 || maxChars <= 0 {
		return strings.Join(words, " ")
	}
	total := wordsLen(words)
	if total <= maxChars {
		return strings.Join(words, " ")
	}
	if maxLines == 2 {
		best, bestMax := 1, -1
		for k := 1; k < len(words); k++ {
			a, b := wordsLen(words[:k]), wordsLen(words[k:])
			if a > maxChars && k > 1 {
				break
			}
			if m := max(a, b); bestMax < 0 || m < bestMax {
				best, bestMax = k, m
			}
		}
		return strings.Join(words[:best], " ") + "\n" + strings.Join(words[best:], " ")
	}
	lines := make([]string, 0, maxLines)
	target := (total + maxLines - 1) / maxLines
	if target > maxChars {
		target = maxChars
	}
	i := 0
	for len(lines) < maxLines-1 && i < len(words) {
		n := 0
		j := i
		for j < len(words) {
			w := utf8.RuneCountInString(words[j])
			next := n + w
			if n > 0 {
				next++
			}
			if next > maxChars || (n > 0 && next > target) {
				break
			}
			n = next
			j++
		}
		if j == i {
			j = i + 1 // a single word longer than the line
		}
		lines = append(lines, strings.Join(words[i:j], " "))
		i = j
	}
	if i < len(words) {
		lines = append(lines, strings.Join(words[i:], " "))
	}
	return strings.Join(lines, "\n")
}

func wordsLen(words []string) int {
	n := 0
	for i, w := range words {
		if i > 0 {
			n++
		}
		n += utf8.RuneCountInString(w)
	}
	return n
}

// NormalizeWord lowercases a word and strips punctuation, for comparing two
// transcriptions of the same audio.
func NormalizeWord(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
