package media

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A streamer has exactly one live ingest: claiming a second session for the
// same DID ends the first, and a displaced session's release can't disturb
// the session that replaced it.
func TestClaimIngestSessionEndsPrevious(t *testing.T) {
	mm := &MediaManager{}
	var ends1, ends2, ends3 int
	release1 := mm.ClaimIngestSession("did:test:a", func(reason string) { ends1++ })
	release2 := mm.ClaimIngestSession("did:test:a", func(reason string) { ends2++ })
	require.Equal(t, 1, ends1, "displaced session's end should fire exactly once")
	require.Equal(t, 0, ends2)
	release1() // already displaced — must not disturb session 2
	release3 := mm.ClaimIngestSession("did:test:a", func(reason string) { ends3++ })
	require.Equal(t, 1, ends2, "session 2 should be displaced by session 3")
	require.Equal(t, 0, ends3)
	release2() // already displaced — no-op
	release3()
	require.Equal(t, 0, ends3, "releasing the current session must not end it")
}

// Releasing a session deregisters it: the next claim finds no session to end.
func TestClaimIngestSessionReleaseDeregisters(t *testing.T) {
	mm := &MediaManager{}
	release := mm.ClaimIngestSession("did:test:a", func(reason string) {
		t.Fatal("end fired for a session that had already ended cleanly")
	})
	release()
	release2 := mm.ClaimIngestSession("did:test:a", func(reason string) {})
	release2()
}

func TestClaimIngestSessionIndependentStreamers(t *testing.T) {
	mm := &MediaManager{}
	releaseA := mm.ClaimIngestSession("did:test:a", func(reason string) {
		t.Fatal("streamer a's session ended by streamer b's claim")
	})
	releaseB := mm.ClaimIngestSession("did:test:b", func(reason string) {})
	releaseA()
	releaseB()
}
