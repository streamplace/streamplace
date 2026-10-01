package api

import (
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/records"
)

// recordCaptions is the node's VideoCaptions: the place.stream.caption.transcript
// records in the index, with live captions placed on VODs through the
// recording objects this node keeps.
func (a *StreamplaceAPI) recordCaptions() captions.VideoCaptions {
	p := &records.Provider{Store: a.Model, NodeDID: a.CLI.ServerDID()}
	if a.StatefulDB != nil {
		p.Recording = records.StatedbRecording{State: a.StatefulDB}
	}
	return p
}
