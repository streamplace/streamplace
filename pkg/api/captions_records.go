package api

import (
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/records"
	"stream.place/streamplace/pkg/vod"
)

// recordCaptions combines the archival MUXL text tracks with indexed transcript
// records. Canonical records are a copy, not a second rendition.
func (a *StreamplaceAPI) recordCaptions() captions.VideoCaptions {
	p := &records.Provider{Store: a.Model, NodeDID: a.CLI.ServerDID()}
	if a.StatefulDB != nil {
		p.Recording = records.StatedbRecording{State: a.StatefulDB}
	}
	return &vod.VideoCaptions{Model: a.Model, Store: a.PlaybackStore, Records: p}
}
