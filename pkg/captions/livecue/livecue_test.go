package livecue

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/captions"
)

var now = time.Date(2026, 9, 30, 12, 0, 30, 0, time.UTC)

var enTrack = captions.Track{
	ID: "canonical-auto-en", Language: "en", Kind: captions.KindCaptions,
	Source: captions.SourceAuto, Origin: captions.OriginCanonical, Label: "English", Model: "base",
}

func TestMessageShape(t *testing.T) {
	bs, err := Message(captions.Event{
		Streamer: "did:plc:streamer",
		Track:    enTrack,
		Cue: captions.Cue{
			ID: "c1", Text: "hello", Final: true,
			Start: now.Add(1234567 * time.Microsecond), End: now.Add(2 * time.Second),
		},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{
		"$type": "place.stream.caption.defs#liveCue",
		"id": "c1",
		"streamer": "did:plc:streamer",
		"track": {"id":"canonical-auto-en","language":"en","kind":"captions","source":"auto","origin":"canonical","label":"English","model":"base"},
		"startTime": "2026-09-30T12:00:31.234Z",
		"endTime": "2026-09-30T12:00:32.000Z",
		"text": "hello",
		"final": true
	}`, string(bs))
}

func TestMessageInterimWithoutOptionalFields(t *testing.T) {
	bs, err := Message(captions.Event{
		Track: captions.Track{ID: "t", Language: "de", Source: captions.SourceIngest, Origin: captions.OriginSidecar},
		Cue:   captions.Cue{ID: "c", Text: "hallo", Start: now, End: now.Add(time.Second)},
	})
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(bs, &m))
	require.Equal(t, false, m["final"])
	require.NotContains(t, m, "streamer")
	require.Equal(t, map[string]any{"id": "t", "language": "de", "source": "ingest", "origin": "sidecar"}, m["track"])
}

func TestRecentOnlyCarriesTheCurrentLine(t *testing.T) {
	hub := captions.NewHub(0)
	pub := func(id string, endAgo time.Duration) {
		end := now.Add(-endAgo)
		hub.Publish("did:plc:s", enTrack, captions.Cue{ID: id, Start: end.Add(-2 * time.Second), End: end, Text: id, Final: true})
	}
	pub("old", 40*time.Second)
	pub("recent", 8*time.Second)
	pub("latest", time.Second)
	hub.Publish("did:plc:s", enTrack, captions.Cue{ID: "interim", Start: now, End: now.Add(time.Second), Text: "typing"})
	hub.Publish("did:plc:other", enTrack, captions.Cue{ID: "other", Start: now, End: now.Add(time.Second), Text: "x", Final: true})

	var ids []string
	for _, ev := range Recent(hub, "did:plc:s", JoinWindow, now) {
		require.Equal(t, "did:plc:s", ev.Streamer)
		require.Equal(t, enTrack, ev.Track)
		ids = append(ids, ev.Cue.ID)
	}
	require.Equal(t, []string{"recent", "latest"}, ids, "final cues of the last 10s only, oldest first")
	require.Empty(t, Recent(nil, "did:plc:s", JoinWindow, now))
}

// What a websocket forwards: hub events become liveCue JSON, interim and final.
func TestHubEventsBecomeMessages(t *testing.T) {
	hub := captions.NewHub(0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := hub.Subscribe(ctx, "did:plc:s")

	hub.Publish("did:plc:s", enTrack, captions.Cue{ID: "c", Text: "hel", Start: now, End: now.Add(time.Second)})
	hub.Publish("did:plc:s", enTrack, captions.Cue{ID: "c", Text: "hello", Start: now, End: now.Add(time.Second), Final: true})

	for _, want := range []struct {
		text  string
		final bool
	}{{"hel", false}, {"hello", true}} {
		select {
		case ev := <-events:
			bs, err := Message(ev)
			require.NoError(t, err)
			var got map[string]any
			require.NoError(t, json.Unmarshal(bs, &got))
			require.Equal(t, "place.stream.caption.defs#liveCue", got["$type"])
			require.Equal(t, want.text, got["text"])
			require.Equal(t, want.final, got["final"])
		case <-time.After(time.Second):
			t.Fatal("no event")
		}
	}
}
