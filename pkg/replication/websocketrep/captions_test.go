package websocketrep

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/gorilla/websocket"
	glex "github.com/streamplace/glex/runtime"
	upstream "github.com/streamplace/muxl/go"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
)

func replicationCaptionFixture(t *testing.T) (string, []byte) {
	t.Helper()
	ctx := context.Background()
	priv, err := atcrypto.GeneratePrivateKeyK256()
	require.NoError(t, err)
	secp, _ := secp256k1.PrivKeyFromBytes(priv.Bytes())
	key := secp.ToECDSA()
	pub, err := atproto.ParsePubKey(key.Public().(*ecdsa.PublicKey))
	require.NoError(t, err)
	did := pub.DIDKey()
	cert, err := signers.GenerateES256KCert(key)
	require.NoError(t, err)
	pem, err := signers.MarshalES256KPrivateKeyPEM(key)
	require.NoError(t, err)
	manifest := []byte(fmt.Sprintf(`{"title":"replication","assertions":[{"label":"c2pa.actions.v2","data":{"actions":[{"action":"c2pa.created"},{"action":"c2pa.published"}]}},{"label":"cawg.metadata","data":{"@context":{"dc":"http://purl.org/dc/elements/1.1/"},"dc:creator":%q,"dc:title":"replication","dc:date":"2026-09-30T00:00:00.000Z"}},{"label":"place.stream.metadata.configuration","data":{"captionPolicy":{"canonical":"off","allowNodeCaptions":true}}}]}`, did))
	_, file, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../test/fixtures/h264-opus-frag.mp4"))
	require.NoError(t, err)
	eng, err := captions.TextEngine()
	require.NoError(t, err)
	ch := make(chan *upstream.Event, 32)
	errs := make(chan error, 1)
	go func() {
		errs <- eng.SignSegment(ctx, bytes.NewReader(data), upstream.SignerInput{CertPEM: cert, KeyPEM: pem, TrackManifest: manifest, WrapperManifest: manifest}, nil, nil, ch)
		close(ch)
	}()
	var segment []byte
	for ev := range ch {
		if ev.Type == "signed-segment" && segment == nil {
			var ids []string
			for id := range ev.Tracks {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			for _, id := range ids {
				segment = append(segment, ev.Tracks[id]...)
			}
		}
	}
	require.NoError(t, <-errs)
	require.NotEmpty(t, segment)
	return did, segment
}

func TestCaptionSyndicationWebsocketRoundTripAndLegacyOrigin(t *testing.T) {
	did, segment := replicationCaptionFixture(t)
	for _, supportsCaptions := range []bool{true, false} {
		t.Run(fmt.Sprint(supportsCaptions), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cli := &config.CLI{WideOpen: true, Captions: true, BroadcasterHost: "relay.example", DataDir: t.TempDir()}
			m, err := model.MakeDB(":memory:")
			require.NoError(t, err)
			ldb, err := localdb.MakeDB(":memory:")
			require.NoError(t, err)
			b := bus.NewBus()
			mm, err := media.MakeMediaManager(ctx, cli, nil, m, b, nil, ldb)
			require.NoError(t, err)
			defer mm.EndCaptionSession(did)
			segments := mm.NewSegment()
			wall := time.Now().UTC()
			ev := captions.Event{Streamer: did, Track: captions.Track{ID: "sidecar-auto-en", Language: "en", Kind: captions.KindCaptions, Source: captions.SourceAuto, Origin: captions.OriginSidecar, Author: "did:web:upstream.example"}, Cue: captions.Cue{ID: "line", Text: "Upstream captions", Start: wall, End: wall.Add(time.Second), Final: true}}
			encoded, err := captions.EncodeSidecar(ev)
			require.NoError(t, err)
			capability := make(chan string, 1)
			serverErr := make(chan error, 1)
			closeServer := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capability <- r.URL.Query().Get("captions")
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					serverErr <- err
					return
				}
				defer conn.Close()
				// Replay can arrive before the first validated segment. It must be
				// held until the segment's signed allowNodeCaptions policy is known.
				if supportsCaptions {
					if err = conn.WriteMessage(websocket.TextMessage, encoded); err != nil {
						serverErr <- err
						return
					}
				}
				if err = conn.WriteMessage(websocket.BinaryMessage, segment); err != nil {
					serverErr <- err
					return
				}
				if supportsCaptions {
					if err = conn.WriteMessage(websocket.TextMessage, encoded); err != nil {
						serverErr <- err
						return
					}
				}
				serverErr <- nil
				<-closeServer
			}))
			defer server.Close()
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(closeServer) }) }
			defer release()
			wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
			view := &placestream.BroadcastDefs_BroadcastOriginView{Record: &glex.LexiconTypeDecoder{Val: &placestream.BroadcastOrigin{Streamer: did, Server: "did:web:upstream.example", WebsocketURL: &wsURL}}}
			r := NewWebsocketReplicator(b, m, mm, nil)
			done := make(chan error, 1)
			go func() { done <- r.openWebsocket(ctx, view) }()
			require.Equal(t, captions.SyndicationVersion, <-capability)
			require.NoError(t, <-serverErr)
			if supportsCaptions {
				require.Eventually(t, func() bool { return len(b.Captions.Cues(did, ev.Track.ID, wall, wall.Add(2*time.Second))) == 1 }, 20*time.Second, time.Millisecond)
				require.Equal(t, []captions.Cue{ev.Cue}, b.Captions.Cues(did, ev.Track.ID, wall, wall.Add(2*time.Second)))
				require.Equal(t, "did:web:upstream.example", b.Captions.Tracks(did)[0].Author)
			} else {
				select {
				case not := <-segments:
					require.Equal(t, did, not.Segment.RepoDID)
					require.Equal(t, segment, not.Muxl, "legacy origin media remains byte-identical")
				case <-time.After(20 * time.Second):
					t.Fatal("legacy origin's binary media was not validated and distributed")
				}
				require.Empty(t, b.Captions.Tracks(did), "an older origin can send binary media only")
			}
			release()
			err = <-done
			require.Error(t, err, "socket close ends the pull")
			require.NotContains(t, err.Error(), "expected binary message")
		})
	}
}
