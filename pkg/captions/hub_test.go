package captions

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var t0 = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

func cue(id string, startSec, endSec float64, text string, final bool) Cue {
	return Cue{
		ID:    id,
		Start: t0.Add(time.Duration(startSec * float64(time.Second))),
		End:   t0.Add(time.Duration(endSec * float64(time.Second))),
		Text:  text,
		Final: final,
	}
}

func TestHubFinalCuesAreImmutableAndOrdered(t *testing.T) {
	h := NewHub(time.Minute)
	tr := Track{ID: TrackID(OriginLocal, SourceAuto, "en"), Language: "en", Source: SourceAuto, Origin: OriginLocal}

	h.Publish("did:plc:a", tr, cue("2", 2, 3, "world", false))
	h.Publish("did:plc:a", tr, cue("2", 2, 3, "world!", true))
	h.Publish("did:plc:a", tr, cue("1", 0, 1, "hello", true))
	h.Publish("did:plc:a", tr, cue("2", 2, 3, "rewritten", true))
	h.Publish("did:plc:a", tr, cue("3", 4, 5, "interim only", false))

	cues := h.Cues("did:plc:a", tr.ID, t0, t0.Add(time.Hour))
	require.Len(t, cues, 2, "interim cues are not part of the final window")
	require.Equal(t, "hello", cues[0].Text)
	require.Equal(t, "world!", cues[1].Text, "a final cue never changes")
}

func TestHubCuesOverlapWindow(t *testing.T) {
	h := NewHub(time.Minute)
	tr := Track{ID: "x"}
	h.Publish("s", tr, cue("a", 0, 2, "a", true))
	h.Publish("s", tr, cue("b", 2, 4, "b", true))
	h.Publish("s", tr, cue("c", 4, 6, "c", true))

	got := h.Cues("s", "x", t0.Add(2*time.Second), t0.Add(4*time.Second))
	require.Len(t, got, 1, "[from,to) excludes cues that only touch the edges")
	require.Equal(t, "b", got[0].Text)

	got = h.Cues("s", "x", t0.Add(1*time.Second), t0.Add(5*time.Second))
	require.Len(t, got, 3)
}

func TestHubRetentionPrunesOldFinalCues(t *testing.T) {
	h := NewHub(10 * time.Second)
	tr := Track{ID: "x"}
	h.Publish("s", tr, cue("old", 0, 1, "old", true))
	h.Publish("s", tr, cue("new", 30, 31, "new", true))
	got := h.Cues("s", "x", t0, t0.Add(time.Hour))
	require.Len(t, got, 1)
	require.Equal(t, "new", got[0].Text)
}

func TestHubSubscribeReceivesEventsAndCloses(t *testing.T) {
	h := NewHub(0)
	ctx, cancel := context.WithCancel(context.Background())
	ch := h.Subscribe(ctx, "s")
	h.Publish("s", Track{ID: "x"}, cue("a", 0, 1, "hi", false))
	ev := <-ch
	require.Equal(t, "hi", ev.Cue.Text)
	require.Equal(t, "x", ev.Track.ID)
	cancel()
	for range ch {
	}
}

func TestHubCanonicalAnonymousContinuationAndReplay(t *testing.T) {
	h := NewHub(time.Minute)
	tr := Track{ID: "canonical-ingest-en", Origin: OriginCanonical}
	h.PublishCanonical("s", tr, cue("muxl-9-100", 0.1, 1, "same line", true))
	h.PublishCanonical("s", tr, cue("muxl-9-1000", 1, 2, "same line", true))
	h.PublishCanonical("s", tr, cue("muxl-9-1000", 1, 2, "same line", true))
	h.PublishCanonical("s", tr, cue("muxl-9-3000", 3, 4, "same line", true))
	got := h.Cues("s", tr.ID, t0, t0.Add(time.Minute))
	require.Equal(t, []Cue{
		cue("muxl-9-100", 0.1, 2, "same line", true),
		cue("muxl-9-3000", 3, 4, "same line", true),
	}, got)
}
