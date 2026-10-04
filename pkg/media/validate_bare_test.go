package media

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/livehls"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/muxl"
)

// newBareSegmentSigner is an ephemeral signer with the test streamer name,
// enough to drive SignSegmentStream in tests.
func newBareSegmentSigner(t testing.TB) *MediaSignerLocal {
	t.Helper()
	ms, err := NewEphemeralMediaSigner("test-streamer")
	require.NoError(t, err)
	return ms
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

	mm := &MediaManager{liveWindows: map[string]*liveWindowState{}}

	// Pre-live (unpublished) segments are folded in for the streamer's own
	// preview, and the window remembers that its latest segment is not
	// public — the getLive* handlers keep it to holders of a playback token.
	t0 := time.Now()
	t.Run("publication preserves the newest observed timestamp", func(t *testing.T) {
		const did = "did:test:out-of-order"
		mm.feedLiveWindow(ctx, did, m4s, t0.Add(3*time.Second), false)
		mm.feedLiveWindow(ctx, did, m4s, t0.Add(time.Second), true)
		public := mm.GetLiveWindow(did)
		require.NotNil(t, public)
		mm.feedLiveWindow(ctx, did, m4s, t0.Add(2*time.Second), false)
		require.True(t, mm.LiveWindowPublished(did), "an older preview must not undo publication")
		require.Same(t, public, mm.GetLiveWindow(did))
	})
	mm.feedLiveWindow(ctx, "did:test:streamer", m4s, t0, false)
	require.NotNil(t, mm.GetLiveWindow("did:test:streamer"), "pre-live segments make a window")
	require.False(t, mm.LiveWindowPublished("did:test:streamer"), "…but it is not public")

	preLive := mm.GetLiveWindow("did:test:streamer")
	t.Run("malformed publication preserves preview", func(t *testing.T) {
		var logs logCapture
		previousLogger := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
		t.Cleanup(func() { slog.SetDefault(previousLogger) })
		previousVerbosity := flag.Lookup("v").Value.String()
		require.NoError(t, flag.Set("v", "3"))
		t.Cleanup(func() { require.NoError(t, flag.Set("v", previousVerbosity)) })

		mm.feedLiveWindow(ctx, "did:test:streamer", m4s[:len(m4s)-1], t0.Add(time.Second), true)
		require.Equal(t, 1, strings.Count(logs.String(), "live-hls: window feed failed"))
		require.Same(t, preLive, mm.GetLiveWindow("did:test:streamer"), "failed decode must preserve the valid preview")
		require.False(t, mm.LiveWindowPublished("did:test:streamer"), "failed decode must not publish preview media")
	})
	mm.feedLiveWindow(ctx, "did:test:streamer", m4s, t0.Add(2*time.Second), true)
	require.True(t, mm.LiveWindowPublished("did:test:streamer"))

	w := mm.GetLiveWindow("did:test:streamer")
	require.NotNil(t, w, "window created on feed")
	require.NotSame(t, preLive, w, "going public starts the window over: the preview segments are not served to the public")
	tids := w.TrackIDs()
	require.NotEmpty(t, tids, "window has tracks")
	for _, tid := range tids {
		require.Len(t, w.Track(tid).Segments, len(preLive.Track(tid).Segments), "track %s: only the published segment, none of the pre-live ones", tid)
	}
	// Once public, further segments extend the same window.
	mm.feedLiveWindow(ctx, "did:test:streamer", m4s, t0.Add(4*time.Second), true)
	require.Same(t, w, mm.GetLiveWindow("did:test:streamer"))
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

	// Segments are fed concurrently, so a pre-live segment can finish
	// validating after the stream went public. It is older than the window,
	// so it is dropped: the public window neither restarts nor turns back
	// into a preview under its viewers.
	segs := len(w.Track(tids[0]).Segments)
	mm.feedLiveWindow(ctx, "did:test:streamer", m4s, t0.Add(time.Second), false)
	require.Same(t, w, mm.GetLiveWindow("did:test:streamer"), "a late pre-live segment does not touch the public window")
	require.True(t, mm.LiveWindowPublished("did:test:streamer"), "…and does not make it a preview")
	require.Len(t, w.Track(tids[0]).Segments, segs, "…nor does it add to it")

	// A pre-live segment newer than the window is the stream going back to
	// preview (the streamer ended it but is still sending), and does flip it.
	mm.feedLiveWindow(ctx, "did:test:streamer", m4s, t0.Add(6*time.Second), false)
	require.False(t, mm.LiveWindowPublished("did:test:streamer"), "a newer pre-live segment takes the stream back to preview")
}

func waitLiveWindowWork(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("live window work did not reach the expected synchronization point")
	}
}

func waitLiveWindowStack(t *testing.T, caller, callee string) {
	t.Helper()
	require.Eventually(t, func() bool {
		stack := make([]byte, 64<<10)
		n := runtime.Stack(stack, true)
		for _, goroutine := range strings.Split(string(stack[:n]), "\n\n") {
			if strings.Contains(goroutine, caller) && strings.Contains(goroutine, callee) {
				return true
			}
		}
		return false
	}, 5*time.Second, time.Millisecond)
}

func TestFeedLiveWindowDoesNotBlockOtherStreams(t *testing.T) {
	ctx := t.Context()
	ms := newBareSegmentSigner(t)
	segments := allSignedBareSegments(t, ctx, ms, getFixture("h264-opus-frag.mp4"))
	require.NotEmpty(t, segments)
	mm := &MediaManager{liveWindows: map[string]*liveWindowState{}}
	mm.feedLiveWindow(ctx, "did:test:busy", segments[0], time.Now(), true)
	window := mm.GetLiveWindow("did:test:busy")
	require.NotNil(t, window)

	entered := make(chan struct{})
	release := make(chan struct{})
	playlistDone := make(chan struct{})
	go func() {
		defer close(playlistDone)
		first := true
		window.MasterPlaylist(func(tid string) string {
			if first {
				first = false
				close(entered)
				<-release
			}
			return tid + ".m3u8"
		})
	}()
	unblock := sync.OnceFunc(func() { close(release) })
	t.Cleanup(func() {
		unblock()
		waitLiveWindowWork(t, playlistDone)
	})
	waitLiveWindowWork(t, entered)
	feedDone := make(chan struct{})
	go func() {
		defer close(feedDone)
		mm.feedLiveWindow(ctx, "did:test:busy", segments[0], time.Now(), true)
	}()
	t.Cleanup(func() {
		unblock()
		waitLiveWindowWork(t, feedDone)
	})
	otherDone := make(chan struct{})

	// Wait for the feed to reach the writer held by the playlist callback,
	// rather than depending on how long decoding or goroutine scheduling takes.
	waitLiveWindowStack(t, "(*MediaManager).feedLiveWindow", "(*Writer).Observe")
	go func() {
		defer close(otherDone)
		mm.feedLiveWindow(ctx, "did:test:other", segments[0], time.Now(), true)
		mm.GetLiveWindow("did:test:other")
		mm.LiveWindowPublished("did:test:other")
	}()
	t.Cleanup(func() {
		unblock()
		waitLiveWindowWork(t, otherDone)
	})
	select {
	case <-otherDone:
	case <-time.After(time.Second):
		t.Error("unrelated stream feed and reads blocked by busy stream")
	}
}

func TestFeedLiveWindowRecreatesPrunedState(t *testing.T) {
	ctx := t.Context()
	ms := newBareSegmentSigner(t)
	segments := allSignedBareSegments(t, ctx, ms, getFixture("h264-opus-frag.mp4"))
	require.NotEmpty(t, segments)
	events, err := unwrapMuxlEvents(ctx, segments[0])
	require.NoError(t, err)
	window := livehls.NewWriter()
	for _, ev := range events {
		require.NoError(t, window.Observe(ev))
	}
	const did = "did:test:pruned"
	mm := &MediaManager{liveWindows: map[string]*liveWindowState{
		did: {w: window, published: true},
	}}
	entered := make(chan struct{})
	release := make(chan struct{})
	playlistDone := make(chan struct{})
	go func() {
		defer close(playlistDone)
		first := true
		window.MasterPlaylist(func(tid string) string {
			if first {
				first = false
				// The playlist owns the writer lock; expire its segments before
				// the queued read can check whether the window is empty.
				livehls.WithRetention(time.Nanosecond)(window)
				close(entered)
				<-release
			}
			return tid + ".m3u8"
		})
	}()
	unblock := sync.OnceFunc(func() { close(release) })
	t.Cleanup(func() {
		unblock()
		waitLiveWindowWork(t, playlistDone)
	})
	waitLiveWindowWork(t, entered)
	readDone := make(chan struct{})
	var readWindow *livehls.Writer
	go func() {
		defer close(readDone)
		readWindow = mm.GetLiveWindow(did)
	}()
	t.Cleanup(func() {
		unblock()
		waitLiveWindowWork(t, readDone)
	})
	waitLiveWindowStack(t, "(*MediaManager).GetLiveWindow", "(*Writer).Empty")
	feedDone := make(chan struct{})
	go func() {
		defer close(feedDone)
		mm.feedLiveWindow(ctx, did, segments[0], time.Now(), true)
	}()
	t.Cleanup(func() {
		unblock()
		waitLiveWindowWork(t, feedDone)
	})
	waitLiveWindowStack(t, "(*MediaManager).feedLiveWindow", "(*MediaManager).lockLiveWindow")
	unblock()
	waitLiveWindowWork(t, playlistDone)
	waitLiveWindowWork(t, feedDone)
	waitLiveWindowWork(t, readDone)
	require.Nil(t, readWindow)
	recreated := mm.GetLiveWindow(did)
	require.NotNil(t, recreated, "queued feed must populate the current state, not the pruned one")
	require.NotSame(t, window, recreated)
	require.True(t, mm.LiveWindowPublished(did))
	require.Contains(t, recreated.MasterPlaylist(func(tid string) string { return tid + ".m3u8" }), "opus")
}

func TestFeedLiveWindowSurvivesConcurrentReads(t *testing.T) {
	ms := newBareSegmentSigner(t)
	segments := allSignedBareSegments(t, t.Context(), ms, getFixture("h264-opus-frag.mp4"))
	require.NotEmpty(t, segments)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	mm := &MediaManager{liveWindows: map[string]*liveWindowState{}}
	for i := range 10 {
		did := fmt.Sprintf("did:test:window-%d", i)
		done := make(chan struct{})
		go func() {
			defer close(done)
			mm.feedLiveWindow(ctx, did, segments[0], time.Now(), true)
		}()
	read:
		for {
			select {
			case <-done:
				break read
			case <-ctx.Done():
				t.Fatal("live window feed did not finish")
			default:
				mm.GetLiveWindow(did)
				runtime.Gosched()
			}
		}
		window := mm.GetLiveWindow(did)
		require.NotNil(t, window, "playback reads must not discard an in-progress feed")
		require.True(t, mm.LiveWindowPublished(did))
		require.Contains(t, window.MasterPlaylist(func(tid string) string { return tid + ".m3u8" }), "opus")
	}
}

func BenchmarkFeedInitialLiveWindow(b *testing.B) {
	ctx := context.Background()
	ms := newBareSegmentSigner(b)
	segments := allSignedBareSegments(b, ctx, ms, getFixture("h264-opus-frag.mp4"))
	require.NotEmpty(b, segments)
	b.ReportAllocs()
	b.SetBytes(int64(len(segments[0])))
	b.ResetTimer()
	for b.Loop() {
		mm := &MediaManager{liveWindows: map[string]*liveWindowState{}}
		mm.feedLiveWindow(ctx, "did:test:streamer", segments[0], time.Now(), true)
	}
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

// The flat presentation produced for media validation must survive until direct
// distribution; rebuilding it would repeat a whole muxl operation per GoP.
func TestValidateSourcePreservesPresentation(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)
	ms.PrebuiltManifest = bytes.Replace(ms.PrebuiltManifest, []byte("did:example"), []byte("did:key:test"), 1)
	segs := allSignedBareSegments(t, ctx, ms, getFixture("h264-opus-frag.mp4"))
	require.NotEmpty(t, segs)
	mm := NewOffline(&config.CLI{WideOpen: true})
	vs, playable, err := mm.validateSource(ctx, segs[0], true)
	require.NoError(t, err)
	require.NotNil(t, vs)
	require.NotEmpty(t, playable)
	var expected bytes.Buffer
	require.NoError(t, muxl.RunMuxlWrap(ctx, bytes.NewReader(segs[0]), "flat", &expected))
	require.Equal(t, expected.Bytes(), playable)

	// Observe the actual subscriber queue without starting a forwarding goroutine.
	sub := &segmentSubscriber{queue: make(chan *NewSegmentNotification, 1)}
	mm.newSegmentSubs = []*segmentSubscriber{sub}
	require.NoError(t, mm.distributeSegment(ctx, vs, segs[0], playable))
	not := <-sub.queue
	require.Equal(t, playable, not.Data)
	require.True(t, &playable[0] == &not.Data[0], "distribution reuses the validated presentation buffer")
	require.Equal(t, segs[0], not.Muxl, "canonical signed bytes are unchanged")
}

// Audio completion adds a signed track after source validation. Its presentation
// must describe those completed bytes rather than reuse the source-only MP4.
func TestDistributeCompletedSegmentPresentation(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)
	ms.StreamerName = "did:key:test"
	ms.PrebuiltManifest = bytes.Replace(ms.PrebuiltManifest, []byte("did:example:rtmp-shadow"), []byte(ms.Streamer()), 1)
	segs := allSignedBareSegments(t, ctx, ms, getFixture("h264-opus-frag.mp4"))
	require.GreaterOrEqual(t, len(segs), 2)
	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)
	mm := NewOffline(&config.CLI{WideOpen: true, BroadcasterHost: "test.example.com"})
	mm.transcoders = map[string]*streamTranscoder{}
	sub := &segmentSubscriber{queue: make(chan *NewSegmentNotification, len(segs))}
	mm.newSegmentSubs = []*segmentSubscriber{sub}
	for _, seg := range segs {
		vs, _, err := mm.validateSource(ctx, seg, true)
		require.NoError(t, err)
		require.NoError(t, mm.feedStreamTranscoder(ctx, vs, seg, "aac", ms.Cert, keyPEM, nil))
	}
	require.NoError(t, mm.transcoders[ms.Streamer()].Close())
	require.NotEmpty(t, sub.queue, "the continuous transcoder distributes its completed segments")
	for len(sub.queue) > 0 {
		not := <-sub.queue
		var expected bytes.Buffer
		require.NoError(t, muxl.RunMuxlWrap(ctx, bytes.NewReader(not.Muxl), "flat", &expected))
		require.Equal(t, expected.Bytes(), not.Data)
		codecs := audioCodecsOf(t, ctx, not.Muxl)
		var aac, opus bool
		for _, codec := range codecs {
			aac = aac || isAACCodec(codec)
			opus = opus || isOpusCodec(codec)
		}
		require.True(t, aac)
		require.True(t, opus)
	}
}

// BenchmarkValidateAndPresent measures signed-segment validation and preparation
// of the flat MP4 delivered to local media consumers.
func BenchmarkValidateAndPresent(b *testing.B) {
	ctx := context.Background()
	ms := newBareSegmentSigner(b)
	segs := allSignedBareSegments(b, ctx, ms, getFixture("h264-opus-frag.mp4"))
	require.NotEmpty(b, segs)
	b.ReportAllocs()
	b.SetBytes(int64(len(segs[0])))
	for b.Loop() {
		res, err := ValidateMP4Media(ctx, segs[0])
		if err != nil {
			b.Fatal(err)
		}
		if len(res.Playable) == 0 {
			b.Fatal("missing presentation")
		}
	}
}
