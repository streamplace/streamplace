package statedb

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A rescheduled finalize ends the record only if the heartbeat it saw when
// rescheduling has not moved; a first pass, or a moved heartbeat, does not.
func TestHeartbeatFrozen(t *testing.T) {
	require.False(t, heartbeatFrozen(FinalizeLivestreamTask{}, "2026-09-16T08:43:39Z"), "first pass: give the heartbeat a window")
	require.True(t, heartbeatFrozen(FinalizeLivestreamTask{StaleSince: "2026-09-16T08:43:39Z"}, "2026-09-16T08:43:39Z"))
	require.False(t, heartbeatFrozen(FinalizeLivestreamTask{StaleSince: "2026-09-16T08:43:39Z"}, "2026-09-16T08:49:00Z"), "heartbeat moved: still live")
}
