package webvtt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func ms(n int64) time.Duration { return time.Duration(n) * time.Millisecond }

func TestEncodeVTT(t *testing.T) {
	cues := []Cue{
		{ID: "a", Start: ms(1000), End: ms(3500), Text: "Hello <world> & friends"},
		{Start: ms(3661001), End: ms(3662000), Text: "line one\n\n  line two  "},
		{Start: ms(5000), End: ms(6000), Text: "  \n "}, // nothing to show: dropped
		{ID: "x --> y\nz", Start: ms(7000), End: ms(8000), Text: "id sanitised"},
	}
	got := string(EncodeVTT(cues, &TimestampMap{MPEGTS: MPEGTSModulus + 900000}))
	require.Equal(t, "WEBVTT\n"+
		"X-TIMESTAMP-MAP=MPEGTS:900000,LOCAL:00:00:00.000\n"+
		"\n"+
		"a\n"+
		"00:00:01.000 --> 00:00:03.500\n"+
		"Hello &lt;world&gt; &amp; friends\n"+
		"\n"+
		"01:01:01.001 --> 01:01:02.000\n"+
		"line one\nline two\n"+
		"\n"+
		"x -> y z\n"+
		"00:00:07.000 --> 00:00:08.000\n"+
		"id sanitised\n"+
		"\n", got)
}

func TestEncodeVTTNoCuesIsValid(t *testing.T) {
	require.Equal(t, "WEBVTT\n\n", string(EncodeVTT(nil, nil)))
}

func TestEncodeSRT(t *testing.T) {
	got := string(EncodeSRT([]Cue{
		{Start: ms(0), End: ms(1999), Text: "one\r\ntwo"},
		{Start: ms(2000), End: ms(2500), Text: ""},
		{Start: ms(100*3600*1000 + 5), End: ms(100*3600*1000 + 1000), Text: "long clock"},
	}))
	require.Equal(t, "1\n00:00:00,000 --> 00:00:01,999\none\ntwo\n\n"+
		"2\n100:00:00,005 --> 100:00:01,000\nlong clock\n\n", got)
}

func TestEncodeJSON(t *testing.T) {
	epoch := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	got, err := EncodeJSON([]Cue{{ID: "c1", Start: ms(1500), End: ms(2500), Text: "hi\n\nthere"}}, epoch)
	require.NoError(t, err)
	require.JSONEq(t, `{"epoch":"2026-09-30T12:00:00Z","cues":[{"id":"c1","startMs":1500,"endMs":2500,"text":"hi\nthere"}]}`, string(got))

	got, err = EncodeJSON(nil, time.Time{})
	require.NoError(t, err)
	require.JSONEq(t, `{"cues":[]}`, string(got))
}

func TestTimeFormatting(t *testing.T) {
	require.Equal(t, "00:00:00.000", FormatVTTTime(-time.Second))
	require.Equal(t, "00:00:01.235", FormatVTTTime(1234600*time.Microsecond), "rounds to the nearest millisecond")
	require.Equal(t, "00:00:01,235", FormatSRTTime(1234600*time.Microsecond))
}

func TestParseVTT(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []Cue
	}{
		{
			name: "full file with BOM, CRLF, notes, style, region, settings, ids, multi-line",
			in: "\ufeffWEBVTT - a title\r\nKind: captions\r\nX-TIMESTAMP-MAP=MPEGTS:900000,LOCAL:00:00:00.000\r\n\r\n" +
				"STYLE\r\n::cue { color: red }\r\n\r\n" +
				"REGION\r\nid:r1\r\nwidth:40%\r\n\r\n" +
				"NOTE a comment\r\nthat spans lines\r\n\r\n" +
				"intro\r\n00:00:01.000 --> 00:00:02.500 line:90% align:start\r\nHello <b>there</b>\r\nsecond &amp; line\r\n\r\n" +
				"00:02.500 --> 00:03.000\r\n<v Roger>Short form</v>\r\n\r\n",
			want: []Cue{
				{ID: "intro", Start: ms(1000), End: ms(2500), Text: "Hello there\nsecond & line"},
				{Start: ms(2500), End: ms(3000), Text: "Short form"},
			},
		},
		{
			name: "bare CR line endings",
			in:   "WEBVTT\r\r00:00:01.000 --> 00:00:02.000\rhi\r",
			want: []Cue{{Start: ms(1000), End: ms(2000), Text: "hi"}},
		},
		{
			name: "cue timestamps tags and entities",
			in:   "WEBVTT\n\n00:00:00.000 --> 00:00:04.000\n<00:00:00.500><c>wo</c><00:00:01.000>rds&nbsp;&lt;ok&gt;\n",
			want: []Cue{{Start: 0, End: ms(4000), Text: "words\u00a0<ok>"}},
		},
		{
			name: "out of order cues are sorted; bad blocks skipped",
			in: "WEBVTT\n\n" +
				"00:00:05.000 --> 00:00:06.000\nlate\n\n" +
				"garbage block without timing\nsecond line\n\n" +
				"00:00:01.000 --> 00:00:00.500\nends before it starts\n\n" +
				"00:00:02.000 --> 00:00:03.000\n\n" + // no text
				"00:00:61.000 --> 00:00:62.000\nbad seconds\n\n" +
				"00:00:01.000 --> 00:00:02.000\nearly\n",
			want: []Cue{
				{Start: ms(1000), End: ms(2000), Text: "early"},
				{Start: ms(5000), End: ms(6000), Text: "late"},
			},
		},
		{
			name: "header only line and trailing text without final newline",
			in:   "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nno trailing newline",
			want: []Cue{{Start: ms(1000), End: ms(2000), Text: "no trailing newline"}},
		},
		{
			name: "NOTE with no space and identifier that looks like NOTE-ish text",
			in:   "WEBVTT\n\nNOTE\nonly a note\n\nNOTES\n00:00:01.000 --> 00:00:02.000\ncue named NOTES\n",
			want: []Cue{{ID: "NOTES", Start: ms(1000), End: ms(2000), Text: "cue named NOTES"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseVTT([]byte(tc.in))
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestParseVTTErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		err  error
	}{
		{"empty", "", ErrNotWebVTT},
		{"no header", "00:00:01.000 --> 00:00:02.000\nhi\n", ErrNotWebVTT},
		{"header lookalike", "WEBVTTX\n\n00:00:01.000 --> 00:00:02.000\nhi\n", ErrNotWebVTT},
		{"header only", "WEBVTT\n", ErrNoCues},
		{"only notes", "WEBVTT\n\nNOTE hi\n\nSTYLE\n::cue{}\n", ErrNoCues},
		{"only malformed cues", "WEBVTT\n\n00:01 --> nope\ntext\n", ErrNoCues},
		{"binary junk", "WEBVTT\n\n\x00\x01\x02\xff\xfe", ErrNoCues},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseVTT([]byte(tc.in))
			require.ErrorIs(t, err, tc.err)
		})
	}
}

func TestParseSRT(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []Cue
	}{
		{
			name: "canonical with BOM and CRLF",
			in:   "\ufeff1\r\n00:00:01,000 --> 00:00:02,500\r\nHello\r\nworld\r\n\r\n2\r\n00:00:03,000 --> 00:00:04,000\r\nBye\r\n",
			want: []Cue{
				{Start: ms(1000), End: ms(2500), Text: "Hello\nworld"},
				{Start: ms(3000), End: ms(4000), Text: "Bye"},
			},
		},
		{
			name: "missing blank line between cues keeps the next cue's number out of the text",
			in:   "1\n00:00:01,000 --> 00:00:02,000\nfirst\n2\n00:00:03,000 --> 00:00:04,000\nsecond\n",
			want: []Cue{
				{Start: ms(1000), End: ms(2000), Text: "first"},
				{Start: ms(3000), End: ms(4000), Text: "second"},
			},
		},
		{
			name: "dots, short fields, odd arrows, missing numbers, extra blank lines",
			in:   "\n\n\n00:00:01.5 -> 0:0:2.25\nrough\n\n\n\n3\n1:02:03,004 ---> 1:02:04,000\n<i>styled</i> &amp; fine\n",
			want: []Cue{
				{Start: ms(1500), End: ms(2250), Text: "rough"},
				{Start: ms(3723004), End: ms(3724000), Text: "styled &amp; fine"},
			},
		},
		{
			name: "formatting tags stripped but non-formatting tags and entities stay literal",
			in: "1\n00:00:01,000 --> 00:00:02,000\n" +
				`<I>italic</I> <b>bold</b> <u>underline</u> <font color="red">font</font> A & B > C <3 &amp; &#65; <widget>x</widget> {\an8}` + "\n",
			want: []Cue{{Start: ms(1000), End: ms(2000), Text: `italic bold underline font A & B > C <3 &amp; &#65; <widget>x</widget> {\an8}`}},
		},
		{
			name: "a number that is cue text stays text",
			in:   "1\n00:00:01,000 --> 00:00:02,000\nThe year was\n1984\n\n2\n00:00:03,000 --> 00:00:04,000\nnext\n",
			want: []Cue{
				{Start: ms(1000), End: ms(2000), Text: "The year was\n1984"},
				{Start: ms(3000), End: ms(4000), Text: "next"},
			},
		},
		{
			name: "out of order, end before start, empty text, settings ignored",
			in: "2\n00:00:05,000 --> 00:00:06,000 X1:100\nlate\n\n" +
				"3\n00:00:07,000 --> 00:00:06,000\nbackwards\n\n" +
				"4\n00:00:08,000 --> 00:00:09,000\n\n" +
				"1\n00:00:01,000 --> 00:00:02,000\nearly\n",
			want: []Cue{
				{Start: ms(1000), End: ms(2000), Text: "early"},
				{Start: ms(5000), End: ms(6000), Text: "late"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseSRT([]byte(tc.in))
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestParseSRTErrors(t *testing.T) {
	for name, in := range map[string]string{
		"empty":        "",
		"prose":        "this is not a caption file\nat all\n",
		"timing only":  "1\n00:00:01,000 --> 00:00:02,000\n",
		"invalid utf8": "1\n\xff\xfe\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseSRT([]byte(in))
			require.ErrorIs(t, err, ErrNoCues)
		})
	}
}

// Plain text round-trips; SRT formatting tags are interpreted, unlike escaped VTT.
func TestRoundTrip(t *testing.T) {
	in := []Cue{
		{ID: "1", Start: ms(1000), End: ms(2000), Text: "a < b && c > d\nsecond line"},
		{ID: "2", Start: ms(2000), End: ms(3500), Text: "a --> b"},
		{ID: "3", Start: ms(3500), End: ms(4000), Text: "<i>hello</i> &amp; &#65;"},
		{ID: "4", Start: ms(4000), End: ms(4500), Text: "A & B > C <3 <widget>literal</widget>"},
	}
	cues, err := ParseVTT(EncodeVTT(in, &TimestampMap{MPEGTS: 90000}))
	require.NoError(t, err)
	require.Equal(t, in, cues)

	cues, err = ParseSRT(EncodeSRT(in))
	require.NoError(t, err)
	// SRT carries no cue ids.
	wantSRT := []string{in[0].Text, in[1].Text, "hello &amp; &#65;", in[3].Text}
	for i := range in {
		require.Equal(t, wantSRT[i], cues[i].Text)
		require.Equal(t, in[i].Start, cues[i].Start)
		require.Equal(t, in[i].End, cues[i].End)
	}
}
