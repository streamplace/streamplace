package spxrpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/psession"
	"stream.place/streamplace/pkg/vod"
)

func TestVideoManifestPlayback(t *testing.T) {
	ctx := context.Background()
	m := newTestModel(t)
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	s := &Server{e: echo.New(), model: m, playbackStore: store, cli: &config.CLI{VODMoQ: true}, MoqURL: func() string { return "moqt://example.com:443?certhash=abcd" }}
	key := []byte("01234567890123456789012345678901")
	s.sessionKeyCache.Store(&key)
	parse := func(uri string) syntax.ATURI {
		parsed, err := syntax.ParseATURI(uri)
		require.NoError(t, err)
		return parsed
	}
	const trackURI = "at://did:plc:abc/place.stream.media.track/track"
	require.NoError(t, m.UpsertMediaTrack(ctx, placestream.MediaTrack{Track: placestream.MediaTrack_Track{MediaDefs_MuxlTrack: &placestream.MediaDefs_MuxlTrack{Blob: "bafyblob", TrackId: "1", MediaType: "video"}}}, parse(trackURI)))
	require.NoError(t, m.UpsertVideo(ctx, placestream.Video{Title: "test", Source: placestream.Video_Source{MediaDefs_SourceTracks: &placestream.MediaDefs_SourceTracks{Tracks: []comatproto.RepoStrongRef{{Uri: trackURI, Cid: "bafyref"}}}}}, parse(fixtureURI)))
	put := func(key string, data []byte) {
		writer, err := store.NewWriter(ctx, key, "application/octet-stream")
		require.NoError(t, err)
		_, err = writer.Write(data)
		require.NoError(t, err)
		require.NoError(t, writer.Complete())
	}
	meta := fixtureMetafile()
	meta.FlatHeaderSize = 42
	data, err := json.Marshal(meta)
	require.NoError(t, err)
	put(vod.BlobsPrefix+"bafyblob.json", data)
	put(vod.BlobsPrefix+"bafyvideoinit.mp4", []byte("video-init"))
	put(vod.BlobsPrefix+"bafyaudioinit.mp4", []byte("audio-init"))
	request := func(uri string) (*httptest.ResponseRecorder, echo.Context) {
		req := httptest.NewRequest(http.MethodGet, "/xrpc/place.stream.playback.getVideoManifest?uri="+url.QueryEscape(uri), nil)
		rec := httptest.NewRecorder()
		return rec, s.e.NewContext(req, rec)
	}
	rec, c := request(fixtureURI)
	require.NoError(t, s.HandleGetVideoManifest(c))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "public, max-age=60", rec.Header().Get("Cache-Control"))
	var manifest placestream.PlaybackGetVideoManifest_Output
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &manifest))
	require.NotEmpty(t, manifest.Sid)
	require.Equal(t, "dmlkZW8taW5pdA==", manifest.Tracks[0].Init)
	require.Equal(t, "moq", manifest.Sources[0].Type)
	served, err := s.ResolveServableVideo(ctx, fixtureDID, "rkey1", manifest.Sid)
	require.NoError(t, err)
	require.Equal(t, "bafyblob", served.BlobCID)
	require.Equal(t, []vod.ByteRange{{Offset: 142, Size: 2000}, {Offset: 2142, Size: 1800}}, served.Tracks["1"])
	_, err = s.ResolveServableVideo(ctx, fixtureDID, "rkey1", "forged")
	require.Error(t, err)
	_, wrongOwner := psession.Mint(key, "did:plc:other", psession.ScopePublic, time.Hour, time.Now())
	_, err = s.ResolveServableVideo(ctx, fixtureDID, "rkey1", wrongOwner)
	require.Error(t, err)

	const clipURI = "at://did:plc:abc/place.stream.video/clip"
	require.NoError(t, m.UpsertVideo(ctx, placestream.Video{Source: placestream.Video_Source{MediaDefs_SourceClip: &placestream.MediaDefs_SourceClip{Video: fixtureURI, Start: 1000, End: 2000}}}, parse(clipURI)))
	served, err = s.ResolveServableVideo(ctx, fixtureDID, "clip", manifest.Sid)
	require.NoError(t, err)
	require.Equal(t, "bafyblob", served.BlobCID)
	rec, c = request(clipURI)
	require.NoError(t, s.HandleGetVideoManifest(c))
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &manifest))
	require.Equal(t, int64(1), manifest.Tracks[0].First)
	require.Equal(t, []int64{2142}, manifest.Tracks[0].Offsets)
	boot, err := s.VideoBootPayload(httptest.NewRequest(http.MethodGet, "/video?t=", nil), clipURI, true)
	require.NoError(t, err)
	require.Nil(t, boot.Starter)

	putLabel(t, m, fixtureURI, atproto.LabelWarn)
	_, c = request(fixtureURI)
	require.Equal(t, http.StatusForbidden, requireHTTPError(t, s.HandleGetVideoManifest(c)).Code)
	boot, err = s.VideoBootPayload(httptest.NewRequest(http.MethodGet, "/video", nil), fixtureURI, false)
	require.Error(t, err)
	require.Nil(t, boot)
	_, err = s.ResolveServableVideo(ctx, fixtureDID, "rkey1", manifest.Sid)
	require.Error(t, err)
	putLabel(t, m, fixtureDID, atproto.LabelTakedown)
	_, c = request(clipURI)
	require.Equal(t, http.StatusForbidden, requireHTTPError(t, s.HandleGetVideoManifest(c)).Code)

	t.Run("clip of banned content owner", func(t *testing.T) {
		const clipperDID = "did:plc:other"
		const otherClipURI = "at://" + clipperDID + "/place.stream.video/clip"
		require.NoError(t, m.UpsertVideo(ctx, placestream.Video{Source: placestream.Video_Source{MediaDefs_SourceClip: &placestream.MediaDefs_SourceClip{Video: fixtureURI, Start: 0, End: 2000}}}, parse(otherClipURI)))
		banned, err := s.accountBanned(clipperDID)
		require.NoError(t, err)
		require.False(t, banned)
		_, sid := psession.Mint(key, clipperDID, psession.ScopePublic, time.Hour, time.Now())
		served, err := s.ResolveServableVideo(ctx, clipperDID, "clip", sid)
		require.Equal(t, http.StatusForbidden, requireHTTPError(t, err).Code)
		require.Nil(t, served)
		_, c := request(otherClipURI)
		require.Equal(t, http.StatusForbidden, requireHTTPError(t, s.HandleGetVideoManifest(c)).Code)
		boot, err := s.VideoBootPayload(httptest.NewRequest(http.MethodGet, "/video", nil), otherClipURI, false)
		require.Equal(t, http.StatusForbidden, requireHTTPError(t, err).Code)
		require.Nil(t, boot)
	})
}
