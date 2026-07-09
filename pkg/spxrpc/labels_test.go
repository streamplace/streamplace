package spxrpc

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/vod"
)

const (
	testLabeler    = "did:plc:labeler"
	testOwner      = "did:plc:owner"
	testOtherUser  = "did:plc:someoneelse"
	testContentCID = "bafkrcontentblob"
	testInitCID    = "bafkrinitsegment"
)

// newTestModel returns a fresh in-memory model for a single test.
func newTestModel(t *testing.T) model.Model {
	t.Helper()
	m, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	return m
}

// putLabel writes one active (non-expired, non-negated) label for uri.
func putLabel(t *testing.T, m model.Model, uri, val string) {
	t.Helper()
	lex := &comatproto.LabelDefs_Label{
		Cts: time.Now().UTC().Format(time.RFC3339),
		Src: testLabeler,
		Uri: uri,
		Val: val,
	}
	var buf bytes.Buffer
	require.NoError(t, lex.MarshalCBOR(&buf))
	require.NoError(t, m.CreateLabel(&model.Label{
		Src:     testLabeler,
		Uri:     uri,
		Val:     val,
		Cts:     time.Now().UTC(),
		Record:  buf.Bytes(),
		RepoDID: uri,
	}))
}

func TestAccountBanned(t *testing.T) {
	m := newTestModel(t)
	putLabel(t, m, "did:plc:banned", atproto.LabelTakedown)
	putLabel(t, m, "did:plc:warned", atproto.LabelWarn) // not a ban-worthy label
	s := &Server{model: m}

	for _, tc := range []struct {
		name string
		did  string
		want bool
	}{
		{"banned", "did:plc:banned", true},
		{"non-ban label", "did:plc:warned", false},
		{"unlabeled", "did:plc:clean", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.accountBanned(tc.did)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestRecordLabeled(t *testing.T) {
	m := newTestModel(t)
	labeled := "at://did:plc:owner/place.stream.video/abc"
	clean := "at://did:plc:owner/place.stream.video/def"
	// Any label value at all suppresses a record — even an advisory one.
	putLabel(t, m, labeled, atproto.LabelWarn)
	s := &Server{model: m}

	got, err := s.recordLabeled(labeled)
	require.NoError(t, err)
	require.True(t, got)

	got, err = s.recordLabeled(clean)
	require.NoError(t, err)
	require.False(t, got)
}

// writeBlob stashes a placeholder content blob so the serve path has
// something to return once the labeler gate lets a request through.
func writeBlob(t *testing.T, store blob.Store, cid string) {
	t.Helper()
	w, err := store.NewWriter(context.Background(), vod.BlobsPrefix+cid+".mp4", "video/mp4")
	require.NoError(t, err)
	_, err = w.Write([]byte("not really an mp4, but enough bytes to serve"))
	require.NoError(t, err)
	require.NoError(t, w.Complete())
}

// setupBlobTest builds a fresh server where testContentCID is a real
// content blob owned by testOwner, and testInitCID is an un-indexed
// blob (stands in for a per-track init segment). Returns the model too
// so callers can layer on labels.
func setupBlobTest(t *testing.T) (*Server, model.Model) {
	t.Helper()
	m := newTestModel(t)
	aturi, err := syntax.ParseATURI("at://" + testOwner + "/place.stream.media.track/1")
	require.NoError(t, err)
	require.NoError(t, m.UpsertMediaTrack(context.Background(), &streamplace.MediaTrack{
		LexiconTypeID: "place.stream.media.track",
		Track: &streamplace.MediaTrack_Track{
			MediaDefs_MuxlTrack: &streamplace.MediaDefs_MuxlTrack{
				LexiconTypeID: "place.stream.media.defs#muxlTrack",
				Blob:          testContentCID,
				TrackId:       "1",
				MediaType:     "video",
			},
		},
	}, aturi))
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	writeBlob(t, store, testContentCID)
	writeBlob(t, store, testInitCID)
	return &Server{model: m, playbackStore: store}, m
}

// blobReq builds an echo context for a getVideoBlob request.
func blobReq(did, cid string) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodGet,
		"/xrpc/place.stream.playback.getVideoBlob?did="+did+"&cid="+cid, nil)
	rec := httptest.NewRecorder()
	return echo.New().NewContext(req, rec), rec
}

func TestHandleGetVideoBlob_LabelerGating(t *testing.T) {
	t.Run("clip by another user (non-owner did) serves", func(t *testing.T) {
		// The blob is content-addressed; a clip references it with the
		// clipper's did, which doesn't own a track. It still serves.
		s, _ := setupBlobTest(t)
		c, rec := blobReq(testOtherUser, testContentCID)
		require.NoError(t, s.HandleGetVideoBlob(c))
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("banned content owner blocks even a non-owner did", func(t *testing.T) {
		// A clip can't bypass the original owner's ban.
		s, m := setupBlobTest(t)
		putLabel(t, m, testOwner, atproto.LabelTakedown)
		c, _ := blobReq(testOtherUser, testContentCID)
		he := requireHTTPError(t, s.HandleGetVideoBlob(c))
		require.Equal(t, http.StatusForbidden, he.Code)
	})

	t.Run("banned owner is forbidden", func(t *testing.T) {
		s, m := setupBlobTest(t)
		putLabel(t, m, testOwner, atproto.LabelTakedown)
		c, _ := blobReq(testOwner, testContentCID)
		he := requireHTTPError(t, s.HandleGetVideoBlob(c))
		require.Equal(t, http.StatusForbidden, he.Code)
	})

	t.Run("real owner, unbanned, serves", func(t *testing.T) {
		s, _ := setupBlobTest(t)
		c, rec := blobReq(testOwner, testContentCID)
		require.NoError(t, s.HandleGetVideoBlob(c))
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("init segment / unknown cid serves ungated", func(t *testing.T) {
		// testInitCID has no MediaTrack, so the gate doesn't apply — it
		// serves regardless of the did.
		s, _ := setupBlobTest(t)
		c, rec := blobReq(testOtherUser, testInitCID)
		require.NoError(t, s.HandleGetVideoBlob(c))
		require.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestHandleGetVideoPlaylist_LabelerGating(t *testing.T) {
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)

	playlistReq := func(uri string) echo.Context {
		req := httptest.NewRequest(http.MethodGet,
			"/xrpc/place.stream.playback.getVideoPlaylist?uri="+uri, nil)
		return echo.New().NewContext(req, httptest.NewRecorder())
	}

	t.Run("labeled video record is forbidden", func(t *testing.T) {
		m := newTestModel(t)
		uri := "at://" + testOwner + "/place.stream.video/labeled"
		putLabel(t, m, uri, atproto.LabelWarn)
		s := &Server{model: m, playbackStore: store}
		he := requireHTTPError(t, s.HandleGetVideoPlaylist(playlistReq(uri)))
		require.Equal(t, http.StatusForbidden, he.Code)
	})

	t.Run("banned owner account is forbidden", func(t *testing.T) {
		m := newTestModel(t)
		uri := "at://" + testOwner + "/place.stream.video/clean"
		putLabel(t, m, testOwner, atproto.LabelTakedown)
		s := &Server{model: m, playbackStore: store}
		he := requireHTTPError(t, s.HandleGetVideoPlaylist(playlistReq(uri)))
		require.Equal(t, http.StatusForbidden, he.Code)
	})
}

func requireHTTPError(t *testing.T, err error) *echo.HTTPError {
	t.Helper()
	require.Error(t, err)
	he, ok := err.(*echo.HTTPError)
	require.True(t, ok, "expected *echo.HTTPError, got %T", err)
	return he
}
