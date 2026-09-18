package media

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/aqpub"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/livehls"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/muxl"
)

const latencyBudgetTestEnv = "STREAMPLACE_RUN_LATENCY_BUDGET_TESTS"

func requireLatencyBudgetTest(t *testing.T) {
	t.Helper()
	if os.Getenv(latencyBudgetTestEnv) == "" {
		t.Skipf("set %s=1 to run machine-sensitive latency budget assertions", latencyBudgetTestEnv)
	}
}

func TestSourceStartForTimingUsesSignedCompletionAndMediaDuration(t *testing.T) {
	receivedAt := time.Date(2026, 9, 17, 2, 0, 0, 0, time.UTC)
	signedAt := receivedAt.Add(-10 * time.Minute)
	meta := &SegmentMetadata{StartTime: aqtime.FromTime(signedAt)}

	require.Equal(t, signedAt.Add(-800*time.Millisecond), sourceStartForTiming(meta, 800*time.Millisecond),
		"source age should use the signed completion timestamp minus media duration")
	require.Equal(t, signedAt.Add(-800*time.Millisecond), sourceStartForTiming(meta, 800*time.Millisecond),
		"replayed and replicated media should use the same source timeline")
	require.Equal(t, signedAt, sourceStartForTiming(meta, 0),
		"missing duration should keep the signed timestamp")
	require.Equal(t, signedAt, sourceStartForTiming(meta, -time.Second),
		"invalid duration should keep the signed timestamp")
	require.Equal(t, time.Time{}, sourceStartForTiming(nil, time.Second),
		"missing metadata should not fabricate a source timestamp")
}

func TestIngestProtocolDefaultsAndPropagates(t *testing.T) {
	ctx := context.Background()
	require.Equal(t, "unknown", ingestProtocol(ctx))
	require.Equal(t, "whip", ingestProtocol(withIngestProtocol(ctx, "whip")))
	require.Equal(t, "rtmp", ingestProtocol(withIngestProtocol(ctx, "rtmp")))
	require.Equal(t, "unknown", ingestProtocol(withIngestProtocol(ctx, "unexpected")))
}

func TestValidateMP4PublishesOpusBeforeAACCompletion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ms := newBareSegmentSigner(t)
	pub, err := atproto.ParsePubKey(ms.Signer.Public())
	require.NoError(t, err)
	ms.StreamerName = pub.DIDKey()
	ms.PrebuiltManifest = nil
	segments := allSignedBareSegments(t, ctx, ms, getFixture("h264-opus-frag.mp4"))
	require.NotEmpty(t, segments)

	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)
	ldb, err := localdb.MakeDB(":memory:")
	require.NoError(t, err)
	streamer := ms.StreamerName
	b := bus.NewBus()
	mm := &MediaManager{
		cli:         &config.CLI{BroadcasterHost: "test.example.com", WideOpen: true},
		bus:         b,
		liveWindows: map[string]*livehls.Writer{},
		localDB:     ldb,
		transcoders: map[string]*streamTranscoder{},
	}
	mm.nodeSignerOnce.Do(func() {
		mm.nodeCert = ms.Cert
		mm.nodeKeyPEM = keyPEM
	})

	// ValidateMP4 starts the AAC derivative, so close that real transcoder after
	// observing the immediate WebRTC publication. This keeps the integration
	// test from leaving a native pipeline behind.
	t.Cleanup(func() {
		mm.transcodersMu.Lock()
		tr := mm.transcoders[streamer]
		delete(mm.transcoders, streamer)
		mm.transcodersMu.Unlock()
		if tr != nil {
			_ = tr.Close()
		}
	})

	sub := b.SubscribeSegment(ctx, streamer, WebRTCSourceRendition)
	defer b.UnsubscribeSegment(ctx, streamer, WebRTCSourceRendition, sub)
	canonicalSub := mm.NewSegment()

	valid, err := ValidateMP4Media(ctx, segments[0])
	require.NoError(t, err)
	segmentDuration := time.Duration(valid.MediaData.Duration)
	require.Positive(t, segmentDuration)

	require.NoError(t, mm.ValidateMP4(ctx, bytes.NewReader(segments[0]), true))
	select {
	case got := <-sub.C:
		require.Equal(t, WebRTCSourceRendition, got.Rendition)
		require.NotEmpty(t, got.PacketizedData.Video)
		require.NotEmpty(t, got.PacketizedData.Audio)
		require.NotNil(t, got.Timing)
		require.True(t, got.Timing.MasteringCompleted.IsZero(),
			"immediate playback must not wait for mastering completion")
		age := got.Timing.SourceAge(time.Now())
		require.GreaterOrEqual(t, age, segmentDuration,
			"source age must include the media duration")
		require.Less(t, age, segmentDuration+10*time.Second,
			"source age should be anchored to local receipt, not the epoch signed timestamp")
	case <-ctx.Done():
		t.Fatal("ValidateMP4 did not publish an immediate WebRTC source")
	}

	mm.transcodersMu.Lock()
	tr := mm.transcoders[streamer]
	mm.transcodersMu.Unlock()
	require.NotNil(t, tr, "ValidateMP4 should keep the AAC derivative running")
	require.NoError(t, tr.Close(), "AAC derivative should flush its canonical segment")
	select {
	case notification := <-canonicalSub:
		require.NotNil(t, notification.Timing)
		require.False(t, notification.Timing.MasteringCompleted.IsZero(),
			"canonical notification should carry mastering completion timing")
		codecs := audioCodecsOf(t, ctx, notification.Muxl)
		require.Contains(t, codecs, "opus")
		require.Contains(t, codecs, "mp4a.40.2")
	case <-ctx.Done():
		t.Fatal("AAC derivative did not produce a canonical dual-codec segment")
	}
}

func TestValidateMP4ToWebRTCFirstRTP(t *testing.T) {
	firstSendAge := measureValidateMP4ToWebRTCFirstRTP(t)
	require.GreaterOrEqual(t, firstSendAge, 0.0,
		"source-start-to-first-RTP should be a non-negative measured age")
}

func TestValidateMP4ToWebRTCFirstRTPP95UnderOneSecond(t *testing.T) {
	requireLatencyBudgetTest(t)
	// Keep the real GStreamer/Pion samples large enough to catch regressions.
	// This remains opt-in because the serial run is machine-sensitive.
	const captureCount = 20
	ages := make([]float64, 0, captureCount)
	for i := 0; i < captureCount; i++ {
		i := i
		t.Run(fmt.Sprintf("capture-%02d", i), func(t *testing.T) {
			ages = append(ages, measureValidateMP4ToWebRTCFirstRTP(t))
		})
	}
	sort.Float64s(ages)
	p95Index := int(math.Ceil(float64(len(ages))*.95)) - 1
	p95 := ages[p95Index]
	require.Less(t, p95, 1000.0,
		"empirical p95 source-start-to-first-RTP should stay below one second")
	t.Logf("empirical p95 source-start-to-first-RTP_ms=%.1f samples=%d", p95, len(ages))
}

func measureValidateMP4ToWebRTCFirstRTP(t *testing.T) float64 {
	return measureValidateMP4ToWebRTCFirstRTPWithGOP(t, 15)
}

func measureValidateMP4ToWebRTCFirstRTPWithGOP(t *testing.T, gopFrames int) float64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ms := newBareSegmentSigner(t)
	pub, err := atproto.ParsePubKey(ms.Signer.Public())
	require.NoError(t, err)
	ms.StreamerName = pub.DIDKey()
	ms.PrebuiltManifest = nil
	fragPath := filepath.Join(t.TempDir(), "source.mp4")
	require.NoError(t, os.WriteFile(fragPath, makeLiveH264OpusFMP4WithGOP(t, ctx, gopFrames), 0600))
	segments := allSignedBareSegments(t, ctx, ms, fragPath)
	require.NotEmpty(t, segments)

	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)
	ldb, err := localdb.MakeDB(":memory:")
	require.NoError(t, err)
	api, webrtcConfig, err := newWebRTCAPI()
	require.NoError(t, err)
	streamer := ms.StreamerName
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

	receiverAPI, receiverConfig, err := newWebRTCAPI()
	require.NoError(t, err)
	receiver, err := receiverAPI.NewPeerConnection(receiverConfig)
	require.NoError(t, err)
	defer receiver.Close()

	connected := make(chan struct{})
	var connectedOnce sync.Once
	receiver.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			connectedOnce.Do(func() { close(connected) })
		}
	})
	received := make(chan struct{})
	var receivedOnce sync.Once
	receiver.OnTrack(func(*webrtc.TrackRemote, *webrtc.RTPReceiver) {
		receivedOnce.Do(func() { close(received) })
	})
	_, err = receiver.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo)
	require.NoError(t, err)
	_, err = receiver.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio)
	require.NoError(t, err)
	offer, err := receiver.CreateOffer(nil)
	require.NoError(t, err)
	require.NoError(t, receiver.SetLocalDescription(offer))
	gatherComplete := webrtc.GatheringCompletePromise(receiver)
	select {
	case <-gatherComplete:
	case <-ctx.Done():
		t.Fatal("receiver ICE gathering timed out")
	}
	answer, err := mm.WebRTCPlayback2(ctx, streamer, "source", receiver.LocalDescription(), streamer)
	require.NoError(t, err)
	require.NoError(t, receiver.SetRemoteDescription(*answer))
	select {
	case <-connected:
	case <-ctx.Done():
		t.Fatal("local WebRTC playback did not connect")
	}

	labels := map[string]string{
		"streamer":  streamer,
		"rendition": WebRTCSourceRendition,
		"stage":     "first_sent",
	}
	durationLabels := map[string]string{"streamer": streamer}
	durationBeforeCount, durationBeforeSum := histogramSample(t, "streamplace_media_segment_duration_ms", durationLabels)
	beforeCount, beforeSum := histogramSample(t, "streamplace_webrtc_source_age_ms", labels)
	require.NoError(t, mm.ValidateMP4(ctx, bytes.NewReader(segments[0]), true))
	select {
	case <-received:
	case <-ctx.Done():
		t.Fatal("validated segment did not reach the local WebRTC receiver")
	}

	var firstSendAge float64
	require.Eventually(t, func() bool {
		count, sum := histogramSample(t, "streamplace_webrtc_source_age_ms", labels)
		if count-beforeCount != 1 {
			return false
		}
		firstSendAge = (sum - beforeSum) / float64(count-beforeCount)
		return true
	}, 5*time.Second, 25*time.Millisecond, "ValidateMP4 should record the first RTP source age")
	require.Eventually(t, func() bool {
		count, sum := histogramSample(t, "streamplace_media_segment_duration_ms", durationLabels)
		if count-durationBeforeCount != 1 {
			return false
		}
		measuredDuration := (sum - durationBeforeSum) / float64(count-durationBeforeCount)
		return measuredDuration > 0
	}, 5*time.Second, 25*time.Millisecond, "ValidateMP4 should record source segment duration")
	t.Logf("source-start-to-first-RTP_ms=%.1f", firstSendAge)
	return firstSendAge
}

// newBareSegmentSigner builds a MediaSignerLocal with a fresh ES256K key +
// self-signed cert and a cawg.metadata-bearing prebuilt manifest — enough to
// drive SignSegmentStream in tests.
func newBareSegmentSigner(t *testing.T) *MediaSignerLocal {
	t.Helper()
	atPriv, err := atcrypto.GeneratePrivateKeyK256()
	require.NoError(t, err)
	secpPriv, _ := secp256k1.PrivKeyFromBytes(atPriv.Bytes())
	require.NotNil(t, secpPriv)
	var signer crypto.Signer = secpPriv.ToECDSA()
	cert, err := signers.GenerateES256KCert(signer)
	require.NoError(t, err)
	pub, err := aqpub.FromPublicKey(secpPriv.ToECDSA().Public().(*ecdsa.PublicKey))
	require.NoError(t, err)
	return &MediaSignerLocal{
		StreamerName: "test-streamer",
		Signer:       signer,
		AQPub:        pub,
		Cert:         cert,
		PrebuiltManifest: []byte(`{
			"title": "bare segment test",
			"assertions": [
				{"label":"c2pa.actions","data":{"actions":[{"action":"c2pa.created"}]}},
				{"label":"cawg.metadata","data":{
					"@context":{"dc":"http://purl.org/dc/elements/1.1/"},
					"dc:creator":"did:example","dc:title":"t",
					"dc:date":"1970-01-01T00:00:00.000Z"
				}}
			]
		}`),
	}
}

// TestValidateMP4MediaBareSegment exercises the full .m4s-native validate
// path: sign the fragmented fixture per-segment (the live ingest shape),
// reassemble one GoP's bare canonical .m4s, and run ValidateMP4Media over it.
// That wraps the bare segment to a flat MP4 for gstreamer (codec/dimensions)
// and verifies the signatures in-wasm — proving a signed bare .m4s parses
// through qtdemux with the correct co64 offsets wrap-flat synthesizes.
func TestValidateMP4MediaBareSegment(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)

	frag, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)

	// Sign per-segment; keep the first GoP's bare .m4s (what ValidateMP4 gets
	// per call in the live path).
	eventCh := make(chan *muxl.MuxlEvent, 16)
	errCh := make(chan error, 1)
	go func() {
		err := ms.SignSegmentStream(ctx, bytes.NewReader(frag), eventCh)
		close(eventCh)
		errCh <- err
	}()
	var m4s []byte
	for ev := range eventCh {
		if ev.Type == "signed-segment" && m4s == nil {
			m4s = concatTracksSorted(ev.Tracks)
		}
	}
	require.NoError(t, <-errCh)
	require.NotEmpty(t, m4s, "expected at least one signed GoP")

	res, err := ValidateMP4Media(ctx, m4s)
	require.NoError(t, err)
	require.NotNil(t, res.MediaData)
	require.NotEmpty(t, res.MediaData.Video, "should parse a video track")
	require.Greater(t, res.MediaData.Video[0].Width, 0, "video width from qtdemux")
	require.Greater(t, res.MediaData.Video[0].Height, 0, "video height from qtdemux")
	require.NotEmpty(t, res.MediaData.Audio, "should parse an audio track")
}

// TestFeedLiveWindow signs the fixture per-segment, folds the resulting bare
// .m4s into a MediaManager live-HLS window via feedLiveWindow, and confirms the
// window is populated per track with retrievable signed segments + valid
// playlists — the feed path ValidateMP4 drives for every validated segment.
func TestFeedLiveWindow(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)
	frag, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)

	eventCh := make(chan *muxl.MuxlEvent, 16)
	errCh := make(chan error, 1)
	go func() {
		err := ms.SignSegmentStream(ctx, bytes.NewReader(frag), eventCh)
		close(eventCh)
		errCh <- err
	}()
	var m4s []byte
	for ev := range eventCh {
		if ev.Type != "signed-segment" {
			continue
		}
		tids := make([]string, 0, len(ev.Tracks))
		for tid := range ev.Tracks {
			tids = append(tids, tid)
		}
		sort.Strings(tids)
		for _, tid := range tids {
			m4s = append(m4s, ev.Tracks[tid]...)
		}
	}
	require.NoError(t, <-errCh)
	require.NotEmpty(t, m4s)

	mm := &MediaManager{liveWindows: map[string]*livehls.Writer{}}
	streamer := "did:test:streamer"

	// Pre-live (unpublished) segments must NOT be folded into the live window —
	// live HLS is unauthenticated, so anything in the window is world-readable.
	mm.feedLiveWindow(ctx, streamer, m4s, false, nil)
	require.Nil(t, mm.GetLiveWindow(streamer),
		"unpublished segment must not create a live-HLS window")

	timing := &bus.SegmentTiming{
		SegmentID:   "hls-availability-test",
		SourceStart: time.Now().Add(-500 * time.Millisecond),
		Signed:      time.Now().Add(-10 * time.Millisecond),
	}
	beforeAvailable := histogramSampleCount(t, "streamplace_hls_segment_available_ms", map[string]string{
		"streamer": streamer,
	})
	beforeSourceAge := histogramSampleCount(t, "streamplace_media_source_age_ms", map[string]string{
		"streamer": streamer,
		"stage":    "hls_available",
	})
	mm.feedLiveWindow(ctx, streamer, m4s, true, timing)
	require.Greater(t, histogramSampleCount(t, "streamplace_hls_segment_available_ms", map[string]string{
		"streamer": streamer,
	}), beforeAvailable, "HLS availability timing should be recorded")
	require.Greater(t, histogramSampleCount(t, "streamplace_media_source_age_ms", map[string]string{
		"streamer": streamer,
		"stage":    "hls_available",
	}), beforeSourceAge, "HLS source age should be recorded")

	w := mm.GetLiveWindow(streamer)
	require.NotNil(t, w, "window created on feed")
	tids := w.TrackIDs()
	require.NotEmpty(t, tids, "window has tracks")
	for _, tid := range tids {
		require.NotEmpty(t, w.InitSegment(tid), "track %s has an init segment", tid)
		tr := w.Track(tid)
		require.NotEmpty(t, tr.Segments, "track %s has segments", tid)
		data := w.SegmentData(tid, tr.Segments[0].Seq)
		require.GreaterOrEqual(t, len(data), 8)
		require.Equal(t, "uuid", string(data[4:8]), "track %s segment is the signed .m4s", tid)
		pl := w.MediaPlaylist(tid, "init.mp4", func(seq uint64) string { return fmt.Sprintf("seg%d.m4s", seq) })
		require.Contains(t, pl, "#EXT-X-MAP")
		require.Contains(t, pl, "#EXTINF")
	}
	require.Contains(t, w.MasterPlaylist(func(tid string) string { return tid + ".m3u8" }), "#EXTM3U")
}

// seedBanLabel writes an active ban label for did into the model.
func seedBanLabel(t *testing.T, mod model.Model, did string) {
	t.Helper()
	lex := &comatproto.LabelDefs_Label{
		Cts: time.Now().UTC().Format(time.RFC3339),
		Src: "did:plc:test-labeler",
		Uri: did,
		Val: atproto.LabelDMCAViolation,
	}
	var buf bytes.Buffer
	require.NoError(t, lex.MarshalCBOR(&buf))
	require.NoError(t, mod.CreateLabel(&model.Label{
		Src:    lex.Src,
		Uri:    did,
		Val:    atproto.LabelDMCAViolation,
		Record: buf.Bytes(),
	}))
}

// TestStreamerIsBanned exercises the defense-in-depth gate validateSource applies
// at the ingest chokepoint: a streamer with an active ban label is rejected
// regardless of whether their ingest worker was torn down. A clean streamer
// passes; the ONLY change between the two checks is the ban label.
func TestStreamerIsBanned(t *testing.T) {
	mm, _ := getStaticTestMediaManager(t)
	did := "did:plc:bannedstreamer"

	banned, err := mm.streamerIsBanned(did)
	require.NoError(t, err)
	require.False(t, banned, "a clean streamer is not banned")

	seedBanLabel(t, mm.model, did)

	banned, err = mm.streamerIsBanned(did)
	require.NoError(t, err)
	require.True(t, banned, "an active ban label is detected at the validate chokepoint")
}
