package rtcrec

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
)

type RecordingPeerConnection struct {
	enabled   bool
	pionpc    *webrtc.PeerConnection
	file      config.DebugRecordingFile
	stream    *RecorderStream
	logCtx    context.Context // for finishRecording's logs (it outlives the session)
	closeOnce sync.Once
	recDone   chan struct{} // closed once the finalize ATTEMPT is over — check the logs for commit failures
}

func NewRecordingPeerConnection(ctx context.Context, cli config.CLI, user string, pionpc *webrtc.PeerConnection, enabled bool) (PeerConnection, error) {
	if !enabled {
		return &RecordingPeerConnection{
			pionpc:  pionpc,
			enabled: enabled,
		}, nil
	}
	aqt := aqtime.FromTime(time.Now())
	// Streams to S3 when configured (production), else a local file under DataDir
	// (dev). Close (after the drain delay below) finalizes either target.
	f, err := cli.DebugRecordingCreate(ctx, []string{"debug-recordings", user, fmt.Sprintf("%s.rtcrec.cbor", aqt.FileSafeString())}, "application/cbor", true)
	if err != nil {
		return nil, fmt.Errorf("failed to create debug recording: %w", err)
	}
	log.Log(ctx, "logging webrtc session to file", "file", f.Name())
	stream, err := MakeWebRTCEncoder(f)
	if err != nil {
		return nil, fmt.Errorf("failed to create recorder stream: %w", err)
	}
	return &RecordingPeerConnection{
		pionpc:  pionpc,
		file:    f,
		stream:  stream,
		enabled: enabled,
		logCtx:  context.WithoutCancel(ctx),
		recDone: make(chan struct{}),
	}, nil
}

func (pc *RecordingPeerConnection) Do(f func()) {
	if pc.enabled {
		go f()
	}
}

func (pc *RecordingPeerConnection) Close() error {
	pc.Do(pc.finishRecording)
	return pc.pionpc.Close()
}

// finishRecording drains stragglers, commits the recording (for S3, Close IS
// the commit), and signals recDone. Idempotent — Close on the disconnect path
// and FinalizeRecording at worker exit can both trigger it. recDone means the
// attempt finished, not that it succeeded: a failed commit is logged loudly
// (there is nothing better to do with it at this point — the session is over).
func (pc *RecordingPeerConnection) finishRecording() {
	pc.closeOnce.Do(func() {
		// This is sloppy but there might be other goroutines still writing so let's chill for a sec
		time.Sleep(10 * time.Second)
		if err := pc.file.Close(); err != nil {
			log.Error(pc.logCtx, "debug recording commit FAILED; the recording is lost", "file", pc.file.Name(), "error", err)
		} else {
			log.Log(pc.logCtx, "debug recording committed", "file", pc.file.Name())
		}
		close(pc.recDone)
	})
}

// FinalizeRecording blocks until the debug recording's commit attempt finishes
// (bounded; failures are logged by finishRecording). Call it before process
// exit on paths like the WHIP ingest worker: Close only *starts* the
// drain+commit on a goroutine, and a process that exits first strands an
// uncommitted S3 upload — the object never appears. No-op when not recording.
func (pc *RecordingPeerConnection) FinalizeRecording(ctx context.Context) {
	if !pc.enabled {
		return
	}
	go pc.finishRecording() // in case nothing called Close (e.g. pipeline error)
	select {
	case <-pc.recDone:
	case <-time.After(recordingFinalizeTimeout):
		log.Error(ctx, "debug recording did not finalize in time", "file", pc.file.Name())
	}
}

// recordingFinalizeTimeout bounds FinalizeRecording: the 10s straggler drain in
// finishRecording plus generous headroom for the S3 commit.
const recordingFinalizeTimeout = 40 * time.Second

func (pc *RecordingPeerConnection) CreateAnswer(options *webrtc.AnswerOptions) (webrtc.SessionDescription, error) {
	now := time.Now()
	ret, err := pc.pionpc.CreateAnswer(options)
	if err != nil {
		return ret, err
	}
	pc.Do(func() {
		pc.stream.Event(WebRTCEvent{
			CreateAnswer: &CreateAnswer{
				SDPAnswer: ret.SDP,
			},
			Time: now,
		})
	})
	return ret, nil
}

func (pc *RecordingPeerConnection) SetLocalDescription(desc webrtc.SessionDescription) error {
	now := time.Now()
	pc.Do(func() {
		pc.stream.Event(WebRTCEvent{
			SetRemoteDescription: &SetRemoteDescription{
				SDPRemoteDescription: desc.SDP,
			},
			Time: now,
		})
	})
	return pc.pionpc.SetLocalDescription(desc)
}

func (pc *RecordingPeerConnection) SetRemoteDescription(desc webrtc.SessionDescription) error {
	now := time.Now()
	pc.Do(func() {
		pc.stream.Event(WebRTCEvent{
			SetRemoteDescription: &SetRemoteDescription{
				SDPRemoteDescription: desc.SDP,
			},
			Time: now,
		})
	})
	return pc.pionpc.SetRemoteDescription(desc)
}

func (pc *RecordingPeerConnection) LocalDescription() *webrtc.SessionDescription {
	now := time.Now()
	desc := pc.pionpc.LocalDescription()
	pc.Do(func() {
		pc.stream.Event(WebRTCEvent{
			LocalDescription: &LocalDescription{
				SDPLocalDescription: pc.pionpc.LocalDescription().SDP,
			},
			Time: now,
		})
	})
	return desc
}

// func (pc *RecorderPeerConnection) RemoteDescription() *webrtc.SessionDescription {
// 	return pc.pionpc.RemoteDescription()
// }

func (pc *RecordingPeerConnection) OnICEConnectionStateChange(f func(webrtc.ICEConnectionState)) {
	pc.pionpc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		now := time.Now()
		pc.Do(func() {
			pc.stream.Event(WebRTCEvent{
				ICEConnectionStateChange: &ICEConnectionStateChange{
					ICEConnectionState: state,
				},
				Time: now,
			})
		})
		f(state)
	})
}

func (pc *RecordingPeerConnection) OnConnectionStateChange(f func(webrtc.PeerConnectionState)) {
	pc.pionpc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		now := time.Now()
		pc.Do(func() {
			pc.stream.Event(WebRTCEvent{
				ConnectionStateChange: &ConnectionStateChange{
					ConnectionState: state,
				},
				Time: now,
			})
		})
		f(state)
	})
}

func (pc *RecordingPeerConnection) OnTrack(f func(TrackRemote, RTPReceiver)) {
	pc.pionpc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		now := time.Now()
		wrappedTrack := &RecordingTrackRemote{track: track, stream: pc.stream, pc: pc}
		id := track.ID()
		kind := track.Kind()
		ssrc := track.SSRC()
		payloadType := track.PayloadType()
		streamID := track.StreamID()
		msid := track.Msid()
		rid := track.RID()
		pc.Do(func() {
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
		})
		f(wrappedTrack, receiver)
	})
}

func (pc *RecordingPeerConnection) WriteRTCP(pkts []rtcp.Packet) error {
	return pc.pionpc.WriteRTCP(pkts)
}

func (pc *RecordingPeerConnection) AddTransceiverFromKind(kind webrtc.RTPCodecType, init ...webrtc.RTPTransceiverInit) (RTPTransceiver, error) {
	now := time.Now()
	ret, err := pc.pionpc.AddTransceiverFromKind(kind, init...)
	pc.Do(func() {
		pc.stream.Event(WebRTCEvent{
			AddTransceiverFromKind: &AddTransceiverFromKind{
				Kind: kind,
			},
			Time: now,
		})
	})
	return ret, err
}

func (pc *RecordingPeerConnection) ICEGatheringState() webrtc.ICEGatheringState {
	now := time.Now()
	state := pc.pionpc.ICEGatheringState()
	pc.Do(func() {
		pc.stream.Event(WebRTCEvent{
			ICEGatheringState: &ICEGatheringState{
				State: state,
			},
			Time: now,
		})
	})
	return state
}

func (pc *RecordingPeerConnection) OnDataChannel(f func(*webrtc.DataChannel)) {
	pc.pionpc.OnDataChannel(func(dc *webrtc.DataChannel) {
		now := time.Now()
		pc.Do(func() {
			pc.stream.Event(WebRTCEvent{
				DataChannel: &DataChannel{
					Label: dc.Label(),
				},
				Time: now,
			})
		})
		f(dc)
	})
}

func (pc *RecordingPeerConnection) OnNegotiationNeeded(f func()) {
	pc.pionpc.OnNegotiationNeeded(func() {
		now := time.Now()
		pc.Do(func() {
			pc.stream.Event(WebRTCEvent{
				NegotiationNeeded: &NegotiationNeeded{},
				Time:              now,
			})
		})
		f()
	})
}
