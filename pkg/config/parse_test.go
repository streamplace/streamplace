package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSI(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"30", 30, false},
		{"30000000", 30_000_000, false},
		{"30k", 30_000, false},
		{"30K", 30_000, false},
		{"30000k", 30_000_000, false},
		{"30m", 30_000_000, false},
		{"30M", 30_000_000, false},
		{"1.5M", 1_500_000, false},
		{"2G", 2_000_000_000, false},
		{"1T", 1_000_000_000_000, false},
		{" 30M ", 30_000_000, false}, // surrounding whitespace
		{"30 M", 30_000_000, false},  // whitespace before suffix
		{"-5M", 0, true},             // negative
		{"abc", 0, true},             // not a number
		{"30X", 0, true},             // unknown suffix
		{"M", 0, true},               // suffix with no number
	} {
		got, err := ParseSI(tc.in)
		if tc.wantErr {
			require.Error(t, err, "input %q", tc.in)
			continue
		}
		require.NoError(t, err, "input %q", tc.in)
		require.Equal(t, tc.want, got, "input %q", tc.in)
	}
}
