package websocketrep

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// archive persists signed groups before live publication. Retention is explicit;
// disk pressure returns an error rather than silently evicting pending media.
// Group files contain timestampUs, expiry UnixNano (zero means none), then MUXL.
// A separate durable high-water mark prevents sequence reuse after expiration.
type archive struct {
	mu     sync.Mutex
	dir    string
	byHash map[[sha256.Size]byte]uint64
	groups map[uint64]archiveEntry
	byTime map[uint64]uint64
	next   uint64
}
type archiveEntry struct {
	hash        [sha256.Size]byte
	timestampUs uint64
	expiresAt   time.Time
}
type archivedGroup struct {
	seq         uint64
	timestampUs uint64
	payload     []byte
	expiresAt   time.Time
}

func newArchive(dir string) (*archive, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create MoQ archive: %w", err)
	}
	a := &archive{dir: dir, byHash: map[[sha256.Size]byte]uint64{}, groups: map[uint64]archiveEntry{}, byTime: map[uint64]uint64{}}
	water, err := os.ReadFile(filepath.Join(dir, "next"))
	if err == nil {
		if len(water) != 8 {
			return nil, fmt.Errorf("invalid MoQ archive high-water mark")
		}
		a.next = binary.BigEndian.Uint64(water)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".muxl") {
			continue
		}
		seq, err := strconv.ParseUint(strings.TrimSuffix(entry.Name(), ".muxl"), 10, 64)
		if err != nil {
			return nil, err
		}
		if seq == ^uint64(0) {
			return nil, fmt.Errorf("MoQ archive sequence exhausted")
		}
		group, err := a.read(seq)
		if err != nil {
			return nil, err
		}
		if other, ok := a.byTime[group.timestampUs]; ok && other != seq {
			return nil, fmt.Errorf("conflicting archived source timestamp %d", group.timestampUs)
		}
		hash := sha256.Sum256(group.payload)
		a.groups[seq] = archiveEntry{hash: hash, timestampUs: group.timestampUs, expiresAt: group.expiresAt}
		a.byHash[hash] = seq
		a.byTime[group.timestampUs] = seq
		if seq >= a.next {
			a.next = seq + 1
		}
	}
	// Persist recovered numbering before any pruning can remove its last evidence.
	if err := a.persistNext(a.next); err != nil {
		return nil, err
	}
	if err := a.prune(time.Now()); err != nil {
		return nil, err
	}
	return a, nil
}
func (a *archive) filename(seq uint64) string {
	return filepath.Join(a.dir, strconv.FormatUint(seq, 10)+".muxl")
}
func (a *archive) read(seq uint64) (*archivedGroup, error) {
	f, err := os.Open(a.filename(seq))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var h [16]byte
	if _, err := io.ReadFull(f, h[:]); err != nil {
		return nil, fmt.Errorf("read archived group %d timestamp: %w", seq, err)
	}
	payload, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	g := &archivedGroup{seq: seq, timestampUs: binary.BigEndian.Uint64(h[:8]), payload: payload}
	if expiry := int64(binary.BigEndian.Uint64(h[8:])); expiry != 0 {
		g.expiresAt = time.Unix(0, expiry)
	}
	return g, nil
}
func writeArchiveFile(dir, path string, header, payload []byte) error {
	f, err := os.CreateTemp(dir, ".group-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err = f.Write(header); err != nil {
		return err
	}
	if _, err = f.Write(payload); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return commitMoqArchive(f.Name(), path)
}
func (a *archive) persistNext(next uint64) error {
	var h [8]byte
	binary.BigEndian.PutUint64(h[:], next)
	return writeArchiveFile(a.dir, filepath.Join(a.dir, "next"), h[:], nil)
}
func (a *archive) putWithExpiry(seq, timestampUs uint64, payload []byte, expiry time.Time) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.putLocked(seq, timestampUs, payload, expiry)
}
func (a *archive) putLocked(seq, timestampUs uint64, payload []byte, expiry time.Time) error {
	hash := sha256.Sum256(payload)
	if old, ok := a.groups[seq]; ok {
		if old.hash != hash || old.timestampUs != timestampUs {
			return fmt.Errorf("MoQ archive sequence %d has conflicting signed bytes", seq)
		}
		if !old.expiresAt.IsZero() && !time.Now().Before(old.expiresAt) {
			return fmt.Errorf("MoQ archive retention already expired")
		}
		// Replays never extend retention; a more restrictive policy may shorten it.
		if expiry.IsZero() || (!old.expiresAt.IsZero() && !expiry.Before(old.expiresAt)) {
			return nil
		}
	}
	if other, ok := a.byTime[timestampUs]; ok && other != seq {
		return fmt.Errorf("conflicting archived source timestamp %d", timestampUs)
	}
	if seq == ^uint64(0) {
		return fmt.Errorf("MoQ archive sequence exhausted")
	}
	if !expiry.IsZero() && !time.Now().Before(expiry) {
		return fmt.Errorf("MoQ archive retention already expired")
	}
	if seq >= a.next {
		if err := a.persistNext(seq + 1); err != nil {
			return err
		}
		a.next = seq + 1
	}
	var h [16]byte
	binary.BigEndian.PutUint64(h[:8], timestampUs)
	if !expiry.IsZero() {
		binary.BigEndian.PutUint64(h[8:], uint64(expiry.UnixNano()))
	}
	if err := writeArchiveFile(a.dir, a.filename(seq), h[:], payload); err != nil {
		return err
	}
	a.groups[seq] = archiveEntry{hash: hash, timestampUs: timestampUs, expiresAt: expiry}
	a.byHash[hash] = seq
	a.byTime[timestampUs] = seq
	return nil
}
func (a *archive) assignWithExpiry(timestampUs uint64, payload []byte, expiry time.Time) (uint64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if seq, ok := a.byHash[sha256.Sum256(payload)]; ok {
		return seq, a.putLocked(seq, a.groups[seq].timestampUs, payload, expiry)
	}
	seq := a.next
	return seq, a.putLocked(seq, timestampUs, payload, expiry)
}
func (a *archive) get(seq uint64) (*archivedGroup, bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	entry, ok := a.groups[seq]
	if !ok || (!entry.expiresAt.IsZero() && !time.Now().Before(entry.expiresAt)) {
		return nil, false, nil
	}
	group, err := a.read(seq)
	if err != nil {
		return nil, false, fmt.Errorf("archived group %d disappeared or unreadable: %w", seq, err)
	}
	if sha256.Sum256(group.payload) != entry.hash {
		return nil, false, fmt.Errorf("archived group %d bytes changed", seq)
	}
	return group, true, nil
}
func (a *archive) sequenceAt(timestampUs uint64) (uint64, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	seq, ok := a.byTime[timestampUs]
	if ok {
		e := a.groups[seq]
		ok = e.expiresAt.IsZero() || time.Now().Before(e.expiresAt)
	}
	return seq, ok
}
func (a *archive) latestSequence() (uint64, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var latest uint64
	found := false
	for seq, e := range a.groups {
		if !e.expiresAt.IsZero() && !time.Now().Before(e.expiresAt) {
			continue
		}
		if !found || seq > latest {
			latest = seq
			found = true
		}
	}
	return latest, found
}
func (a *archive) prune(now time.Time) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for seq, e := range a.groups {
		if e.expiresAt.IsZero() || now.Before(e.expiresAt) {
			continue
		}
		if err := os.Remove(a.filename(seq)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		delete(a.groups, seq)
		delete(a.byTime, e.timestampUs)
		if a.byHash[e.hash] == seq {
			delete(a.byHash, e.hash)
		}
	}
	return nil
}
