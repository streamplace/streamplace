package rtcrec

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
	"stream.place/streamplace/pkg/log"
)

type ReplayPeerConnection struct {
	startTime time.Time
	group     *WebRTCEventGroup
	ctx       context.Context
}

func NewReplayPeerConnection(ctx context.Context, r io.Reader) (PeerConnection, error) {
	group, err := ReadAllEvents(r)
	if err != nil {
		return nil, fmt.Errorf("failed to create web rtc decoder: %w", err)
	}

	return &ReplayPeerConnection{
		startTime: time.Now(),
		group:     group,
		ctx:       context.Background(),
	}, nil
}

func (pc *ReplayPeerConnection) wait(label string, t time.Time) <-chan time.Time {
	now := time.Now()
	historicalDiff := t.Sub(pc.group.FirstTime)
	currentDiff := time.Since(pc.startTime)
	diff := historicalDiff - currentDiff
	log.Debug(pc.ctx, "waiting for event", "event", label, "diff", diff, "t", t, "first", pc.group.FirstTime, "startTime", pc.startTime, "now", now)
	return time.After(diff)
}

func (pc *ReplayPeerConnection) Close() error {
	// todo: implment stopping here
	return nil
}

func (pc *ReplayPeerConnection) CreateAnswer(options *webrtc.AnswerOptions) (webrtc.SessionDescription, error) {
	ev := pc.group.Peek(EventTypeCreateAnswer)
	if ev == nil {
		return webrtc.SessionDescription{}, fmt.Errorf("no create answer event found")
	}
	return webrtc.SessionDescription{SDP: ev.CreateAnswer.SDPAnswer}, nil
}

func (pc *ReplayPeerConnection) SetLocalDescription(desc webrtc.SessionDescription) error {
	return nil
}

func (pc *ReplayPeerConnection) SetRemoteDescription(desc webrtc.SessionDescription) error {
	return nil
}

func (pc *ReplayPeerConnection) LocalDescription() *webrtc.SessionDescription {
	ev := pc.group.Peek(EventTypeLocalDescription)
	if ev == nil {
		return nil
	}
	return &webrtc.SessionDescription{SDP: ev.LocalDescription.SDPLocalDescription}
}

// func (pc *ReplayPeerConnection) RemoteDescription() *webrtc.SessionDescription {
// 	return pc.pionpc.RemoteDescription()
// }

func (pc *ReplayPeerConnection) OnICEConnectionStateChange(f func(webrtc.ICEConnectionState)) {
	go func() {
		for {
			ev := pc.group.Next(EventTypeICEConnectionState)
			if ev == nil {
				return
			}
			select {
			case <-pc.wait("OnICEConnectionStateChange", ev.Time):
				f(ev.ICEConnectionStateChange.ICEConnectionState)
			case <-pc.ctx.Done():
				return
			}
		}
	}()
}

func (pc *ReplayPeerConnection) OnConnectionStateChange(f func(webrtc.PeerConnectionState)) {
	go func() {
		for {
			ev := pc.group.Next(EventTypeConnectionState)
			if ev == nil {
				return
			}
			select {
			case <-pc.wait("OnConnectionStateChange", ev.Time):
				f(ev.ConnectionStateChange.ConnectionState)
			case <-pc.ctx.Done():
				return
			}
		}
	}()
}

func (pc *ReplayPeerConnection) OnTrack(f func(TrackRemote, RTPReceiver)) {
	go func() {
		for {
			ev := pc.group.Next(EventTypeTrack)
			if ev == nil {
				return
			}
			select {
			case <-pc.wait("OnTrack", ev.Time):
				track := &ReplayTrackRemote{
					ssrc:       ev.Track.SSRC,
					trackEvent: ev,
					events:     pc.group.Tracks[ev.Track.SSRC],
					pc:         pc,
				}
				go func() {
					f(track, nil)
				}()
			case <-pc.ctx.Done():
				return
			}
		}
	}()
}

func (pc *ReplayPeerConnection) WriteRTCP(pkts []rtcp.Packet) error {
	return nil
}

func (pc *ReplayPeerConnection) AddTransceiverFromKind(kind webrtc.RTPCodecType, init ...webrtc.RTPTransceiverInit) (RTPTransceiver, error) {
	return nil, nil
}

func (pc *ReplayPeerConnection) ICEGatheringState() webrtc.ICEGatheringState {
	ev := pc.group.Peek(EventTypeICEGatheringState)
	if ev == nil {
		return webrtc.ICEGatheringStateNew
	}
	return ev.ICEGatheringState.State
}
