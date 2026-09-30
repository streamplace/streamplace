package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

func mustURI(t *testing.T, s string) syntax.ATURI {
	t.Helper()
	u, err := syntax.ParseATURI(s)
	require.NoError(t, err)
	return u
}

// putHostedVideo writes a media.track backed by blobCID plus a sourceTracks
// video referencing it — the shape getVideoList resolves to a content blob.
func putHostedVideo(t *testing.T, m model.Model, videoURI, trackURI, blobCID string) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, m.UpsertMediaTrack(ctx, placestream.MediaTrack{
		LexiconTypeID: constants.PLACE_STREAM_MEDIA_TRACK,
		Track: placestream.MediaTrack_Track{
			MediaDefs_MuxlTrack: &placestream.MediaDefs_MuxlTrack{
				LexiconTypeID: "place.stream.media.defs#muxlTrack",
				Blob:          blobCID,
				TrackId:       "1",
				MediaType:     "video",
			},
		},
	}, mustURI(t, trackURI)))

	require.NoError(t, m.UpsertVideo(ctx, placestream.Video{
		LexiconTypeID: constants.PLACE_STREAM_VIDEO,
		Title:         videoURI,
		Source: placestream.Video_Source{
			MediaDefs_SourceTracks: &placestream.MediaDefs_SourceTracks{
				LexiconTypeID: "place.stream.media.defs#sourceTracks",
				Tracks: []comatproto.RepoStrongRef{
					{LexiconTypeID: "com.atproto.repo.strongRef", Uri: trackURI, Cid: "bafytrackcid"},
				},
			},
		},
	}, mustURI(t, videoURI)))
}

// TestReindexOriginsRepairsListing reproduces the production failure and its
// repair: the node has published media.origin records to its server repo and
// genuinely holds the blobs, but the firehose never round-tripped them into the
// local index (on a --secure node the self-subscription could not connect at
// all). The videos are therefore invisible to getVideoList despite being fully
// playable. Reindexing from the server repo — the authority — must restore them.
func TestReindexOriginsRepairsListing(t *testing.T) {
	ctx := context.Background()
	const serverHost = "server1.example.com"
	serverDID := "did:web:" + serverHost

	cli := config.CLI{
		BroadcasterHost: "example.com",
		ServerHost:      serverHost,
		DBURL:           ":memory:",
	}
	cli.DataDir = t.TempDir()

	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(ctx, &cli, nil, mod)
	require.NoError(t, err)

	handle, err := atproto.MakeServerRepo(ctx, &cli, state)
	require.NoError(t, err)
	defer func() { require.NoError(t, handle.Close()) }()
	t.Cleanup(func() {
		atproto.ServerRepo = nil
		atproto.ServerCarStore = nil
		atproto.ServerPubMultibase = ""
	})

	// More than one origin page, persisted without local hosting rows.
	// An unrelated collection must not interfere with origin reconciliation.
	updatedAt := "2026-07-25T00:00:00Z"
	require.NoError(t, atproto.CommitServerRepoRecord(ctx, &cli,
		constants.PLACE_STREAM_LIVE_VIEWERCOUNT, "did:plc:alice", &placestream.LiveViewerCount{
			LexiconTypeID: constants.PLACE_STREAM_LIVE_VIEWERCOUNT,
			Count:         1,
			Server:        serverDID,
			Streamer:      "did:plc:alice",
			UpdatedAt:     &updatedAt,
		}))
	blobs := make([]string, 103)
	wantVideos := make(map[string]bool, len(blobs))
	for i := range blobs {
		blob := fmt.Sprintf("blob%03d", i)
		blobs[i] = blob
		wantVideos[fmt.Sprintf("at://did:plc:alice/place.stream.video/v%d", i)] = true
		require.NoError(t, atproto.CommitServerRepoRecord(ctx, &cli,
			constants.PLACE_STREAM_MEDIA_ORIGIN, blob, &placestream.MediaOrigin{
				LexiconTypeID: constants.PLACE_STREAM_MEDIA_ORIGIN,
				Blob:          blob,
				Size:          int64(1000 + i),
				MimeType:      "video/mp4",
			}))
		putHostedVideo(t,
			mod,
			fmt.Sprintf("at://did:plc:alice/place.stream.video/v%d", i),
			fmt.Sprintf("at://did:plc:alice/place.stream.media.track/t%d", i),
			blob,
		)
	}
	require.NoError(t, handle.Close())
	handle, err = atproto.MakeServerRepo(ctx, &cli, state)
	require.NoError(t, err)

	// The symptom: nothing is listable, even though every blob is ours.
	before, err := mod.GetVideoList(ctx, "", 25, "", serverDID)
	require.NoError(t, err)
	require.Empty(t, before.Videos, "precondition: origins are unindexed, so nothing lists")

	// Unfiltered the videos are plainly there — they are hidden by the hosted
	// filter alone, which is exactly why they still play by direct link.
	unfiltered, err := mod.GetVideoList(ctx, "", 100, "", "")
	require.NoError(t, err)
	require.Len(t, unfiltered.Videos, 100)

	a := StreamplaceAPI{CLI: &cli, Model: mod}
	rr := httptest.NewRecorder()
	a.HandleReindexOrigins(ctx)(rr, httptest.NewRequest(http.MethodPost, "/reindex-origins", nil), httprouter.Params{})

	require.Equal(t, http.StatusOK, rr.Result().StatusCode)
	var res atproto.MediaOriginReindexResult
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
	require.Equal(t, serverDID, res.ServerDID)
	require.Equal(t, len(blobs), res.Scanned)
	require.Equal(t, len(blobs), res.Indexed)
	require.Empty(t, res.Errors)

	// The repair: every video the node hosts is listable again.
	after, err := mod.GetVideoList(ctx, "", 25, "", serverDID)
	require.NoError(t, err)
	gotVideos := make(map[string]bool, len(blobs))
	cursor := ""
	for {
		page, err := mod.GetVideoList(ctx, "", 100, cursor, serverDID)
		require.NoError(t, err)
		for _, video := range page.Videos {
			require.False(t, gotVideos[video.Uri], "duplicate listing entry")
			gotVideos[video.Uri] = true
		}
		if page.Cursor == nil {
			break
		}
		cursor = *page.Cursor
	}
	require.Equal(t, wantVideos, gotVideos)

	// Size/mimeType came from the record body, not invented from the rkey.
	origin, err := mod.GetMediaOriginByURI(ctx, fmt.Sprintf(
		"at://%s/%s/%s", serverDID, constants.PLACE_STREAM_MEDIA_ORIGIN, "blob001"))
	require.NoError(t, err)
	require.Equal(t, "blob001", origin.Blob)
	require.Equal(t, int64(1001), origin.Size)
	require.Equal(t, "video/mp4", origin.MimeType)

	// Idempotent: a second pass writes the same rows and changes nothing.
	rr2 := httptest.NewRecorder()
	a.HandleReindexOrigins(ctx)(rr2, httptest.NewRequest(http.MethodPost, "/reindex-origins", nil), httprouter.Params{})
	require.Equal(t, http.StatusOK, rr2.Result().StatusCode)
	var res2 atproto.MediaOriginReindexResult
	require.NoError(t, json.NewDecoder(rr2.Body).Decode(&res2))
	require.Equal(t, len(blobs), res2.Indexed)

	again, err := mod.GetVideoList(ctx, "", 25, "", serverDID)
	require.NoError(t, err)
	require.Equal(t, after.Videos, again.Videos)
}
