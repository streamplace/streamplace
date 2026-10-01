package transcript

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/captions"
)

func ms(n int64) time.Duration { return time.Duration(n) * time.Millisecond }

// talk makes words of the given texts, each 300ms long, with the given pause
// (ms) after each word.
func talk(pause int64, texts ...string) []Word {
	var out []Word
	var t int64
	for _, text := range texts {
		out = append(out, Word{text, t, t + 300})
		t += 300 + pause
	}
	return out
}

func TestCuesGrouping(t *testing.T) {
	tests := []struct {
		name  string
		words []Word
		want  []captions.TimedCue
	}{
		{
			name: "short sentence is one cue",
			words: []Word{
				{"Hello", 500, 800}, {"there", 800, 1100}, {"friend.", 1100, 1600},
			},
			want: []captions.TimedCue{{ID: "1", Start: ms(500), End: ms(1600), Text: "Hello there friend."}},
		},
		{
			name:  "wraps at 42 characters onto a second line",
			words: talk(0, strings.Fields("aaaaaaaaaa bbbbbbbbbb cccccccccc dddddddddd eeeeeeeeee ffffffffff")...),
			want: []captions.TimedCue{
				{ID: "1", Start: 0, End: ms(1800), Text: "aaaaaaaaaa bbbbbbbbbb cccccccccc\ndddddddddd eeeeeeeeee ffffffffff"},
			},
		},
		{
			name:  "a third line starts a new cue",
			words: talk(0, strings.Fields("aaaaaaaaaa bbbbbbbbbb cccccccccc dddddddddd eeeeeeeeee ffffffffff gggggggggg hhhhhhhhhh iiiiiiiiii jjjjjjjjjj kkkkkkkkkk")...),
			want: []captions.TimedCue{
				{ID: "1", Start: 0, End: ms(1800), Text: "aaaaaaaaaa bbbbbbbbbb cccccccccc\ndddddddddd eeeeeeeeee ffffffffff"},
				{ID: "2", Start: ms(1800), End: ms(3300), Text: "gggggggggg hhhhhhhhhh iiiiiiiiii\njjjjjjjjjj kkkkkkkkkk"},
			},
		},
		{
			name:  "a word longer than a line sits alone on one",
			words: talk(0, "a", strings.Repeat("x", 60), "b"),
			want: []captions.TimedCue{
				{ID: "1", Start: 0, End: ms(600), Text: "a\n" + strings.Repeat("x", 60)},
				{ID: "2", Start: ms(600), End: ms(1600), Text: "b"},
			},
		},
		{
			name:  "ends the cue at a sentence end once it has been up a second",
			words: talk(0, "One", "two", "three", "four.", "Five", "six."),
			want: []captions.TimedCue{
				{ID: "1", Start: 0, End: ms(1200), Text: "One two three four."},
				{ID: "2", Start: ms(1200), End: ms(2200), Text: "Five six."},
			},
		},
		{
			name:  "does not cut a sentence end that would leave a flash of a cue",
			words: talk(0, "No.", "Yes", "it", "is."),
			want: []captions.TimedCue{
				{ID: "1", Start: 0, End: ms(1200), Text: "No. Yes it is."},
			},
		},
		{
			name:  "a long pause starts a new cue",
			words: append(talk(0, "first", "part"), []Word{{"second", 3000, 3300}, {"part", 3300, 3600}}...),
			want: []captions.TimedCue{
				{ID: "1", Start: 0, End: ms(1000), Text: "first part"},
				{ID: "2", Start: ms(3000), End: ms(4000), Text: "second part"},
			},
		},
		{
			name:  "a pause under the limit does not",
			words: talk(1000, "first", "part"),
			want:  []captions.TimedCue{{ID: "1", Start: 0, End: ms(1600), Text: "first part"}},
		},
		{
			name:  "caps a cue at 7 seconds",
			words: talk(0, "a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x"),
			want: []captions.TimedCue{
				{ID: "1", Start: 0, End: ms(6900), Text: "a b c d e f g h i j k l m n o p q r s t u\nv w"},
				{ID: "2", Start: ms(6900), End: ms(7900), Text: "x"},
			},
		},
		{
			name:  "unicode counts characters, not bytes",
			words: talk(0, strings.Repeat("日本語", 6), strings.Repeat("é", 20), "okay"),
			want:  []captions.TimedCue{{ID: "1", Start: 0, End: ms(1000), Text: strings.Repeat("日本語", 6) + " " + strings.Repeat("é", 20) + "\nokay"}},
		},
		{
			name:  "punctuation-only tokens ride along",
			words: talk(0, "wait", "—", "what", "♪", "huh"),
			want:  []captions.TimedCue{{ID: "1", Start: 0, End: ms(1500), Text: "wait — what ♪ huh"}},
		},
		{
			name:  "a trailing cue shorter than a second is extended",
			words: []Word{{"hi", 0, 300}},
			want:  []captions.TimedCue{{ID: "1", Start: 0, End: ms(1000), Text: "hi"}},
		},
		{
			name:  "extension stops where the next cue starts",
			words: []Word{{"hi", 0, 300}, {"there", 2000, 2300}},
			want: []captions.TimedCue{
				{ID: "1", Start: 0, End: ms(1000), Text: "hi"},
				{ID: "2", Start: ms(2000), End: ms(3000), Text: "there"},
			},
		},
		{
			name:  "a sentence end under a second long is kept with what follows",
			words: []Word{{"ok.", 0, 300}, {"Go", 500, 800}, {"on", 800, 900}},
			want:  []captions.TimedCue{{ID: "1", Start: 0, End: ms(1000), Text: "ok. Go on"}},
		},
		{
			name:  "nothing in, nothing out",
			words: []Word{{" ", 0, 100}},
			want:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, Cues(tt.words, CueOptions{}))
		})
	}
}

// TestCuesLimits checks the display limits over a long run of varied words,
// not just the hand-picked cases above.
func TestCuesLimits(t *testing.T) {
	var words []Word
	var at int64
	for i := range 3000 {
		text := strings.Repeat("w", 1+i%11)
		if i%13 == 12 {
			text += "."
		}
		dur := int64(120 + (i*37)%400)
		pause := int64((i * 53) % 700)
		words = append(words, Word{text, at, at + dur})
		at += dur + pause
	}
	cues := Cues(words, CueOptions{})
	require.NotEmpty(t, cues)
	for i, c := range cues {
		lines := strings.Split(c.Text, "\n")
		require.LessOrEqual(t, len(lines), 2, "cue %d", i)
		for _, l := range lines {
			require.LessOrEqual(t, utf8.RuneCountInString(l), 42, "cue %d line %q", i, l)
		}
		require.GreaterOrEqual(t, c.End-c.Start, time.Second, "cue %d", i)
		require.LessOrEqual(t, c.End-c.Start, 7*time.Second, "cue %d", i)
		if i > 0 {
			require.LessOrEqual(t, cues[i-1].End, c.Start, "cue %d overlaps the one before", i)
		}
	}
	var got []string
	for _, c := range cues {
		got = append(got, strings.Fields(c.Text)...)
	}
	var want []string
	for _, w := range words {
		want = append(want, w.Text)
	}
	require.Equal(t, want, got, "every word appears exactly once, in order")
}

// An imported file survives the trip into a record and back out as cues:
// the cue-level timing comes back within the 1ms the encoding can lose.
func TestImportedCuesRoundTrip(t *testing.T) {
	in := []captions.TimedCue{
		{Start: ms(1000), End: ms(3500), Text: "Welcome back to the stream."},
		{Start: ms(4000), End: ms(6000), Text: "Today: ünïcode, — and ♪ music ♪"},
		{Start: ms(70_000), End: ms(72_000), Text: "A cue after a long silence."},
	}
	chunks := ChunkAuthored(WordsFromCues(in), ChunkOptions{})
	require.Len(t, chunks, 1)
	var words []Word
	for _, c := range chunks {
		words = append(words, DecodeAuthored(c)...)
	}
	cues := AuthoredCues(words)
	require.Len(t, cues, len(in))
	for i, c := range cues {
		require.Equal(t, in[i].Text, c.Text)
		require.InDelta(t, in[i].Start.Milliseconds(), c.Start.Milliseconds(), 1)
		require.InDelta(t, in[i].End.Milliseconds(), c.End.Milliseconds(), 1)
	}
}

func TestLiveConversions(t *testing.T) {
	base := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	at := func(n int64) time.Time { return base.Add(ms(n)) }

	t.Run("words use their own timing", func(t *testing.T) {
		got := WordsFromCue(base, captions.Cue{
			Start: at(1000), End: at(2000), Text: "hello world",
			Words: []captions.Word{{Text: "hello", Start: at(1100), End: at(1400)}, {Text: "world", Start: at(1500), End: at(1950)}},
		})
		require.Equal(t, []Word{{"hello", 1100, 1400}, {"world", 1500, 1950}}, got)
	})
	t.Run("a cue without word timing is spread over its span", func(t *testing.T) {
		got := WordsFromCue(base, captions.Cue{Start: at(1000), End: at(2000), Text: "one two"})
		require.Equal(t, []Word{{"one", 1000, 1500}, {"two", 1500, 2000}}, got)
	})
	t.Run("an empty cue has no words", func(t *testing.T) {
		require.Empty(t, WordsFromCue(base, captions.Cue{Start: at(0), End: at(100), Text: " "}))
	})
	t.Run("sub-millisecond times round to the nearest millisecond", func(t *testing.T) {
		got := WordsFromCaptions(base, []captions.Word{{
			Text: "x", Start: base.Add(1_499_999 * time.Nanosecond), End: base.Add(2_500_000 * time.Nanosecond),
		}})
		require.Equal(t, []Word{{"x", 1, 3}}, got)
	})
	t.Run("a word before the base rounds the way Math.round does", func(t *testing.T) {
		require.Equal(t, int64(-1), durationMs(-1_500_000)) // -1.5ms -> -1
		require.Equal(t, int64(-2), durationMs(-1_500_001))
		require.Equal(t, int64(0), durationMs(-499_999))
	})
}

func ExampleCues() {
	words := Decode(Encode([]Word{{"Hello", 0, 400}, {"world.", 400, 1200}}))
	for _, c := range Cues(words, CueOptions{}) {
		fmt.Printf("%s %v-%v %q\n", c.ID, c.Start, c.End, c.Text)
	}
	// Output: 1 0s-1.2s "Hello world."
}

func TestCuesOptions(t *testing.T) {
	t.Run("a short cue is extended only up to the next cue", func(t *testing.T) {
		got := Cues([]Word{{"aaaa", 0, 300}, {"bbbb", 300, 600}}, CueOptions{MaxLineChars: 5, MaxLines: 1})
		require.Equal(t, []captions.TimedCue{
			{ID: "1", Start: 0, End: ms(300), Text: "aaaa"},
			{ID: "2", Start: ms(300), End: ms(1300), Text: "bbbb"},
		}, got)
	})
	t.Run("limits come from the options", func(t *testing.T) {
		got := Cues(talk(0, "a", "b", "c", "d"), CueOptions{MaxDuration: 2 * time.Second, MinDuration: time.Second})
		require.Equal(t, []captions.TimedCue{
			{ID: "1", Start: 0, End: ms(1200), Text: "a b c d"},
		}, got)
		got = Cues(talk(0, "a", "b", "c", "d", "e", "f", "g"), CueOptions{MaxDuration: 2 * time.Second, MinDuration: time.Second})
		require.Len(t, got, 2)
		require.Equal(t, "a b c d e f", got[0].Text)
	})
}
