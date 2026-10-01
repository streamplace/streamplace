package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/config"
	ct "stream.place/streamplace/pkg/config/configtesting"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/muxl"
)

func websocketCaptionFixture(t *testing.T) (*StreamplaceAPI, string, []byte) {
	t.Helper()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	ldb, err := localdb.MakeDB(":memory:")
	require.NoError(t, err)
	cli := ct.CLI(t, &config.CLI{DBURL: ":memory:"})
	b := bus.NewBus()
	atsync := &atproto.ATProtoSynchronizer{CLI: cli, Model: mod, Bus: b}
	mm, err := media.MakeMediaManager(context.Background(), cli, nil, mod, b, atsync, ldb)
	require.NoError(t, err)

	_, testFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	frag, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), "..", "..", "test", "fixtures", "h264-opus-frag.mp4"))
	require.NoError(t, err)
	rendition, err := muxl.RunMuxlCanonicalize(context.Background(), frag, nil)
	require.NoError(t, err)

	a := &StreamplaceAPI{
		CLI: cli, Model: mod, LocalDB: ldb, MediaManager: mm,
		Aliases: map[string]string{}, Bus: b, ATSync: atsync,
		connTracker: NewWebsocketTracker(0),
	}
	router := httprouter.New()
	router.GET("/api/websocket/:repoDID", a.HandleWebsocket(context.Background()))
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return a, "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/websocket/did:key:zCaptionPrivacy", rendition
}

func setWebsocketCaptionPublished(t *testing.T, a *StreamplaceAPI, did string, rendition []byte, published bool) {
	t.Helper()
	a.MediaManager.FeedLiveRenditions(context.Background(), did, rendition, time.Now(), published)
	require.NotNil(t, a.MediaManager.GetLiveWindow(did))
	require.Equal(t, published, a.MediaManager.LiveWindowPublished(did))
}

func requireNoWebsocketCaption(t *testing.T, messages <-chan map[string]any) {
	t.Helper()
	select {
	case msg := <-messages:
		t.Fatalf("unexpected websocket caption: %#v", msg)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestWebsocketCaptionsFollowPublicLiveState(t *testing.T) {
	a, endpoint, rendition := websocketCaptionFixture(t)
	a.CLI.WideOpen = true // unauthenticated websocket captions still require public live state
	const did = "did:key:zCaptionPrivacy"
	setWebsocketCaptionPublished(t, a, did, rendition, false)
	now := time.Now()
	track := captions.Track{ID: "canonical-auto-en", Language: "en", Kind: captions.KindCaptions, Source: captions.SourceAuto, Origin: captions.OriginCanonical}
	a.Bus.Captions.Publish(did, track, captions.Cue{ID: "private-replay", Start: now.Add(-time.Second), End: now, Text: "private replay", Final: true})

	conn, _, err := websocket.DefaultDialer.Dial(endpoint, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	captionMessages := make(chan map[string]any, 16)
	go func() {
		for {
			_, body, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var msg map[string]any
			if json.Unmarshal(body, &msg) == nil && msg["$type"] == "place.stream.caption.defs#liveCue" {
				captionMessages <- msg
			}
		}
	}()

	requireNoWebsocketCaption(t, captionMessages)
	a.Bus.Captions.Publish(did, track, captions.Cue{ID: "private-future", Start: now, End: now.Add(time.Second), Text: "private future", Final: true})
	requireNoWebsocketCaption(t, captionMessages)

	setWebsocketCaptionPublished(t, a, did, rendition, true)
	a.Bus.Captions.Publish(did, track, captions.Cue{ID: "public", Start: now.Add(time.Second), End: now.Add(2 * time.Second), Text: "public", Final: true})
	select {
	case msg := <-captionMessages:
		require.Equal(t, "public", msg["id"])
		require.Equal(t, "public", msg["text"])
	case <-time.After(10 * time.Second):
		t.Fatal("published websocket caption was not delivered")
	}

	setWebsocketCaptionPublished(t, a, did, rendition, false)
	a.Bus.Captions.Publish(did, track, captions.Cue{ID: "private-again", Start: now.Add(2 * time.Second), End: now.Add(3 * time.Second), Text: "private again", Final: true})
	requireNoWebsocketCaption(t, captionMessages)
}
