package media

import (
	"sync"
	"time"
)

// hlsSessionTTL is how long an HLS playback session stays counted after its
// last request. A live player re-fetches the media playlist every target
// duration (~2s) and segments continuously, so a session silent this long is
// gone. Generous enough to ride out a stall/refresh without dropping to zero.
const hlsSessionTTL = 30 * time.Second

// hlsSessionSweepInterval is how often the janitor checks for idle sessions.
const hlsSessionSweepInterval = 5 * time.Second

type hlsSessionKey struct {
	streamer string
	sid      string
}

// hlsSessionTracker turns stateless HLS requests into viewer sessions: the
// first request carrying a given (streamer, sid) starts a session (onNew),
// and a session that goes idle for ttl ends it (onExpire). The two callbacks
// fire exactly once per session, in order, so they can safely drive an
// increment/decrement pair.
type hlsSessionTracker struct {
	ttl      time.Duration
	onNew    func(streamer string)
	onExpire func(streamer string)
	now      func() time.Time // clock, overridable in tests

	mu             sync.Mutex
	lastSeen       map[hlsSessionKey]time.Time
	janitorRunning bool
}

func newHLSSessionTracker(ttl time.Duration, onNew, onExpire func(streamer string)) *hlsSessionTracker {
	return &hlsSessionTracker{
		ttl:      ttl,
		onNew:    onNew,
		onExpire: onExpire,
		now:      time.Now,
		lastSeen: map[hlsSessionKey]time.Time{},
	}
}

// Touch records a request for (streamer, sid), starting a session if it's
// new. Empty sids are ignored — there's no session to attribute the request
// to.
func (t *hlsSessionTracker) Touch(streamer, sid string) {
	if streamer == "" || sid == "" {
		return
	}
	k := hlsSessionKey{streamer: streamer, sid: sid}
	t.mu.Lock()
	_, known := t.lastSeen[k]
	t.lastSeen[k] = t.now()
	startJanitor := false
	if !known && !t.janitorRunning {
		t.janitorRunning = true
		startJanitor = true
	}
	t.mu.Unlock()
	if !known {
		t.onNew(streamer)
	}
	if startJanitor {
		go t.janitor()
	}
}

// janitor sweeps periodically while any session exists, and exits once the
// tracker drains (the next Touch starts a fresh one).
func (t *hlsSessionTracker) janitor() {
	ticker := time.NewTicker(hlsSessionSweepInterval)
	defer ticker.Stop()
	for range ticker.C {
		if t.sweep() {
			return
		}
	}
}

// sweep expires sessions idle past the ttl and reports whether the tracker is
// now empty (meaning the calling janitor should exit).
func (t *hlsSessionTracker) sweep() bool {
	cutoff := t.now().Add(-t.ttl)
	t.mu.Lock()
	var expired []string
	for k, last := range t.lastSeen {
		if last.Before(cutoff) {
			delete(t.lastSeen, k)
			expired = append(expired, k.streamer)
		}
	}
	empty := len(t.lastSeen) == 0
	if empty {
		t.janitorRunning = false
	}
	t.mu.Unlock()
	for _, streamer := range expired {
		t.onExpire(streamer)
	}
	return empty
}

// TouchHLSSession marks HLS playback activity for one (streamer, sid)
// session. Called by the live HLS handlers on media-playlist and segment
// requests; the first touch counts the viewer, going idle uncounts them.
func (mm *MediaManager) TouchHLSSession(streamer, sid string) {
	mm.hlsSessions.Touch(streamer, sid)
}
