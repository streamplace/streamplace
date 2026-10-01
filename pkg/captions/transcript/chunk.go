package transcript

import (
	"time"
	"unicode/utf8"
)

// Limits of place.stream.caption.transcript.
const (
	MaxTextBytes = 100000
	MaxTimings   = 40000
)

// Default chunking targets: a record covers about five minutes of speech,
// which is a few thousand bytes and well inside the lexicon limits.
const (
	DefaultTargetSpan = 5 * time.Minute
	DefaultMaxSpan    = 6 * time.Minute
	DefaultSilence    = time.Second
)

// ChunkOptions controls how Chunk splits a long transcript. Zero values mean
// the defaults.
type ChunkOptions struct {
	// TargetSpan is the span of speech after which a chunk ends at the next
	// natural boundary: the end of a sentence, or a silence of at least Silence.
	TargetSpan time.Duration
	// MaxSpan ends a chunk wherever it stands once it covers this much, for
	// speech that never pauses and never punctuates.
	MaxSpan time.Duration
	// Silence is the gap that counts as a boundary once TargetSpan is reached.
	Silence time.Duration
	// MaxTextBytes and MaxTimings cap a chunk; they default to the lexicon's.
	MaxTextBytes int
	MaxTimings   int
}

func (o ChunkOptions) withDefaults() ChunkOptions {
	if o.TargetSpan <= 0 {
		o.TargetSpan = DefaultTargetSpan
	}
	if o.MaxSpan < o.TargetSpan {
		o.MaxSpan = max(DefaultMaxSpan, o.TargetSpan)
	}
	if o.Silence <= 0 {
		o.Silence = DefaultSilence
	}
	if o.MaxTextBytes <= 0 || o.MaxTextBytes > MaxTextBytes {
		o.MaxTextBytes = MaxTextBytes
	}
	if o.MaxTimings <= 0 || o.MaxTimings > MaxTimings {
		o.MaxTimings = MaxTimings
	}
	return o
}

// Chunk encodes words, which must be in time order, as one or more compact
// transcripts that each fit a record. Words that start before the time base
// are clamped to it (a record's startMs cannot be negative).
//
// Chunks end after roughly TargetSpan of speech, at the first sentence end or
// long silence after that point; failing that, before the word that would take
// a chunk past MaxSpan, or past the size limits. A chunk never splits a word,
// and consecutive chunks together hold every word.
func Chunk(words []Word, opts ChunkOptions) []Compact {
	opts = opts.withDefaults()
	toks := tokenize(words)
	for i := range toks {
		toks[i] = clamp(toks[i], opts.MaxTextBytes)
	}
	target, maxSpan, silence := opts.TargetSpan.Milliseconds(), opts.MaxSpan.Milliseconds(), opts.Silence.Milliseconds()

	var out []Compact
	first := 0
	var textBytes, entries int
	var cursor int64
	for i, w := range toks {
		if i == first {
			textBytes, entries, cursor = len(w.Text), 1, w.EndMs
		} else {
			add := 1
			if w.StartMs > cursor {
				add = 2
			}
			if textBytes+1+len(w.Text) > opts.MaxTextBytes || entries+add > opts.MaxTimings || w.EndMs-toks[first].StartMs > maxSpan {
				out = append(out, Encode(toks[first:i]))
				first = i
				textBytes, entries, cursor = len(w.Text), 1, w.EndMs
				continue
			}
			textBytes += 1 + len(w.Text)
			entries += add
			cursor = w.EndMs
		}
		if i+1 == len(toks) {
			break
		}
		span := w.EndMs - toks[first].StartMs
		next := toks[i+1]
		if span >= target && (endsSentence(w.Text) || next.StartMs-w.EndMs >= silence) {
			out = append(out, Encode(toks[first:i+1]))
			first = i + 1
		}
	}
	if first < len(toks) {
		out = append(out, Encode(toks[first:]))
	}
	return out
}

// clamp makes a token legal for a record: no earlier than the time base, no
// ending before it starts, no longer than a record's text can hold.
func clamp(w Word, maxBytes int) Word {
	w.StartMs = max(w.StartMs, 0)
	w.EndMs = max(w.EndMs, w.StartMs)
	for len(w.Text) > maxBytes {
		_, size := utf8.DecodeLastRuneInString(w.Text)
		w.Text = w.Text[:len(w.Text)-size]
	}
	return w
}

// endsSentence reports whether a word closes a sentence: terminal punctuation,
// Latin or CJK, possibly followed by closing quotes or brackets.
func endsSentence(word string) bool {
	for len(word) > 0 {
		r, size := utf8.DecodeLastRuneInString(word)
		switch r {
		case '.', '?', '!', '…', '。', '！', '？', '｡':
			return true
		case '"', '\'', ')', ']', '}', '”', '’', '»', '」', '』', '）':
			word = word[:len(word)-size]
		default:
			return false
		}
	}
	return false
}
