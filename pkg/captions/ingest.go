package captions

import (
	"strconv"
	"time"

	"stream.place/streamplace/pkg/captions/cea608"
)

// ingestCueHold is the provisional length of an ingest cue while it is on
// screen; the final cue carries the real end once the display changes.
const ingestCueHold = 4 * time.Second

// IngestTap turns the closed captions embedded in a stream's video
// (CEA-608/708 cc_data in the H264 SEI) into cues on the hub. It follows
// one channel: the first that shows text. Each display state becomes a
// cue, published as interim when it appears and as final once it changes.
type IngestTap struct {
	streamer string
	hub      *Hub
	dec      *cea608.Decoder
	track    Track
	publish  bool
	channel  cea608.Channel
	seen     bool
	seq      int
	open     *Cue
	// first is the media time of the first sample; cc_data times are
	// offsets from it.
	first    time.Time
	haveTime bool
}

// NewIngestTap starts a tap publishing on a track of the given origin and
// language (the hint, or "und"). Publishing is off until Publish(true).
func NewIngestTap(streamer string, hub *Hub, origin Origin, author, language string) *IngestTap {
	t := &IngestTap{streamer: streamer, hub: hub, dec: cea608.NewDecoder()}
	t.SetTrack(origin, author, language)
	return t
}

// SetTrack changes the track the cues go on, ending the open cue on the
// old one.
func (t *IngestTap) SetTrack(origin Origin, author, language string) {
	if language == "" {
		language = "und"
	}
	tr := Track{
		ID:       TrackID(origin, SourceIngest, language),
		Language: language,
		Kind:     KindCaptions,
		Source:   SourceIngest,
		Origin:   origin,
		Label:    "Captions",
		Author:   author,
	}
	if tr == t.track {
		return
	}
	if t.open != nil {
		t.closeOpen(t.open.Start.Add(ingestCueHold))
	}
	t.track = tr
}

// Publish turns publication on or off; decoding continues either way so
// Seen stays accurate. Turning it off ends the open cue.
func (t *IngestTap) Publish(on bool) {
	if t.publish && !on && t.open != nil {
		t.closeOpen(t.open.Start.Add(ingestCueHold))
	}
	t.publish = on
}

// Seen reports whether the video has carried caption text so far.
func (t *IngestTap) Seen() bool { return t.seen }

// Sample decodes one H264 access unit with four-byte AVC NAL lengths,
// presented at the given media time in presentation order. It reports
// whether the sample carried cc_data.
func (t *IngestTap) Sample(sample []byte, at time.Time) bool {
	cc := cea608.ExtractCCData(sample)
	if len(cc) == 0 {
		return false
	}
	if !t.haveTime {
		t.first, t.haveTime = at, true
	}
	t.apply(t.dec.Decode(cc, at.Sub(t.first)))
	return true
}

// Close ends the open displayed cue at the given time.
func (t *IngestTap) Close(at time.Time) {
	if t.open != nil {
		t.closeOpen(at)
	}
}

func (t *IngestTap) apply(events []cea608.Event) {
	for _, ev := range events {
		if t.channel == 0 {
			if ev.Text == "" {
				continue
			}
			t.channel = ev.Channel
		}
		if ev.Channel != t.channel {
			continue
		}
		when := t.first.Add(ev.Time)
		if ev.Text != "" {
			t.seen = true
		}
		if t.open != nil {
			t.closeOpen(when)
		}
		if ev.Text == "" {
			continue
		}
		t.open = &Cue{ID: "i" + strconv.Itoa(t.seq), Start: when, End: when.Add(ingestCueHold), Text: ev.Text}
		t.seq++
		if t.publish {
			t.hub.Publish(t.streamer, t.track, *t.open)
		}
	}
}

func (t *IngestTap) closeOpen(end time.Time) {
	c := *t.open
	t.open = nil
	if !end.After(c.Start) {
		end = c.Start.Add(100 * time.Millisecond)
	}
	c.End = end
	c.Final = true
	if t.publish {
		t.hub.Publish(t.streamer, t.track, c)
	}
}
