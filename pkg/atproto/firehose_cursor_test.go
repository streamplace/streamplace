package atproto

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/model"
)

func TestRelayCursorResume(t *testing.T) {
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	atsync := &ATProtoSynchronizer{Model: mod}
	ctx := context.Background()
	const host = "wss://relay.example"

	// A fresh relay has no stored cursor, so we dial with none and tail live
	// rather than backfilling the relay's whole history.
	rc := atsync.newRelayCursor(ctx, host)
	if _, ok := rc.param(); ok {
		t.Fatal("fresh relay should not send a cursor")
	}

	// observe tracks the high-water mark and never regresses on out-of-order
	// frames (the parallel scheduler can surface them out of sequence).
	rc.observe(100)
	rc.observe(50)
	rc.observe(120)
	if v, ok := rc.param(); !ok || v != 120 {
		t.Fatalf("expected cursor 120, got %d (ok=%v)", v, ok)
	}

	// flush persists the high-water mark to the index DB.
	rc.flush(ctx)
	stored, err := mod.GetRelayCursor(host)
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, int64(120), stored.Cursor)

	// A second flush with no advance is a no-op, and the stored value is stable.
	rc.observe(120)
	rc.flush(ctx)
	stored, err = mod.GetRelayCursor(host)
	require.NoError(t, err)
	require.Equal(t, int64(120), stored.Cursor)

	// A new cursor (as if the process restarted) resumes from the stored value.
	resumed := atsync.newRelayCursor(ctx, host)
	v, ok := resumed.param()
	require.True(t, ok)
	require.Equal(t, int64(120), v)

	// Cursors are independent per relay.
	other := atsync.newRelayCursor(ctx, "wss://other.example")
	if _, ok := other.param(); ok {
		t.Fatal("a different relay must not inherit another relay's cursor")
	}
}
