package media

import (
	"sync"

	"stream.place/streamplace/pkg/spmetrics"
)

// ingestSession is one streamer's registered live ingest: how to end it (ctx
// cancel for an in-process pipeline, process kill for a detached worker).
type ingestSession struct {
	id  uint64
	end func(reason string)
}

// ClaimIngestSession establishes did's single live ingest session, ending any
// previous one. A streamer has exactly one live ingest: a second push for the
// same DID is a stale/duplicate connection (an OBS reconnect whose old session
// is still lingering server-side, or a stray second encoder), and running both
// mints two segment streams into one stream — duplicate ingest work at best,
// interleaved media timelines for realtime consumers at worst. The returned
// release deregisters the session when it ends; it's a no-op if the session
// was already replaced by a newer one.
func (mm *MediaManager) ClaimIngestSession(did string, end func(reason string)) (release func()) {
	mm.ingestSessionsMu.Lock()
	if mm.ingestSessions == nil {
		mm.ingestSessions = map[string]*ingestSession{}
	}
	s := &ingestSession{id: mm.nextIngestSession(), end: end}
	prev := mm.ingestSessions[did]
	mm.ingestSessions[did] = s
	mm.ingestSessionsMu.Unlock()
	if prev != nil {
		spmetrics.IngestSessionsReplaced.WithLabelValues(did).Inc()
		prev.end("replaced by a newer ingest session")
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			mm.ingestSessionsMu.Lock()
			defer mm.ingestSessionsMu.Unlock()
			if mm.ingestSessions[did] == s {
				delete(mm.ingestSessions, did)
			}
		})
	}
}
