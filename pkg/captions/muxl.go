package captions

import (
	"bytes"
	"context"
	"fmt"
	"time"

	upstream "github.com/streamplace/muxl/go"
	"stream.place/streamplace/pkg/captions/fmp4"
	"stream.place/streamplace/pkg/muxl"
)

// CanonicalTrack decodes the fixed MUXL language/source convention.
func CanonicalTrack(t upstream.TextTrack, author string) Track {
	lang := t.Language
	if lang == "" {
		lang = "und"
	}
	source := Source(t.Label)
	switch source {
	case SourceAuto, SourceIngest, SourceHuman:
	default:
		source = SourceIngest
	}
	return Track{ID: TrackID(OriginCanonical, source, lang), Language: lang, Kind: KindCaptions, Source: source, Origin: OriginCanonical, Author: author}
}

// SegmentClock reads the reference AV decode time without a wasm invocation.
// header is the already synthesized presentation header; segment stays canonical.
func SegmentClock(header, segment []byte) (time.Duration, bool, error) {
	tracks, err := fmp4.Tracks(header)
	if err != nil {
		return 0, false, err
	}
	hasText := false
	var ref fmp4.TrackInfo
	for _, t := range tracks {
		switch t.Handler {
		case "text", "sbtl", "subt":
			hasText = true
		case "vide":
			ref = t
		case "soun":
			if ref.ID == 0 {
				ref = t
			}
		}
	}
	if !hasText {
		return 0, false, nil
	}
	frags, err := fmp4.Fragments(segment)
	if err != nil {
		return 0, true, err
	}
	for _, f := range frags {
		if f.TrackID == ref.ID && ref.Timescale != 0 {
			return time.Duration(f.BaseDecodeTime/uint64(ref.Timescale))*time.Second + time.Duration(f.BaseDecodeTime%uint64(ref.Timescale))*time.Second/time.Duration(ref.Timescale), true, nil
		}
	}
	return 0, true, fmt.Errorf("caption reference track missing")
}

// ReadCanonicalWithClock reads cues after admission has classified the segment.
func ReadCanonicalWithClock(ctx context.Context, segment []byte, media time.Duration, wall time.Time, author string) ([]Event, error) {
	tracks, err := muxl.RunMuxlTextTracks(ctx, bytes.NewReader(segment))
	if err != nil {
		return nil, err
	}
	var out []Event
	for _, t := range tracks {
		cues, err := muxl.RunMuxlReadTextCues(ctx, bytes.NewReader(segment), t.TrackID)
		if err != nil {
			return nil, err
		}
		track := CanonicalTrack(t, author)
		for _, c := range cues {
			if c.Text == "" || c.End <= c.Start {
				continue
			}
			id := c.ID
			if id == "" {
				id = fmt.Sprintf("muxl-%d-%d", t.TrackID, c.Start)
			}
			out = append(out, Event{Track: track, Cue: Cue{ID: id, Text: c.Text, Start: wall.Add(time.Duration(c.Start)*time.Millisecond - media), End: wall.Add(time.Duration(c.End)*time.Millisecond - media), Final: true}})
		}
	}
	return out, nil
}
