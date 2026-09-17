package media

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/livehls"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/rtcrec"
)

// TestWebRTCIngestToPlaybackFirstRTP exercises the live boundary end to end:
// a real Pion WHIP publisher sends GStreamer-generated H264+Opus RTP through
// WebRTCIngest, and a separate Pion WHEP receiver reads the first RTP packet
// from WebRTCPlayback2. The source-age metric is the authoritative latency
// measurement because it includes the segment duration and every server stage
// between capture and first send.
func TestWebRTCIngestToPlaybackFirstRTP(t *testing.T) {
	firstSendAge := measureWebRTCIngestToPlaybackFirstRTP(t)
	require.Less(t, firstSendAge, 1000.0)
}

func TestWebRTCIngestToPlaybackFirstRTPP95UnderOneSecond(t *testing.T) {
	const captureCount = 20
	ages := make([]float64, 0, captureCount)
	for i := 0; i < captureCount; i++ {
		i := i
		t.Run(fmt.Sprintf("capture-%02d", i), func(t *testing.T) {
			ages = append(ages, measureWebRTCIngestToPlaybackFirstRTP(t))
		})
	}
	sort.Float64s(ages)
	p95 := ages[int(math.Ceil(float64(len(ages))*.95))-1]
	require.Less(t, p95, 1000.0,
		"live WHIP-to-WHEP p95 source-start-to-first-RTP should stay below one second")
	t.Logf("live WHIP-to-WHEP empirical p95 source-start-to-first-RTP_ms=%.1f samples=%d", p95, len(ages))
}

func measureWebRTCIngestToPlaybackFirstRTP(t *testing.T) float64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ms := newBareSegmentSigner(t)
	pub, err := atproto.ParsePubKey(ms.Signer.Public())
	require.NoError(t, err)
	ms.StreamerName = pub.DIDKey()
	ms.PrebuiltManifest = nil
	streamer := ms.Streamer()

	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)
	ldb, err := localdb.MakeDB(":memory:")
	require.NoError(t, err)
	api, webrtcConfig, err := newWebRTCAPI()
	require.NoError(t, err)
	b := bus.NewBus()
	mm := &MediaManager{
		cli:          &config.CLI{BroadcasterHost: "test.example.com", WideOpen: true},
		bus:          b,
		webrtcAPI:    api,
		webrtcConfig: webrtcConfig,
		liveWindows:  map[string]*livehls.Writer{},
		localDB:      ldb,
		transcoders:  map[string]*streamTranscoder{},
	}
	mm.nodeSignerOnce.Do(func() {
		mm.nodeCert = ms.Cert
		mm.nodeKeyPEM = keyPEM
	})
	t.Cleanup(func() {
		mm.transcodersMu.Lock()
		tr := mm.transcoders[streamer]
		delete(mm.transcoders, streamer)
		mm.transcodersMu.Unlock()
		if tr != nil {
			_ = tr.Close()
		}
	})

	ingestPion, err := api.NewPeerConnection(webrtcConfig)
	require.NoError(t, err)
	ingestPC, err := rtcrec.NewRecordingPeerConnection(ctx, *mm.cli, streamer, ingestPion, false)
	require.NoError(t, err)
	defer ingestPC.Close()

	publisher, videoTrack, audioTrack, offer := whipClientOffer(t)
	defer publisher.Close()
	publisherConnected := make(chan struct{})
	var publisherConnectedOnce sync.Once
	publisher.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			publisherConnectedOnce.Do(func() { close(publisherConnected) })
		}
	})
	ingestDone := make(chan error, 1)
	answer, err := mm.WebRTCIngest(ctx, &offer, ms, ingestPC, ingestDone)
	require.NoError(t, err)
	require.NoError(t, publisher.SetRemoteDescription(*answer))
	select {
	case <-publisherConnected:
	case <-ctx.Done():
		t.Fatal("WHIP publisher did not connect")
	}

	receiver, err := api.NewPeerConnection(webrtcConfig)
	require.NoError(t, err)
	defer receiver.Close()
	receiverConnected := make(chan struct{})
	var receiverConnectedOnce sync.Once
	receiver.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			receiverConnectedOnce.Do(func() { close(receiverConnected) })
		}
	})
	firstRTP := make(chan struct{})
	var firstRTPOnce sync.Once
	receiver.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		go func() {
			buf := make([]byte, 1500)
			if _, _, readErr := track.Read(buf); readErr == nil {
				firstRTPOnce.Do(func() { close(firstRTP) })
			}
		}()
	})
	_, err = receiver.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo)
	require.NoError(t, err)
	_, err = receiver.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio)
	require.NoError(t, err)
	receiverOffer, err := receiver.CreateOffer(nil)
	require.NoError(t, err)
	require.NoError(t, receiver.SetLocalDescription(receiverOffer))
	gatherComplete := webrtc.GatheringCompletePromise(receiver)
	select {
	case <-gatherComplete:
	case <-ctx.Done():
		t.Fatal("WHEP receiver ICE gathering timed out")
	}

	playbackAnswer, err := mm.WebRTCPlayback2(ctx, streamer, "source", receiver.LocalDescription(), streamer)
	require.NoError(t, err)
	require.NoError(t, receiver.SetRemoteDescription(*playbackAnswer))
	select {
	case <-receiverConnected:
	case <-ctx.Done():
		t.Fatal("WHEP receiver did not connect")
	}

	labels := map[string]string{
		"streamer":  streamer,
		"rendition": WebRTCSourceRendition,
		"stage":     "first_sent",
	}
	beforeCount, beforeSum := histogramSample(t, "streamplace_webrtc_source_age_ms", labels)
	produceWHIPMedia(t, ctx, videoTrack, audioTrack)
	select {
	case <-firstRTP:
	case <-ctx.Done():
		t.Fatal("live WHIP media did not reach the WHEP receiver")
	}

	var firstSendAge float64
	require.Eventually(t, func() bool {
		count, sum := histogramSample(t, "streamplace_webrtc_source_age_ms", labels)
		if count-beforeCount != 1 {
			return false
		}
		firstSendAge = (sum - beforeSum) / float64(count-beforeCount)
		return true
	}, 5*time.Second, 25*time.Millisecond, "live ingest should record first RTP source age")
	require.Less(t, firstSendAge, 1000.0)
	t.Logf("live WHIP-to-WHEP source-start-to-first-RTP_ms=%.1f", firstSendAge)

	cancel()
	select {
	case <-ingestDone:
	case <-time.After(5 * time.Second):
		t.Fatal("WebRTC ingest did not shut down")
	}
	return firstSendAge
}
