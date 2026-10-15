package websocketrep

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/moq"
	"stream.place/streamplace/pkg/placestream"
)

func TestDialMoqRejectsMalformedPin(t *testing.T) {
	r := &WebsocketReplicator{}
	_, err := r.dialMoq(context.Background(), "moqt://127.0.0.1:1?certhash=ab", nil)
	require.ErrorContains(t, err, "SHA-256 fingerprint")
}

// A --peers entry naming a node's web origin resolves to the MoQ URL that
// node currently reports; one with no listener is an error to retry, not a
// session to open.
func TestResolvePeer(t *testing.T) {
	var moqURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		require.Equal(t, "/xrpc/place.stream.broadcast.getBroadcaster", req.URL.Path)
		out := placestream.BroadcastGetBroadcaster_Output{Broadcaster: "did:web:peer"}
		if moqURL != "" {
			out.MoqURL = &moqURL
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()
	r := &WebsocketReplicator{}

	_, err := r.resolvePeer(context.Background(), srv.URL)
	require.ErrorContains(t, err, "no moq listener")

	moqURL = "moqt://127.0.0.1:38443?certhash=ab"
	got, err := r.resolvePeer(context.Background(), srv.URL+"/ignored?x=1")
	require.NoError(t, err)
	require.Equal(t, moqURL, got)

	got, err = r.resolvePeer(context.Background(), "moqt://peer.example:38443?certhash=cd")
	require.NoError(t, err)
	require.Equal(t, "moqt://peer.example:38443?certhash=cd", got)

	_, err = r.resolvePeer(context.Background(), "ftp://peer.example")
	require.Error(t, err)
}

func TestMoqPublicationDisabled(t *testing.T) {
	r := NewWebsocketReplicator(&config.CLI{DisableSyndication: true}, nil, nil, nil, nil, nil)
	require.NoError(t, r.PublishMoqSource(context.Background(), "did:plc:streamer", nil, nil, true))
	require.Empty(t, r.moqBroadcasts)
}

func TestMoqRenditionRequiresAlignedSource(t *testing.T) {
	did, payload := signedMoqArchiveSource(t, 3600)
	r := archiveTestReplicator(t.TempDir())
	defer closeArchiveTestReplicator(r)
	ctx := context.Background()
	meta, err := r.mm.ValidateArchivedMP4(ctx, bytes.NewReader(payload), did)
	require.NoError(t, err)
	timestamp := uint64(meta.StartTime.Time().UnixMicro())
	require.ErrorContains(t, r.publishMoqSegment(ctx, did, moq.RenditionTrack(100), payload, timestamp, true, time.Time{}), "no archived source")
	require.NoError(t, r.PublishMoqSource(ctx, did, payload, meta, true))
	require.NoError(t, r.publishMoqSegment(ctx, did, moq.RenditionTrack(100), payload, timestamp, true, time.Time{}))
	b := r.moqBroadcasts[did]
	source, ok := b.tracks[moq.SourceTrack].archive.sequenceAt(timestamp)
	require.True(t, ok)
	rendition, ok := b.tracks[moq.RenditionTrack(100)].archive.sequenceAt(timestamp)
	require.True(t, ok)
	require.Equal(t, source, rendition)
	stored, ok, err := b.tracks[moq.RenditionTrack(100)].archive.get(source)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, payload, stored.payload)
	require.NoError(t, r.PublishMoqSource(ctx, did, payload, meta, true), "duplicate source keeps its sequence")
}
