package media

import (
	"sync"
	"testing"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/report"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
)

type clockGateTicker struct{ ticks chan time.Time }

func (t *clockGateTicker) Ch() <-chan time.Time { return t.ticks }
func (t *clockGateTicker) Stop()                {}

type clockGateWriter struct{ writer interceptor.RTPWriter }

func (w clockGateWriter) WriteRTP(h *rtp.Header, p []byte) (int, error) {
	return w.writer.Write(h, p, nil)
}
func (w clockGateWriter) Write(p []byte) (int, error) { return len(p), nil }

type clockGateContext struct {
	earlyTrackContext
	writer webrtc.TrackLocalWriter
	ssrc   uint32
}

func (c clockGateContext) WriteStream() webrtc.TrackLocalWriter { return c.writer }
func (c clockGateContext) SSRC() webrtc.SSRC                    { return webrtc.SSRC(c.ssrc) }

// RTP anchors are unrelated random clock values, not comparable media times.
// Their sender reports must map to one wall-clock origin after scheduled writes.
func TestPionCommonClockUnequalAnchors(t *testing.T) {
	const audioAnchor uint32 = 0xf0000000
	const videoAnchor uint32 = 1234567
	origin := time.Unix(1700000000, 0)
	var mu sync.Mutex
	now := origin
	setNow := func(d time.Duration) { mu.Lock(); now = origin.Add(d); mu.Unlock() }
	ticker := &clockGateTicker{make(chan time.Time, 1)}
	factory, err := report.NewSenderInterceptor(report.SenderNow(func() time.Time { mu.Lock(); defer mu.Unlock(); return now }), report.SenderTicker(func(time.Duration) report.Ticker { return ticker }))
	require.NoError(t, err)
	sender, err := factory.NewInterceptor("common-clock")
	require.NoError(t, err)
	defer sender.Close()
	reports := make(chan *rtcp.SenderReport, 4)
	sender.BindRTCPWriter(interceptor.RTCPWriterFunc(func(packets []rtcp.Packet, _ interceptor.Attributes) (int, error) {
		for _, packet := range packets {
			if sr, ok := packet.(*rtcp.SenderReport); ok {
				reports <- sr
			}
		}
		return len(packets), nil
	}))
	makeTrack := func(mime string, rate, ssrc uint32, rec *earlyRTPRecorder) *webrtc.TrackLocalStaticRTP {
		codec := webrtc.RTPCodecCapability{MimeType: mime, ClockRate: rate, Channels: 2}
		track, e := webrtc.NewTrackLocalStaticRTP(codec, mime, "clock")
		require.NoError(t, e)
		writer := sender.BindLocalStream(&interceptor.StreamInfo{SSRC: ssrc, ClockRate: rate}, interceptor.RTPWriterFunc(func(h *rtp.Header, p []byte, _ interceptor.Attributes) (int, error) { return rec.WriteRTP(h, p) }))
		_, e = track.Bind(clockGateContext{earlyTrackContext{webrtc.RTPCodecParameters{RTPCodecCapability: codec, PayloadType: 96}, rec}, clockGateWriter{writer}, ssrc})
		require.NoError(t, e)
		return track
	}
	audioRec, videoRec := &earlyRTPRecorder{}, &earlyRTPRecorder{}
	audio := makeTrack(webrtc.MimeTypeOpus, 48000, 1, audioRec)
	video := makeTrack(webrtc.MimeTypeH264, 90000, 2, videoRec)
	ap := rtp.NewPacketizerWithOptions(1200, &codecs.OpusPayloader{}, rtp.NewFixedSequencer(100), 48000, rtp.WithTimestamp(audioAnchor))
	vp := rtp.NewPacketizerWithOptions(1200, &codecs.H264Payloader{}, rtp.NewFixedSequencer(200), 90000, rtp.WithTimestamp(videoAnchor))
	// The bytes here only exercise the payloader. Native encoded Opus is tested
	// separately; no fabricated sample is inserted into playback to advance time.
	setNow(0)
	for _, p := range vp.Packetize([]byte{0x65, 1, 2, 3}, 90000) {
		require.NoError(t, video.WriteRTP(p))
	}
	// Preserve the real native initial 13.5ms duration, then 20ms durations.
	// WithTimestamp is also tested with unequal nonzero RTP anchors.
	setNow(0)
	for _, p := range ap.Packetize([]byte{0xfc, 1}, 648) {
		require.NoError(t, audio.WriteRTP(p))
	}
	for i := 0; i < 51; i++ {
		setNow(13500*time.Microsecond + time.Duration(i)*20*time.Millisecond)
		packets := ap.Packetize([]byte{0xfc, 1}, 960)
		require.Len(t, packets, 1)
		require.NoError(t, audio.WriteRTP(packets[0]))
	}
	setNow(time.Second)
	for _, p := range vp.Packetize([]byte{0x41, 1, 2, 3}, 90000) {
		require.NoError(t, video.WriteRTP(p))
	}
	require.Equal(t, audioAnchor, audioRec.headers[0].Timestamp)
	require.Equal(t, uint32(648), audioRec.headers[1].Timestamp-audioRec.headers[0].Timestamp, "exact 13.5ms initial Opus duration")
	require.Equal(t, videoAnchor, videoRec.headers[0].Timestamp)
	for i := 2; i < len(audioRec.headers); i++ {
		require.Equal(t, uint32(960), audioRec.headers[i].Timestamp-audioRec.headers[i-1].Timestamp)
	}
	require.Equal(t, uint32(90000), videoRec.headers[1].Timestamp-videoRec.headers[0].Timestamp)
	setNow(2 * time.Second)
	ticker.ticks <- origin.Add(2 * time.Second)
	var got []*rtcp.SenderReport
	for len(got) < 2 {
		select {
		case sr := <-reports:
			got = append(got, sr)
		case <-time.After(time.Second):
			t.Fatal("sender reports did not arrive")
		}
	}
	for _, sr := range got {
		anchor, rate := videoAnchor, uint32(90000)
		if sr.SSRC == 1 {
			anchor, rate = audioAnchor, 48000
		}
		require.Equal(t, anchor+2*rate, sr.RTPTime, "both sender reports map to the same media origin")
		require.Equal(t, got[0].NTPTime, sr.NTPTime)
	}
}
