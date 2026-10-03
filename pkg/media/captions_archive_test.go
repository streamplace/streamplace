package media

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCaptionArchiveClaimWaitsForItsGoP(t *testing.T) {
	ctx := context.Background()
	archive := newCaptionArchive(time.Hour)
	got := make(chan bool, 1)
	go func() {
		_, ok := archive.claim(ctx, 2000)
		got <- ok
	}()
	require.NoError(t, archive.put(archiveText{StartMs: 0}))
	select {
	case <-got:
		t.Fatal("another GoP's captions released the recording")
	case <-time.After(50 * time.Millisecond):
	}
	require.NoError(t, archive.put(archiveText{StartMs: 2000}))
	require.True(t, <-got)

	archive.finish()
	_, ok := archive.claim(ctx, 4000)
	require.False(t, ok, "a finished session never holds up the recording")
}
