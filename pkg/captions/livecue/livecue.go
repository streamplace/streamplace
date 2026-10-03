// Package livecue renders caption hub events as the place.stream.caption.defs
// liveCue messages that the livestream websocket and the WHEP "captions" data
// channel carry, and the track views listTracks returns.
package livecue

import (
	"encoding/json"
	"time"

	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/placestream"
)

// Label is the WebRTC data channel label captions are delivered on.
const Label = "captions"

// JoinWindow is how much final caption history a joining viewer is sent: the
// current line, not the stream's backlog.
const JoinWindow = 10 * time.Second

// timeFormat is a lexicon datetime at millisecond precision.
const timeFormat = "2006-01-02T15:04:05.000Z"

// TrackView maps a caption track to its place.stream.caption.defs#trackView.
func TrackView(t captions.Track) placestream.CaptionDefs_TrackView {
	v := placestream.CaptionDefs_TrackView{
		Id:       t.ID,
		Language: t.Language,
		Source:   string(t.Source),
		Origin:   string(t.Origin),
	}
	if t.Kind != "" {
		k := string(t.Kind)
		v.Kind = &k
	}
	if t.Label != "" {
		v.Label = &t.Label
	}
	if t.Author != "" {
		v.Author = &t.Author
	}
	if t.Model != "" {
		v.Model = &t.Model
	}
	return v
}

// Cue maps a hub event to its place.stream.caption.defs#liveCue.
func Cue(ev captions.Event) placestream.CaptionDefs_LiveCue {
	c := placestream.CaptionDefs_LiveCue{
		LexiconTypeID: "place.stream.caption.defs#liveCue",
		Id:            ev.Cue.ID,
		Track:         TrackView(ev.Track),
		StartTime:     ev.Cue.Start.UTC().Format(timeFormat),
		EndTime:       ev.Cue.End.UTC().Format(timeFormat),
		Text:          ev.Cue.Text,
		Final:         ev.Cue.Final,
	}
	if ev.Streamer != "" {
		s := ev.Streamer
		c.Streamer = &s
	}
	return c
}

// Message is the JSON for an event: {"$type":"place.stream.caption.defs#liveCue",...}.
func Message(ev captions.Event) ([]byte, error) {
	c := Cue(ev)
	return json.Marshal(&c)
}

// Recent returns the final cues of every track of a streamer that ended within
// window before now, oldest first per track, as events: what a viewer joining
// mid-stream is sent to see the current line.
func Recent(hub *captions.Hub, streamer string, window time.Duration, now time.Time) []captions.Event {
	if hub == nil {
		return nil
	}
	var out []captions.Event
	for _, track := range hub.Tracks(streamer) {
		for _, cue := range hub.Cues(streamer, track.ID, now.Add(-window), now.Add(24*time.Hour)) {
			out = append(out, captions.Event{Streamer: streamer, Track: track, Cue: cue})
		}
	}
	return out
}
