package aqtime

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTimeFormat(t *testing.T) {
	aqt := FromMillis(1726251017090)
	require.Equal(t, "2024-09-13T18:10:17.090Z", aqt.String())
	require.Equal(t, "2024-09-13T18-10-17-090Z", aqt.FileSafeString())
	yr, mon, day, hr, min, sec, ms := aqt.Parts()
	require.Equal(t, "2024", yr)
	require.Equal(t, "09", mon)
	require.Equal(t, "13", day)
	require.Equal(t, "18", hr)
	require.Equal(t, "10", min)
	require.Equal(t, "17", sec)
	require.Equal(t, "090", ms)
}

func TestTimeParse(t *testing.T) {
	for _, str := range []string{"2024-09-13T18:10:17.090Z", "2024-09-13T18-10-17-090Z"} {
		aqt, err := FromString(str)
		require.NoError(t, err)
		yr, mon, day, hr, min, sec, ms := aqt.Parts()
		require.Equal(t, "2024", yr)
		require.Equal(t, "09", mon)
		require.Equal(t, "13", day)
		require.Equal(t, "18", hr)
		require.Equal(t, "10", min)
		require.Equal(t, "17", sec)
		require.Equal(t, "090", ms)
	}
}

// Valid ATProto datetime examples from the spec
// https://atproto.com/specs/lexicon#datetime
func TestATProtoValidCases(t *testing.T) {
	tests := []struct {
		input   string
		wantMs  string // expected millisecond portion after normalization
		wantHr  string // expected hour after UTC normalization
		wantMin string
	}{
		{"1985-04-12T23:20:50.123Z", "123", "23", "20"},
		{"1985-04-12T23:20:50.123456Z", "123", "23", "20"},
		{"1985-04-12T23:20:50.120Z", "120", "23", "20"},
		{"1985-04-12T23:20:50.120000Z", "120", "23", "20"},
		{"0001-01-01T00:00:00.000Z", "000", "00", "00"},
		{"0000-01-01T00:00:00.000Z", "000", "00", "00"},
		{"1985-04-12T23:20:50.12345678912345Z", "123", "23", "20"},
		{"1985-04-12T23:20:50Z", "000", "23", "20"},
		{"1985-04-12T23:20:50.0Z", "000", "23", "20"},
		{"1985-04-12T23:20:50.123+00:00", "123", "23", "20"},
		{"1985-04-12T23:20:50.123-07:00", "123", "06", "20"}, // 23+7=30 -> next day 06:20
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			aqt, err := FromString(tt.input)
			require.NoError(t, err, "input: %s", tt.input)
			_, _, _, hr, min, _, ms := aqt.Parts()
			require.Equal(t, tt.wantMs, ms, "millis mismatch for %s", tt.input)
			require.Equal(t, tt.wantHr, hr, "hour mismatch for %s", tt.input)
			require.Equal(t, tt.wantMin, min, "minute mismatch for %s", tt.input)
		})
	}
}

func TestBadCases(t *testing.T) {
	for _, str := range []string{
		// existing cases
		"prefix2024-09-13T18:10:17.090Z",
		"2024-09-13T18-10-17-090Zsuffix",
		"2024-09-13T18-10-17-090ZZZZ",
		"2024-09-13T18-10-17*090ZZZZ",
		// ATProto spec invalid examples
		"1985-04-12",
		"1985-04-12T23:20Z",
		"1985-04-12T23:20:5Z",
		"1985-04-12T23:20:50.123",
		"+001985-04-12T23:20:50.123Z",
		"23:20:50.123Z",
		"-1985-04-12T23:20:50.123Z",
		"1985-4-12T23:20:50.123Z",
		"01985-04-12T23:20:50.123Z",
		"1985-04-12T23:20:50.123+00",
		"1985-04-12T23:20:50.123+0000",
		// ISO-8601 strict capitalization
		"1985-04-12t23:20:50.123Z",
		"1985-04-12T23:20:50.123z",
		// RFC-3339, but not ISO-8601
		"1985-04-12T23:20:50.123-00:00",
		"1985-04-12 23:20:50.123Z",
		// timezone is required
		"1985-04-12T23:20:50.123",
		// syntax looks ok, but datetime is not valid
		"1985-04-12T23:99:50.123Z",
		"1985-00-12T23:20:50.123Z",
		// ISO-8601, but normalizes to a negative time
		"0000-01-01T00:00:00+01:00",
	} {
		t.Run(str, func(t *testing.T) {
			_, err := FromString(str)
			require.Error(t, err, "expected error for: %s", str)
		})
	}
}
