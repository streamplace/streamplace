package captions

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHubRejectsCrossOriginCollision(t *testing.T) {
	h := NewHub(0)
	now := time.Now()
	canonical := Track{ID: "canonical-auto-en", Origin: OriginCanonical, Source: SourceAuto, Language: "en"}
	h.Publish("alice", canonical, Cue{ID: "real", Text: "Real", Start: now, End: now.Add(time.Second), Final: true})
	forged := canonical
	forged.Origin = OriginSidecar
	h.Publish("alice", forged, Cue{ID: "forged", Text: "Forged", Start: now, End: now.Add(time.Second), Final: true})
	require.Equal(t, []Track{canonical}, h.Tracks("alice"))
	require.Equal(t, "Real", h.Cues("alice", canonical.ID, now, now.Add(time.Second))[0].Text)
	require.Len(t, h.Cues("alice", canonical.ID, now, now.Add(time.Second)), 1)
}

func TestHubAnonymousCuesPreserveShortGap(t *testing.T) {
	h := NewHub(0)
	now := time.Now()
	track := Track{ID: "canonical-ingest-en", Origin: OriginCanonical}
	h.PublishCanonical("alice", track, Cue{ID: "muxl-1", Text: "Yes", Start: now, End: now.Add(time.Second), Final: true})
	h.PublishCanonical("alice", track, Cue{ID: "muxl-2", Text: "Yes", Start: now.Add(1050 * time.Millisecond), End: now.Add(2 * time.Second), Final: true})
	require.Empty(t, h.Cues("alice", track.ID, now.Add(time.Second), now.Add(1050*time.Millisecond)), "an intentional subtitle gap is not filled")
}

func TestHubBoundsTrackAdmissionAndReleasesEndedStreams(t *testing.T) {
	h := NewHub(0)
	now := time.Now()
	for i := range 1000 {
		h.Publish("alice", Track{ID: fmt.Sprint(i), Origin: OriginSidecar}, Cue{ID: fmt.Sprint(i), Start: now, End: now.Add(time.Second)})
	}
	require.LessOrEqual(t, len(h.Tracks("alice")), 64)
	ctx, cancel := context.WithCancel(context.Background())
	events := h.Subscribe(ctx, "alice")
	h.EndSession("alice")
	cancel()
	for range events {
	}
	h.mu.Lock()
	_, exists := h.streams["alice"]
	h.mu.Unlock()
	require.False(t, exists, "ended streams with no remaining subscribers release their registry entry")
}
