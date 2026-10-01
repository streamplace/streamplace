// Package captions carries timed captions for live streams and videos from
// their sources (speech recognition, CEA-608/708 in ingest, pushed captions,
// upstream nodes) to their outputs (the canonical MUXL text track, transcript
// records, HLS WebVTT, and live cue events for websockets and WebRTC).
//
// Live cue times are absolute media times on the same wall clock as segment
// startTime, so a cue lines up with the segment whose time range contains it
// no matter which node or protocol delivers it.
package captions

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Source says who or what produced caption text.
type Source string

const (
	SourceAuto     Source = "auto"     // speech recognition
	SourceIngest   Source = "ingest"   // streamer-supplied at ingest: CEA-608/708 or pushCaptions
	SourceHuman    Source = "human"    // authored or corrected by a person
	SourceImported Source = "imported" // converted from an uploaded caption file
)

// Origin says how this node came to have a caption track.
type Origin string

const (
	OriginCanonical Origin = "canonical" // mastered into the streamer's signed stream
	OriginSidecar   Origin = "sidecar"   // node-generated and shared with syndication peers
	OriginLocal     Origin = "local"     // node-generated for this node's own viewers only
	OriginRecord    Origin = "record"    // read from place.stream.caption.transcript records
)

// Kind distinguishes same-language captions from translated subtitles.
type Kind string

const (
	KindCaptions  Kind = "captions"
	KindSubtitles Kind = "subtitles"
)

// Track describes one caption track of a stream or video.
type Track struct {
	ID       string
	Language string // BCP 47
	Kind     Kind
	Source   Source
	Origin   Origin
	Label    string
	Author   string // DID of the publishing account or node, when known
	Model    string // speech model, for SourceAuto
}

// TrackID builds the conventional track id for a live track. Ids only need to
// be unique per stream on one node; the convention keeps them readable in
// playlists and logs.
func TrackID(origin Origin, source Source, language string) string {
	return fmt.Sprintf("%s-%s-%s", origin, source, strings.ToLower(language))
}

// Word is one recognized word with its media time span.
type Word struct {
	Text  string
	Start time.Time
	End   time.Time
}

// Cue is a span of caption text. Interim cues may be republished with the
// same ID and new text; once a cue is published with Final set it never
// changes.
type Cue struct {
	ID    string
	Start time.Time
	End   time.Time
	Text  string
	Words []Word // optional word-level timing
	Final bool
}

// Event is one cue publication on a track.
type Event struct {
	Streamer string
	Track    Track
	Cue      Cue
}

// TimedCue is a final cue on a video's timeline, as an offset from the
// start of the video.
type TimedCue struct {
	ID    string
	Start time.Duration
	End   time.Duration
	Text  string
}

// VideoCaptions supplies the caption tracks of videos (VOD): text tracks
// mastered into the video's MUXL plus place.stream.caption.transcript
// records.
type VideoCaptions interface {
	Tracks(ctx context.Context, video string) ([]Track, error)
	Cues(ctx context.Context, video, trackID string) ([]TimedCue, error)
}
