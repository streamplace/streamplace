package config

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseSI parses an integer written with an optional decimal SI suffix into its
// full value: k/K = 1e3, M = 1e6, G = 1e9, T = 1e12. Suffixes are DECIMAL
// (1k = 1000), matching bitrate/throughput convention — not binary (1 KiB).
// A bare number is taken verbatim ("30000000"), a decimal mantissa is allowed
// ("1.5M" => 1500000), surrounding/internal whitespace is ignored ("30 M"), and
// an empty string is 0. Negative values and unknown suffixes are errors.
//
// It's the shared helper behind human-friendly bits- or bytes-valued flags
// (e.g. --maximum-live-bitrate accepts "30M"); reuse it for future size flags.
func ParseSI(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	mult := int64(1)
	switch s[len(s)-1] {
	case 'k', 'K':
		mult = 1_000
	case 'm', 'M':
		mult = 1_000_000
	case 'g', 'G':
		mult = 1_000_000_000
	case 't', 'T':
		mult = 1_000_000_000_000
	}
	num := s
	if mult != 1 {
		num = strings.TrimSpace(s[:len(s)-1])
	}
	f, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a number with an optional k/M/G/T suffix", s)
	}
	if f < 0 {
		return 0, fmt.Errorf("%q must not be negative", s)
	}
	return int64(math.Round(f * float64(mult))), nil
}
