package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/pion/webrtc/v4"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/muxl"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEarlyAACOrderlyEOFKeepsQueuedPeerTail(t *testing.T) {
	s := earlyState(t, 1)
	s.publish(earlyItem(true, 0, 1))
	s.publish(earlyItem(false, 0, 1))
	p := s.join()
	require.NotNil(t, p)
	s.fail(io.EOF)
	require.NoError(t, p.ctx.Err(), "normal EOS must preserve queued peer packets")
	require.Nil(t, s.join(), "ended stream stops new admission")
	require.Len(t, p.queue, 2)
}

func TestEarlyAACFiniteNativePeerDrainsAllPackets(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	signer := newBareSegmentSigner(t)
	segs := nativeRTMPSignedFixture(t, ctx, signer)
	mm := loopbackPlaybackManager()
	mm.cli.ExperimentalEarlyAACPlayback = true
	s, err := newEarlyAACSession(ctx, 1, t.Name())
	require.NoError(t, err)
	defer s.fail(fmt.Errorf("test cleanup"))
	mm.earlySessions = map[string]*earlyAACSession{t.Name(): s}
	videoPackets := make([][]bus.PacketizedSample, len(segs))
	for i := range segs {
		events, e := unwrapMuxlEvents(ctx, segs[i])
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
		require.NoError(t, restoreEarlyVideoOrigin(events, cat, packet.Video))
		videoPackets[i] = packet.Video
	}
	finalVideo := videoPackets[len(videoPackets)-1]
	wantLastVideo := uint32(finalVideo[len(finalVideo)-1].Timestamp.Nanoseconds() * 90000 / int64(time.Second))
	feeds := make(chan int)
	fed := make(chan error)
	var sourceCtx context.Context
	_, signingDone, err := mm.ingestSigningElem(ctx, func(_ context.Context, _ io.Reader, events chan *muxl.MuxlEvent) error {
		for i := range feeds {
			events <- &muxl.MuxlEvent{Type: "signed-segment", Tracks: map[string][]byte{"1": segs[i]}}
		}
		return nil
	}, func(callbackCtx context.Context, data []byte) error {
		sourceCtx = callbackCtx
		state := ingestState(callbackCtx)
		state.mu.Lock()
		state.did = t.Name()
		state.mu.Unlock()
		// The fixture already verified these real native signed source bytes.
		index := 0
		for !bytes.Equal(data, segs[index]) {
			index++
		}
		err := s.feed(callbackCtx, data, videoPackets[index])
		fed <- err
		return err
	})
	require.NoError(t, err)
	feed := func(i int) { feeds <- i; require.NoError(t, <-fed) }
	feed(0)
	require.Equal(t, s.epoch, ingestSessionFromContext(sourceCtx))
	require.Eventually(t, func() bool {
		p := s.join()
		if p == nil {
			return false
		}
		s.leave(p)
		return true
	}, time.Second, time.Millisecond)
	receiver, err := mm.webrtcAPI.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	defer receiver.Close()
	for _, kind := range []webrtc.RTPCodecType{webrtc.RTPCodecTypeVideo, webrtc.RTPCodecTypeAudio} {
		_, err = receiver.AddTransceiverFromKind(kind, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly})
		require.NoError(t, err)
	}
	var mu sync.Mutex
	audioCount := 0
	lastVideo := uint32(0)
	receiver.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		for {
			packet, _, e := track.ReadRTP()
			if e != nil {
				return
			}
			mu.Lock()
			if track.Kind() == webrtc.RTPCodecTypeAudio {
				audioCount++
			} else if packet.Timestamp > lastVideo {
				lastVideo = packet.Timestamp
			}
			mu.Unlock()
		}
	})
	offer, err := receiver.CreateOffer(nil)
	require.NoError(t, err)
	require.NoError(t, receiver.SetLocalDescription(offer))
	<-webrtc.GatheringCompletePromise(receiver)
	answer, err := mm.WebRTCPlayback2(ctx, t.Name(), "source", receiver.LocalDescription(), "")
	require.NoError(t, err)
	require.NoError(t, receiver.SetRemoteDescription(*answer))
	require.Eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return audioCount > 0 }, time.Second, time.Millisecond)
	feedOrigin := time.Now()
	for i := 1; i < 4; i++ {
		timer := time.NewTimer(time.Until(feedOrigin.Add(time.Duration(i)*time.Second - 100*time.Millisecond)))
		select {
		case <-timer.C:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		feed(i)
	}
	close(feeds)
	select {
	case <-signingDone:
	case <-time.After(6 * time.Second):
		t.Fatal("signing owner omitted epoch termination")
	}
	defer func() { mu.Lock(); t.Logf("received audio=%d lastvideo=%d", audioCount, lastVideo); mu.Unlock() }()
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return audioCount == 203 && lastVideo == wantLastVideo
	}, 5*time.Second, 5*time.Millisecond)
	mu.Lock()
	require.Equal(t, 203, audioCount)
	require.Equal(t, wantLastVideo, lastVideo)
	mu.Unlock()
	require.Eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.peers) == 0 }, time.Second, time.Millisecond)
	select {
	case <-s.encoderDone:
	case <-time.After(time.Second):
		t.Fatal("native converter cleanup not bounded")
	}
}

func TestEarlyAACEpochRetirementDoesNotRetainHistoricalDIDs(t *testing.T) {
	mm := loopbackPlaybackManager()
	mm.earlySessions = map[string]*earlyAACSession{}
	for i := 0; i < 100; i++ {
		did := fmt.Sprint("did:churn:", i)
		ctx := withIngestSession(context.Background(), uint64(i+1))
		require.True(t, mm.prepareEarlyAAC(ctx, did))
		s := earlyState(t, uint64(i+1))
		mm.earlySessions[did] = s
		mm.abortEarlyEpoch(ctx)
		require.Empty(t, mm.earlySessions)
		require.False(t, mm.prepareEarlyAAC(ctx, did), "retired callback cannot recreate session")
	}
	ctx := withIngestSession(context.Background(), 200)
	require.True(t, mm.prepareEarlyAAC(ctx, "did:shared"))
	old := earlyState(t, 200)
	mm.earlySessions["did:shared"] = old
	newer := earlyState(t, 201)
	mm.earlySessions["did:shared"] = newer
	mm.abortEarlyEpoch(ctx)
	require.Same(t, newer, mm.earlySessions["did:shared"])
	require.NoError(t, newer.ctx.Err())
	require.False(t, mm.prepareEarlyAAC(ctx, "did:shared"))
}

func TestEarlyAACFailedEpochRejectsPreparationBeforeMediaOperations(t *testing.T) {
	mm := loopbackPlaybackManager()
	mm.earlySessions = map[string]*earlyAACSession{}
	ctx := withIngestSession(context.Background(), 1)
	require.True(t, mm.prepareEarlyAAC(ctx, "user"))
	s := earlyState(t, 1)
	mm.earlySessions["user"] = s
	s.fail(fmt.Errorf("converter failed"))
	require.False(t, mm.prepareEarlyAAC(ctx, "user"), "preparation decision must reject before unwrap/wrap/Packetize")
	mm.abortEarlyEpoch(ctx)
	require.Empty(t, mm.earlySessions)
	require.False(t, mm.prepareEarlyAAC(ctx, "user"))
}

func TestEarlyAACIngestFlushClosesOnlyMatchingEpoch(t *testing.T) {
	mm := loopbackPlaybackManager()
	ctx := withIngestSession(context.Background(), 1)
	ingestState(ctx).did = "user"
	s := earlyState(t, 1)
	s.publish(earlyItem(true, 0, 1))
	s.publish(earlyItem(false, 0, 1))
	mm.earlySessions = map[string]*earlyAACSession{"user": s}
	mm.closeEarlyEpoch(ctx)
	require.Nil(t, s.join())
	require.True(t, s.ended)
	require.Empty(t, mm.earlySessions)
}
