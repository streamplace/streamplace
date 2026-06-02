package atproto

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"stream.place/streamplace/pkg/config"
)

func TestFirehoseDeduperWithinWindow(t *testing.T) {
	d := newFirehoseDeduper(time.Minute)

	if d.seen("cid-a") {
		t.Fatal("first sighting of cid-a should not be a duplicate")
	}
	if !d.seen("cid-a") {
		t.Fatal("second sighting of cid-a should be a duplicate")
	}
	if d.seen("cid-b") {
		t.Fatal("first sighting of cid-b should not be a duplicate")
	}
	if !d.seen("cid-b") {
		t.Fatal("second sighting of cid-b should be a duplicate")
	}
}

func TestFirehoseDeduperForgetsAfterTwoWindows(t *testing.T) {
	const window = 20 * time.Millisecond
	d := newFirehoseDeduper(window)

	if d.seen("cid-a") {
		t.Fatal("first sighting should not be a duplicate")
	}

	// One full window: cid-a rotates from cur into prev (still remembered).
	time.Sleep(3 * window)
	if d.seen("cid-b") { // triggers the rotation; prev={cid-a}, cur={cid-b}
		t.Fatal("cid-b is new")
	}
	if !d.seen("cid-a") {
		t.Fatal("cid-a should still be remembered after a single window (lives in prev)")
	}

	// A second window with no sighting of cid-a drops it from both generations.
	time.Sleep(3 * window)
	if d.seen("cid-c") { // rotation: prev={cid-b,cid-a-promoted?}, cur={cid-c}
		t.Fatal("cid-c is new")
	}
	time.Sleep(3 * window)
	if d.seen("cid-d") { // another rotation drops the generation that held cid-a
		t.Fatal("cid-d is new")
	}
	if d.seen("cid-a") {
		t.Fatal("cid-a should have aged out after going unseen for two windows")
	}
}

// TestFirehoseDeduperConcurrent simulates the same commit arriving from many
// relays at once: exactly one caller must win (see it as new) so it is indexed
// exactly once.
func TestFirehoseDeduperConcurrent(t *testing.T) {
	d := newFirehoseDeduper(time.Minute)

	const goroutines = 200
	var newSightings atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if !d.seen("the-one-true-commit") {
				newSightings.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := newSightings.Load(); got != 1 {
		t.Fatalf("expected exactly one caller to see the commit as new, got %d", got)
	}
}

func TestRelayHostsDedupesAndAppendsSelf(t *testing.T) {
	cases := []struct {
		name      string
		relayHost string
		relaySelf bool
		httpAddr  string
		want      []string
	}{
		{
			name:      "single relay",
			relayHost: "wss://bsky.network",
			want:      []string{"wss://bsky.network"},
		},
		{
			name:      "comma separated with whitespace",
			relayHost: "wss://relay1.example, wss://relay2.example ,wss://relay1.example",
			want:      []string{"wss://relay1.example", "wss://relay2.example"},
		},
		{
			name:      "appends self",
			relayHost: "wss://bsky.network",
			relaySelf: true,
			httpAddr:  "127.0.0.1:39000",
			want:      []string{"wss://bsky.network", "ws://127.0.0.1:39000"},
		},
		{
			name:      "self already listed is not duplicated",
			relayHost: "wss://bsky.network,ws://127.0.0.1:39000",
			relaySelf: true,
			httpAddr:  "127.0.0.1:39000",
			want:      []string{"wss://bsky.network", "ws://127.0.0.1:39000"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			atsync := &ATProtoSynchronizer{CLI: &config.CLI{
				RelayHost: tc.relayHost,
				RelaySelf: tc.relaySelf,
				HTTPAddr:  tc.httpAddr,
			}}
			got := atsync.relayHosts()
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}
