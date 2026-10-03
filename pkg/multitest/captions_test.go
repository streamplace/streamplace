package multitest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/mr-tron/base58"
	"github.com/slok/go-http-metrics/metrics"
	"github.com/slok/go-http-metrics/middleware"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/api"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/spkey"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/replication/websocketrep"
	"stream.place/streamplace/pkg/spxrpc"
	"stream.place/streamplace/pkg/stt"
)

const captionTimeout = 30 * time.Second

// These nodes run the real validator, signer, syndication client/server and
// public caption handlers in process. Unlike the binary-based legacy harness,
// this uses MediaManager.STT's existing injection point: no production seam or
// bundled model, CPU scheduling, remote fixture, PDS, or fixed port is needed.
type captionNode struct {
	cli      *config.CLI
	mod      model.Model
	bus      *bus.Bus
	mm       *media.MediaManager
	server   *httptest.Server
	segments <-chan *media.NewSegmentNotification
	engine   *captionEngine
}

type captionEngine struct {
	leases atomic.Int32
	passes atomic.Int32
}

func (e *captionEngine) Lease(context.Context) (stt.Lease, error) {
	e.leases.Add(1)
	return e, nil
}
func (*captionEngine) Close() error        { return nil }
func (e *captionEngine) Model() stt.Model  { return e }
func (*captionEngine) Release()            {}
func (*captionEngine) Info() stt.ModelInfo { return stt.ModelInfo{Name: "multitest-deterministic"} }
func (e *captionEngine) Transcribe(ctx context.Context, _ []float32, _ stt.Options) (*stt.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	e.passes.Add(1)
	return &stt.Result{Language: "en", Words: []stt.Word{{Text: "Deterministic sidecar.", Start: 250 * time.Millisecond, End: 750 * time.Millisecond, Prob: 1}}}, nil
}

func newCaptionNode(t *testing.T, ctx context.Context, name string) *captionNode {
	t.Helper()
	cli := &config.CLI{WideOpen: true, Captions: true, CaptionsMasterDelay: time.Second, DataDir: t.TempDir(), BroadcasterHost: name + ".example", ServerHost: name + ".example", Syndicate: []string{"*"}}
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	ldb, err := localdb.MakeDB(":memory:")
	require.NoError(t, err)
	b := bus.NewBus()
	atsync := &atproto.ATProtoSynchronizer{CLI: cli, Model: mod, Bus: b}
	mm, err := media.MakeMediaManager(ctx, cli, nil, mod, b, atsync, ldb)
	require.NoError(t, err)
	engine := &captionEngine{}
	mm.STT = engine
	op := oatproxy.New(&oatproxy.Config{Host: cli.BroadcasterHost, Scope: atproto.OAuthString, Public: true})
	xrpc, err := spxrpc.NewServer(ctx, cli, mod, nil, op, middleware.New(middleware.Config{Recorder: metrics.Dummy}), atsync, b, ldb, mm, nil, nil, nil, nil)
	require.NoError(t, err)
	a := &api.StreamplaceAPI{CLI: cli, Model: mod, LocalDB: ldb, MediaManager: mm, Bus: b, ATSync: atsync}
	router := httprouter.New()
	router.GET("/api/websocket/:repoDID", a.HandleWebsocket(ctx))
	router.Handler("GET", "/xrpc/*path", xrpc)
	router.Handler("POST", "/xrpc/*path", xrpc)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	cli.WebsocketURL = "ws" + strings.TrimPrefix(server.URL, "http")
	return &captionNode{cli: cli, mod: mod, bus: b, mm: mm, server: server, segments: mm.NewSegment(), engine: engine}
}

// Same bounded synthetic fMP4 pattern as media's ingest tests, with continuous
// non-silent Opus audio long enough for the real recognizer's two-pass agreement.
func captionMedia(t *testing.T, ctx context.Context) []byte {
	t.Helper()
	gstinit.InitGST()
	pipeline, err := gst.NewPipelineFromString("videotestsrc num-buffers=180 ! video/x-raw,width=160,height=120,framerate=30/1 ! x264enc key-int-max=30 tune=zerolatency speed-preset=ultrafast ! h264parse ! mp4mux name=mux fragment-duration=100 ! appsink name=sink sync=false audiotestsrc num-buffers=300 samplesperbuffer=960 ! audio/x-raw,rate=48000,channels=1 ! audioconvert ! opusenc ! opusparse ! mux.")
	require.NoError(t, err)
	sink, err := pipeline.GetElementByName("sink")
	require.NoError(t, err)
	var data bytes.Buffer
	app.SinkFromElement(sink).SetCallbacks(&app.SinkCallbacks{NewSampleFunc: media.WriterNewSample(ctx, &data)})
	done := make(chan error, 1)
	go func() { done <- media.HandleBusMessages(ctx, pipeline) }()
	require.NoError(t, pipeline.SetState(gst.StatePlaying))
	defer func() { _ = pipeline.SetState(gst.StateNull) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("caption media generator timed out")
	}
	return data.Bytes()
}

func captionSigner(t *testing.T, ctx context.Context, origin *captionNode, canonical string, allowed bool) (media.MediaSigner, string, string) {
	t.Helper()
	priv, pub, err := spkey.GenerateStreamKey()
	require.NoError(t, err)
	did := pub.DIDKey()
	require.NoError(t, origin.mod.UpdateSigningKey(&model.SigningKey{DID: did, RepoDID: did, RKey: "captions"}))
	signer, err := spkey.KeyToSigner(priv)
	require.NoError(t, err)
	ms, err := media.MakeMediaSigner(ctx, origin.cli, did, signer, origin.mod)
	require.NoError(t, err)
	ms.(*media.MediaSignerLocal).PrebuiltManifest = []byte(fmt.Sprintf(`{"title":"caption multitest","assertions":[{"label":"c2pa.actions.v2","data":{"actions":[{"action":"c2pa.created"},{"action":"c2pa.published"}]}},{"label":"cawg.metadata","data":{"@context":{"dc":"http://purl.org/dc/elements/1.1/"},"dc:creator":%q,"dc:title":"captions","dc:date":"2026-10-01T00:00:00.000Z"}},{"label":"place.stream.metadata.configuration","data":{"captionPolicy":{"canonical":%q,"allowNodeCaptions":%t,"languages":["en"]}}}]}`, did, canonical, allowed))
	t.Cleanup(func() { origin.mm.EndCaptionSession(did) })
	return ms, did, "z" + base58.Encode(priv.Bytes())
}

func captionSegment(event *muxl.MuxlEvent) []byte {
	// Keep the primary AV manifest first, including when IDs are not contiguous.
	ids := make([]string, 0, len(event.Tracks))
	for id := range event.Tracks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var data []byte
	for _, id := range ids {
		data = append(data, event.Tracks[id]...)
	}
	return data
}

func ingestCaptionSegments(t *testing.T, ctx context.Context, origin *captionNode, segments [][]byte) {
	t.Helper()
	for _, segment := range segments {
		require.NoError(t, origin.mm.ValidateMP4(ctx, bytes.NewReader(segment), true))
		select {
		case not := <-origin.segments:
			// The binary node's director publishes these validated notifications
			// on the source bus. Keep only that boundary in this focused harness.
			origin.bus.PublishSegment(ctx, not.Segment.RepoDID, "source", &bus.Seg{Muxl: not.Muxl, Published: not.Metadata.Published})
		case <-ctx.Done():
			t.Fatal("origin did not distribute its signed segment")
		}
	}
}

func signCaptionSegments(t *testing.T, ctx context.Context, origin *captionNode, ms media.MediaSigner, input io.Reader) [][]byte {
	t.Helper()
	events := make(chan *muxl.MuxlEvent, 32)
	done := make(chan error, 1)
	go func() { done <- origin.mm.SignOriginStream(ctx, ms, input, events); close(events) }()
	var segments [][]byte
	for {
		select {
		case event, ok := <-events:
			if !ok {
				require.NoError(t, <-done)
				return segments
			}
			if event.Type == "signed-segment" {
				segments = append(segments, captionSegment(event))
			}
		case <-ctx.Done():
			t.Fatal("origin signing timed out")
		}
	}
}

func syndicateCaptionStream(t *testing.T, ctx context.Context, relay, origin *captionNode, did string, segments [][]byte) {
	t.Helper()
	wsURL := origin.cli.WebsocketURL + "/xrpc/place.stream.live.subscribeSegments?streamer=" + url.QueryEscape(did)
	record := placestream.BroadcastOrigin{Streamer: did, Server: origin.cli.ServerDID(), WebsocketURL: &wsURL, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	uri := syntax.ATURI(fmt.Sprintf("at://%s/place.stream.broadcast.origin/%s::%s", did, did, record.Server))
	require.NoError(t, relay.mod.UpdateBroadcastOrigin(ctx, record, uri))
	r := websocketrep.NewWebsocketReplicator(relay.bus, relay.mod, relay.mm, nil)
	done := make(chan error, 1)
	go func() { done <- r.Start(ctx, relay.cli) }()
	t.Cleanup(func() { relay.mm.EndCaptionSession(did) })
	// A published window alone only proves the first cached GoP arrived.
	// Drain every replayed segment before asserting recognition stayed off.
	replayed := segments[max(0, len(segments)-2):]
	for _, expected := range replayed {
		select {
		case not := <-relay.segments:
			require.Equal(t, did, not.Segment.RepoDID)
			require.Equal(t, expected, not.Muxl, "syndication must preserve signed media bytes")
		case <-ctx.Done():
			t.Fatal("relay never validated all syndicated public media")
		}
	}
	require.True(t, relay.mm.LiveWindowPublished(did))
	select {
	case err := <-done:
		require.NoError(t, err)
	default:
	}
}

func captionTracks(t *testing.T, node *captionNode, did string) []placestream.CaptionDefs_TrackView {
	t.Helper()
	tracks, err := listCaptionTracks(node, did)
	require.NoError(t, err)
	return tracks
}

// listCaptionTracks is safe in an assert.Never/Eventually condition, which
// runs on its own goroutine and may outlive the test.
func listCaptionTracks(node *captionNode, did string) ([]placestream.CaptionDefs_TrackView, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(node.server.URL + "/xrpc/place.stream.caption.listTracks?streamer=" + url.QueryEscape(did))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("listTracks: HTTP %d", resp.StatusCode)
	}
	var out placestream.CaptionListTracks_Output
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Tracks, nil
}

func publicCaption(t *testing.T, ctx context.Context, node *captionNode, did, text string) placestream.CaptionDefs_LiveCue {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, node.cli.WebsocketURL+"/api/websocket/"+did, nil)
	require.NoError(t, err)
	defer conn.Close()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(captionTimeout)))
	for {
		_, body, err := conn.ReadMessage()
		require.NoError(t, err, "relay did not publish expected live cue %q", text)
		var cue placestream.CaptionDefs_LiveCue
		if json.Unmarshal(body, &cue) == nil && cue.LexiconTypeID == "place.stream.caption.defs#liveCue" && cue.Text == text {
			return cue
		}
	}
}

func TestCaptionCanonicalPassthrough(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	origin := newCaptionNode(t, ctx, "caption-origin")
	relay := newCaptionNode(t, ctx, "caption-relay")
	ms, did, key := captionSigner(t, ctx, origin, "ingest", true)
	_, file, _, _ := runtime.Caller(0)
	input, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "test", "fixtures", "h264-opus-frag.mp4"))
	require.NoError(t, err)
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	events := make(chan *muxl.MuxlEvent, 32)
	done := make(chan error, 1)
	started := time.Now()
	go func() { done <- origin.mm.SignOriginStream(ctx, ms, reader, events); close(events) }()
	writeDone := make(chan error, 1)
	go func() { _, err := writer.Write(input); writeDone <- err }()
	// The first GoP establishes the signed ingest policy. Push into the next
	// unsigned GoP, keeping input open so session ownership remains live.
	for {
		select {
		case event := <-events:
			require.NotNil(t, event)
			if event.Type == "signed-segment" {
				goto live
			}
		case <-ctx.Done():
			t.Fatal("first signed GoP did not arrive")
		}
	}
live:
	id := "canonical-known"
	body, err := json.Marshal(&placestream.CaptionPushCaptions_Input{Streamer: &did, Language: "en", Cues: []placestream.CaptionDefs_PushedCue{{Id: &id, StartTime: started.Add(250 * time.Millisecond).Format(time.RFC3339Nano), EndTime: started.Add(750 * time.Millisecond).Format(time.RFC3339Nano), Text: "Pushed words survive syndication."}}})
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, origin.server.URL+"/xrpc/place.stream.caption.pushCaptions", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	require.NoError(t, err)
	responseBody, err := io.ReadAll(resp.Body)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s", responseBody)
	require.Empty(t, origin.bus.Captions.Tracks(did), "pushed canonical cues must not bypass signed MUXL")
	select {
	case err := <-writeDone:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("ingest input blocked")
	}
	require.NoError(t, writer.Close())
	var segments [][]byte
	for {
		select {
		case event, ok := <-events:
			if !ok {
				goto mastered
			}
			if event.Type == "signed-segment" {
				segments = append(segments, captionSegment(event))
			}
		case <-ctx.Done():
			t.Fatal("canonical mastering timed out")
		}
	}
mastered:
	require.NoError(t, <-done)
	// Join once canonical text is introduced. Every segment this relay sees
	// now contains the canonical track, so it must never start recognition.
	require.NotEmpty(t, segments)
	tracks, err := muxl.RunMuxlTextTracks(ctx, bytes.NewReader(segments[0]))
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	require.Equal(t, "human", tracks[0].Label)
	archived := bytes.Join(segments, nil)
	cues, err := muxl.RunMuxlReadTextCues(ctx, bytes.NewReader(archived), tracks[0].TrackID)
	require.NoError(t, err)
	require.Len(t, cues, 1)
	require.Equal(t, "Pushed words survive syndication.", cues[0].Text)
	ingestCaptionSegments(t, ctx, origin, segments)
	syndicateCaptionStream(t, ctx, relay, origin, did, segments)
	cue := publicCaption(t, ctx, relay, did, cues[0].Text)
	require.Equal(t, "canonical", cue.Track.Origin)
	require.True(t, cue.Final)
	listed := captionTracks(t, relay, did)
	require.Len(t, listed, 1)
	require.Equal(t, "canonical", listed[0].Origin)
	require.Zero(t, origin.engine.leases.Load(), "ingest-only origin must not recognize")
	require.Zero(t, relay.engine.leases.Load(), "canonical relay must not recognize")
	require.Zero(t, relay.engine.passes.Load())
}

func TestCaptionSidecarPassthrough(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	origin := newCaptionNode(t, ctx, "sidecar-origin")
	relay := newCaptionNode(t, ctx, "sidecar-relay")
	ms, did, _ := captionSigner(t, ctx, origin, "off", true)
	segments := signCaptionSegments(t, ctx, origin, ms, bytes.NewReader(captionMedia(t, ctx)))
	ingestCaptionSegments(t, ctx, origin, segments)
	// Join after an upstream final exists, as a real late-joining relay does.
	// The replay must prevent the relay from leasing its own recognizer even
	// though the media contains no canonical text track.
	require.Eventually(t, func() bool {
		return len(origin.bus.Captions.Cues(did, "sidecar-auto-en", time.Time{}, time.Now().Add(time.Minute))) > 0
	}, captionTimeout, 10*time.Millisecond, "fake STT did not produce final sidecar captions from decoded audio")
	require.Positive(t, origin.engine.passes.Load())
	syndicateCaptionStream(t, ctx, relay, origin, did, segments)
	cue := publicCaption(t, ctx, relay, did, "Deterministic sidecar.")
	require.Equal(t, "sidecar", cue.Track.Origin)
	require.NotNil(t, cue.Track.Author)
	require.Equal(t, origin.cli.ServerDID(), *cue.Track.Author)
	listed := captionTracks(t, relay, did)
	require.Len(t, listed, 1)
	require.Equal(t, "sidecar", listed[0].Origin)
	require.Equal(t, origin.cli.ServerDID(), *listed[0].Author)
	require.Zero(t, relay.engine.leases.Load(), "upstream sidecar must prevent relay recognition")
	require.Zero(t, relay.engine.passes.Load())
}

func TestCaptionNodeOptOut(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	origin := newCaptionNode(t, ctx, "optout-origin")
	relay := newCaptionNode(t, ctx, "optout-relay")
	ms, did, _ := captionSigner(t, ctx, origin, "off", false)
	segments := signCaptionSegments(t, ctx, origin, ms, bytes.NewReader(captionMedia(t, ctx)))
	ingestCaptionSegments(t, ctx, origin, segments)
	syndicateCaptionStream(t, ctx, relay, origin, did, segments)
	// A stale node sidecar on the origin's hub must not be syndicated after
	// the streamer's signed opt-out. This also exercises the public output
	// gate, rather than merely observing the absence of recognition.
	now := time.Now()
	track := captions.Track{ID: "sidecar-auto-en", Language: "en", Kind: captions.KindCaptions, Source: captions.SourceAuto, Origin: captions.OriginSidecar, Author: origin.cli.ServerDID()}
	origin.bus.Captions.Publish(did, track, captions.Cue{ID: "forbidden", Start: now, End: now.Add(time.Second), Text: "Must not be distributed.", Final: true})
	require.Never(t, func() bool {
		tracks, err := listCaptionTracks(relay, did)
		return (err == nil && len(tracks) > 0) || relay.engine.leases.Load() > 0 || origin.engine.leases.Load() > 0
	}, time.Second, 20*time.Millisecond, "opt-out must prevent recognition and public sidecars")
	require.Empty(t, captionTracks(t, relay, did))
	require.Zero(t, relay.engine.passes.Load())
}
