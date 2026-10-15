package websocketrep

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMoqArchiveSurvivesRestartAndRejectsConflict(t *testing.T) {
	dir := t.TempDir()
	a, err := newArchive(dir)
	require.NoError(t, err)
	first := []byte("signed high group one")
	seq, err := a.assignWithExpiry(1234, first, time.Time{})
	require.NoError(t, err)
	require.Equal(t, uint64(0), seq)
	require.NoError(t, a.putWithExpiry(7, 5678, []byte("signed high group seven"), time.Time{}))

	reopened, err := newArchive(dir)
	require.NoError(t, err)
	group, found, err := reopened.get(0)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, first, group.payload)
	require.Equal(t, uint64(1234), group.timestampUs)
	seq, err = reopened.assignWithExpiry(9999, first, time.Time{})
	require.NoError(t, err)
	require.Equal(t, uint64(0), seq, "reconnect preserves sequence identity")
	seq, err = reopened.assignWithExpiry(9999, []byte("next signed high group"), time.Time{})
	require.NoError(t, err)
	require.Equal(t, uint64(8), seq)
	matched, ok := reopened.sequenceAt(5678)
	require.True(t, ok)
	require.Equal(t, uint64(7), matched, "renditions recover their source sequence after restart")
	require.ErrorContains(t, reopened.putWithExpiry(9, 5678, []byte("conflicting source"), time.Time{}), "conflicting archived source timestamp")
	require.ErrorContains(t, reopened.putWithExpiry(7, 5678, first, time.Time{}), "conflicting signed bytes")
	group, found, err = reopened.get(7)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, []byte("signed high group seven"), group.payload)
}

func TestMoqArchiveRefusesLostOrTruncatedData(t *testing.T) {
	a, err := newArchive(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, a.putWithExpiry(1, 123, []byte("signed high group"), time.Time{}))
	bytes, err := os.ReadFile(a.filename(1))
	require.NoError(t, err)
	bytes[len(bytes)-1] ^= 1
	require.NoError(t, os.WriteFile(a.filename(1), bytes, 0600))
	_, found, err := a.get(1)
	require.False(t, found)
	require.ErrorContains(t, err, "bytes changed")
	require.NoError(t, os.Remove(a.filename(1)))
	_, found, err = a.get(1)
	require.False(t, found)
	require.ErrorContains(t, err, "disappeared")
	require.NoError(t, os.WriteFile(a.filename(1), []byte{0}, 0600))
	_, err = newArchive(a.dir)
	require.ErrorContains(t, err, "timestamp")
}

func TestMoqArchiveExpiryDoesNotReuseSequences(t *testing.T) {
	dir := t.TempDir()
	a, err := newArchive(dir)
	require.NoError(t, err)
	expiry := time.Now().Add(time.Hour)
	require.NoError(t, a.putWithExpiry(7, 123, []byte("signed group"), expiry))
	require.NoError(t, a.putWithExpiry(7, 123, []byte("signed group"), expiry.Add(time.Hour)))
	group, found, err := a.get(7)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, group.expiresAt.Equal(expiry), "replay cannot extend retention")
	require.NoError(t, a.prune(expiry.Add(time.Second)))
	reopened, err := newArchive(dir)
	require.NoError(t, err)
	_, found, err = reopened.get(7)
	require.NoError(t, err)
	require.False(t, found)
	seq, err := reopened.assignWithExpiry(456, []byte("new signed group"), time.Time{})
	require.NoError(t, err)
	require.Equal(t, uint64(8), seq, "expired cursors cannot name future content")
}
