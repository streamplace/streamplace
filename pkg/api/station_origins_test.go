package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/vod"
)

func TestStationBackfillRequiresPublishedOwnedUpload(t *testing.T) {
	ctx := t.Context()
	const owner = "did:plc:alice"
	const videoURI = "at://" + owner + "/place.stream.video/public"
	const trackURI = "at://" + owner + "/place.stream.media.track/track"
	cli := &config.CLI{DBURL: ":memory:", ServerHost: "node.example", StationDID: "did:web:station.example"}
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(ctx, cli, nil, mod)
	require.NoError(t, err)
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	a := &StreamplaceAPI{CLI: cli, Model: mod, StatefulDB: state, PlaybackStore: store}
	cid, initCID := bdasl.CID([]byte("asset")), bdasl.CID([]byte("init"))
	for id, data := range map[string]string{cid: "asset", initCID: "init"} {
		require.NoError(t, blob.CreateOnce(ctx, store, vod.BlobsPrefix+id+".mp4", []byte(data)))
	}
	meta, err := json.Marshal(vod.Metafile{BlobCID: cid, BlobSize: 5, Tracks: map[string]vod.MetafileTrack{"1": {InitCID: initCID}}})
	require.NoError(t, err)
	require.NoError(t, blob.CreateOnce(ctx, store, vod.BlobsPrefix+cid+".json", meta))
	require.NoError(t, state.CreateUpload(ctx, &statedb.Upload{ID: "upload", RepoDID: owner, ContentCID: cid, ProcessingStatus: "done"}))
	require.NoError(t, mod.UpsertOwnMediaOrigin(ctx, cli.ServerDID(), cid, 5, "video/mp4")) // old processors announced drafts
	backfill := func(want int) {
		t.Helper()
		res, err := a.backfillStationOrigins(ctx)
		require.NoError(t, err)
		require.Empty(t, res.Errors)
		require.Equal(t, want, res.Indexed)
	}
	backfill(0)
	track := placestream.MediaTrack{LexiconTypeID: constants.PLACE_STREAM_MEDIA_TRACK, Track: placestream.MediaTrack_Track{
		MediaDefs_MuxlTrack: &placestream.MediaDefs_MuxlTrack{LexiconTypeID: "place.stream.media.defs#muxlTrack", Blob: cid, TrackId: "1", MediaType: "video"},
	}}
	require.NoError(t, mod.UpsertMediaTrack(ctx, track, mustURI(t, trackURI)))
	hash, err := spid.GetCID(&track)
	require.NoError(t, err)
	refs := []comatproto.RepoStrongRef{{Uri: trackURI, Cid: hash.String()}}
	require.NoError(t, mod.UpsertVideo(ctx, placestream.Video{Title: "public", Source: placestream.Video_Source{MediaDefs_SourceTracks: &placestream.MediaDefs_SourceTracks{
		LexiconTypeID: "place.stream.media.defs#sourceTracks", Tracks: refs,
	}}}, mustURI(t, videoURI), ""))
	backfill(0) // indexed video with guessed CID, without completed creator publication
	stored, err := json.Marshal(refs)
	require.NoError(t, err)
	require.NoError(t, state.SetUploadTrackURIs(ctx, "upload", string(stored)))
	backfill(1)
	body, err := json.Marshal(map[string]string{"cid": cid})
	require.NoError(t, err)
	resp := httptest.NewRecorder()
	a.HandleWithdrawOrigin(ctx)(resp, httptest.NewRequest(http.MethodPost, "/withdraw-origin", bytes.NewReader(body)), nil)
	require.Equal(t, http.StatusNoContent, resp.Code)
	backfill(0)
	ok, err := vod.StationCanServe(ctx, store, mod, cli.StationDID, cid, false)
	require.NoError(t, err)
	require.False(t, ok)
}
