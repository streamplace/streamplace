package statedb

import (
	"testing"
	"time"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
)

// CompleteQRLogin is the race guard behind completeQRLogin's status check:
// whatever the caller saw, only a pending, unexpired attempt completes, once.
func TestCompleteQRLogin(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		create := func(id string, expiresAt time.Time) {
			require.NoError(t, state.CreateQRLogin(ctx, &QRLogin{
				ID:        id,
				Status:    QRLoginStatusPending,
				CreatedAt: time.Now().UTC(),
				ExpiresAt: expiresAt.UTC(),
			}))
		}
		auth := &oatproxy.AuthorizationResponse{Code: "code", State: "state", Iss: "https://stream.example"}

		create("live", time.Now().Add(time.Minute))
		ok, err := state.CompleteQRLogin(ctx, "live", "did:plc:first", auth)
		require.NoError(t, err)
		require.True(t, ok)
		ok, err = state.CompleteQRLogin(ctx, "live", "did:plc:second", nil)
		require.NoError(t, err)
		require.False(t, ok)
		l, err := state.GetQRLogin(ctx, "live")
		require.NoError(t, err)
		require.Equal(t, QRLoginStatusApproved, l.Status)
		require.Equal(t, "did:plc:first", l.ApproverDID)
		require.Equal(t, "code", l.Code)

		create("expired", time.Now().Add(-time.Second))
		ok, err = state.CompleteQRLogin(ctx, "expired", "did:plc:first", auth)
		require.NoError(t, err)
		require.False(t, ok)
		l, err = state.GetQRLogin(ctx, "expired")
		require.NoError(t, err)
		require.Equal(t, QRLoginStatusExpired, l.CurrentStatus())
		require.Empty(t, l.Code)
	})
}
