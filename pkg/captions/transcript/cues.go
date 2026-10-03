package transcript

import (
	"strconv"
	"strings"
	"time"

	"stream.place/streamplace/pkg/captions"
)

// CueOptions controls how Cues groups words for display. Zero values mean the
// defaults, which are the usual broadcast limits.
type CueOptions struct {
	MaxLineChars int           // characters per line (default 42)
	MaxLines     int           // lines per cue (default 2)
	MinDuration  time.Duration // a cue stays up at least this long (default 1s) when the next cue allows
	MaxDuration  time.Duration // a cue spans at most this long (default 7s)
	Silence      time.Duration // a pause this long between words starts a new cue (default 1.5s)
}

func (o CueOptions) withDefaults() CueOptions {
	if o.MaxLineChars <= 0 {
		o.MaxLineChars = 42
	}
	if o.MaxLines <= 0 {
		o.MaxLines = 2
	}
	if o.MinDuration <= 0 {
		o.MinDuration = time.Second
	}
	if o.MaxDuration < o.MinDuration {
		o.MaxDuration = max(7*time.Second, o.MinDuration)
	}
	if o.Silence <= 0 {
		o.Silence = 1500 * time.Millisecond
	}
	return o
}

// Cues groups timed words, in time order, into display cues. A cue holds at
// most MaxLines lines of at most MaxLineChars characters (a word longer than a
// line gets a line to itself), spans at most MaxDuration, and ends early at a
// sentence end once it has been up for MinDuration, or at a long silence. The
// lines of a cue are joined with "\n". Cue IDs are 1-based positions.
func Cues(words []Word, opts CueOptions) []captions.TimedCue {
	opts = opts.withDefaults()
	minDur, maxDur, silence := opts.MinDuration.Milliseconds(), opts.MaxDuration.Milliseconds(), opts.Silence.Milliseconds()

	var out []captions.TimedCue
	var cur []Word
	var lines []string
	flush := func() {
		if len(cur) == 0 {
			return
		}
		out = append(out, captions.TimedCue{
			ID:    strconv.Itoa(len(out) + 1),
			Start: msDuration(cur[0].StartMs),
			End:   msDuration(cur[len(cur)-1].EndMs),
			Text:  strings.Join(lines, "\n"),
		})
		cur, lines = nil, nil
	}
	for _, w := range tokenize(words) {
		if len(cur) > 0 {
			prev := cur[len(cur)-1]
			if w.StartMs-prev.EndMs >= silence || w.EndMs-cur[0].StartMs > maxDur {
				flush()
			}
		}
		next, ok := captions.WrapWord(lines, w.Text, opts.MaxLineChars, opts.MaxLines)
		if !ok {
			flush()
			next, _ = captions.WrapWord(nil, w.Text, opts.MaxLineChars, opts.MaxLines)
		}
		lines = next
		cur = append(cur, w)
		if endsSentence(w.Text) && w.EndMs-cur[0].StartMs >= minDur {
			flush()
		}
	}
	flush()

	for i := range out {
		if out[i].End-out[i].Start >= opts.MinDuration {
			continue
		}
		end := out[i].Start + opts.MinDuration
		if i+1 < len(out) {
			end = min(end, out[i+1].Start)
		}
		out[i].End = max(out[i].End, end)
	}
	return out
}

// WordsFromCaptions converts live words, which carry absolute times, into
// words offset from base. Times round to the nearest millisecond.
func WordsFromCaptions(base time.Time, words []captions.Word) []Word {
	out := make([]Word, len(words))
	for i, w := range words {
		out[i] = Word{Text: w.Text, StartMs: durationMs(w.Start.Sub(base)), EndMs: durationMs(w.End.Sub(base))}
	}
	return out
}

// WordsFromCue returns the words of a live cue as offsets from base: its word
// timings when it has them, otherwise its text spread evenly over [Start, End).
func WordsFromCue(base time.Time, cue captions.Cue) []Word {
	if len(cue.Words) > 0 {
		return WordsFromCaptions(base, cue.Words)
	}
	toks := Tokens(cue.Text)
	if len(toks) == 0 {
		return nil
	}
	return spread(toks, durationMs(cue.Start.Sub(base)), durationMs(cue.End.Sub(base)))
}

func msDuration(ms int64) time.Duration { return time.Duration(ms) * time.Millisecond }

// durationMs rounds a duration to the nearest millisecond, halves toward +inf,
// like the Math.round(seconds * 1000) of the reference encoder.
func durationMs(d time.Duration) int64 {
	n := int64(d) + int64(time.Millisecond)/2
	q := n / int64(time.Millisecond)
	if n%int64(time.Millisecond) < 0 {
		q--
	}
	return q
}
