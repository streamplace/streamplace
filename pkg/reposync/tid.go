package reposync

import (
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
)

// TIDForTime returns the TID that marks the instant t.
//
// TIDs are 13 characters of sortable base32 over (unix microseconds << 10 |
// clock id), so they sort chronologically and MST keys of the form
// "collection/<tid>" sort chronologically within a collection. That makes a
// time window a key range: everything a repo wrote to a TID-keyed collection
// since t lives in ["collection/"+TIDForTime(t), end of collection).
//
// The clock id is zero, which is the smallest one, so the result sorts at or
// below every real TID stamped at t and strictly above every TID stamped
// before it. A range starting here therefore cannot miss a record.
//
// Times before the unix epoch clamp to it: TIDs cannot express them, and the
// only sensible reading of "before 1970" for a repo window is "from the
// beginning".
func TIDForTime(t time.Time) string {
	micros := t.UTC().UnixMicro()
	if micros < 0 {
		micros = 0
	}
	return string(syntax.NewTID(micros, 0))
}

// TimeForTID is the inverse of [TIDForTime]: the wall-clock instant a TID
// encodes. It errors on anything that is not TID syntax, so a caller reading a
// watermark out of a database can tell a real timestamp from a stray string.
func TimeForTID(tid string) (time.Time, error) {
	parsed, err := syntax.ParseTID(tid)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.Time(), nil
}
