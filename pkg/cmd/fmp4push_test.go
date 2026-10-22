package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/muxl"
)

// The e2e push client sends every video track of its file, each through its
// own codec's parser, ahead of the audio tracks its labels name.
func TestPushFMP4EveryVideoTrack(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	catCh := make(chan *muxl.MuxlCatalog, 1)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/ingest/client-configuration" {
			_ = json.NewEncoder(w).Encode(map[string]any{"ingest_endpoints": []map[string]string{
				{"protocol": "FMP4", "url_template": srv.URL + "/ingest/{stream_key}"},
			}})
			return
		}
		events := make(chan *muxl.MuxlEvent, 16)
		go func() {
			_ = muxl.RunMuxlSegmenterEvents(context.Background(), r.Body, events, muxl.WithSegmentSourceTrackMeta())
			close(events)
		}()
		for ev := range events {
			if ev.Type == "init" && ev.Catalog != nil {
				select {
				case catCh <- ev.Catalog:
				default:
				}
			}
		}
	}))
	defer srv.Close()

	go func() { _ = pushFMP4(ctx, srv.URL, "key", "../../test/fixtures/av1-h264-opus.mp4") }()
	var cat *muxl.MuxlCatalog
	select {
	case cat = <-catCh:
	case <-time.After(time.Minute):
	}
	cancel() // ends the push, and with it the handler srv.Close waits for
	require.NotNil(t, cat, "the push sent an init")

	videos := map[uint32]string{}
	for _, v := range cat.Video.Renditions {
		videos[v.TrackID()] = v.Codec[:4]
	}
	require.Equal(t, map[uint32]string{1: "av01", 2: "avc1"}, videos)
	labels := map[uint32]string{}
	for _, a := range cat.Audio.Renditions {
		labels[a.TrackID()] = a.Label
	}
	require.Equal(t, map[uint32]string{3: "Track 1", 4: "Track 1", 5: "Music", 6: "Music"}, labels)
}
