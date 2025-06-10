package atproto

import "sync"

// handleLocks provides per-handle synchronization
var handleLocks = struct {
	sync.Mutex
	locks map[string]*sync.Mutex
}{
	locks: make(map[string]*sync.Mutex),
}

// getHandleLock returns a mutex for the given handle
func getHandleLock(handle string) *sync.Mutex {
	handleLocks.Lock()
	defer handleLocks.Unlock()

	if lock, exists := handleLocks.locks[handle]; exists {
		return lock
	}

	lock := &sync.Mutex{}
	handleLocks.locks[handle] = lock
	return lock
}

// pdsLocks provides per-pds synchronization
var pdsLocks = struct {
	sync.Mutex
	locks map[string]*sync.Mutex
}{
	locks: make(map[string]*sync.Mutex),
}

// getpdsLock returns a mutex for the given pds
func getPDSLock(pds string) *sync.Mutex {
	pdsLocks.Lock()
	defer pdsLocks.Unlock()

	if lock, exists := pdsLocks.locks[pds]; exists {
		return lock
	}

	lock := &sync.Mutex{}
	pdsLocks.locks[pds] = lock
	return lock
}
