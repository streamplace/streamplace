package aqtime

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var RE *regexp.Regexp
var Pattern string = `^(\d\d\d\d)-(\d\d)-(\d\d)T(\d\d)(?:[:-])(\d\d)(?:[:-])(\d\d)(?:[.-])(\d\d\d)Z$`

type AQTime string

func init() {
	RE = regexp.MustCompile(fmt.Sprintf(`^%s$`, Pattern))
}

var fstr = "2006-01-02T15:04:05.000Z"

// return a consistently formatted timestamp
func FromMillis(ms int64) AQTime {
	return AQTime(time.UnixMilli(ms).UTC().Format(fstr))
}

// return a consistently formatted timestamp
func FromSec(sec int64) AQTime {
	return AQTime(time.Unix(sec, 0).UTC().Format(fstr))
}

func FromString(str string) (AQTime, error) {
	bits := RE.FindStringSubmatch(str)
	if bits == nil {
		return "", fmt.Errorf("bad time format, expected=%s got=%s", fstr, str)
	}
	return AQTime(str), nil
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
