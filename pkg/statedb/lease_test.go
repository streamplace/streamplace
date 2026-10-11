package statedb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A lease is held by one node at a time: a second asker is refused while
// it is live, gets it once it has expired or been released, and the first
// holder's next renewal then fails instead of silently sharing the job.
func TestLease(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		const name = "stream:record:did:plc:a"
		held, err := state.AcquireLease(name, "did:web:a", time.Minute)
		require.NoError(t, err)
		require.True(t, held)

		held, err = state.AcquireLease(name, "did:web:b", time.Minute)
		require.NoError(t, err)
		require.False(t, held, "a live lease is exclusive")

		held, err = state.AcquireLease(name, "did:web:a", time.Minute)
		require.NoError(t, err)
		require.True(t, held, "the holder renews")

		require.NoError(t, state.ReleaseLease(name, "did:web:b"))
		held, err = state.AcquireLease(name, "did:web:b", time.Minute)
		require.NoError(t, err)
		require.False(t, held, "only the holder can release")

		require.NoError(t, state.ReleaseLease(name, "did:web:a"))
		held, err = state.AcquireLease(name, "did:web:b", time.Minute)
		require.NoError(t, err)
		require.True(t, held, "released, so the next asker takes it")

		held, err = state.AcquireLease(name, "did:web:a", time.Minute)
		require.NoError(t, err)
		require.False(t, held, "the old holder has lost it")

		// Expiry: a holder that stops renewing loses the lease to the
		// next asker once its TTL has passed.
		held, err = state.AcquireLease(name, "did:web:b", -time.Millisecond)
		require.NoError(t, err)
		require.True(t, held)
		held, err = state.AcquireLease(name, "did:web:a", time.Minute)
		require.NoError(t, err)
		require.True(t, held, "an expired lease is up for grabs")

		// Leases are independent of each other.
		held, err = state.AcquireLease("stream:transcode:did:plc:a", "did:web:b", time.Minute)
		require.NoError(t, err)
		require.True(t, held)
	})
}
