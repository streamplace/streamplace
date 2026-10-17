package media

import "stream.place/streamplace/pkg/spmetrics"

func (mm *MediaManager) IncrementViewerCount(user string, protocol string) {
	mm.bus.IncrementViewerCount(user, "local")
	spmetrics.ViewerInc(user, protocol)
	// A new playback session is a "view" for the running total (see
	// statedb.StreamViewTotal); recorded off the hot path.
	if rec := mm.viewRecorder; rec != nil {
		go rec(user)
	}
}

// SetViewRecorder installs the callback that counts a new playback session
// toward the streamer's running view total.
func (mm *MediaManager) SetViewRecorder(rec func(streamer string)) {
	mm.viewRecorder = rec
}

func (mm *MediaManager) DecrementViewerCount(user string, protocol string) {
	mm.bus.DecrementViewerCount(user, "local")
	spmetrics.ViewerDec(user, protocol)
}
