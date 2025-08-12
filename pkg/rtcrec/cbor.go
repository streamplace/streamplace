package rtcrec

import (
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/pion/webrtc/v4"
)

type WebRTCEventDecoder struct {
	dec *cbor.Decoder
}

func Opts() (cbor.EncMode, error) {
	opts := cbor.CoreDetEncOptions()
	opts.Time = cbor.TimeRFC3339Nano
	em, err := opts.EncMode()
	if err != nil {
		return nil, fmt.Errorf("failed to create encoder mode: %w", err)
	}
	return em, nil
}

func MakeWebRTCDecoder(r io.Reader) (*WebRTCEventDecoder, error) {
	dec := cbor.NewDecoder(r)
	return &WebRTCEventDecoder{dec: dec}, nil
}

func (d *WebRTCEventDecoder) Next() (*WebRTCEvent, error) {
	var ev WebRTCEvent
	err := d.dec.Decode(&ev)
	if err != nil {
		return nil, err
	}
	return &ev, err
}

type WebRTCEventGroup struct {
	Events     map[string][]*WebRTCEvent
	Tracks     map[webrtc.SSRC]map[string][]*WebRTCEvent
	FirstTime  time.Time
	EventMutex sync.Mutex
}

const (
	EventTypeOffer                  = "Offer"
	EventTypeCreateAnswer           = "CreateAnswer"
	EventTypeSetRemoteDescription   = "SetRemoteDescription"
	EventTypeSetLocalDescription    = "SetLocalDescription"
	EventTypeLocalDescription       = "LocalDescription"
	EventTypeICEConnectionState     = "ICEConnectionStateChange"
	EventTypeConnectionState        = "ConnectionStateChange"
	EventTypeTrack                  = "Track"
	EventTypeAddTransceiverFromKind = "AddTransceiverFromKind"
	EventTypeICEGatheringState      = "ICEGatheringState"
	EventTypeDataChannel            = "DataChannel"
	EventTypeNegotiationNeeded      = "NegotiationNeeded"
	EventTypeTrackRead              = "TrackRead"
	EventTypeTrackCodec             = "TrackCodec"
	EventTypeTrackKind              = "TrackKind"
	EventTypeTrackPayloadType       = "TrackPayloadType"
	EventTypeTrackSSRC              = "TrackSSRC"
	EventTypeUnknown                = "Unknown"
)

// ReadAllEvents reads all WebRTC events from a CBOR reader and organizes them by type.
// Returns a map where keys are event types and values are slices of events of that type.
func ReadAllEvents(r io.Reader) (*WebRTCEventGroup, error) {
	dec, err := MakeWebRTCDecoder(r)
	if err != nil {
		return nil, err
	}

	eventList := []*WebRTCEvent{}
	events := make(map[string][]*WebRTCEvent)
	tracks := make(map[webrtc.SSRC]map[string][]*WebRTCEvent)
	var firstTime time.Time
	for {
		ev, err := dec.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		eventList = append(eventList, ev)
	}

	sort.Slice(eventList, func(i, j int) bool {
		return eventList[i].Time.Before(eventList[j].Time)
	})

	for _, ev := range eventList {
		if firstTime.IsZero() {
			firstTime = ev.Time
		}

		// Determine the event type based on which field is non-nil
		var eventType string
		var trackSSRC *webrtc.SSRC
		switch {
		case ev.Offer != nil:
			eventType = EventTypeOffer
		case ev.CreateAnswer != nil:
			eventType = EventTypeCreateAnswer
		case ev.SetRemoteDescription != nil:
			eventType = EventTypeSetRemoteDescription
		case ev.SetLocalDescription != nil:
			eventType = EventTypeSetLocalDescription
		case ev.LocalDescription != nil:
			eventType = EventTypeLocalDescription
		case ev.ICEConnectionStateChange != nil:
			eventType = EventTypeICEConnectionState
		case ev.ConnectionStateChange != nil:
			eventType = EventTypeConnectionState
		case ev.Track != nil:
			eventType = EventTypeTrack
		case ev.AddTransceiverFromKind != nil:
			eventType = EventTypeAddTransceiverFromKind
		case ev.ICEGatheringState != nil:
			eventType = EventTypeICEGatheringState
		case ev.DataChannel != nil:
			eventType = EventTypeDataChannel
		case ev.NegotiationNeeded != nil:
			eventType = EventTypeNegotiationNeeded

		case ev.TrackRead != nil:
			trackSSRC = &ev.TrackRead.SSRC
			eventType = EventTypeTrackRead
		case ev.TrackCodec != nil:
			trackSSRC = &ev.TrackCodec.SSRC
			eventType = EventTypeTrackCodec
		case ev.TrackKind != nil:
			trackSSRC = &ev.TrackKind.SSRC
			eventType = EventTypeTrackKind
		case ev.TrackPayloadType != nil:
			trackSSRC = &ev.TrackPayloadType.SSRC
			eventType = EventTypeTrackPayloadType
		case ev.TrackSSRC != nil:
			trackSSRC = &ev.TrackSSRC.SSRC
			eventType = EventTypeTrackSSRC
		default:
			eventType = EventTypeUnknown
			panic(fmt.Sprintf("unknown event type: %+v", ev))
		}

		if trackSSRC != nil {
			if tracks[*trackSSRC] == nil {
				tracks[*trackSSRC] = make(map[string][]*WebRTCEvent)
			}
			if tracks[*trackSSRC][eventType] == nil {
				tracks[*trackSSRC][eventType] = []*WebRTCEvent{}
			}
			tracks[*trackSSRC][eventType] = append(tracks[*trackSSRC][eventType], ev)
		}

		if events[eventType] == nil {
			events[eventType] = []*WebRTCEvent{}
		}

		events[eventType] = append(events[eventType], ev)
	}

	return &WebRTCEventGroup{
		Events:    events,
		Tracks:    tracks,
		FirstTime: firstTime,
	}, nil
}

func (g *WebRTCEventGroup) Peek(eventType string) *WebRTCEvent {
	if g.Events[eventType] == nil {
		panic(fmt.Sprintf("no events of type %s", eventType))
	}
	if len(g.Events[eventType]) == 0 {
		return nil
	}
	return g.Events[eventType][0]
}

func (g *WebRTCEventGroup) PeekTrack(ssrc webrtc.SSRC, eventType string) *WebRTCEvent {
	if g.Tracks[ssrc] == nil {
		panic(fmt.Sprintf("no tracks for ssrc %d", ssrc))
	}
	if g.Tracks[ssrc][eventType] == nil {
		panic(fmt.Sprintf("no events of type %s for ssrc %d", eventType, ssrc))
	}
	if len(g.Tracks[ssrc][eventType]) == 0 {
		return nil
	}
	return g.Tracks[ssrc][eventType][0]
}

func (g *WebRTCEventGroup) Next(eventType string) *WebRTCEvent {
	g.EventMutex.Lock()
	defer g.EventMutex.Unlock()
	if g.Events[eventType] == nil {
		panic(fmt.Sprintf("no events of type %s", eventType))
	}
	if len(g.Events[eventType]) == 0 {
		return nil
	}
	ev := g.Events[eventType][0]
	g.Events[eventType] = g.Events[eventType][1:]
	return ev
}

func (g *WebRTCEventGroup) NextTrack(ssrc webrtc.SSRC, eventType string) *WebRTCEvent {
	if g.Tracks[ssrc] == nil {
		panic(fmt.Sprintf("no tracks for ssrc %d", ssrc))
	}
	if g.Tracks[ssrc][eventType] == nil {
		panic(fmt.Sprintf("no events of type %s for ssrc %d", eventType, ssrc))
	}
	if len(g.Tracks[ssrc][eventType]) == 0 {
		return nil
	}
	ev := g.Tracks[ssrc][eventType][0]
	g.Tracks[ssrc][eventType] = g.Tracks[ssrc][eventType][1:]
	return ev
}
