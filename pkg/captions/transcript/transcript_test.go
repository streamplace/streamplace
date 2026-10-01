package transcript

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The expectations in TestEncode mirror ionosphere's transcript-encoding.test.ts.
func TestEncode(t *testing.T) {
	tests := []struct {
		name  string
		words []Word
		want  Compact
	}{
		{
			name:  "empty",
			words: nil,
			want:  Compact{Timings: []int64{}},
		},
		{
			name: "contiguous words are all positive",
			words: []Word{
				{"hello", 0, 100}, {"world", 100, 200}, {"this", 200, 350},
				{"is", 350, 450}, {"a", 450, 500}, {"test", 500, 900},
			},
			want: Compact{Text: "hello world this is a test", Timings: []int64{100, 100, 150, 100, 50, 400}},
		},
		{
			name: "a pause becomes a negative entry",
			words: []Word{
				{"before", 1000, 1300}, {"pause", 1300, 1600},
				{"after", 3600, 3900}, {"pause", 3900, 4200},
			},
			want: Compact{Text: "before pause after pause", StartMs: 1000, Timings: []int64{300, 300, -2000, 300, 300}},
		},
		{
			name:  "gap before the first word is startMs, not an entry",
			words: []Word{{"hi", 5000, 5200}},
			want:  Compact{Text: "hi", StartMs: 5000, Timings: []int64{200}},
		},
		{
			name:  "zero-length word lasts 1ms but the cursor sits at its real end",
			words: []Word{{"a", 0, 0}, {"b", 0, 100}},
			want:  Compact{Text: "a b", Timings: []int64{1, 100}},
		},
		{
			name:  "zero-length word followed by a gap",
			words: []Word{{"a", 100, 100}, {"b", 150, 200}},
			want:  Compact{Text: "a b", StartMs: 100, Timings: []int64{1, -50, 50}},
		},
		{
			name:  "inverted word is clamped to 1ms",
			words: []Word{{"a", 100, 90}, {"b", 100, 150}},
			want:  Compact{Text: "a b", StartMs: 100, Timings: []int64{1, -10, 50}},
		},
		{
			name:  "overlap writes no gap and no negative duration",
			words: []Word{{"a", 0, 300}, {"b", 200, 500}},
			want:  Compact{Text: "a b", Timings: []int64{300, 300}},
		},
		{
			name:  "words are trimmed and blank words dropped",
			words: []Word{{" hello", 0, 100}, {"  ", 100, 150}, {"", 150, 160}, {"world\n", 160, 300}},
			want:  Compact{Text: "hello world", Timings: []int64{100, -60, 140}},
		},
		{
			name:  "a word with internal whitespace splits its span evenly",
			words: []Word{{"New York", 0, 400}, {"City", 400, 700}},
			want:  Compact{Text: "New York City", Timings: []int64{200, 200, 300}},
		},
		{
			name:  "punctuation-only tokens are words",
			words: []Word{{"wait", 0, 300}, {"—", 300, 400}, {"what?!", 400, 800}, {"...", 800, 900}},
			want:  Compact{Text: "wait — what?! ...", Timings: []int64{300, 100, 400, 100}},
		},
		{
			name:  "unicode text is stored as is",
			words: []Word{{"héllo", 0, 100}, {"世界", 100, 300}, {"🎉", 300, 400}, {"ß", 500, 600}},
			want:  Compact{Text: "héllo 世界 🎉 ß", Timings: []int64{100, 200, 100, -100, 100}},
		},
		{
			name:  "non-breaking and ideographic spaces separate tokens, as in JS",
			words: []Word{{"a\u00a0b\u3000c", 0, 300}},
			want:  Compact{Text: "a b c", Timings: []int64{100, 100, 100}},
		},
		{
			name:  "U+0085 is not whitespace in JS and stays inside its token",
			words: []Word{{"a\u0085b", 0, 100}},
			want:  Compact{Text: "a\u0085b", Timings: []int64{100}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, Encode(tt.words))
		})
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name string
		in   Compact
		want []Word
	}{
		{
			name: "pause and start offset",
			in:   Compact{Text: "before pause after pause", StartMs: 1000, Timings: []int64{300, 300, -2000, 300, 300}},
			want: []Word{{"before", 1000, 1300}, {"pause", 1300, 1600}, {"after", 3600, 3900}, {"pause", 3900, 4200}},
		},
		{
			name: "more positive entries than words: the extras are ignored",
			in:   Compact{Text: "a b", Timings: []int64{10, 10, 10, -5, 10}},
			want: []Word{{"a", 0, 10}, {"b", 10, 20}},
		},
		{
			name: "fewer entries than words: untimed words are dropped",
			in:   Compact{Text: "a b c", Timings: []int64{10, 10}},
			want: []Word{{"a", 0, 10}, {"b", 10, 20}},
		},
		{
			name: "leading and consecutive gaps accumulate",
			in:   Compact{Text: "a b", StartMs: 100, Timings: []int64{-50, -50, 10, -5, -5, 10}},
			want: []Word{{"a", 200, 210}, {"b", 220, 230}},
		},
		{
			name: "extra whitespace in text is ignored",
			in:   Compact{Text: "  a \n\n b  ", Timings: []int64{10, 10}},
			want: []Word{{"a", 0, 10}, {"b", 10, 20}},
		},
		{
			name: "zero duration entry still takes a word",
			in:   Compact{Text: "a b", Timings: []int64{0, 10}},
			want: []Word{{"a", 0, 0}, {"b", 0, 10}},
		},
		{
			name: "empty",
			in:   Compact{},
			want: []Word{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, Decode(tt.in))
		})
	}
}

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		words []Word
		// tolerance in ms on each boundary; 0 means exact
		tolerance int64
	}{
		{"contiguous", []Word{{"hello", 0, 100}, {"world", 100, 200}, {"again", 200, 650}}, 0},
		{"pauses", []Word{{"a", 1000, 1300}, {"b", 1300, 1600}, {"c", 3600, 3900}, {"d", 9000, 9010}}, 0},
		{"unicode and punctuation", []Word{{"¿Qué", 10, 400}, {"tal?", 400, 700}, {"—", 900, 950}, {"日本語。", 950, 2000}}, 0},
		// The 1ms minimum duration shifts everything after a zero-length word
		// only when the next word starts exactly where the empty one did.
		{"zero-length word", []Word{{"a", 0, 100}, {"uh", 100, 100}, {"b", 100, 200}}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decode(Encode(tt.words))
			require.Len(t, got, len(tt.words))
			for i, w := range tt.words {
				require.Equal(t, w.Text, got[i].Text)
				require.InDelta(t, w.StartMs, got[i].StartMs, float64(tt.tolerance), "start of %q", w.Text)
				require.InDelta(t, w.EndMs, got[i].EndMs, float64(tt.tolerance), "end of %q", w.Text)
			}
		})
	}
}

func TestChunk(t *testing.T) {
	// speech builds n words of 400ms each, one every 500ms (100ms gaps), with
	// a sentence end after every sentenceLen words.
	speech := func(n, sentenceLen int) []Word {
		var ws []Word
		for i := range n {
			text := "word"
			if sentenceLen > 0 && (i+1)%sentenceLen == 0 {
				text = "end."
			}
			ws = append(ws, Word{text, int64(i) * 500, int64(i)*500 + 400})
		}
		return ws
	}
	// flatten decodes chunks and concatenates them.
	flatten := func(cs []Compact) []Word {
		var out []Word
		for _, c := range cs {
			out = append(out, Decode(c)...)
		}
		return out
	}
	spanOf := func(c Compact) int64 {
		ws := Decode(c)
		return ws[len(ws)-1].EndMs - ws[0].StartMs
	}

	t.Run("short transcript is one chunk", func(t *testing.T) {
		cs := Chunk(speech(100, 10), ChunkOptions{})
		require.Len(t, cs, 1)
		require.Equal(t, Encode(speech(100, 10)), cs[0])
	})

	t.Run("empty", func(t *testing.T) {
		require.Empty(t, Chunk(nil, ChunkOptions{}))
	})

	t.Run("splits at a sentence end after the target span", func(t *testing.T) {
		in := speech(2000, 7) // 1000s of speech
		cs := Chunk(in, ChunkOptions{})
		require.Greater(t, len(cs), 2)
		for i, c := range cs[:len(cs)-1] {
			words := Decode(c)
			require.GreaterOrEqual(t, spanOf(c), int64(300_000), "chunk %d", i)
			require.Less(t, spanOf(c), int64(305_000), "chunk %d", i)
			require.Equal(t, "end.", words[len(words)-1].Text, "chunk %d ends on a sentence", i)
		}
		require.Equal(t, in, flatten(cs), "no word lost, none duplicated, timing intact")
	})

	t.Run("splits at a silence when nothing is punctuated", func(t *testing.T) {
		in := speech(1000, 0)
		// A two second pause after word 700 (350s in).
		for i := 701; i < len(in); i++ {
			in[i].StartMs += 2000
			in[i].EndMs += 2000
		}
		cs := Chunk(in, ChunkOptions{})
		require.Len(t, cs, 2)
		require.Len(t, Decode(cs[0]), 701)
		require.Equal(t, in, flatten(cs))
	})

	t.Run("splits before the word that would pass max span when there is no boundary at all", func(t *testing.T) {
		in := speech(2000, 0)
		cs := Chunk(in, ChunkOptions{})
		for _, c := range cs[:len(cs)-1] {
			require.Greater(t, spanOf(c), int64(359_000))
			require.LessOrEqual(t, spanOf(c), int64(360_000))
		}
		require.Equal(t, in, flatten(cs))
	})

	t.Run("a silence longer than max span is where the chunk ends", func(t *testing.T) {
		in := speech(5, 0)
		for _, w := range speech(5, 0) {
			in = append(in, Word{w.Text, w.StartMs + 7*60_000, w.EndMs + 7*60_000})
		}
		cs := Chunk(in, ChunkOptions{})
		require.Len(t, cs, 2)
		require.Len(t, Decode(cs[0]), 5)
		require.Equal(t, in, flatten(cs))
	})

	t.Run("custom span", func(t *testing.T) {
		in := speech(300, 5)
		cs := Chunk(in, ChunkOptions{TargetSpan: 30_000_000_000})
		require.Len(t, cs, 5)
		require.Equal(t, in, flatten(cs))
	})

	t.Run("text size limit", func(t *testing.T) {
		var in []Word
		for i := range 50 {
			in = append(in, Word{strings.Repeat("x", 9), int64(i) * 10, int64(i)*10 + 10})
		}
		cs := Chunk(in, ChunkOptions{MaxTextBytes: 100})
		require.Greater(t, len(cs), 4)
		for _, c := range cs {
			require.LessOrEqual(t, len(c.Text), 100)
		}
		require.Equal(t, in, flatten(cs))
	})

	t.Run("timings limit counts gaps too", func(t *testing.T) {
		var in []Word
		for i := range 100 {
			in = append(in, Word{"w", int64(i) * 100, int64(i)*100 + 50}) // every word after the first has a gap
		}
		cs := Chunk(in, ChunkOptions{MaxTimings: 20})
		for _, c := range cs {
			require.LessOrEqual(t, len(c.Timings), 20)
		}
		require.Equal(t, in, flatten(cs))
		require.Equal(t, 10, len(cs), "a chunk holds 10 words: one duration and one gap each, minus the leading gap")
	})

	t.Run("limits default to the lexicon's and cannot be raised", func(t *testing.T) {
		o := ChunkOptions{MaxTextBytes: 1 << 30, MaxTimings: 1 << 30}.withDefaults()
		require.Equal(t, MaxTextBytes, o.MaxTextBytes)
		require.Equal(t, MaxTimings, o.MaxTimings)
	})

	t.Run("record limits hold for a pathological transcript", func(t *testing.T) {
		var in []Word
		for i := range 60000 {
			in = append(in, Word{"ab", int64(i) * 10, int64(i)*10 + 5})
		}
		cs := Chunk(in, ChunkOptions{TargetSpan: 24 * 3600 * 1e9, MaxSpan: 24 * 3600 * 1e9})
		require.Greater(t, len(cs), 1)
		for _, c := range cs {
			require.LessOrEqual(t, len(c.Timings), MaxTimings)
			require.LessOrEqual(t, len(c.Text), MaxTextBytes)
		}
		require.Equal(t, in, flatten(cs))
	})

	t.Run("negative times are clamped to the time base", func(t *testing.T) {
		cs := Chunk([]Word{{"early", -500, -100}, {"on", -50, 200}}, ChunkOptions{})
		require.Len(t, cs, 1)
		require.Equal(t, int64(0), cs[0].StartMs)
		// "early" ends at the time base and so lasts the 1ms minimum, which is
		// the zero-length-word drift Encode documents.
		require.Equal(t, []Word{{"early", 0, 1}, {"on", 1, 201}}, Decode(cs[0]))
	})

	t.Run("an oversized token is cut on a rune boundary", func(t *testing.T) {
		cs := Chunk([]Word{{strings.Repeat("世", 10), 0, 100}}, ChunkOptions{MaxTextBytes: 10})
		require.Len(t, cs, 1)
		require.Equal(t, "世世世", cs[0].Text)
	})
}

func TestEndsSentence(t *testing.T) {
	for word, want := range map[string]bool{
		"done.": true, "what?": true, "wow!": true, "hmm…": true, `"quoted."`: true, "(aside.)": true,
		"日本語。": true, "ok": false, "e.g": false, "3.5": false, "—": false, "": false, `"`: false, "word,": false,
	} {
		require.Equal(t, want, endsSentence(word), "%q", word)
	}
}
