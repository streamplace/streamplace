package rtcrec

import (
	"errors"
	"fmt"

	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"
)

type ReplayTrackRemote struct {
	ssrc       webrtc.SSRC
	trackEvent *WebRTCEvent
	events     map[string][]*WebRTCEvent
	pc         *ReplayPeerConnection
}

func (t *ReplayTrackRemote) Read(p []byte) (n int, attrs interceptor.Attributes, err error) {
	ev := t.pc.group.NextTrack(t.ssrc, EventTypeTrackRead)
	if ev == nil {
		return 0, nil, nil
	}
	select {
	case <-t.pc.wait(fmt.Sprintf("TrackRead %s", t.trackEvent.Track.ID), ev.Time):
		var err error
		if ev.TrackRead.Err != "" {
			err = errors.New(ev.TrackRead.Err)
		}
		if ev.TrackRead.Data != nil {
			copied := copy(p, ev.TrackRead.Data)
			if copied != len(ev.TrackRead.Data) {
				panic("copied != len(ev.TrackRead.Data)")
			}
		}
		// log.Log(t.pc.ctx, "TrackRead", "trackId", t.trackEvent.Track.ID, "count", ev.TrackRead.Count, "err", err)
		return ev.TrackRead.Count, nil, err
	case <-t.pc.ctx.Done():
		return 0, nil, t.pc.ctx.Err()
	}
}

func (t *ReplayTrackRemote) Codec() webrtc.RTPCodecParameters {
	ev := t.pc.group.PeekTrack(t.ssrc, EventTypeTrackCodec)
	if ev == nil {
		panic("no codec found")
	}
	return ev.TrackCodec.Codec
}

func (t *ReplayTrackRemote) ID() string {
	return t.trackEvent.Track.ID
}

func (t *ReplayTrackRemote) Kind() webrtc.RTPCodecType {
	return t.trackEvent.Track.Kind
}

func (t *ReplayTrackRemote) PayloadType() webrtc.PayloadType {
	return t.trackEvent.Track.PayloadType
}

func (t *ReplayTrackRemote) SSRC() webrtc.SSRC {
	return t.ssrc
}
