package media

import "stream.place/streamplace/pkg/spmetrics"

func (mm *MediaManager) IncrementViewerCount(user string, protocol string) {
	mm.bus.IncrementViewerCount(user, "local")
	spmetrics.ViewerInc(user, protocol)
}

func (mm *MediaManager) DecrementViewerCount(user string, protocol string) {
	mm.bus.DecrementViewerCount(user, "local")
	spmetrics.ViewerDec(user, protocol)
}
