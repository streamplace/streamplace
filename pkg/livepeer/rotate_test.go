package livepeer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/renditions"
)

// A gateway that has fallen into "No sessions available" for a manifest
// stays there while pushes keep coming under it; after RotateAfterFailures
// refusals in a row the session moves to a fresh manifest, whose sequence
// starts at 0 again, and that one is taken.
func TestRotatesManifestAfterRefusals(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	seg, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "test", "fixtures", "h264-opus-frag.mp4"))
	require.NoError(t, err)

	var mu sync.Mutex
	var seen []string // manifest/seq per push
	var stuck string  // the first manifest: refused forever
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		// /live/<manifest>/<seq>.ts
		rest := r.URL.Path[len("/live/"):]
		seen = append(seen, rest)
		manifest := rest[:len(rest)-len(filepath.Base(rest))-1]
		if stuck == "" {
			stuck = manifest
		}
		if manifest == stuck {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("No sessions available\n"))
			return
		}
		w.Header().Set("Content-Type", "multipart/mixed; boundary=x")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("--x--\r\n")) // no parts
	}))
	defer gw.Close()

	ctx := context.Background()
	ls, err := NewLivepeerSession(ctx, &config.CLI{}, "did:plc:test", gw.URL)
	require.NoError(t, err)
	first := ls.SessionID
	dur := int64(1e9)
	spseg := &placestream.Segment{Creator: "did:plc:test", Duration: &dur, Video: []placestream.Segment_Video{{Width: 320, Height: 240}}}
	rs := renditions.Renditions{{Name: "160p", Width: 284, Height: 160, Bitrate: 250000}}

	for i := 0; i < RotateAfterFailures; i++ {
		_, err := ls.PostSegmentToGateway(ctx, seg, spseg, rs)
		require.Error(t, err, "push %d is refused", i)
		require.Contains(t, err.Error(), "No sessions available")
	}
	require.NotEqual(t, first, ls.SessionID, "moved to a fresh manifest")
	require.Equal(t, 0, ls.Count, "the sequence restarts with it")

	_, err = ls.PostSegmentToGateway(ctx, seg, spseg, rs)
	require.NoError(t, err, "the fresh manifest is taken")
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, seen, RotateAfterFailures+1)
	require.Equal(t, first+"-1ren/0.ts", seen[0])
	require.Equal(t, first+"-1ren/4.ts", seen[RotateAfterFailures-1])
	require.Equal(t, ls.SessionID+"-1ren/0.ts", seen[RotateAfterFailures])
}

func TestNoteResultResetsOnSuccess(t *testing.T) {
	ls, err := NewLivepeerSession(context.Background(), &config.CLI{}, "did:plc:test", "http://gw")
	require.NoError(t, err)
	first := ls.SessionID
	for i := 0; i < RotateAfterFailures-1; i++ {
		ls.noteResult(context.Background(), false)
	}
	ls.noteResult(context.Background(), true)
	for i := 0; i < RotateAfterFailures-1; i++ {
		ls.noteResult(context.Background(), false)
	}
	require.Equal(t, first, ls.SessionID, "a success in between resets the run")
	ls.noteResult(context.Background(), false)
	require.NotEqual(t, first, ls.SessionID)
}
