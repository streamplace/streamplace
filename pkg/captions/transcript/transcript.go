// Package transcript converts between timed words and the compact transcript
// encoding of place.stream.caption.transcript, which is the encoding of
// ionosphere's tv.ionosphere.transcript: `text` holds the words separated by
// whitespace, `startMs` is where the first word starts, and `timings` is a flat
// list in which a positive entry is the duration of the next word in
// milliseconds and a negative entry is a silence gap. Words are contiguous
// unless a gap says otherwise.
//
// Encode and Decode follow ionosphere's reference implementation
// (formats/tv.ionosphere/ts/transcript-encoding.ts) step for step, including
// its quirks, so records written here read back identically there and the
// other way around:
//
//   - a gap entry is written only when a word starts after the end of the
//     previous word; a word that starts early (overlap) just follows the
//     previous word, so on decode it lands late;
//   - every word lasts at least 1 ms. The encoder's cursor still advances to the
//     word's real end, so a zero-length word makes everything after it decode
//     1 ms late when the next word started exactly where it did;
//   - decoding pairs the Nth positive entry with the Nth whitespace token and
//     ignores positive entries beyond the last token; tokens left without a
//     timing are dropped.
package transcript

import (
	"strings"
)

// Compact is the stored form of one transcript chunk.
type Compact struct {
	Text    string
	StartMs int64
	Timings []int64
}

// Word is one word with its span in milliseconds from the chunk's time base
// (the subject's media start; see place.stream.caption.transcript).
type Word struct {
	Text    string
	StartMs int64
	EndMs   int64
}

// isSpace reports whether r is whitespace in the sense of a JavaScript regular
// expression's \s, which is how ionosphere tokenizes `text`. It differs from
// unicode.IsSpace on U+0085 (not whitespace in JS) and U+FEFF (whitespace in
// JS), and keeping to the JS set means every reader agrees on which token is
// which.
func isSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x00a0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// Tokens splits text into words the way every reader of a compact transcript
// must: on runs of whitespace, discarding empty tokens.
func Tokens(text string) []string {
	return strings.FieldsFunc(text, isSpace)
}

// Encode packs words, which must be in time order, into the compact form. A
// word whose text holds internal whitespace is several tokens in the stored
// form, so it is split and its span divided evenly between the pieces; a word
// with no text is dropped.
func Encode(words []Word) Compact {
	tokens := tokenize(words)
	if len(tokens) == 0 {
		return Compact{Timings: []int64{}}
	}
	text := make([]string, len(tokens))
	for i, t := range tokens {
		text[i] = t.Text
	}
	startMs := tokens[0].StartMs
	timings := make([]int64, 0, len(tokens))
	cursor := startMs
	for _, t := range tokens {
		if gap := t.StartMs - cursor; gap > 0 {
			timings = append(timings, -gap)
		}
		timings = append(timings, max(t.EndMs-t.StartMs, 1))
		cursor = t.EndMs
	}
	return Compact{Text: strings.Join(text, " "), StartMs: startMs, Timings: timings}
}

// tokenize makes every Word hold exactly one whitespace-free token.
func tokenize(words []Word) []Word {
	out := make([]Word, 0, len(words))
	for _, w := range words {
		toks := Tokens(w.Text)
		switch len(toks) {
		case 0:
		case 1:
			out = append(out, Word{Text: toks[0], StartMs: w.StartMs, EndMs: w.EndMs})
		default:
			out = append(out, spread(toks, w.StartMs, w.EndMs)...)
		}
	}
	return out
}

// spread lays tokens evenly, back to back, across [startMs, endMs). Boundaries
// are computed from the whole span, not accumulated, so rounding never drifts
// and the last token ends exactly at endMs.
func spread(tokens []string, startMs, endMs int64) []Word {
	n := int64(len(tokens))
	span := max(endMs-startMs, 0)
	out := make([]Word, len(tokens))
	for i, t := range tokens {
		out[i] = Word{
			Text:    t,
			StartMs: startMs + span*int64(i)/n,
			EndMs:   startMs + span*(int64(i)+1)/n,
		}
	}
	return out
}

// Decode unpacks a compact transcript into words with their spans.
func Decode(c Compact) []Word {
	tokens := Tokens(c.Text)
	out := make([]Word, 0, len(tokens))
	cursor := c.StartMs
	for _, v := range c.Timings {
		if v < 0 {
			cursor += -v
			continue
		}
		if len(out) >= len(tokens) {
			break
		}
		out = append(out, Word{Text: tokens[len(out)], StartMs: cursor, EndMs: cursor + v})
		cursor += v
	}
	return out
}
