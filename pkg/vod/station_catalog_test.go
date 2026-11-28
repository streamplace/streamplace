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
	require.NoError(t, RetractStationRecord(ctx, b, did, "at://"+did+"/"+constants.PLACE_STREAM_MEDIA_ORIGIN+"/"+unseen, "", ""))
	require.ErrorIs(t, AdvertiseStationVideo(ctx, a, did, unseen, uri, ""), ErrStationOriginWithdrawn)
	// Media and sidecars can precede the creator's publication for days.
	// Their presence is not consent and does not make them playable on B.
	require.NoError(t, blob.CreateOnce(ctx, a, BlobsPrefix+cid+".mp4", content))
	meta := Metafile{BlobCID: cid, FlatHeaderSize: 8, BlobSize: int64(len(content)) - 8,
		Tracks: map[string]MetafileTrack{"1": {InitCID: initCID, BlobCID: cid, Type: "video"}}}
	require.NoError(t, writeMetafile(ctx, a, cid, &meta))
	require.ErrorIs(t, AdvertiseStationVideo(ctx, a, did, cid, uri, ""), blob.ErrNotFound)
	require.NoError(t, blob.CreateOnce(ctx, a, BlobsPrefix+initCID+".mp4", []byte("init")))
	assertServe(cid, true, false)
	require.NoError(t, AdvertiseStationVideo(ctx, a, did, cid, uri, ""))
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
	require.NoError(t, mod.UpsertVideo(ctx, video, syntax.ATURI(uri), ""))
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
	require.NoError(t, mod.UpsertVideo(ctx, video, syntax.ATURI(secondURI), ""))
	require.NoError(t, AdvertiseStationVideo(ctx, a, did, cid, secondURI, ""))
	require.NoError(t, RetractStationRecord(ctx, b, did, uri, "2222222222223", ""))
	require.ErrorIs(t, AdvertiseStationVideo(ctx, a, did, cid, uri, ""), ErrStationOriginWithdrawn)
	assertServe(cid, true, true)
	page, err = mod.GetStationVideoList(ctx, "", 25, "", allowed)
	require.NoError(t, err)
	require.Len(t, page.Videos, 1)
	require.Equal(t, secondURI, page.Videos[0].Uri)
	// Source changes retire the last approval without authorizing the new CID.
	require.NoError(t, UpdateStationVideoSource(ctx, b, did, secondURI, bdasl.CID([]byte("replacement")), "2222222222224"))
	assertServe(cid, true, false)
	require.ErrorIs(t, AdvertiseStationVideo(ctx, a, did, cid, secondURI, ""), ErrStationOriginWithdrawn)
	require.NoError(t, WithdrawStationOrigin(ctx, b, did, cid))
	require.ErrorIs(t, AdvertiseStationVideo(ctx, a, did, cid, fmt.Sprintf("at://%s/place.stream.video/v3", owner), "2222222222225"), ErrStationOriginWithdrawn)
	assertServe(cid, true, false)
	page, err = mod.GetStationVideoList(ctx, "", 25, "", allowed)
	require.NoError(t, err)
	require.Empty(t, page.Videos)
}

func TestStationCatalogPublicationOrdering(t *testing.T) {
	ctx := t.Context()
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	index, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state := newDraftTestState(t)
	state.CLI.StationDID = "did:web:station.example"
	did := state.CLI.StationDID
	const (
		uri         = "at://did:plc:alice/place.stream.video/video"
		firstRev    = "2222222222223"
		staleUpdate = "2222222222224"
		staleDelete = "2222222222225"
		replaceRev  = "2222222222226"
		deleteRev   = "2222222222227"
		readdRev    = "222222222222a"
	)
	init := []byte("init")
	initCID := bdasl.CID(init)
	require.NoError(t, blob.CreateOnce(ctx, store, BlobsPrefix+initCID+".mp4", init))
	stage := func(rkey string) (string, placestream.Video) {
		t.Helper()
		content := []byte("media for " + rkey)
		cid := bdasl.CID(content)
		require.NoError(t, blob.CreateOnce(ctx, store, BlobsPrefix+cid+".mp4", content))
		require.NoError(t, writeMetafile(ctx, store, cid, &Metafile{
			BlobCID: cid, BlobSize: int64(len(content)),
			Tracks: map[string]MetafileTrack{"1": {InitCID: initCID}},
		}))
		trackURI := syntax.ATURI("at://did:plc:alice/place.stream.media.track/" + rkey)
		track := placestream.MediaTrack{Track: placestream.MediaTrack_Track{MediaDefs_MuxlTrack: &placestream.MediaDefs_MuxlTrack{
			LexiconTypeID: "place.stream.media.defs#muxlTrack", Blob: cid, TrackId: "1", MediaType: "video",
		}}}
		refCID, err := spid.GetCID(&track)
		require.NoError(t, err)
		require.NoError(t, index.UpsertMediaTrack(ctx, track, trackURI))
		return cid, placestream.Video{Title: rkey, Source: placestream.Video_Source{
			MediaDefs_SourceTracks: &placestream.MediaDefs_SourceTracks{
				LexiconTypeID: "place.stream.media.defs#sourceTracks",
				Tracks:        []comatproto.RepoStrongRef{{Uri: trackURI.String(), Cid: refCID.String()}},
			},
		}}
	}
	first, oldVideo := stage("first")
	replacement, newVideo := stage("replacement")
	publish := func(cid, rev string) error {
		return advertisePublishedVideo(ctx, state, store, cid, comatproto.RepoPutRecord_Output{
			Uri: uri, Commit: &comatproto.RepoDefs_CommitMeta{Rev: rev},
		})
	}
	visible := func(cid string, want bool) {
		t.Helper()
		ok, err := StationVideoVisible(ctx, store, index, did, uri, cid)
		require.NoError(t, err)
		require.Equal(t, want, ok)
	}
	require.NoError(t, publish(first, firstRev))
	require.NoError(t, index.UpsertVideo(ctx, oldVideo, uri, firstRev))
	visible(first, true)
	require.NoError(t, publish(replacement, replaceRev))
	visible(first, false)       // the writer retired it despite the lagging AppView
	visible(replacement, false) // approval alone is not enough to serve it
	require.NoError(t, index.UpsertVideo(ctx, newVideo, uri, replaceRev))
	require.NoError(t, UpdateStationVideoSource(ctx, store, did, uri, first, staleUpdate))
	require.NoError(t, RetractStationRecord(ctx, store, did, uri, staleDelete, staleUpdate))
	require.NoError(t, index.UpsertVideo(ctx, oldVideo, uri, staleUpdate))
	require.NoError(t, index.DeleteVideo(ctx, uri, staleDelete))
	visible(replacement, true)

	require.NoError(t, RetractStationRecord(ctx, store, did, uri, deleteRev, replaceRev))
	visible(replacement, false) // even before the local delete is indexed
	require.ErrorIs(t, AdvertiseStationVideo(ctx, store, did, replacement, uri, ""), ErrStationOriginWithdrawn)
	require.ErrorIs(t, publish(replacement, replaceRev), ErrStationOriginWithdrawn)
	require.NoError(t, index.DeleteVideo(ctx, uri, deleteRev))
	require.NoError(t, index.UpsertVideo(ctx, newVideo, uri, replaceRev))
	video, err := index.GetVideoByURI(ctx, uri)
	require.NoError(t, err)
	require.Nil(t, video)

	require.NoError(t, index.UpsertVideo(ctx, newVideo, uri, readdRev))
	visible(replacement, false) // a creator re-add alone is not a station approval
	require.NoError(t, publish(replacement, readdRev))
	require.NoError(t, RetractStationRecord(ctx, store, did, uri, deleteRev, replaceRev))
	require.NoError(t, index.DeleteVideo(ctx, uri, deleteRev))
	visible(replacement, true)
	require.NoError(t, withdrawStationVideo(ctx, store, did, uri))
	require.ErrorIs(t, publish(replacement, "222222222222b"), ErrStationOriginWithdrawn)
	visible(replacement, false)
}
