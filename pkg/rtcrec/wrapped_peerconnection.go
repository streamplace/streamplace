package rtcrec

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
)

type RecorderPeerConnection struct {
	pionpc *webrtc.PeerConnection
	file   *os.File
	stream *RecorderStream
}

func NewRecorderPeerConnection(ctx context.Context, cli config.CLI, user string, pionpc *webrtc.PeerConnection) (PeerConnection, error) {
	aqt := aqtime.FromTime(time.Now())
	f, err := cli.DataFileCreate([]string{user, "rtcrec", fmt.Sprintf("%s.cbor", aqt.FileSafeString())}, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create data file: %w", err)
	}
	log.Log(ctx, "logging webrtc session to file", "file", f.Name())
	stream, err := NewRecorderStream(f)
	if err != nil {
		return nil, fmt.Errorf("failed to create recorder stream: %w", err)
	}
	return &RecorderPeerConnection{
		pionpc: pionpc,
		file:   f,
		stream: stream,
	}, nil
}

func (pc *RecorderPeerConnection) Close() error {
	return pc.pionpc.Close()
}

func (pc *RecorderPeerConnection) CreateAnswer(options *webrtc.AnswerOptions) (webrtc.SessionDescription, error) {
	now := time.Now()
	ret, err := pc.pionpc.CreateAnswer(options)
	if err != nil {
		return ret, err
	}
	go func() {
		pc.stream.Event(WebRTCEvent{
			CreateAnswer: &CreateAnswer{
				SDPAnswer: ret.SDP,
			},
			Time: now,
		})
	}()
	return ret, nil
}

func (pc *RecorderPeerConnection) SetLocalDescription(desc webrtc.SessionDescription) error {
	now := time.Now()
	go func() {
		pc.stream.Event(WebRTCEvent{
			SetRemoteDescription: &SetRemoteDescription{
				SDPRemoteDescription: desc.SDP,
			},
			Time: now,
		})
	}()
	return pc.pionpc.SetLocalDescription(desc)
}

func (pc *RecorderPeerConnection) SetRemoteDescription(desc webrtc.SessionDescription) error {
	now := time.Now()
	go func() {
		pc.stream.Event(WebRTCEvent{
			SetRemoteDescription: &SetRemoteDescription{
				SDPRemoteDescription: desc.SDP,
			},
			Time: now,
		})
	}()
	return pc.pionpc.SetRemoteDescription(desc)
}

func (pc *RecorderPeerConnection) LocalDescription() *webrtc.SessionDescription {
	now := time.Now()
	desc := pc.pionpc.LocalDescription()
	go func() {
		pc.stream.Event(WebRTCEvent{
			LocalDescription: &LocalDescription{
				SDPLocalDescription: pc.pionpc.LocalDescription().SDP,
			},
			Time: now,
		})
	}()
	return desc
}

// func (pc *RecorderPeerConnection) RemoteDescription() *webrtc.SessionDescription {
// 	return pc.pionpc.RemoteDescription()
// }

func (pc *RecorderPeerConnection) OnICEConnectionStateChange(f func(webrtc.ICEConnectionState)) {
	pc.pionpc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		now := time.Now()
		go func() {
			pc.stream.Event(WebRTCEvent{
				ICEConnectionStateChange: &ICEConnectionStateChange{
					ICEConnectionState: state,
				},
				Time: now,
			})
		}()
		f(state)
	})
}

func (pc *RecorderPeerConnection) OnConnectionStateChange(f func(webrtc.PeerConnectionState)) {
	pc.pionpc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		now := time.Now()
		go func() {
			pc.stream.Event(WebRTCEvent{
				ConnectionStateChange: &ConnectionStateChange{
					ConnectionState: state,
				},
				Time: now,
			})
		}()
		f(state)
	})
}

func (pc *RecorderPeerConnection) OnTrack(f func(TrackRemote, RTPReceiver)) {
	pc.pionpc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		now := time.Now()
		wrappedTrack := &WrappedTrackRemote{track: track, stream: pc.stream}
		id := track.ID()
		kind := track.Kind()
		ssrc := track.SSRC()
		payloadType := track.PayloadType()
		streamID := track.StreamID()
		msid := track.Msid()
		rid := track.RID()
		go func() {
			pc.stream.Event(WebRTCEvent{
				Track: &Track{
					ID:          id,
					Kind:        kind,
					SSRC:        ssrc,
					PayloadType: payloadType,
					StreamID:    streamID,
					Msid:        msid,
					RID:         rid,
				},
				Time: now,
			})
		}()
		f(wrappedTrack, receiver)
	})
}

func (pc *RecorderPeerConnection) WriteRTCP(pkts []rtcp.Packet) error {
	return pc.pionpc.WriteRTCP(pkts)
}

func (pc *RecorderPeerConnection) AddTransceiverFromKind(kind webrtc.RTPCodecType, init ...webrtc.RTPTransceiverInit) (RTPTransceiver, error) {
	return pc.pionpc.AddTransceiverFromKind(kind, init...)
}

func (pc *RecorderPeerConnection) ICEGatheringState() webrtc.ICEGatheringState {
	return pc.pionpc.ICEGatheringState()
}

func (pc *RecorderPeerConnection) OnDataChannel(f func(*webrtc.DataChannel)) {
	pc.pionpc.OnDataChannel(f)
}

func (pc *RecorderPeerConnection) OnNegotiationNeeded(f func()) {
	pc.pionpc.OnNegotiationNeeded(f)
}
