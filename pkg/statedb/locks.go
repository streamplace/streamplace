package statedb

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"
)

func (state *StatefulDB) GetNamedLock(name string) (func(), error) {
	switch state.Type {
	case DBTypeSQLite:
		return state.getNamedLockSQLite(name)
	case DBTypePostgres:
		return state.getNamedLockPostgres(name)
	}
	panic("unsupported database type")
}

func (state *StatefulDB) getNamedLockPostgres(name string) (func(), error) {
	// we also use a local lock here - whoever is locking wants exclusive access even within the node
	lock := state.locks.GetLock(name)
	lock.Lock()
	// Convert string to sha256 hash and use decimal value for advisory lock
	h := sha256.Sum256([]byte(name))
	nameInt := int64(binary.BigEndian.Uint64(h[:8]))

	err := state.DB.Exec("SELECT pg_advisory_lock($1)", nameInt).Error
	if err != nil {
		lock.Unlock()
		return nil, err
	}
	return func() {
		err := state.DB.Exec("SELECT pg_advisory_unlock($1)", nameInt).Error
		lock.Unlock()
		if err != nil {
			// unfortunate, but the risk is that we're holding on to the lock forever,
			// so it's responsible to crash in this case
			panic(fmt.Errorf("error unlocking named lock: %w", err))
		}
	}, nil
}

func (state *StatefulDB) getNamedLockSQLite(name string) (func(), error) {
	lock := state.locks.GetLock(name)
	lock.Lock()
	return func() {
		lock.Unlock()
	}, nil
}

// Local mutex implementation for sqlite
type NamedLocks struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// NewNamedLocks creates a new NamedLocks instance
func NewNamedLocks() *NamedLocks {
	return &NamedLocks{
		locks: make(map[string]*sync.Mutex),
	}
}

// GetLock returns the mutex for the given name, creating it if it doesn't exist
func (n *NamedLocks) GetLock(name string) *sync.Mutex {
	n.mu.Lock()
	defer n.mu.Unlock()

	lock, exists := n.locks[name]
	if !exists {
		lock = &sync.Mutex{}
		n.locks[name] = lock
	}
	return lock
}
