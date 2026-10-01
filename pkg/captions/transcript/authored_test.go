package transcript

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/captions"
)

func TestAuthoredAdjacentMinimumDuration(t *testing.T) {
	input := []captions.TimedCue{
		{Start: 0, End: time.Millisecond, Text: "one"},
		{Start: time.Millisecond, End: 2 * time.Millisecond, Text: "two"},
	}
	chunks := ChunkAuthored(WordsFromCues(input), ChunkOptions{})
	var words []Word
	for _, chunk := range chunks {
		for _, timing := range chunk.Timings {
			require.NotZero(t, timing, "word durations must remain positive")
		}
		words = append(words, DecodeAuthored(chunk)...)
	}
	got := AuthoredCues(words)
	require.Len(t, got, 2)
	for i := range input {
		require.Equal(t, input[i].Text, got[i].Text)
		require.InDelta(t, input[i].Start.Milliseconds(), got[i].Start.Milliseconds(), 1)
		require.InDelta(t, input[i].End.Milliseconds(), got[i].End.Milliseconds(), 1)
	}
}
