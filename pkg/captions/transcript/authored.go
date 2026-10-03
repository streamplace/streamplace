package transcript

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"stream.place/streamplace/pkg/captions"
)

// authoredTokens keeps each word's preceding whitespace, including line breaks.
// The compact format still has exactly one duration per whitespace token.
func authoredTokens(text string) []string {
	var out []string
	start, inWord := 0, false
	for i, r := range text {
		if isSpace(r) {
			if inWord {
				out = append(out, text[start:i])
				start, inWord = i, false
			}
		} else {
			inWord = true
		}
	}
	if inWord {
		out = append(out, text[start:])
	}
	return out
}

// WordsFromCues encodes authored cue boundaries as silence, not display-layout
// hints. Adjacent cues reserve 1 ms from the preceding cue's final word so the
// next cue retains its original start and the total timeline is unchanged.
func WordsFromCues(cues []captions.TimedCue) []Word {
	sorted := slices.SortedStableFunc(slices.Values(cues), func(a, b captions.TimedCue) int {
		return cmp.Compare(a.Start, b.Start)
	})
	var out []Word
	var cursor int64
	for i, cue := range sorted {
		tokens := authoredTokens(strings.TrimSpace(cue.Text))
		if len(tokens) == 0 {
			continue
		}
		start, end := max(durationMs(cue.Start), cursor), durationMs(cue.End)
		if i+1 < len(sorted) {
			if next := durationMs(sorted[i+1].Start); next > start {
				end = min(end, next)
			}
		}
		end = max(start, end)
		if len(out) > 0 && start == out[len(out)-1].EndMs {
			last := &out[len(out)-1]
			if last.EndMs-last.StartMs > 1 {
				last.EndMs--
			} else {
				// A positive 1 ms word cannot be shortened further.
				start++
				end = max(end, start+int64(len(tokens)))
			}
		}
		out = append(out, spread(tokens, start, end)...)
		cursor = end
	}
	return out
}

// ChunkAuthored applies the same record limits as Chunk without normalizing
// authored whitespace. Chunk boundaries do not become cue boundaries.
func ChunkAuthored(words []Word, opts ChunkOptions) []Compact {
	return chunkTokens(words, opts, func(tokens []Word) Compact {
		return encodeTokens(tokens, true)
	})
}

// DecodeAuthored retains whitespace while decoding the standard timings.
func DecodeAuthored(c Compact) []Word {
	return decodeTokens(c, authoredTokens(c.Text))
}

// AuthoredCues reconstructs imported/human cue boundaries only at explicit
// silence gaps. Duration, punctuation and line-length heuristics must not
// rewrite an author's timing or text.
func AuthoredCues(words []Word) []captions.TimedCue {
	var out []captions.TimedCue
	for _, word := range words {
		if len(out) == 0 || word.StartMs > out[len(out)-1].End.Milliseconds() {
			out = append(out, captions.TimedCue{
				ID: strconv.Itoa(len(out) + 1), Start: msDuration(word.StartMs),
				End: msDuration(word.EndMs), Text: strings.TrimLeftFunc(word.Text, isSpace),
			})
			continue
		}
		cue := &out[len(out)-1]
		if word.Text == strings.TrimLeftFunc(word.Text, isSpace) {
			cue.Text += " "
		}
		cue.Text += word.Text
		cue.End = max(cue.End, msDuration(word.EndMs))
	}
	return out
}
