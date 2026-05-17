package viewlog

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// SaltStorage persists per-UTC-day HMAC salts. The localdb satisfies
// it; tests can substitute an in-memory implementation. Concrete
// methods are expected to handle "not present" by returning a nil
// salt + nil error so the manager can mint one on first use.
type SaltStorage interface {
	GetViewLogSalt(date string) ([]byte, error)
	PutViewLogSalt(date string, salt []byte) error
}

// SaltDateFormat is the canonical UTC date key used by the storage
// layer. Exposed so callers (e.g. retention prune) can format their
// own dates consistently.
const SaltDateFormat = "2006-01-02"

// SaltManager caches the per-day salt in memory; the first request of
// a UTC day mints a fresh 32-byte salt and persists it, every later
// request returns the cached value.
//
// Safe for concurrent use.
type SaltManager struct {
	storage SaltStorage

	mu    sync.Mutex
	cache map[string][]byte
}

func NewSaltManager(storage SaltStorage) *SaltManager {
	return &SaltManager{
		storage: storage,
		cache:   make(map[string][]byte),
	}
}

// Salt returns the salt for t's UTC date, minting + persisting one on
// the first call for a given day.
func (m *SaltManager) Salt(t time.Time) ([]byte, error) {
	date := t.UTC().Format(SaltDateFormat)
	m.mu.Lock()
	defer m.mu.Unlock()
	if salt, ok := m.cache[date]; ok {
		return salt, nil
	}
	salt, err := m.storage.GetViewLogSalt(date)
	if err != nil {
		return nil, fmt.Errorf("read view-log salt for %s: %w", date, err)
	}
	if salt == nil {
		salt = make([]byte, 32)
		if _, err := rand.Read(salt); err != nil {
			return nil, fmt.Errorf("generate view-log salt: %w", err)
		}
		if err := m.storage.PutViewLogSalt(date, salt); err != nil {
			return nil, fmt.Errorf("persist view-log salt for %s: %w", date, err)
		}
	}
	m.cache[date] = salt
	return salt, nil
}

// HashIP returns hex(HMAC-SHA256(daily_salt, ip)). Empty ip returns
// an empty string so callers can pass through whatever c.RealIP()
// gives them without a branch.
func (m *SaltManager) HashIP(ip string, t time.Time) (string, error) {
	if ip == "" {
		return "", nil
	}
	salt, err := m.Salt(t)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte(ip))
	return hex.EncodeToString(mac.Sum(nil)), nil
}
