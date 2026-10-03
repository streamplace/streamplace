package captions

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestGrouperWordBoundaryLimits(t *testing.T) {
	for _, text := range []string{
		"the quick brown fox jumps over the lazy dog while the cat watches from the window",
		"a " + strings.Repeat("é", 60) + " b c",
	} {
		g := NewGrouper(DefaultCueLayout(), "c")
		words := strings.Fields(text)
		var cues []Cue
		base := time.Unix(0, 0)
		for i, word := range words {
			start := base.Add(time.Duration(i) * 250 * time.Millisecond)
			cues = append(cues, g.Add(Word{Text: word, Start: start, End: start.Add(200 * time.Millisecond)})...)
		}
		cues = append(cues, g.Flush()...)
		var got []string
		for _, cue := range cues {
			lines := strings.Split(cue.Text, "\n")
			require.LessOrEqual(t, len(lines), 2)
			for _, line := range lines {
				if utf8.RuneCountInString(line) > 37 {
					require.Len(t, strings.Fields(line), 1, "only an overlong word may exceed the line limit")
				}
			}
			got = append(got, strings.Fields(cue.Text)...)
			require.Equal(t, cue.Words[0].Start, cue.Start)
			require.GreaterOrEqual(t, cue.End, cue.Words[len(cue.Words)-1].End)
		}
		require.Equal(t, words, got, "every word appears once in order")
	}
}
