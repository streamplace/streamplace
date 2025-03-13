package thumbnail

import "sync"

var thumbnailLocks = struct {
	sync.Mutex
	locks map[string]*sync.Mutex
}{
	locks: make(map[string]*sync.Mutex),
}

// GetThumbnailLock returns a mutex for the given user
func GetThumbnailLock(handle string) *sync.Mutex {
	thumbnailLocks.Lock()
	defer thumbnailLocks.Unlock()

	if lock, exists := thumbnailLocks.locks[handle]; exists {
		return lock
	}

	lock := &sync.Mutex{}
	thumbnailLocks.locks[handle] = lock
	return lock
}
