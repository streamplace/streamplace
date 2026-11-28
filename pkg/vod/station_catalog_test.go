package vod

import (
	"context"
	"fmt"
	"testing"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/internal/stationtest"
	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/s3"
	"stream.place/streamplace/pkg/spid"
)

func TestStationCatalogHandoff(t *testing.T) {
	root := t.TempDir()
	a, err := blob.NewFileStore(root)
	require.NoError(t, err)
	b, err := blob.NewFileStore(root)
	require.NoError(t, err)
	exerciseStationCatalog(t, a, b)
}

func TestStationCatalogHandoffS3(t *testing.T) {
	cfg := stationtest.Config(t)
	exerciseStationCatalog(t, blob.NewS3Store(s3.NewClient(cfg), cfg.Bucket), blob.NewS3Store(s3.NewClient(cfg), cfg.Bucket))
}

func exerciseStationCatalog(t *testing.T, a, b blob.Store) {
	t.Helper()
	ctx := t.Context()
	const owner = "did:plc:ewvi7nxzyoun6zhxrhs64oiz"
	did := "did:web:" + uuid.NewString() + ".example"
	content := []byte(uuid.NewString())
	cid, initCID := bdasl.CID(content), bdasl.CID([]byte("init"))
	uri, trackURI := "at://"+owner+"/place.stream.video/v", "at://"+owner+"/place.stream.media.track/t"
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	assertServe := func(c string, allowInit, want bool) {
		t.Helper()
		got, err := StationCanServe(ctx, b, mod, did, c, allowInit)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	assertServe(cid, false, false)
	// A station record can be deleted at its PDS before any local backfill
	// sees the corresponding historic advertisement.
	unseen := bdasl.CID([]byte("withdrawn before migration"))
	require.NoError(t, RetractStationRecord(ctx, b, did, "at://"+did+"/"+constants.PLACE_STREAM_MEDIA_ORIGIN+"/"+unseen))
	require.ErrorIs(t, AdvertiseStationVideo(ctx, a, did, unseen, uri), ErrStationOriginWithdrawn)
	// Media and sidecars can precede the creator's publication for days.
	// Their presence is not consent and does not make them playable on B.
	require.NoError(t, blob.CreateOnce(ctx, a, BlobsPrefix+cid+".mp4", content))
	meta := Metafile{BlobCID: cid, FlatHeaderSize: 8, BlobSize: int64(len(content)) - 8,
		Tracks: map[string]MetafileTrack{"1": {InitCID: initCID, BlobCID: cid, Type: "video"}}}
	require.NoError(t, writeMetafile(ctx, a, cid, &meta))
	require.ErrorIs(t, AdvertiseStationVideo(ctx, a, did, cid, uri), blob.ErrNotFound)
	require.NoError(t, blob.CreateOnce(ctx, a, BlobsPrefix+initCID+".mp4", []byte("init")))
	assertServe(cid, true, false)
	require.NoError(t, AdvertiseStationVideo(ctx, a, did, cid, uri))
	assertServe(cid, false, false) // a cold edge has no trusted creator/track yet
	track := placestream.MediaTrack{LexiconTypeID: constants.PLACE_STREAM_MEDIA_TRACK, Track: placestream.MediaTrack_Track{
		MediaDefs_MuxlTrack: &placestream.MediaDefs_MuxlTrack{LexiconTypeID: "place.stream.media.defs#muxlTrack", Blob: cid, TrackId: "1", MediaType: "video"},
	}}
	hash, err := spid.GetCID(&track)
	require.NoError(t, err)
	require.NoError(t, mod.UpsertMediaTrack(ctx, track, syntax.ATURI(trackURI)))
	video := placestream.Video{Title: "handoff", Source: placestream.Video_Source{MediaDefs_SourceTracks: &placestream.MediaDefs_SourceTracks{
		LexiconTypeID: "place.stream.media.defs#sourceTracks", Tracks: []comatproto.RepoStrongRef{{Uri: trackURI, Cid: hash.String()}},
	}}}
	require.NoError(t, mod.UpsertVideo(ctx, video, syntax.ATURI(uri)))
	// B has not received a station origin firehose event and has no session.
	assertServe(cid, false, true)
	assertServe(initCID, true, true)
	assertServe(initCID, false, false)
	allowed := func(ctx context.Context, uri, cid string) (bool, error) {
		return StationVideoVisible(ctx, b, mod, did, uri, cid)
	}
	page, err := mod.GetStationVideoList(ctx, "", 25, "", allowed)
	require.NoError(t, err)
	require.Len(t, page.Videos, 1)
	other, err := StationCanServe(ctx, b, mod, "did:web:other.example", cid, true)
	require.NoError(t, err)
	require.False(t, other)
	// A creator record deletion retires only that approved source. A second
	// public record for the same CID keeps the station's promise alive.
	secondURI := fmt.Sprintf("at://%s/place.stream.video/v2", owner)
	require.NoError(t, mod.UpsertVideo(ctx, video, syntax.ATURI(secondURI)))
	require.NoError(t, AdvertiseStationVideo(ctx, a, did, cid, secondURI))
	require.NoError(t, RetractStationRecord(ctx, b, did, uri))
	require.ErrorIs(t, AdvertiseStationVideo(ctx, a, did, cid, uri), ErrStationOriginWithdrawn)
	assertServe(cid, true, true)
	page, err = mod.GetStationVideoList(ctx, "", 25, "", allowed)
	require.NoError(t, err)
	require.Len(t, page.Videos, 1)
	require.Equal(t, secondURI, page.Videos[0].Uri)
	// Source changes retire the last approval without authorizing the new CID.
	require.NoError(t, UpdateStationVideoSource(ctx, b, did, secondURI, bdasl.CID([]byte("replacement"))))
	assertServe(cid, true, false)
	require.ErrorIs(t, AdvertiseStationVideo(ctx, a, did, cid, secondURI), ErrStationOriginWithdrawn)
	require.NoError(t, WithdrawStationOrigin(ctx, b, did, cid))
	require.ErrorIs(t, AdvertiseStationVideo(ctx, a, did, cid, fmt.Sprintf("at://%s/place.stream.video/v3", owner)), ErrStationOriginWithdrawn)
	assertServe(cid, true, false)
	page, err = mod.GetStationVideoList(ctx, "", 25, "", allowed)
	require.NoError(t, err)
	require.Empty(t, page.Videos)
}
