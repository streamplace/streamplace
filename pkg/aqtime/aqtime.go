package aqtime

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// RE matches the canonical internal format: 2006-01-02T15:04:05.000Z
// It also accepts the file-safe variant with dashes/dots swapped, for backward compat.
var RE *regexp.Regexp
var Pattern string = `(\d\d\d\d)-(\d\d)-(\d\d)T(\d\d)(?:[:-])(\d\d)(?:[:-])(\d\d)(?:[.-])(\d\d\d)Z`

func init() {
	RE = regexp.MustCompile(fmt.Sprintf(`^%s$`, Pattern))
}

var fstr = "2006-01-02T15:04:05.000Z"

type AQTime string

// return a consistently formatted timestamp
func FromMillis(ms int64) AQTime {
	return AQTime(time.UnixMilli(ms).UTC().Format(fstr))
}

// return a consistently formatted timestamp
func FromSec(sec int64) AQTime {
	return AQTime(time.Unix(sec, 0).UTC().Format(fstr))
}

func FromString(str string) (AQTime, error) {
	// Reject -00:00 (valid RFC 3339 but disallowed by ATProto)
	if strings.HasSuffix(str, "-00:00") {
		return "", fmt.Errorf("bad time format, -00:00 timezone offset is not allowed, got=%s", str)
	}

	t, err := time.Parse(time.RFC3339Nano, str)
	if err != nil {
		// Fall back to file-safe variant (e.g. 2024-09-13T18-10-17-090Z)
		if bits := RE.FindStringSubmatch(str); bits != nil {
			if bits[2] < "01" || bits[2] > "12" || bits[3] < "01" || bits[3] > "31" ||
				bits[4] > "23" || bits[5] > "59" || bits[6] > "60" {
				return "", fmt.Errorf("bad time format, invalid date/time values in %s", str)
			}
			return AQTime(str), nil
		}
		return "", fmt.Errorf("bad time format: %w", err)
	}

	// Reject if UTC normalization results in a negative year
	utc := t.UTC()
	if utc.Year() < 0 {
		return "", fmt.Errorf("bad time format, datetime normalizes to negative year: %s", str)
	}

	// Normalize to canonical UTC millisecond format
	return AQTime(utc.Format(fstr)), nil
}

func FromTime(t time.Time) AQTime {
	return AQTime(t.UTC().Format(fstr))
}

// year, month, day, hour, min, sec, millisecond
func (aqt AQTime) Parts() (string, string, string, string, string, string, string) {
	bits := RE.FindStringSubmatch(aqt.String())
	return bits[1], bits[2], bits[3], bits[4], bits[5], bits[6], bits[7]
}

func (aqt AQTime) String() string {
	return string(aqt)
}

// version of AQTime suitable for saving as a file (esp on windows)
func (aqt AQTime) FileSafeString() string {
	str := string(aqt)
	str = strings.ReplaceAll(str, ":", "-")
	str = strings.ReplaceAll(str, ".", "-")
	return str
}

func (aqt AQTime) Time() time.Time {
	t, err := time.Parse(fstr, aqt.String())
	if err != nil {
		panic(err)
	}
	return t
}
