package spxrpc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/spkey"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/pkg/placestream"
)

func captionOAuthContext(t *testing.T, ctx context.Context, did string) context.Context {
	t.Helper()
	raw, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	key, err := jwk.FromRaw(raw)
	require.NoError(t, err)
	data, err := json.Marshal(key)
	require.NoError(t, err)
	ctx = context.WithValue(ctx, oatproxy.OATProxyContextKey, &oatproxy.OATProxy{})
	return context.WithValue(ctx, oatproxy.OAuthSessionContextKey, &oatproxy.OAuthSession{DID: did, UpstreamDPoPPrivateJWK: string(data)})
}
func requireCaptionHTTPError(t *testing.T, err error, code int, message string) {
	t.Helper()
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, code, httpErr.Code)
	require.Contains(t, httpErr.Message, message)
}
func TestPushCaptionsAuthorizationAndLiveBoundary(t *testing.T) {
	s := &Server{cli: &config.CLI{}, mm: media.NewOffline(&config.CLI{})}
	input := &placestream.CaptionPushCaptions_Input{Language: "en-US"}
	_, err := s.handlePlaceStreamCaptionPushCaptions(context.Background(), input)
	requireCaptionHTTPError(t, err, http.StatusUnauthorized, "authorization")
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer z2")
	ec := echo.New().NewContext(req, httptest.NewRecorder())
	_, err = s.handlePlaceStreamCaptionPushCaptions(context.WithValue(context.Background(), echoContextKey, ec), input)
	requireCaptionHTTPError(t, err, http.StatusUnauthorized, "invalid stream key")
	ctx := captionOAuthContext(t, context.Background(), "did:plc:owner")
	other := "did:plc:other"
	input.Streamer = &other
	_, err = s.handlePlaceStreamCaptionPushCaptions(ctx, input)
	requireCaptionHTTPError(t, err, http.StatusForbidden, "Forbidden")
	input.Streamer = nil
	_, err = s.handlePlaceStreamCaptionPushCaptions(ctx, input)
	requireCaptionHTTPError(t, err, http.StatusBadRequest, "StreamNotLive")
}

func TestPushCaptionsLiveCanonicalAndOffRoutes(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	fixture, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "test", "fixtures", "h264-opus-frag.mp4"))
	require.NoError(t, err)
	for _, tc := range []struct {
		name      string
		canonical string
		nodes     bool
		origin    captions.Origin
	}{
		{"canonical", "ingest", true, captions.OriginCanonical},
		{"sidecar", "off", true, captions.OriginSidecar},
		{"local", "off", false, captions.OriginLocal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			did := "did:plc:owner"
			cli := &config.CLI{CaptionsMasterDelay: time.Second}
			mm := media.NewOffline(cli)
			priv, _, err := spkey.GenerateStreamKey()
			require.NoError(t, err)
			signer, err := spkey.KeyToSigner(priv)
			require.NoError(t, err)
			ms, err := media.MakeMediaSigner(ctx, cli, did, signer, nil)
			require.NoError(t, err)
			policy := map[string]any{"canonical": tc.canonical, "allowNodeCaptions": tc.nodes, "languages": []string{"en-US"}}
			manifest, err := json.Marshal(map[string]any{"title": "push caption test", "assertions": []any{map[string]any{"label": "c2pa.actions", "data": map[string]any{"actions": []any{map[string]any{"action": "c2pa.created"}}}}, map[string]any{"label": "place.stream.metadata.configuration", "data": map[string]any{"captionPolicy": policy}}}})
			require.NoError(t, err)
			ms.(*media.MediaSignerLocal).PrebuiltManifest = manifest
			reader, writer := io.Pipe()
			defer reader.Close()
			defer writer.Close()
			events := make(chan *muxl.MuxlEvent, 16)
			done := make(chan error, 1)
			streamStarted := time.Now()
			go func() { done <- mm.SignOriginStream(ctx, ms, reader, events); close(events) }()
			go func() { _, _ = writer.Write(fixture) }()
			var archived bytes.Buffer
			appendEvent := func(event *muxl.MuxlEvent) {
				ids := make([]string, 0, len(event.Tracks))
				for id := range event.Tracks {
					ids = append(ids, id)
				}
				sort.Strings(ids)
				for _, id := range ids {
					archived.Write(event.Tracks[id])
				}
			}
			for {
				select {
				case event := <-events:
					require.NotNil(t, event)
					if event.Type == "signed-segment" {
						appendEvent(event)
						goto live
					}
				case <-ctx.Done():
					t.Fatal("first signed GoP did not arrive")
				}
			}
		live:
			hub := captions.NewHub(0)
			s := &Server{cli: cli, mm: mm, bus: &bus.Bus{Captions: hub}}
			auth := captionOAuthContext(t, ctx, did)
			id := strings.Repeat("界", 21) // 63 UTF-8 bytes.
			final := false
			start := streamStarted.Add(100 * time.Millisecond)
			end := start.Add(300 * time.Millisecond)
			input := &placestream.CaptionPushCaptions_Input{Language: "en-US", Cues: []placestream.CaptionDefs_PushedCue{{Id: &id, StartTime: start.Format(time.RFC3339Nano), EndTime: end.Format(time.RFC3339Nano), Text: strings.Repeat("界", 666), Final: &final}}}
			// A rejected batch must not publish its valid prefix to either route.
			atomicID := "must-not-publish"
			oversizedID := strings.Repeat("界", 22) // 66 bytes, but only 22 runes.
			badBatch := &placestream.CaptionPushCaptions_Input{Language: "en-US", Cues: []placestream.CaptionDefs_PushedCue{
				{Id: &atomicID, StartTime: start.Format(time.RFC3339Nano), EndTime: end.Format(time.RFC3339Nano), Text: "rejected prefix"},
				{Id: &oversizedID, StartTime: start.Format(time.RFC3339Nano), EndTime: end.Format(time.RFC3339Nano), Text: "rejected suffix"},
			}}
			_, err = s.handlePlaceStreamCaptionPushCaptions(auth, badBatch)
			requireCaptionHTTPError(t, err, http.StatusBadRequest, "lexicon limits")
			require.Empty(t, hub.Tracks(did))
			badBatch.Cues[1].Id = &id
			badBatch.Cues[1].Text = strings.Repeat("界", 667) // 2001 bytes.
			_, err = s.handlePlaceStreamCaptionPushCaptions(auth, badBatch)
			requireCaptionHTTPError(t, err, http.StatusBadRequest, "lexicon limits")
			require.Empty(t, hub.Tracks(did))
			_, err = s.handlePlaceStreamCaptionPushCaptions(auth, input)
			require.NoError(t, err)
			final = true
			input.Cues[0].Text = "CART words survive signing"
			_, err = s.handlePlaceStreamCaptionPushCaptions(auth, input)
			require.NoError(t, err)
			if tc.origin == captions.OriginCanonical {
				require.Empty(t, hub.Tracks(did), "canonical cues cannot bypass validated MUXL")
			} else {
				tracks := hub.Tracks(did)
				require.Len(t, tracks, 1)
				require.Equal(t, tc.origin, tracks[0].Origin)
				cues := hub.Cues(did, tracks[0].ID, start.Add(-time.Second), end.Add(time.Second))
				require.Len(t, cues, 1)
				require.Equal(t, "CART words survive signing", cues[0].Text)
				require.True(t, cues[0].Final)
			}
			require.NoError(t, writer.Close())
			for event := range events {
				if event.Type == "signed-segment" {
					appendEvent(event)
				}
			}
			require.NoError(t, <-done)
			tracks, err := muxl.RunMuxlTextTracks(ctx, bytes.NewReader(archived.Bytes()))
			require.NoError(t, err)
			if tc.origin == captions.OriginCanonical {
				require.Equal(t, []muxl.TextTrack{{TrackID: media.CaptionTrackIDBase, Language: "en-US", Label: "human"}}, tracks)
				cues, err := muxl.RunMuxlReadTextCues(ctx, bytes.NewReader(archived.Bytes()), media.CaptionTrackIDBase)
				require.NoError(t, err)
				require.Len(t, cues, 1)
				require.Equal(t, "CART words survive signing", cues[0].Text)
			} else {
				require.Empty(t, tracks, "canonical=off must not add a text track")
			}
			_, live := mm.OriginCaptionPolicy(did)
			require.False(t, live, "session teardown must revoke push ownership")
		})
	}
}
