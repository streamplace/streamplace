package media

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
)

func TestEarlyAACPendingIdleRefreshDoesNotAbort(t *testing.T) {
	s := earlyState(t, 7)
	mm := &MediaManager{earlySessions: map[string]*earlyAACSession{"user": s}}
	mm.earlyMu.Lock()
	pending := make(chan struct{})
	done := make(chan struct{})
	go func() { close(pending); mm.reapEarlyAAC("user", s); close(done) }()
	<-pending
	// An old callback is pending while a new feed refreshes the idle deadline.
	s.idleDeadline = time.Now().Add(streamTranscoderIdle)
	s.idle = time.AfterFunc(streamTranscoderIdle, func() {})
	defer s.idle.Stop()
	mm.earlyMu.Unlock()
	<-done
	require.NoError(t, s.ctx.Err(), "pending old callback must not abort refreshed feed")
}

func TestEarlyAACLongClockRTPWrap(t *testing.T) {
	recorder := &earlyRTPRecorder{}
	codec := webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}
	track, err := webrtc.NewTrackLocalStaticRTP(codec, "audio", "review")
	require.NoError(t, err)
	_, err = track.Bind(earlyTrackContext{webrtc.RTPCodecParameters{RTPCodecCapability: codec, PayloadType: 111}, recorder})
	require.NoError(t, err)
	packetizer := rtp.NewPacketizerWithOptions(1200, &codecs.OpusPayloader{}, rtp.NewFixedSequencer(1), 48000, rtp.WithTimestamp(0))
	times := []time.Duration{29*time.Hour + 13500*time.Microsecond, time.Duration(math.MaxInt64), time.Duration((uint64(1)<<32)/48000)*time.Second + time.Second}
	for i, ts := range times {
		require.NoError(t, writeEarlyAACPackets(track, packetizer, bus.PacketizedSample{Data: []byte{0xfc, 1}, Timestamp: ts, HasTimestamp: true, Duration: 20 * time.Millisecond}, 0, 48000))
		expected := uint32(uint64(ts/time.Second)*48000 + uint64(ts%time.Second)*48000/uint64(time.Second))
		require.Equal(t, expected, recorder.headers[i].Timestamp)
	}
}

func TestEarlyAACNativeDurationFullRange(t *testing.T) {
	for _, ticks := range []uint64{uint64(29*time.Hour/time.Second)*48000 + 648, uint64(math.MaxInt64/int64(time.Second)) * 48000} {
		got, err := earlyMediaDuration(ticks, 48000)
		require.NoError(t, err)
		expected := time.Duration(ticks/48000)*time.Second + time.Duration(ticks%48000)*time.Second/48000
		require.Equal(t, expected, got)
	}
	_, err := earlyMediaDuration(math.MaxUint64, 1)
	require.Error(t, err)
	_, err = earlyMediaDuration(1, 0)
	require.Error(t, err)
	got, err := earlyMediaDuration(uint64(math.MaxInt64), 1000000000)
	require.NoError(t, err)
	require.Equal(t, time.Duration(math.MaxInt64), got)
}

func TestEarlyAACLateAudioRejectsBeforeRTP(t *testing.T) {
	// This exercises the same scheduled-write boundary as both runtime workers.
	recorder := &earlyRTPRecorder{}
	codec := webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}
	track, err := webrtc.NewTrackLocalStaticRTP(codec, "audio", "late")
	require.NoError(t, err)
	_, err = track.Bind(earlyTrackContext{webrtc.RTPCodecParameters{RTPCodecCapability: codec, PayloadType: 111}, recorder})
	require.NoError(t, err)
	packetizer := rtp.NewPacketizerWithOptions(1200, &codecs.OpusPayloader{}, rtp.NewFixedSequencer(1), 48000, rtp.WithTimestamp(0))
	sample := bus.PacketizedSample{Data: []byte{0xfc, 1}, HasTimestamp: true, Duration: 20 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = writeScheduledEarlyAAC(ctx, track, packetizer, sample, 0, 48000, time.Now().Add(-500*time.Millisecond))
	if err != nil {
		cancel()
	}
	require.Error(t, err)
	require.Error(t, ctx.Err())
	require.Empty(t, recorder.headers, "late audio must close peer before old timestamp reaches Pion")
	require.NoError(t, writeScheduledEarlyAAC(context.Background(), track, packetizer, sample, 0, 48000, time.Now().Add(-time.Millisecond)), "small scheduling jitter is explicitly tolerated")
	require.Len(t, recorder.headers, 1)
}
