package media

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/captions"
)

// viewerOffer is the offer of a WHEP viewer: receive-only H264 video and Opus
// audio, plus a data channel when withDataChannel is set. It returns the
// viewer's peer connection too, to complete the handshake with the answer.
func viewerOffer(t *testing.T, withDataChannel bool) (*webrtc.PeerConnection, *webrtc.SessionDescription) {
	t.Helper()
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = pc.Close() })
	_, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly})
	require.NoError(t, err)
	_, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly})
	require.NoError(t, err)
	if withDataChannel {
		_, err = pc.CreateDataChannel("viewer", nil)
		require.NoError(t, err)
	}
	offer, err := pc.CreateOffer(nil)
	require.NoError(t, err)
	require.NoError(t, pc.SetLocalDescription(offer))
	<-webrtc.GatheringCompletePromise(pc)
	return pc, pc.LocalDescription()
}

func TestOffersDataChannel(t *testing.T) {
	_, withData := viewerOffer(t, true)
	require.True(t, offersDataChannel(withData))

	_, mediaOnly := viewerOffer(t, false)
	require.False(t, offersDataChannel(mediaOnly))

	require.False(t, offersDataChannel(&webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: "not sdp"}))
}

func setCaptionWindowPublished(mm *MediaManager, user string, published bool) {
	mm.liveWindowsMut.Lock()
	defer mm.liveWindowsMut.Unlock()
	mm.liveWindowPublished[user] = published
}

func requireNoCaptionMessage(t *testing.T, messages <-chan map[string]any) {
	t.Helper()
	select {
	case msg := <-messages:
		t.Fatalf("unexpected caption message: %#v", msg)
	case <-time.After(300 * time.Millisecond):
	}
}

func openWHEPCaptionMessages(t *testing.T, mm *MediaManager, user, viewer string) <-chan map[string]any {
	t.Helper()
	viewerPC, offer := viewerOffer(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	messages := make(chan map[string]any, 16)
	opened := make(chan struct{}, 1)
	viewerPC.OnDataChannel(func(dc *webrtc.DataChannel) {
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			var m map[string]any
			if err := json.Unmarshal(msg.Data, &m); err == nil {
				messages <- m
			}
		})
		dc.OnOpen(func() { opened <- struct{}{} })
	})
	answer, err := mm.WebRTCPlayback2(ctx, user, "test-rendition", offer, viewer)
	require.NoError(t, err)
	require.NoError(t, viewerPC.SetRemoteDescription(*answer))
	select {
	case <-opened:
	case <-time.After(20 * time.Second):
		t.Fatal("the captions data channel did not open")
	}
	return messages
}

// A WHEP viewer that negotiates a data channel gets a "captions" channel
// carrying liveCue JSON: the current line on open, then each cue event of the
// streamer. One that does not gets an ordinary answer without one.
func TestWHEPCaptionsDataChannel(t *testing.T) {
	mm, _ := getStaticTestMediaManager(t)
	const user = "did:plc:captioned"
	setCaptionWindowPublished(mm, user, true)

	// The line spoken just before the viewer joins.
	now := time.Now()
	track := captions.Track{ID: "canonical-auto-en", Language: "en", Kind: captions.KindCaptions, Source: captions.SourceAuto, Origin: captions.OriginCanonical}
	mm.bus.Captions.Publish(user, track, captions.Cue{ID: "joined", Start: now.Add(-3 * time.Second), End: now.Add(-time.Second), Text: "before you joined", Final: true})

	viewer, offer := viewerOffer(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	messages := make(chan map[string]any, 16)
	opened := make(chan *webrtc.DataChannel, 1)
	viewer.OnDataChannel(func(dc *webrtc.DataChannel) {
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			var m map[string]any
			if err := json.Unmarshal(msg.Data, &m); err == nil {
				messages <- m
			}
		})
		opened <- dc
	})

	answer, err := mm.WebRTCPlayback2(ctx, user, "test-rendition", offer, "")
	require.NoError(t, err)
	require.NoError(t, viewer.SetRemoteDescription(*answer))

	var dc *webrtc.DataChannel
	select {
	case dc = <-opened:
	case <-time.After(20 * time.Second):
		t.Fatal("the server did not open a data channel")
	}
	require.Equal(t, "captions", dc.Label())

	next := func() map[string]any {
		select {
		case m := <-messages:
			return m
		case <-time.After(10 * time.Second):
			t.Fatal("no caption message")
			return nil
		}
	}
	first := next()
	require.Equal(t, "place.stream.caption.defs#liveCue", first["$type"])
	require.Equal(t, "before you joined", first["text"])
	require.Equal(t, true, first["final"])

	// Live events: interim, then the final text, under the same id.
	require.Eventually(t, func() bool { return len(mm.bus.Captions.Tracks(user)) == 1 }, time.Second, 10*time.Millisecond)
	mm.bus.Captions.Publish(user, track, captions.Cue{ID: "live", Start: now, End: now.Add(time.Second), Text: "hel"})
	mm.bus.Captions.Publish(user, track, captions.Cue{ID: "live", Start: now, End: now.Add(time.Second), Text: "hello", Final: true})
	for _, want := range []struct {
		text  string
		final bool
	}{{"hel", false}, {"hello", true}} {
		m := next()
		require.Equal(t, "live", m["id"])
		require.Equal(t, want.text, m["text"])
		require.Equal(t, want.final, m["final"])
		require.Equal(t, user, m["streamer"])
		require.Equal(t, "canonical-auto-en", m["track"].(map[string]any)["id"])
	}
}

func TestWHEPCaptionsFollowPublicationTransitions(t *testing.T) {
	mm, _ := getStaticTestMediaManager(t)
	const (
		user   = "did:plc:private-captioned"
		viewer = "did:plc:viewer"
	)
	now := time.Now()
	track := captions.Track{ID: "canonical-auto-en", Language: "en", Kind: captions.KindCaptions, Source: captions.SourceAuto, Origin: captions.OriginCanonical}
	mm.bus.Captions.Publish(user, track, captions.Cue{ID: "replay-private", Start: now.Add(-time.Second), End: now, Text: "private replay", Final: true})

	viewerPC, offer := viewerOffer(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	messages := make(chan map[string]any, 16)
	opened := make(chan struct{}, 1)
	viewerPC.OnDataChannel(func(dc *webrtc.DataChannel) {
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			var m map[string]any
			if err := json.Unmarshal(msg.Data, &m); err == nil {
				messages <- m
			}
		})
		dc.OnOpen(func() { opened <- struct{}{} })
	})
	answer, err := mm.WebRTCPlayback2(ctx, user, "test-rendition", offer, viewer)
	require.NoError(t, err)
	require.NoError(t, viewerPC.SetRemoteDescription(*answer))
	select {
	case <-opened:
	case <-time.After(20 * time.Second):
		t.Fatal("the captions data channel did not open")
	}

	requireNoCaptionMessage(t, messages)
	mm.bus.Captions.Publish(user, track, captions.Cue{ID: "future-private", Start: now, End: now.Add(time.Second), Text: "private future", Final: true})
	requireNoCaptionMessage(t, messages)

	setCaptionWindowPublished(mm, user, true)
	mm.bus.Captions.Publish(user, track, captions.Cue{ID: "public", Start: now.Add(time.Second), End: now.Add(2 * time.Second), Text: "public", Final: true})
	select {
	case msg := <-messages:
		require.Equal(t, "public", msg["id"])
		require.Equal(t, "public", msg["text"])
	case <-time.After(10 * time.Second):
		t.Fatal("published caption was not delivered")
	}

	setCaptionWindowPublished(mm, user, false)
	mm.bus.Captions.Publish(user, track, captions.Cue{ID: "private-again", Start: now.Add(2 * time.Second), End: now.Add(3 * time.Second), Text: "private again", Final: true})
	requireNoCaptionMessage(t, messages)
}

func TestWHEPCaptionPreviewExceptions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		owner    bool
		wideOpen bool
	}{
		{name: "owner", owner: true},
		{name: "WideOpen", wideOpen: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mm, _ := getStaticTestMediaManager(t)
			const user = "did:plc:caption-owner"
			viewer := "did:plc:viewer"
			if tc.owner {
				viewer = user
			}
			mm.cli.WideOpen = tc.wideOpen
			now := time.Now()
			track := captions.Track{ID: "canonical-auto-en", Language: "en", Kind: captions.KindCaptions, Source: captions.SourceAuto, Origin: captions.OriginCanonical}
			mm.bus.Captions.Publish(user, track, captions.Cue{ID: tc.name, Start: now.Add(-time.Second), End: now, Text: tc.name, Final: true})

			messages := openWHEPCaptionMessages(t, mm, user, viewer)
			select {
			case msg := <-messages:
				require.Equal(t, tc.name, msg["id"])
				require.Equal(t, tc.name, msg["text"])
			case <-time.After(10 * time.Second):
				t.Fatalf("%s viewer did not receive preview captions", tc.name)
			}
		})
	}
}

// Without a data channel in the offer the session is exactly what it was:
// nothing is added to the answer.
func TestWHEPWithoutDataChannelHasNoCaptions(t *testing.T) {
	mm, _ := getStaticTestMediaManager(t)
	viewer, offer := viewerOffer(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	answer, err := mm.WebRTCPlayback2(ctx, "did:plc:plain", "test-rendition", offer, "")
	require.NoError(t, err)
	require.NoError(t, viewer.SetRemoteDescription(*answer))
	require.False(t, offersDataChannel(answer), "the answer has no application section")
}
