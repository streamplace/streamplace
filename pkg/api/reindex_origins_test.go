package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/vod"
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
	cli, mod := newOriginRecoveryTestNode(t)
	serverDID := cli.ServerDID()

	// Three VODs this node hosts: origin committed to the server repo (as
	// PublishOrigin does), video+track indexed (as the user's own firehose
	// events did), but no media_origins row — the dropped half.
	blobs := []string{"blobAAA", "blobBBB", "blobCCC"}
	for i, blob := range blobs {
		require.NoError(t, atproto.CommitServerRepoRecord(ctx, cli,
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

	// The symptom: nothing is listable, even though every blob is ours.
	before, err := mod.GetVideoList(ctx, "", 25, "", serverDID)
	require.NoError(t, err)
	require.Empty(t, before.Videos, "precondition: origins are unindexed, so nothing lists")

	// Unfiltered the videos are plainly there — they are hidden by the hosted
	// filter alone, which is exactly why they still play by direct link.
	unfiltered, err := mod.GetVideoList(ctx, "", 25, "", "")
	require.NoError(t, err)
	require.Len(t, unfiltered.Videos, len(blobs))

	a := StreamplaceAPI{CLI: cli, Model: mod}
	rr := httptest.NewRecorder()
	a.HandleReindexOrigins(ctx)(rr, httptest.NewRequest(http.MethodPost, "/reindex-origins", nil), httprouter.Params{})

	require.Equal(t, http.StatusOK, rr.Result().StatusCode)
	var res reindexOriginsResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
	require.Equal(t, serverDID, res.ServerDID)
	require.Equal(t, len(blobs), res.Scanned)
	require.Equal(t, len(blobs), res.Indexed)
	require.Empty(t, res.Errors)

	// The repair: every video the node hosts is listable again.
	after, err := mod.GetVideoList(ctx, "", 25, "", serverDID)
	require.NoError(t, err)
	require.Len(t, after.Videos, len(blobs))

	// Size/mimeType came from the record body, not invented from the rkey.
	origin, err := mod.GetMediaOriginByURI(ctx, fmt.Sprintf(
		"at://%s/%s/%s", serverDID, constants.PLACE_STREAM_MEDIA_ORIGIN, "blobBBB"))
	require.NoError(t, err)
	require.Equal(t, "blobBBB", origin.Blob)
	require.Equal(t, int64(1001), origin.Size)
	require.Equal(t, "video/mp4", origin.MimeType)

	// Idempotent: a second pass writes the same rows and changes nothing.
	rr2 := httptest.NewRecorder()
	a.HandleReindexOrigins(ctx)(rr2, httptest.NewRequest(http.MethodPost, "/reindex-origins", nil), httprouter.Params{})
	require.Equal(t, http.StatusOK, rr2.Result().StatusCode)
	var res2 reindexOriginsResponse
	require.NoError(t, json.NewDecoder(rr2.Body).Decode(&res2))
	require.Equal(t, len(blobs), res2.Indexed)

	again, err := mod.GetVideoList(ctx, "", 25, "", serverDID)
	require.NoError(t, err)
	require.Len(t, again.Videos, len(blobs))
}

func newOriginRecoveryTestNode(t *testing.T) (*config.CLI, model.Model) {
	t.Helper()
	ctx := context.Background()
	cli := &config.CLI{
		BroadcasterHost: "example.com",
		ServerHost:      "server1.example.com",
		DBURL:           ":memory:",
		DataDir:         t.TempDir(),
	}
	mod, err := model.MakeDB(filepath.Join(cli.DataDir, "index"))
	require.NoError(t, err)
	db, err := mod.(*model.DBModel).DB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	state, err := statedb.MakeDB(ctx, cli, nil, mod)
	require.NoError(t, err)
	handle, err := atproto.MakeServerRepo(ctx, cli, state)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, handle.Close())
		atproto.ServerRepo = nil
		atproto.ServerCarStore = nil
		atproto.ServerPubMultibase = ""
	})
	return cli, mod
}

func TestReindexOriginsRecoversStoredVODs(t *testing.T) {
	ctx := context.Background()
	cli, mod := newOriginRecoveryTestNode(t)
	serverDID := cli.ServerDID()
	const videos = "at://did:plc:alice/place.stream.video/"
	const tracks = "at://did:plc:alice/place.stream.media.track/"
	oldCID := bdasl.CID([]byte("old video"))
	existingCID := bdasl.CID([]byte("already published"))
	incompleteCID := bdasl.CID([]byte("missing metafile"))
	orphanCID := bdasl.CID([]byte("track with no video"))
	putHostedVideo(t, mod, videos+"existing", tracks+"existing", existingCID)
	putHostedVideo(t, mod, videos+"incomplete", tracks+"incomplete", incompleteCID)
	putHostedVideo(t, mod, videos+"remote", tracks+"remote", bdasl.CID([]byte("another node's video")))
	putHostedVideo(t, mod, videos+"z-old", tracks+"old", oldCID)
	require.NoError(t, vod.PublishOrigin(ctx, cli, existingCID, 123, "video/mp4"))

	remote, err := mod.GetVideoByURI(ctx, videos+"remote")
	require.NoError(t, err)
	for i := range 100 {
		// Force the recoverable VOD onto a later page of the SQLite scan.
		require.NoError(t, mod.UpsertVideo(ctx, *remote, mustURI(t, fmt.Sprintf("%sremote-%03d", videos, i))))
	}
	orphan, err := mod.GetMediaTrackByURI(ctx, tracks+"old")
	require.NoError(t, err)
	orphan.Track.MediaDefs_MuxlTrack.Blob = orphanCID
	require.NoError(t, mod.UpsertMediaTrack(ctx, *orphan, mustURI(t, tracks+"orphan")))
	require.NoError(t, mod.UpsertVideo(ctx, placestream.Video{
		LexiconTypeID: constants.PLACE_STREAM_VIDEO,
		Title:         "A clip from the old video",
		Source: placestream.Video_Source{
			MediaDefs_SourceClip: &placestream.MediaDefs_SourceClip{
				LexiconTypeID: "place.stream.media.defs#sourceClip",
				Video:         videos + "z-old",
				Start:         100,
				End:           200,
			},
		},
	}, mustURI(t, videos+"zz-clip")))

	const bucket = "vods"
	const size = 4_200_000_000
	mp4Key := vod.BlobsPrefix + oldCID + ".mp4"
	s3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/"+bucket && r.URL.Query().Get("prefix") == vod.BlobsPrefix:
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, "<ListBucketResult>")
			if r.URL.Query().Get("continuation-token") == "" {
				fmt.Fprint(w, "<IsTruncated>true</IsTruncated><NextContinuationToken>rest</NextContinuationToken>")
				for _, cid := range []string{oldCID, existingCID, incompleteCID, orphanCID} {
					fmt.Fprintf(w, "<Contents><Key>%s%s.mp4</Key></Contents>", vod.BlobsPrefix, cid)
				}
			} else {
				fmt.Fprint(w, "<IsTruncated>false</IsTruncated>")
				for _, cid := range []string{oldCID, existingCID, orphanCID} {
					fmt.Fprintf(w, "<Contents><Key>%s%s.json</Key></Contents>", vod.BlobsPrefix, cid)
				}
			}
			fmt.Fprint(w, "</ListBucketResult>")
		case r.Method == http.MethodHead && r.URL.Path == "/"+bucket+"/"+mp4Key:
			w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		default:
			http.Error(w, "unexpected object request", http.StatusBadRequest)
		}
	}))
	t.Cleanup(s3.Close)
	client := awss3.New(awss3.Options{
		BaseEndpoint: aws.String(s3.URL),
		Region:       "us-east-1",
		Credentials:  credentials.NewStaticCredentialsProvider("test", "test", ""),
		UsePathStyle: true,
	})
	a := StreamplaceAPI{CLI: cli, Model: mod, PlaybackStore: blob.NewS3Store(client, bucket)}
	admin, err := a.InternalHandler(ctx)
	require.NoError(t, err)
	reindex := func(query string) reindexOriginsResponse {
		t.Helper()
		rr := httptest.NewRecorder()
		admin.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/reindex-origins"+query, nil))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var res reindexOriginsResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&res))
		return res
	}

	require.Zero(t, reindex("").Recovered, "ordinary replay must not publish missing origins")
	res := reindex("?recover=true")
	require.Equal(t, 1, res.Recovered)
	require.Equal(t, []string{incompleteCID + ": missing playback metafile"}, res.Errors)

	page, err := atproto.ServerRepoListRecords(ctx, constants.PLACE_STREAM_MEDIA_ORIGIN, "", 25, serverDID, nil)
	require.NoError(t, err)
	origins := map[string]int64{}
	for _, record := range page.Records {
		origin := record.Value.Val.(*placestream.MediaOrigin)
		origins[origin.Blob] = origin.Size
	}
	require.Equal(t, map[string]int64{oldCID: size, existingCID: 123}, origins)
	list, err := mod.GetVideoList(ctx, "", 25, "", serverDID)
	require.NoError(t, err)
	listed := make([]string, 0, len(list.Videos))
	for _, video := range list.Videos {
		listed = append(listed, video.Uri)
	}
	require.ElementsMatch(t, []string{videos + "existing", videos + "z-old", videos + "zz-clip"}, listed)

	require.Zero(t, reindex("?recover=true").Recovered)
	commits, err := atproto.GetServerCommitEventsSinceSeq(serverDID, 0)
	require.NoError(t, err)
	require.Len(t, commits, 2, "no duplicate commits for the clip or on rerun")
}
