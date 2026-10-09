package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/muxl"
)

func earlyState(t *testing.T, epoch uint64) *earlyAACSession {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	r, w := io.Pipe()
	s := &earlyAACSession{epoch: epoch, ctx: ctx, cancel: cancel, writer: w, peers: map[*earlyAACPeer]struct{}{}}
	t.Cleanup(func() { s.fail(io.EOF); r.Close() })
	return s
}
func earlyItem(video bool, ts time.Duration, size int) earlyAACSample {
	data := make([]byte, size)
	if video {
		data = []byte{0, 0, 0, 1, 0x65, 1}
	}
	return earlyAACSample{video: video, sample: bus.PacketizedSample{Data: data, Duration: 20 * time.Millisecond, Timestamp: ts, HasTimestamp: true}}
}

func TestEarlyAACPausedCacheAtomicLiveJoin(t *testing.T) {
	s := earlyState(t, 1)
	s.publish(earlyItem(true, 0, 10))
	require.Nil(t, s.join(), "video without converted audio is not ready")
	s.publish(earlyItem(false, 0, 10))
	for i := 0; i < 100; i++ {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() { defer wg.Done(); s.publish(earlyItem(false, time.Duration(i+1)*time.Millisecond, 10)) }()
		p := s.join()
		require.NotNil(t, p)
		wg.Wait()
		require.Len(t, p.queue, i+3, "atomic cache snapshot plus live delivery has no gap or duplicate")
		require.True(t, (<-p.queue).video, "startup always establishes video origin first")
		s.leave(p)
	}
	p := s.join()
	require.NotNil(t, p, "paused source needs no successor to join")
	s.leave(p)
}
func TestEarlyAACJoinRejectsPreviousGOPAudio(t *testing.T) {
	s := earlyState(t, 1)
	s.publish(earlyItem(false, 0, 10))
	s.publish(earlyItem(true, time.Second, 10))
	require.Nil(t, s.join(), "previous GOP audio cannot admit the current IDR")
	s.publish(earlyItem(false, time.Second+time.Millisecond, 10))
	p := s.join()
	require.NotNil(t, p)
	require.True(t, (<-p.queue).video)
	s.leave(p)
}

func TestEarlyAACRejectsNonIDRStartup(t *testing.T) {
	s := earlyState(t, 1)
	item := earlyItem(true, 0, 1)
	item.sample.Data = []byte{0, 0, 0, 1, 0x41, 1}
	s.publish(item)
	s.publish(earlyItem(false, 0, 1))
	require.Nil(t, s.join())
	require.Error(t, s.feed(context.Background(), []byte{1}, []bus.PacketizedSample{item.sample}))
	require.Error(t, s.ctx.Err())
}

func TestEarlyAACBoundsFailClosed(t *testing.T) {
	for _, kind := range []string{"cache bytes", "cache count", "subscriber bytes", "subscriber count"} {
		t.Run(kind, func(t *testing.T) {
			s := earlyState(t, 1)
			s.publish(earlyItem(true, 0, 1))
			s.publish(earlyItem(false, 0, 1))
			p := s.join()
			require.NotNil(t, p)
			switch kind {
			case "cache bytes":
				s.publish(earlyItem(false, time.Millisecond, earlyByteLimit))
			case "cache count":
				for i := 0; i < earlySampleLimit; i++ {
					s.publish(earlyItem(false, time.Millisecond, 1))
				}
			case "subscriber bytes":
				s.mu.Lock()
				p.bytes = earlyByteLimit
				s.mu.Unlock()
				s.publish(earlyItem(false, time.Millisecond, 1))
			case "subscriber count":
				for i := 0; i < earlySampleLimit; i++ {
					s.mu.Lock()
					s.cache = nil
					s.cacheBytes = 0
					s.mu.Unlock()
					s.publish(earlyItem(false, time.Millisecond, 1))
				}
			}
			require.Error(t, p.ctx.Err(), "overflow cancels selected peer, never drops and continues")
			if kind == "cache bytes" || kind == "cache count" {
				require.Nil(t, s.join())
				require.Error(t, s.ctx.Err())
			}
		})
	}
}
func TestEarlyAACAdmissionAndEpochAbort(t *testing.T) {
	s := earlyState(t, 2)
	s.publish(earlyItem(true, 0, 1))
	s.publish(earlyItem(false, 0, 1))
	mm := &MediaManager{cli: &config.CLI{}, bus: bus.NewBus(), earlySessions: map[string]*earlyAACSession{"did:test": s}}
	_, p := mm.joinEarlyAAC("did:test", "source")
	require.Nil(t, p, "flag off is legacy")
	mm.cli.ExperimentalEarlyAACPlayback = true
	mm.cli.MaximumLiveBitrate = 1
	_, p = mm.joinEarlyAAC("did:test", "source")
	require.Nil(t, p)
	mm.cli.MaximumLiveBitrate = 0
	_, p = mm.joinEarlyAAC("did:test", "720p")
	require.Nil(t, p)
	_, p = mm.joinEarlyAAC("did:test", "source")
	require.NotNil(t, p)
	mm.feedEarlyAAC(withIngestSession(context.Background(), 1), &validatedSegment{repoDID: "did:test", meta: &SegmentMetadata{Published: true}}, []byte("stale malformed source"))
	require.NoError(t, p.ctx.Err(), "stale feed is rejected before parsing and cannot poison current cache")
	mm.abortEarlyEpoch(withIngestSession(context.Background(), 1))
	require.NoError(t, p.ctx.Err(), "stale failure cannot abort newer ingest")
	abortCtx := withIngestSession(context.Background(), 2)
	ingestState(abortCtx).did = "did:test"
	mm.abortEarlyEpoch(abortCtx)
	require.Error(t, p.ctx.Err())
	_, p = mm.joinEarlyAAC("did:test", "source")
	require.Nil(t, p)
	newer := earlyState(t, 3)
	newer.publish(earlyItem(true, 0, 1))
	newer.publish(earlyItem(false, 0, 1))
	mm.earlySessions["did:test"] = newer
	_, p = mm.joinEarlyAAC("did:test", "source")
	require.NotNil(t, p, "fresh epoch reconnect may select early again")
	newer.leave(p)
}
func TestEarlyAACNativeConverterCanonicalContinuity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	signer := newBareSegmentSigner(t)
	segs := nativeRTMPSignedFixture(t, ctx, signer)
	s, err := newEarlyAACSession(ctx, 1, t.Name())
	require.NoError(t, err)
	defer s.fail(io.EOF)
	key, err := signers.MarshalES256KPrivateKeyPEM(signer.Signer)
	require.NoError(t, err)
	mm := &MediaManager{cli: &config.CLI{BroadcasterHost: "test.example.com"}}
	completed := make(chan []byte, 4)
	canonical := mm.newStreamTranscoder(ctx, "opus", signer.Cert, key, func(_ any, data []byte) { completed <- data })
	defer canonical.Close()
	var p *earlyAACPeer
	var all []bus.PacketizedSample
	drain := func() {
		if p == nil {
			return
		}
		for {
			select {
			case item, ok := <-p.queue:
				if !ok {
					return
				}
				s.take(p, item)
				if !item.video {
					all = append(all, item.sample)
				}
			default:
				return
			}
		}
	}
	for i, seg := range segs {
		events, e := unwrapMuxlEvents(ctx, seg)
		require.NoError(t, e)
		cat, tracks := catalogAndTracks(events)
		var video []byte
		for _, v := range cat.Video.Renditions {
			video = tracks[fmt.Sprint(v.TrackID())]
			break
		}
		var flat bytes.Buffer
		require.NoError(t, muxl.RunMuxlWrap(ctx, bytes.NewReader(video), "flat", &flat))
		packet, e := Packetize(ctx, mm.cli, &bus.Seg{Data: flat.Bytes()})
		require.NoError(t, e)
		require.True(t, packet.Video[0].HasTimestamp)
		require.NoError(t, restoreEarlyVideoOrigin(events, cat, packet.Video))
		require.Equal(t, time.Duration(i)*time.Second, packet.Video[0].Timestamp)
		t.Logf("source %d first video=%s", i, packet.Video[0].Timestamp)
		require.NoError(t, canonical.Feed(seg, i))
		require.NoError(t, s.feed(ctx, seg, packet.Video))
		require.Eventually(t, func() bool {
			s.mu.Lock()
			defer s.mu.Unlock()
			for _, item := range s.cache {
				if !item.video && item.sample.Timestamp >= packet.Video[0].Timestamp {
					return true
				}
			}
			return false
		}, time.Second, 5*time.Millisecond)
		if i == 0 {
			require.Empty(t, completed)
			p = s.join()
			require.NotNil(t, p)
		}
		drain()
	}
	require.NoError(t, s.writer.Close())
	// Normal EOS closes admission while leaving the published tail available.
	require.Eventually(t, func() bool { drain(); return len(all) == 203 }, time.Second, 5*time.Millisecond)
	require.Len(t, all, 203)
	require.Zero(t, all[0].Timestamp)
	require.Equal(t, 13500*time.Microsecond, all[0].Duration)
	for i := 1; i < len(all); i++ {
		require.InDelta(t, (all[i-1].Timestamp + all[i-1].Duration).Nanoseconds(), all[i].Timestamp.Nanoseconds(), float64(time.Millisecond))
	}
	recorder := &earlyRTPRecorder{}
	codec := webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}
	track, e := webrtc.NewTrackLocalStaticRTP(codec, "audio", "native-clock")
	require.NoError(t, e)
	_, e = track.Bind(earlyTrackContext{webrtc.RTPCodecParameters{RTPCodecCapability: codec, PayloadType: 111}, recorder})
	require.NoError(t, e)
	packetizer := rtp.NewPacketizerWithOptions(1200, &codecs.OpusPayloader{}, rtp.NewFixedSequencer(123), 48000, rtp.WithTimestamp(987654))
	for _, sample := range all {
		require.NoError(t, writeEarlyAACPackets(track, packetizer, sample, 0, 48000))
	}
	require.Len(t, recorder.headers, 203)
	for i, sample := range all {
		require.Equal(t, sample.Data, recorder.payloads[i])
		require.Equal(t, uint32(sample.Timestamp.Nanoseconds()*48000/int64(time.Second)), recorder.headers[i].Timestamp)
	}
	require.Equal(t, uint32(648), recorder.headers[1].Timestamp-recorder.headers[0].Timestamp)
	require.Error(t, writeEarlyAACPackets(track, packetizer, bus.PacketizedSample{Data: all[0].Data, Duration: time.Millisecond}, 0, 48000), "missing native timestamp fails closed")
	require.NoError(t, canonical.Close())
	require.Len(t, completed, 4)
	for i := 0; i < 4; i++ {
		data := <-completed
		verification, e := muxl.RunMuxlVerify(ctx, bytes.NewReader(data))
		require.NoError(t, e)
		require.NotContains(t, verification, `"validation_state":"Invalid"`)
		require.Len(t, audioCodecsOf(t, ctx, data), 2)
	}
}
func TestEarlyAACPeerCachedHandshakeAndPartialFailure(t *testing.T) {
	earlyAACPeerFailureGate(t, false)
}
func TestEarlyAACDelayedAudioClosesRealPeer(t *testing.T) { earlyAACPeerFailureGate(t, true) }
func earlyAACPeerFailureGate(t *testing.T, late bool) {
	fixture := playbackPacketFixture(t)
	mm := loopbackPlaybackManager()
	mm.cli.ExperimentalEarlyAACPlayback = true
	s := earlyState(t, 1)
	mm.earlySessions = map[string]*earlyAACSession{t.Name(): s}
	var ts time.Duration
	for _, sample := range fixture.Video {
		if sample.Duration <= 0 {
			sample.Duration = time.Second / 30
		}
		sample.Timestamp = ts
		sample.HasTimestamp = true
		s.publish(earlyAACSample{video: true, sample: sample})
		ts += sample.Duration
	}
	ts = 0
	for _, sample := range fixture.Audio {
		sample.Timestamp = ts
		sample.HasTimestamp = true
		s.publish(earlyAACSample{sample: sample})
		ts += sample.Duration
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	receiver, err := mm.webrtcAPI.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	defer receiver.Close()
	for _, kind := range []webrtc.RTPCodecType{webrtc.RTPCodecTypeVideo, webrtc.RTPCodecTypeAudio} {
		_, err = receiver.AddTransceiverFromKind(kind, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly})
		require.NoError(t, err)
	}
	received := make(chan struct{}, 2)
	receiver.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		if _, _, e := track.ReadRTP(); e == nil {
			received <- struct{}{}
		}
	})
	offer, err := receiver.CreateOffer(nil)
	require.NoError(t, err)
	require.NoError(t, receiver.SetLocalDescription(offer))
	<-webrtc.GatheringCompletePromise(receiver)
	answer, err := mm.WebRTCPlayback2(ctx, t.Name(), "source", receiver.LocalDescription(), "")
	require.NoError(t, err)
	require.Eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.peers) == 1 }, time.Second, time.Millisecond)
	// The startup selection retains a paused cache through a delayed handshake.
	select {
	case <-time.After(100 * time.Millisecond):
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.NoError(t, receiver.SetRemoteDescription(*answer))
	select {
	case <-received:
	case <-ctx.Done():
		t.Fatal("cached early RTP never arrived")
	}
	if late {
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		s.publish(earlyAACSample{sample: bus.PacketizedSample{Data: fixture.Audio[0].Data, HasTimestamp: true, Timestamp: 0, Duration: 20 * time.Millisecond}})
	} else {
		s.fail(fmt.Errorf("conversion failed after RTP"))
	}
	mm.bus.PublishSegment(ctx, t.Name(), WebRTCSourceRendition, &bus.Seg{Published: true, PacketizedData: fixture})
	require.Eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.peers) == 0 }, time.Second, time.Millisecond, "failed selected peer must close and unsubscribe, never fall back")
	require.Eventually(t, func() bool { return mm.bus.GetViewerCount(t.Name()) == 0 }, time.Second, time.Millisecond)
}

func TestEarlyAACVerifiedAdmission(t *testing.T) {
	fixtureCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	signer, _ := earlyWebRTCSource(t, true)
	segs := nativeRTMPSignedFixture(t, fixtureCtx, signer)
	for _, kind := range []string{"enabled", "flag off", "bitrate", "no bus", "unpublished", "malformed"} {
		t.Run(kind, func(t *testing.T) {
			mm := earlyWebRTCManager(t, signer)
			mm.cli.ExperimentalEarlyAACPlayback = true
			ctx := withIngestSession(context.Background(), 1)
			source := segs[0]
			switch kind {
			case "flag off":
				mm.cli.ExperimentalEarlyAACPlayback = false
			case "bitrate":
				mm.cli.MaximumLiveBitrate = 1
			case "no bus":
				mm.bus = nil
			case "malformed":
				source = source[:7]
			}
			if kind == "unpublished" {
				vs := &validatedSegment{repoDID: signer.Streamer(), meta: &SegmentMetadata{Published: false}}
				mm.feedEarlyAAC(ctx, vs, source)
			} else {
				err := mm.ValidateMP4(ctx, bytes.NewReader(source), true)
				if kind == "malformed" {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			}
			mm.earlyMu.Lock()
			s := mm.earlySessions[signer.Streamer()]
			mm.earlyMu.Unlock()
			if kind != "enabled" {
				require.Nil(t, s)
				return
			}
			require.NotNil(t, s)
			defer s.fail(io.EOF)
			defer s.idle.Stop()
			require.Eventually(t, func() bool {
				p := s.join()
				if p == nil {
					return false
				}
				s.leave(p)
				return true
			}, time.Second, time.Millisecond)
			require.Empty(t, mm.newSegmentSubs[0].queue, "canonical output still waits for signed completion")
			mm.transcodersMu.Lock()
			tr := mm.transcoders[signer.Streamer()]
			mm.transcodersMu.Unlock()
			require.NotNil(t, tr)
			require.NoError(t, tr.Close())
			select {
			case notification := <-mm.newSegmentSubs[0].queue:
				require.False(t, notification.WebRTCPublished, "AAC experiment never suppresses canonical WebRTC publication")
				require.Equal(t, source, notification.Muxl[:len(source)], "original canonical source remains present")
			case <-time.After(5 * time.Second):
				t.Fatal("canonical completion missing")
			}
			mm.abortEarlyEpoch(ctx)
			require.Nil(t, s.join())
		})
	}
}

func TestEarlyAACBlockedFeedCancellationJoins(t *testing.T) {
	s := earlyState(t, 1)
	s.started = true
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.feed(ctx, []byte{1, 2, 3}, []bus.PacketizedSample{earlyItem(true, 0, 1).sample}) }()
	cancel()
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("cancelled feed retained blocked pipe writer")
	}
	require.Error(t, s.ctx.Err())
	require.Error(t, s.feed(context.Background(), []byte{4}, nil), "failed same epoch cannot restart")
}

func TestEarlyAACFailedBeforeSelectionUsesLegacy(t *testing.T) {
	mm := loopbackPlaybackManager()
	mm.cli.ExperimentalEarlyAACPlayback = true
	s := earlyState(t, 1)
	s.publish(earlyItem(true, 0, 1))
	s.publish(earlyItem(false, 0, 1))
	s.fail(fmt.Errorf("conversion failed before selection"))
	mm.earlySessions = map[string]*earlyAACSession{t.Name(): s}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	receiver := createAudioReceiver(t, ctx, mm.webrtcAPI)
	defer receiver.Close()
	answer, err := mm.WebRTCPlayback2(ctx, t.Name(), "source", receiver.LocalDescription(), "")
	require.NoError(t, err)
	require.NotNil(t, answer)
	s.mu.Lock()
	require.Empty(t, s.peers)
	s.mu.Unlock()
	require.Zero(t, mm.bus.GetViewerCount(t.Name()))
}

func TestEarlyAACConverterSourceOverflow(t *testing.T) {
	s := earlyState(t, 1)
	require.Error(t, s.feed(context.Background(), make([]byte, earlySourceLimit+1), nil))
	require.Error(t, s.ctx.Err())
	require.Nil(t, s.join())
}
