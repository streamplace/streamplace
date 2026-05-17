package viewlog

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// memSaltStorage is the in-memory SaltStorage used by tests so they
// don't need a real localdb.
type memSaltStorage struct {
	mu      sync.Mutex
	salts   map[string][]byte
	getErr  error
	putErr  error
	getHits int
	putHits int
}

func newMemSaltStorage() *memSaltStorage {
	return &memSaltStorage{salts: make(map[string][]byte)}
}

func (m *memSaltStorage) GetViewLogSalt(date string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getHits++
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.salts[date], nil
}

func (m *memSaltStorage) PutViewLogSalt(date string, salt []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.putHits++
	if m.putErr != nil {
		return m.putErr
	}
	m.salts[date] = append([]byte(nil), salt...)
	return nil
}

func TestSaltManagerMintsAndCachesPerDay(t *testing.T) {
	storage := newMemSaltStorage()
	mgr := NewSaltManager(storage)

	day1 := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	salt1, err := mgr.Salt(day1)
	require.NoError(t, err)
	require.Len(t, salt1, 32)
	require.Equal(t, 1, storage.getHits, "first call reads storage")
	require.Equal(t, 1, storage.putHits, "first call persists the minted salt")

	// Second call same day, same UTC day → cached, no storage hits.
	salt1b, err := mgr.Salt(day1.Add(3 * time.Hour))
	require.NoError(t, err)
	require.Equal(t, salt1, salt1b)
	require.Equal(t, 1, storage.getHits, "subsequent same-day calls don't re-read storage")
	require.Equal(t, 1, storage.putHits, "subsequent same-day calls don't re-write storage")

	// Different day → new salt, new storage hit.
	day2 := day1.Add(24 * time.Hour)
	salt2, err := mgr.Salt(day2)
	require.NoError(t, err)
	require.NotEqual(t, salt1, salt2, "salt must change across days")
	require.Equal(t, 2, storage.getHits)
	require.Equal(t, 2, storage.putHits)
}

func TestSaltManagerReusesPersistedSalt(t *testing.T) {
	// Simulates a process restart: pre-populated storage, new manager
	// instance, same day → same salt comes back.
	storage := newMemSaltStorage()
	preset := []byte("00112233445566778899aabbccddeeff")
	require.NoError(t, storage.PutViewLogSalt("2026-05-17", preset))
	storage.putHits = 0 // reset to count subsequent writes only

	mgr := NewSaltManager(storage)
	day := time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)
	got, err := mgr.Salt(day)
	require.NoError(t, err)
	require.Equal(t, preset, got)
	require.Equal(t, 0, storage.putHits, "no write when storage already has the salt")
}

func TestHashIPSameIPSameDayCollides(t *testing.T) {
	mgr := NewSaltManager(newMemSaltStorage())
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)

	h1, err := mgr.HashIP("203.0.113.7", now)
	require.NoError(t, err)
	h2, err := mgr.HashIP("203.0.113.7", now.Add(2*time.Hour))
	require.NoError(t, err)
	require.Equal(t, h1, h2, "same IP within UTC day collides")

	other, err := mgr.HashIP("198.51.100.42", now)
	require.NoError(t, err)
	require.NotEqual(t, h1, other, "different IPs differ within a day")
}

func TestHashIPDifferentDayDecorrelates(t *testing.T) {
	mgr := NewSaltManager(newMemSaltStorage())
	day1 := time.Date(2026, 5, 17, 23, 30, 0, 0, time.UTC)
	day2 := day1.Add(2 * time.Hour) // crosses UTC midnight

	h1, err := mgr.HashIP("203.0.113.7", day1)
	require.NoError(t, err)
	h2, err := mgr.HashIP("203.0.113.7", day2)
	require.NoError(t, err)
	require.NotEqual(t, h1, h2, "same IP across days must produce different hashes")
}

func TestHashIPEmptyIPYieldsEmpty(t *testing.T) {
	mgr := NewSaltManager(newMemSaltStorage())
	got, err := mgr.HashIP("", time.Now())
	require.NoError(t, err)
	require.Empty(t, got, "no IP → no hash; callers can pass c.RealIP() straight through")
}
