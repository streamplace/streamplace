package muxl

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/test/remote"
)

func init() {
	stderrWriter = func(_ context.Context, _ uint64) io.Writer { return os.Stderr }
}

// TestConcatenatorRealSegments drives the wasm Concatenator with real
// signed flat MP4 segments and confirms it
// emits one segment event per input file (minus one held back until
// Close, which reaches flush() in the wasm).
//
// Reproduces the bug where SegCh stayed silent: concat used to treat the
// outer mdat envelope as opaque and drop all the inner moof+mdat
// fragments. Skipped if no segments are available locally.
func TestConcatenatorRealSegments(t *testing.T) {
	segPaths := []string{
		remote.RemoteFixture("507b7782c4a6855863a5d4c32cbac3e8fc026c9b635dd174d23c889b030dfc71/2026-05-08T21-55-06-837Z.mp4"),
		remote.RemoteFixture("fa42021f9fef60213d801f3521ef725e27c623899ab621a8615fa6107e27a997/2026-05-08T21-55-07-837Z.mp4"),
		remote.RemoteFixture("a8b98878338f3946b257e2b68f63157e1fe81a571d5e1954bee093db293c14d3/2026-05-08T21-55-08-838Z.mp4"),
		remote.RemoteFixture("2a57daaf03afecc45621ba25794cb87e30fb0a3ae7a18961a6f8045f937d8c4a/2026-05-08T21-55-09-836Z.mp4"),
		remote.RemoteFixture("5658128b0c766fb4b20e748ba834df7151c58c0be2f3801e7234d9736a687fc3/2026-05-08T21-55-10-837Z.mp4"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	concat := NewConcatenator(ctx)

	// Spin up consumers BEFORE writing so the producer doesn't block on a
	// full SegCh.
	var (
		initEvents int
		segEvents  int
		consumeWg  sync.WaitGroup
	)
	consumeWg.Add(2)
	go func() {
		defer consumeWg.Done()
		for range concat.InitCh {
			initEvents++
		}
	}()
	go func() {
		defer consumeWg.Done()
		for seg := range concat.SegCh {
			segEvents++
			t.Logf("segment event %d: %d bytes", segEvents, len(seg))
		}
	}()

	for i, p := range segPaths {
		data, err := os.ReadFile(p)
		require.NoError(t, err)
		t.Logf("writing segment %d (%s, %d bytes)", i+1, filepath.Base(p), len(data))
		require.NoError(t, concat.Write(data))
	}

	require.NoError(t, concat.Close())
	consumeWg.Wait()

	require.Equal(t, 1, initEvents, "expected exactly one init event")
	require.Equal(t, len(segPaths), segEvents,
		"expected one segment event per input file (got %d for %d inputs)",
		segEvents, len(segPaths))
}
