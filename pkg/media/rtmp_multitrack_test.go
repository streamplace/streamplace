package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluenviron/gortmplib"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/livehls"
)

// SPS/PPS borrowed from gortmplib's own writer test fixtures.
var testMultitrackSPS = []byte{
	0x67, 0x64, 0x00, 0x0c, 0xac, 0x3b, 0x50, 0xb0,
	0x4b, 0x42, 0x00, 0x00, 0x03, 0x00, 0x02, 0x00,
	0x00, 0x03, 0x00, 0x3d, 0x08,
}

var testMultitrackPPS = []byte{0x68, 0xee, 0x3c, 0x80}

// TestRTMPMultitrackFlvDemuxInterop is the eRTMP interop spike: it proves that
// a multitrack stream re-muxed by gortmplib's Writer (the loopback relay used by
// HandleRTMPPlayback) is correctly demuxed by GStreamer's flvdemux into one pad
// per video track. This is the wire-format assumption the whole multitrack
// ingest path is built on; if it ever breaks (gortmplib or GStreamer upgrade),
// the appsrc-based fallback in RTMPIngest is the plan B.
func TestRTMPMultitrackFlvDemuxInterop(t *testing.T) {
	// Not wrapped in withNoGSTLeaks, matching the other ingest-level tests
	// (ingest_worker_test.go et al.): full network-driven pipelines like this
	// one outlive the leak tracer's async-teardown window and false-positive.
	run := func() {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer ln.Close()
		port := ln.Addr().(*net.TCPAddr).Port

		videoTrack0 := &format.H264{PayloadTyp: 96, SPS: testMultitrackSPS, PPS: testMultitrackPPS, PacketizationMode: 1}
		videoTrack1 := &format.H264{PayloadTyp: 96, SPS: testMultitrackSPS, PPS: testMultitrackPPS, PacketizationMode: 1}
		audioTrack := &format.MPEG4Audio{
			PayloadTyp: 96,
			Config: &mpeg4audio.AudioSpecificConfig{
				Type:         2,
				SampleRate:   44100,
				ChannelCount: 2,
			},
			SizeLength:       13,
			IndexLength:      3,
			IndexDeltaLength: 3,
		}

		done := make(chan struct{})
		serverErr := make(chan error, 1)
		go func() {
			conn, err := ln.Accept()
			if err != nil {
				serverErr <- fmt.Errorf("accept: %w", err)
				return
			}
			sc := &gortmplib.ServerConn{RW: conn}
			if err := sc.Initialize(); err != nil {
				serverErr <- fmt.Errorf("server conn init: %w", err)
				return
			}
			if err := sc.Accept(); err != nil {
				serverErr <- fmt.Errorf("server conn accept: %w", err)
				return
			}
			if sc.Publish {
				serverErr <- fmt.Errorf("expected playback client, got publish")
				return
			}
			w := &gortmplib.Writer{
				Conn:   sc,
				Tracks: []format.Format{videoTrack0, videoTrack1, audioTrack},
			}
			if err := w.Initialize(); err != nil {
				serverErr <- fmt.Errorf("writer init: %w", err)
				return
			}
			// One IDR per track is enough for flvdemux to create all pads, but
			// send a few frames so caps negotiation has data to work with.
			for i := range 5 {
				pts := time.Duration(i) * 33 * time.Millisecond
				if err := w.WriteH264(videoTrack0, pts, pts, [][]byte{{0x65, 0x88, 0x84, 0x21}}); err != nil {
					serverErr <- fmt.Errorf("write h264 track 0: %w", err)
					return
				}
				if err := w.WriteH264(videoTrack1, pts, pts, [][]byte{{0x65, 0x88, 0x84, 0x21}}); err != nil {
					serverErr <- fmt.Errorf("write h264 track 1: %w", err)
					return
				}
				if err := w.WriteMPEG4Audio(audioTrack, pts, []byte{0x11, 0x22, 0x33}); err != nil {
					serverErr <- fmt.Errorf("write aac: %w", err)
					return
				}
			}
			<-done
			serverErr <- nil
		}()

		pipeline, err := gst.NewPipelineFromString(
			fmt.Sprintf("rtmp2src location=rtmp://127.0.0.1:%d/live/test ! flvdemux name=demux", port),
		)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			_ = HandleBusMessages(ctx, pipeline)
		}()

		var padsMu sync.Mutex
		pads := map[string]bool{}
		var padsOnce sync.Once
		allPads := make(chan struct{})
		linkErr := make(chan error, 8)
		demux, err := pipeline.GetElementByName("demux")
		require.NoError(t, err)
		// Pre-create the expected sinks while the pipeline is stopped: the
		// closure must not capture the pipeline itself (a go-gst signal
		// registry reference cycle would leak the whole graph).
		fakesinks := map[string]*gst.Element{}
		for _, padName := range []string{"video", "video_1", "audio"} {
			sink, err := gst.NewElementWithProperties("fakesink", map[string]any{"sync": false})
			require.NoError(t, err)
			require.NoError(t, pipeline.Add(sink))
			fakesinks[padName] = sink
		}
		handle, err := demux.Connect("pad-added", func(self *gst.Element, pad *gst.Pad) {
			sink, ok := fakesinks[pad.GetName()]
			if !ok {
				linkErr <- fmt.Errorf("unexpected pad %s", pad.GetName())
				return
			}
			if ret := pad.Link(sink.GetStaticPad("sink")); ret != gst.PadLinkOK {
				linkErr <- fmt.Errorf("link pad %s: %s", pad.GetName(), ret)
				return
			}
			if !sink.SyncStateWithParent() {
				linkErr <- fmt.Errorf("failed to sync fakesink state for pad %s", pad.GetName())
				return
			}
			padsMu.Lock()
			pads[pad.GetName()] = true
			count := len(pads)
			padsMu.Unlock()
			if count >= 3 {
				padsOnce.Do(func() { close(allPads) })
			}
		})
		require.NoError(t, err)
		defer demux.HandlerDisconnect(handle)

		err = pipeline.SetState(gst.StatePlaying)
		require.NoError(t, err)
		defer func() {
			close(done)
			err := pipeline.SetState(gst.StateNull)
			require.NoError(t, err)
		}()

		select {
		case <-allPads:
		case lErr := <-linkErr:
			t.Fatalf("failed to link demux pad: %v", lErr)
		case sErr := <-serverErr:
			t.Fatalf("relay server failed before all pads appeared: %v", sErr)
		case <-time.After(15 * time.Second):
			padsMu.Lock()
			got := pads
			padsMu.Unlock()
			t.Fatalf("timed out waiting for multitrack pads; got pads: %v", got)
		}

		padsMu.Lock()
		got := pads
		padsMu.Unlock()
		// gortmplib writes video track 0 as a legacy tag (pad "video") and
		// tracks 1..N as VideoExMultitrack OneTrack messages (pads "video_<id>").
		require.True(t, got["video"], "expected legacy video pad, got %v", got)
		require.True(t, got["video_1"], "expected multitrack video_1 pad, got %v", got)
		require.True(t, got["audio"], "expected audio pad, got %v", got)
	}
	run()
}

// --- end-to-end multitrack ingest test ---

type capturedAU struct {
	pts, dts time.Duration
	nalus    [][]byte
}

// captureH264Stream renders numBuffers of videotestsrc through x264enc at the
// given size and returns the SPS/PPS plus AVCC access units (NALUs without
// length prefixes, as gortmplib's Writer wants). tune=zerolatency keeps
// PTS == DTS (no B-frames) so timestamps stay simple; key-int-max=30 at 30fps
// gives 1s GOPs, keyframe-aligned with any other capture started the same way.
func captureH264Stream(t *testing.T, width, height, numBuffers int) (sps, pps []byte, aus []capturedAU) {
	t.Helper()
	pipeline, err := gst.NewPipelineFromString(fmt.Sprintf(
		"videotestsrc num-buffers=%d ! video/x-raw,width=%d,height=%d,framerate=30/1 ! x264enc tune=zerolatency key-int-max=30 ! h264parse config-interval=1 ! video/x-h264,stream-format=avc ! appsink name=sink sync=false",
		numBuffers, width, height))
	require.NoError(t, err)
	sinkElem, err := pipeline.GetElementByName("sink")
	require.NoError(t, err)
	sink := app.SinkFromElement(sinkElem)
	require.NoError(t, pipeline.SetState(gst.StatePlaying))
	defer func() { _ = pipeline.SetState(gst.StateNull) }()

	var firstPTS *time.Time
	for {
		sample := sink.PullSample()
		if sample == nil {
			break
		}
		buf := sample.GetBuffer()
		require.NotNil(t, buf)
		nalus, err := avccSplitNALUs(buf.Bytes())
		require.NoError(t, err)
		pts := buf.PresentationTimestamp().AsTimestamp()
		require.NotNil(t, pts, "video buffer without PTS")
		if firstPTS == nil {
			firstPTS = pts
		}
		dtsDur := pts.Sub(*firstPTS)
		aus = append(aus, capturedAU{pts: dtsDur, dts: dtsDur, nalus: nalus})
	}
	require.NotEmpty(t, aus)

	// config-interval put SPS/PPS in-band ahead of every IDR; lift them from
	// the first AU that carries them (NAL types 7/8) before filtering.
	for _, au := range aus {
		for _, nal := range au.nalus {
			if len(nal) == 0 {
				continue
			}
			switch nal[0] & 0x1f {
			case 7:
				if sps == nil {
					sps = nal
				}
			case 8:
				if pps == nil {
					pps = nal
				}
			}
		}
		if sps != nil && pps != nil {
			break
		}
	}
	require.NotEmpty(t, sps, "no SPS found in captured stream")
	require.NotEmpty(t, pps, "no PPS found in captured stream")

	// Shape the stream like OBS's: AUs carry slice NALUs only — SPS/PPS ride
	// in the sequence header, not in-band, and no SEI/AU-delimiters. Header-y
	// NALUs in the first AU make h264parse emit extra header-only buffers
	// that break mp4mux timestamping.
	filtered := aus[:0]
	for _, au := range aus {
		keep := au.nalus[:0]
		for _, nal := range au.nalus {
			if len(nal) == 0 {
				continue
			}
			switch nal[0] & 0x1f {
			case 1, 5: // non-IDR and IDR slices
				keep = append(keep, nal)
			}
		}
		if len(keep) == 0 {
			continue
		}
		au.nalus = keep
		filtered = append(filtered, au)
	}
	aus = filtered
	require.NotEmpty(t, aus)
	return sps, pps, aus
}

// captureAACStream renders numBuffers of 1024-sample audiotestsrc through
// fdkaacenc and returns raw AAC frames with relative PTS.
func captureAACStream(t *testing.T, numBuffers int) (aus [][]byte, ptss []time.Duration) {
	t.Helper()
	pipeline, err := gst.NewPipelineFromString(fmt.Sprintf(
		"audiotestsrc num-buffers=%d samplesperbuffer=1024 ! audio/x-raw,rate=44100,channels=1 ! fdkaacenc ! appsink name=sink sync=false",
		numBuffers))
	require.NoError(t, err)
	sinkElem, err := pipeline.GetElementByName("sink")
	require.NoError(t, err)
	sink := app.SinkFromElement(sinkElem)
	require.NoError(t, pipeline.SetState(gst.StatePlaying))
	defer func() { _ = pipeline.SetState(gst.StateNull) }()

	var firstPTS *time.Time
	for {
		sample := sink.PullSample()
		if sample == nil {
			break
		}
		buf := sample.GetBuffer()
		require.NotNil(t, buf)
		pts := buf.PresentationTimestamp().AsTimestamp()
		require.NotNil(t, pts, "audio buffer without PTS")
		if firstPTS == nil {
			firstPTS = pts
		}
		aus = append(aus, buf.Bytes())
		ptss = append(ptss, pts.Sub(*firstPTS))
	}
	require.NotEmpty(t, aus)
	return aus, ptss
}

// avccSplitNALUs splits a length-prefixed AVCC access unit into bare NALUs.
func avccSplitNALUs(bs []byte) ([][]byte, error) {
	var out [][]byte
	for len(bs) >= 4 {
		n := binary.BigEndian.Uint32(bs[:4])
		bs = bs[4:]
		if int(n) > len(bs) {
			return nil, fmt.Errorf("malformed AVCC: NAL of %d bytes with %d remaining", n, len(bs))
		}
		out = append(out, bs[:n])
		bs = bs[n:]
	}
	if len(bs) != 0 {
		return nil, fmt.Errorf("malformed AVCC: %d trailing bytes", len(bs))
	}
	return out, nil
}

// runRTMPIngestScenario pushes a synthetic N-video-track + AAC eRTMP stream
// through the exact chain the RTMP server uses — gortmplib Reader
// (HandleRTMPPublisher) → RTMPSession events → relay gortmplib Writer
// (HandleRTMPPlayback) → rtmp2src → flvdemux → signing element — and returns
// the signed segments it emitted. With staticPipeline=true the ingest side
// uses the pre-multitrack static pipeline shape (demux.video/audio links in
// the pipeline string, single video track only); with false it uses the
// production dynamic pad-added linking (buildRTMPTrackChains).
func runRTMPIngestScenario(t *testing.T, videoTracks []capturedH264, staticPipeline bool) [][]byte {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	audioTrack := &format.MPEG4Audio{
		PayloadTyp: 96,
		Config: &mpeg4audio.AudioSpecificConfig{
			Type:         2,
			SampleRate:   44100,
			ChannelCount: 1,
		},
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
	}
	aacAUs, aacPTS := captureAACStream(t, 130)

	session := &RTMPSession{
		EventChan:   make(chan any, 1024),
		VideoTracks: map[uint8]*format.H264{},
		AudioTrack:  audioTrack,
	}
	relayTracks := []format.Format{}
	for i, vt := range videoTracks {
		session.VideoTracks[uint8(i)] = vt.track
		relayTracks = append(relayTracks, vt.track)
	}
	relayTracks = append(relayTracks, audioTrack)

	// --- publish server side: mirrors HandleRTMPPublisher minus auth ---
	pubLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer pubLn.Close()
	readerErr := make(chan error, 1)
	go func() {
		conn, err := pubLn.Accept()
		if err != nil {
			readerErr <- fmt.Errorf("accept: %w", err)
			return
		}
		sc := &gortmplib.ServerConn{RW: conn}
		if err := sc.Initialize(); err != nil {
			readerErr <- fmt.Errorf("server conn init: %w", err)
			return
		}
		if err := sc.Accept(); err != nil {
			readerErr <- fmt.Errorf("server conn accept: %w", err)
			return
		}
		if !sc.Publish {
			readerErr <- fmt.Errorf("expected publish client")
			return
		}
		r := &gortmplib.Reader{Conn: sc}
		if err := r.Initialize(); err != nil {
			readerErr <- fmt.Errorf("reader init: %w", err)
			return
		}
		vid := 0
		for _, track := range r.Tracks() {
			switch track := track.(type) {
			case *format.H264:
				trackID := uint8(vid)
				vid++
				want := session.VideoTracks[trackID]
				if want == nil || !bytes.Equal(want.SPS, track.SPS) {
					readerErr <- fmt.Errorf("video track %d SPS mismatch (wire order shuffled?)", trackID)
					return
				}
				r.OnDataH264(track, func(pts time.Duration, dts time.Duration, au [][]byte) {
					session.EventChan <- &RTMPH264Data{TrackID: trackID, AU: au, PTS: pts, DTS: dts}
				})
			case *format.MPEG4Audio:
				r.OnDataMPEG4Audio(track, func(pts time.Duration, au []byte) {
					session.EventChan <- &RTMPAACData{AU: au, PTS: pts}
				})
			default:
				readerErr <- fmt.Errorf("unexpected track type: %T", track)
				return
			}
		}
		for {
			if err := r.Read(); err != nil {
				// publisher went away: session is over
				close(session.EventChan)
				readerErr <- nil
				return
			}
		}
	}()

	// --- publisher client: interleave AUs of all tracks by timestamp ---
	pubErr := make(chan error, 1)
	go func() {
		u, err := url.Parse(fmt.Sprintf("rtmp://%s/live/test", pubLn.Addr()))
		if err != nil {
			pubErr <- err
			return
		}
		c := &gortmplib.Client{URL: u, Publish: true}
		if err := c.Initialize(ctx); err != nil {
			pubErr <- fmt.Errorf("client init: %w", err)
			return
		}
		w := &gortmplib.Writer{
			Conn:   c,
			Tracks: relayTracks,
		}
		if err := w.Initialize(); err != nil {
			pubErr <- fmt.Errorf("client writer init: %w", err)
			return
		}
		type ev struct {
			ts time.Duration
			fn func() error
		}
		var evs []ev
		for i, vt := range videoTracks {
			track := session.VideoTracks[uint8(i)]
			for _, au := range vt.aus {
				au := au
				evs = append(evs, ev{au.dts, func() error { return w.WriteH264(track, au.pts, au.dts, au.nalus) }})
			}
		}
		for i, au := range aacAUs {
			au, pts := au, aacPTS[i]
			evs = append(evs, ev{pts, func() error { return w.WriteMPEG4Audio(audioTrack, pts, au) }})
		}
		sort.Slice(evs, func(i, j int) bool { return evs[i].ts < evs[j].ts })
		for _, e := range evs {
			if err := e.fn(); err != nil {
				pubErr <- fmt.Errorf("publish write: %w", err)
				return
			}
			time.Sleep(time.Millisecond)
		}
		// closing the connection is what EOFs the reader → session close →
		// relay socket close → EOS through the ingest pipeline
		c.Close()
		pubErr <- nil
	}()

	// --- relay: mirrors HandleRTMPPlayback ---
	relayLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer relayLn.Close()
	relayErr := make(chan error, 1)
	go func() {
		conn, err := relayLn.Accept()
		if err != nil {
			relayErr <- fmt.Errorf("accept: %w", err)
			return
		}
		defer conn.Close() // closing the socket is what EOSes the ingest side
		sc := &gortmplib.ServerConn{RW: conn}
		if err := sc.Initialize(); err != nil {
			relayErr <- fmt.Errorf("server conn init: %w", err)
			return
		}
		if err := sc.Accept(); err != nil {
			relayErr <- fmt.Errorf("server conn accept: %w", err)
			return
		}
		if sc.Publish {
			relayErr <- fmt.Errorf("expected playback client")
			return
		}
		w := &gortmplib.Writer{
			Conn:   sc,
			Tracks: relayTracks,
		}
		if err := w.Initialize(); err != nil {
			relayErr <- fmt.Errorf("relay writer init: %w", err)
			return
		}
		for event := range session.EventChan {
			var werr error
			switch event := event.(type) {
			case *RTMPH264Data:
				werr = w.WriteH264(session.VideoTracks[event.TrackID], event.PTS, event.DTS, event.AU)
			case *RTMPAACData:
				werr = w.WriteMPEG4Audio(session.AudioTrack, event.PTS, event.AU)
			default:
				werr = fmt.Errorf("unsupported event type: %T", event)
			}
			if werr != nil {
				relayErr <- werr
				return
			}
		}
		relayErr <- nil
	}()

	// --- ingest: mirrors RTMPIngest with a bare signer ---
	ms := newBareSegmentSigner(t)
	var segsMu sync.Mutex
	var segs [][]byte
	onSegment := func(ctx context.Context, segment []byte) error {
		segsMu.Lock()
		segs = append(segs, segment)
		segsMu.Unlock()
		return nil
	}
	signer, signerDone, err := muxlSignSegmentElem(ctx, &config.CLI{}, ms.SignSegmentStream, onSegment, len(videoTracks))
	require.NoError(t, err)

	linkErr := make(chan error, 8)
	var pipeline *gst.Pipeline
	if staticPipeline {
		// pre-multitrack production shape: pad links baked into the pipeline string
		require.Equal(t, 1, len(videoTracks), "static pipeline shape is single-track only")
		pipeline, err = gst.NewPipelineFromString(strings.Join([]string{
			fmt.Sprintf("rtmp2src location=rtmp://%s/live/test ! flvdemux name=demux", relayLn.Addr()),
			"demux.audio ! queue ! aacparse name=audioenc",
			"demux.video ! queue ! h264parse name=parse",
		}, "\n"))
		require.NoError(t, err)
		require.NoError(t, pipeline.Add(signer))
		parseEle, err := pipeline.GetElementByName("parse")
		require.NoError(t, err)
		require.NoError(t, parseEle.Link(signer))
		audioenc, err := pipeline.GetElementByName("audioenc")
		require.NoError(t, err)
		require.NoError(t, audioenc.Link(signer))
	} else {
		pipeline, err = gst.NewPipelineFromString(
			fmt.Sprintf("rtmp2src location=rtmp://%s/live/test ! flvdemux name=demux", relayLn.Addr()),
		)
		require.NoError(t, err)
		require.NoError(t, pipeline.Add(signer))
		demux, err := pipeline.GetElementByName("demux")
		require.NoError(t, err)
		queues, err := buildRTMPTrackChains(pipeline, signer, len(videoTracks))
		require.NoError(t, err)
		handle, err := demux.Connect("pad-added", func(self *gst.Element, pad *gst.Pad) {
			chain, ok := queues[pad.GetName()]
			if !ok {
				linkErr <- fmt.Errorf("unexpected flvdemux pad %s", pad.GetName())
				return
			}
			if ret := pad.Link(chain.queue.GetStaticPad("sink")); ret != gst.PadLinkOK {
				linkErr <- fmt.Errorf("link %s: %s", pad.GetName(), ret)
				return
			}
			if !chain.queue.SyncStateWithParent() || !chain.parse.SyncStateWithParent() {
				linkErr <- fmt.Errorf("sync branch state for pad %s", pad.GetName())
			}
		})
		require.NoError(t, err)
		defer demux.HandlerDisconnect(handle)
	}

	busDone := make(chan error, 1)
	go func() { busDone <- HandleBusMessages(ctx, pipeline) }()
	require.NoError(t, pipeline.SetState(gst.StatePlaying))
	defer func() { _ = pipeline.SetState(gst.StateNull) }()

	// Publisher finishes → reader EOFs and closes the session → relay
	// drains and closes its socket → EOS through the pipeline. Then cancel
	// to flush the signer and wait for it to drain.
eosWait:
	for {
		select {
		case err := <-busDone:
			require.NoError(t, err, "ingest pipeline bus error")
			break eosWait
		case err := <-linkErr:
			t.Fatalf("demux pad link failed: %v", err)
		case err := <-readerErr:
			require.NoError(t, err, "reader side failed")
		case err := <-pubErr:
			require.NoError(t, err, "publisher failed")
		case err := <-relayErr:
			require.NoError(t, err, "relay failed")
		case <-time.After(45 * time.Second):
			t.Fatalf("timed out waiting for ingest EOS")
		}
	}
	cancel()
	select {
	case <-signerDone:
	case <-time.After(15 * time.Second):
		t.Fatalf("timed out waiting for signer drain")
	}

	segsMu.Lock()
	defer segsMu.Unlock()
	return segs
}

// capturedH264 is one captured video track: the gortmplib track plus its AUs.
type capturedH264 struct {
	track *format.H264
	aus   []capturedAU
}

// captureVideoTrack captures one video track and shifts its timeline one
// frame interval forward, so the first real frame (33ms) doesn't share a
// timestamp with the SPS/PPS AU gortmplib's Reader synthesizes from the
// sequence header (pts 0) — back-to-back duplicate PTS wedges h264parse's
// timestamp inference ("Buffer has no PTS" at mp4mux).
func captureVideoTrack(t *testing.T, width, height, numBuffers int) capturedH264 {
	t.Helper()
	sps, pps, aus := captureH264Stream(t, width, height, numBuffers)
	const frameDur = 33333333 * time.Nanosecond
	for i := range aus {
		aus[i].pts += frameDur
		aus[i].dts += frameDur
	}
	return capturedH264{
		track: &format.H264{PayloadTyp: 96, SPS: sps, PPS: pps, PacketizationMode: 1},
		aus:   aus,
	}
}

// TestRTMPMultitrackIngestEndToEnd pushes a synthetic 2-video-track + AAC
// eRTMP stream through the exact chain the RTMP server uses — gortmplib
// Reader (HandleRTMPPublisher) → RTMPSession events → relay gortmplib Writer
// (HandleRTMPPlayback) → rtmp2src → flvdemux → dynamic pad linking
// (RTMPIngest) → 2-video-track signing element — and asserts the signed
// segments carry both video tracks through the real ValidateMP4Media path.
func TestRTMPMultitrackIngestEndToEnd(t *testing.T) {
	// Not wrapped in withNoGSTLeaks, matching the other ingest-level tests
	// (ingest_worker_test.go et al.): full network-driven pipelines like this
	// one outlive the leak tracer's async-teardown window and false-positive.
	run := func() {
		ctx := context.Background()

		tracks := []capturedH264{
			captureVideoTrack(t, 640, 360, 90),
			captureVideoTrack(t, 320, 180, 90),
		}
		segs := runRTMPIngestScenario(t, tracks, false)

		require.NotEmpty(t, segs, "expected at least one signed segment")

		// The first fragment is a startup runt (the SPS/PPS AU gortmplib
		// synthesizes lands ahead of the first real GOP), so find the first
		// segment that actually carries both video tracks.
		var res *ValidationResult
		var fullSeg []byte
		for _, seg := range segs {
			r, err := ValidateMP4Media(ctx, seg)
			require.NoError(t, err)
			require.NotNil(t, r.MediaData)
			if len(r.MediaData.Video) == 2 {
				res, fullSeg = r, seg
				break
			}
		}
		require.NotNil(t, res, "no segment carried both video tracks (got %d segments)", len(segs))
		require.Equal(t, 640, res.MediaData.Video[0].Width, "video_0 should be the first wire track (640x360)")
		require.Equal(t, 360, res.MediaData.Video[0].Height)
		require.Equal(t, 320, res.MediaData.Video[1].Width, "video_1 should be the second wire track (320x180)")
		require.Equal(t, 180, res.MediaData.Video[1].Height)
		require.NotEmpty(t, res.MediaData.Audio, "segment should carry the audio track")

		// Fold the segment into a live-HLS window: the master playlist must
		// expose one variant per source video track.
		mm := &MediaManager{liveWindows: map[string]*livehls.Writer{}}
		mm.feedLiveWindow(ctx, "did:test:streamer", fullSeg, true)
		w := mm.GetLiveWindow("did:test:streamer")
		require.NotNil(t, w, "live window created on feed")
		master := w.MasterPlaylist(func(tid string) string { return tid + ".m3u8" })
		require.Equal(t, 2, strings.Count(master, "#EXT-X-STREAM-INF"), "one HLS variant per video track:\n%s", master)
		require.Contains(t, master, "RESOLUTION=640x360")
		require.Contains(t, master, "RESOLUTION=320x180")
	}
	run()
}

// TestRTMPSingleTrackStaticIngestEndToEnd is the pre-multitrack production
// shape as a control: one H.264 track + AAC through the old static pipeline,
// asserting plain RTMP ingest still produces valid signed segments.
func TestRTMPSingleTrackStaticIngestEndToEnd(t *testing.T) {
	withNoGSTLeaks(t, func() {
		ctx := context.Background()

		segs := runRTMPIngestScenario(t, []capturedH264{captureVideoTrack(t, 640, 360, 90)}, true)

		require.NotEmpty(t, segs, "expected at least one signed segment")
		res, err := ValidateMP4Media(ctx, segs[0])
		require.NoError(t, err)
		require.NotNil(t, res.MediaData)
		require.Len(t, res.MediaData.Video, 1, "segment should carry one video track")
		require.Equal(t, 640, res.MediaData.Video[0].Width)
		require.Equal(t, 360, res.MediaData.Video[0].Height)
		require.NotEmpty(t, res.MediaData.Audio)
	})
}

// TestRTMPSingleTrackDynamicIngestEndToEnd is the new dynamic pad-linking
// shape with a single video track — the plain-RTMP regression guard for the
// multitrack ingest rewrite.
func TestRTMPSingleTrackDynamicIngestEndToEnd(t *testing.T) {
	// Not wrapped in withNoGSTLeaks, matching the other ingest-level tests
	// (ingest_worker_test.go et al.): full network-driven pipelines like this
	// one outlive the leak tracer's async-teardown window and false-positive.
	run := func() {
		ctx := context.Background()

		segs := runRTMPIngestScenario(t, []capturedH264{captureVideoTrack(t, 640, 360, 90)}, false)

		require.NotEmpty(t, segs, "expected at least one signed segment")
		res, err := ValidateMP4Media(ctx, segs[0])
		require.NoError(t, err)
		require.NotNil(t, res.MediaData)
		require.Len(t, res.MediaData.Video, 1, "segment should carry one video track")
		require.Equal(t, 640, res.MediaData.Video[0].Width)
		require.Equal(t, 360, res.MediaData.Video[0].Height)
		require.NotEmpty(t, res.MediaData.Audio)
	}
	run()
}
