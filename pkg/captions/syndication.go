package captions

import "encoding/json"

// SyndicationVersion is an opt-in query capability on subscribeSegments. Older
// peers continue receiving only binary media; capable peers also receive JSON
// text frames, on the same ordered socket as the stream they validated.
const SyndicationVersion = "1"

type sidecarMessage struct {
	Type  string `json:"$type"`
	Event Event  `json:"event"`
}

func EncodeSidecar(ev Event) ([]byte, error) {
	return json.Marshal(sidecarMessage{Type: "place.stream.caption.sidecar#event", Event: ev})
}

func DecodeSidecar(data []byte) (Event, bool) {
	var msg sidecarMessage
	if json.Unmarshal(data, &msg) != nil || msg.Type != "place.stream.caption.sidecar#event" || msg.Event.Track.Origin != OriginSidecar {
		return Event{}, false
	}
	return msg.Event, true
}
