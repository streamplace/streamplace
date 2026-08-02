package spxrpc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car"
	carutil "github.com/ipld/go-car/util"
	"github.com/labstack/echo/v4"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/reposync"
	"stream.place/streamplace/pkg/statedb"
)

// newSyncTestNode stands up a real file-backed server repo and an echo router
// carrying the generated com.atproto.* routes, so the sync handlers are
// exercised through the same stub layer production uses (query parsing,
// status codes and all).
//
// serverHost == broadcasterHost puts the node in single-PDS mode, where
// isServerPDS is true no matter what Host header the request carries — which
// is what lets an httptest server (Host: 127.0.0.1:port) reach the server repo.
func newSyncTestNode(t *testing.T, serverHost, broadcasterHost string) (*config.CLI, *echo.Echo) {
	t.Helper()
	cli := &config.CLI{
		BroadcasterHost: broadcasterHost,
		ServerHost:      serverHost,
		DBURL:           ":memory:",
	}
	cli.DataDir = t.TempDir()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(context.Background(), cli, nil, mod)
	require.NoError(t, err)

	// The server repo lives in package-level globals; don't let a previous
	// test in this binary leak into this one.
	atproto.ServerRepo = nil
	atproto.ServerCarStore = nil
	atproto.ServerPubMultibase = ""

	handle, err := atproto.MakeServerRepo(context.Background(), cli, state)
	require.NoError(t, err)
	t.Cleanup(func() { _ = handle.Close() })

	s := &Server{cli: cli}
	e := echo.New()
	e.Use(s.ContextPreservingMiddleware())
	require.NoError(t, s.RegisterHandlersComatproto(e))
	return cli, e
}

// commitTestRecord writes one record at collection/rkey. The value is always a
// viewerCount because the MST doesn't care what the bytes say — only the key
// placement matters for these tests.
func commitTestRecord(t *testing.T, cli *config.CLI, collection, rkey string) {
	t.Helper()
	updatedAt := "2026-03-21T00:00:00Z"
	vc := placestream.LiveViewerCount{
		LexiconTypeID: constants.PLACE_STREAM_LIVE_VIEWERCOUNT,
		Count:         7,
		Server:        cli.ServerDID(),
		Streamer:      "did:plc:" + rkey,
		UpdatedAt:     &updatedAt,
	}
	require.NoError(t, atproto.CommitServerRepoRecord(context.Background(), cli, collection, rkey, &vc))
}

// syncGet issues a GET against e as if it arrived on host.
func syncGet(t *testing.T, e *echo.Echo, host, method string, q url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/xrpc/%s?%s", host, method, q.Encode()), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// readRootlessCAR parses a getBlocks response: a CARv1 whose header declares no
// roots. go-car's NewCarReader rejects that shape, so this mirrors what
// pkg/reposync does with raw car.ReadHeader.
func readRootlessCAR(t *testing.T, raw []byte) map[cid.Cid][]byte {
	t.Helper()
	br := bufio.NewReader(bytes.NewReader(raw))
	hdr, err := car.ReadHeader(br)
	require.NoError(t, err)
	require.EqualValues(t, 1, hdr.Version)
	require.Empty(t, hdr.Roots, "getBlocks must return a rootless CAR")
	out := map[cid.Cid][]byte{}
	for {
		c, data, err := carutil.ReadNode(br)
		if err == io.EOF {
			return out
		}
		require.NoError(t, err)
		out[c] = data
	}
}

func TestComAtprotoSyncGetLatestCommitAndGetBlocks(t *testing.T) {
	const host = "sync1.example.com"
	cli, e := newSyncTestNode(t, host, "broadcaster.example.com")
	did := atproto.ServerRepo.RepoDid()
	require.Equal(t, "did:web:"+host, did)

	commitTestRecord(t, cli, constants.PLACE_STREAM_LIVE_VIEWERCOUNT, "aaa")
	commitTestRecord(t, cli, constants.PLACE_STREAM_MEDIA_ORIGIN, "babczxv1")

	// getLatestCommit reports a decodable commit CID plus its rev.
	rec := syncGet(t, e, host, "com.atproto.sync.getLatestCommit", url.Values{"did": {did}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var latest comatproto.SyncGetLatestCommit_Output
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &latest))
	commitCID, err := cid.Decode(latest.Cid)
	require.NoError(t, err)
	require.NotEmpty(t, latest.Rev)

	// ...and it agrees with the repo's own view of head.
	wantCID, wantRev, err := atproto.ServerRepoLatestCommit(context.Background())
	require.NoError(t, err)
	require.Equal(t, wantCID.String(), latest.Cid)
	require.Equal(t, wantRev, latest.Rev)

	// A repo we don't host is a 404, same as getRepo.
	rec = syncGet(t, e, host, "com.atproto.sync.getLatestCommit", url.Values{"did": {"did:web:somewhere.else"}})
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "RepoNotFound")

	// getBlocks hands back exactly the requested block, and the bytes hash
	// to the CID getLatestCommit just advertised.
	rec = syncGet(t, e, host, "com.atproto.sync.getBlocks", url.Values{
		"did":  {did},
		"cids": {latest.Cid},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	blocks := readRootlessCAR(t, rec.Body.Bytes())
	require.Len(t, blocks, 1)
	require.Contains(t, blocks, commitCID)
	require.NoError(t, reposync.VerifyBlock(commitCID, blocks[commitCID]))

	// Several CIDs in one call, deduped.
	rec = syncGet(t, e, host, "com.atproto.sync.getBlocks", url.Values{
		"did":  {did},
		"cids": {latest.Cid, latest.Cid},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, readRootlessCAR(t, rec.Body.Bytes()), 1)

	// A CID we don't have fails the whole request rather than quietly
	// returning a short bag of blocks.
	missing, err := cid.Prefix{
		Version:  1,
		Codec:    cid.DagCBOR,
		MhType:   multihash.SHA2_256,
		MhLength: 32,
	}.Sum([]byte("definitely not in this repo"))
	require.NoError(t, err)
	rec = syncGet(t, e, host, "com.atproto.sync.getBlocks", url.Values{
		"did":  {did},
		"cids": {latest.Cid, missing.String()},
	})
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "BlockNotFound")

	// Garbage CID -> InvalidRequest, not a 500.
	rec = syncGet(t, e, host, "com.atproto.sync.getBlocks", url.Values{
		"did":  {did},
		"cids": {"not-a-cid"},
	})
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "InvalidRequest")

	// Over the per-request cap -> InvalidRequest.
	tooMany := make([]string, maxGetBlocksCIDs+1)
	for i := range tooMany {
		tooMany[i] = latest.Cid
	}
	rec = syncGet(t, e, host, "com.atproto.sync.getBlocks", url.Values{
		"did":  {did},
		"cids": tooMany,
	})
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "InvalidRequest")

	// Wrong repo -> RepoNotFound.
	rec = syncGet(t, e, host, "com.atproto.sync.getBlocks", url.Values{
		"did":  {"did:web:somewhere.else"},
		"cids": {latest.Cid},
	})
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "RepoNotFound")
}

// TestComAtprotoSyncReposyncRoundTrip is the payoff: the phase 1-3 client
// (verified head + prefix-bounded MST walk) driven over HTTP against our own
// handlers. If this passes, a streamplace node is walkable by any peer running
// the same code path — no full-CAR fallback.
func TestComAtprotoSyncReposyncRoundTrip(t *testing.T) {
	ctx := context.Background()
	const host = "node.example.com"
	cli, e := newSyncTestNode(t, host, host)
	did := atproto.ServerRepo.RepoDid()

	inRange := [][2]string{
		{constants.PLACE_STREAM_LIVE_VIEWERCOUNT, "aaa"},
		{constants.PLACE_STREAM_LIVE_VIEWERCOUNT, "bbb"},
		{constants.PLACE_STREAM_MEDIA_ORIGIN, "babczxv1"},
		{constants.PLACE_STREAM_MEDIA_ORIGIN, "babczxv2"},
	}
	// Keys immediately outside ["place.stream.", "place.stream/") on both
	// sides, so a walker that ignored the range bound would be caught.
	outOfRange := [][2]string{
		{"app.bsky.feed.post", "3abcxyz"},
		{"place.streamx.thing", "1"},
		{"xyz.example.thing", "1"},
	}
	for _, rec := range append(append([][2]string{}, inRange...), outOfRange...) {
		commitTestRecord(t, cli, rec[0], rec[1])
	}

	want := map[string]string{}
	for _, r := range inRange {
		out, err := atproto.ServerRepoGetRecord(ctx, did, r[0], r[1])
		require.NoError(t, err)
		require.NotNil(t, out.Cid)
		want[r[0]+"/"+r[1]] = *out.Cid
	}

	ts := httptest.NewServer(e)
	defer ts.Close()

	client := &xrpc.Client{Host: ts.URL}
	fetcher := &reposync.XRPCBlockFetcher{Client: client, DID: did}

	// The node signs its commits with its server-repo key; a verifying peer
	// gets that key from the did:web document. Mock the directory with the
	// very same multibase the DID doc publishes.
	dir := identity.NewMockDirectory()
	dir.Insert(identity.Identity{
		DID:    syntax.DID(did),
		Handle: syntax.HandleInvalid,
		Keys: map[string]identity.VerificationMethod{
			"atproto": {Type: "Multikey", PublicKeyMultibase: atproto.ServerPubMultibase},
		},
	})

	head, err := reposync.FetchVerifiedHead(ctx, client, fetcher, &dir, did)
	require.NoError(t, err)
	wantCID, wantRev, err := atproto.ServerRepoLatestCommit(ctx)
	require.NoError(t, err)
	require.Equal(t, wantCID, head.CID)
	require.Equal(t, wantRev, head.Rev)
	require.Equal(t, did, head.Commit.DID)

	// Signature verification is load-bearing, not decorative: point the
	// directory at somebody else's key and the head must be rejected.
	otherPriv, err := atcrypto.GeneratePrivateKeyK256()
	require.NoError(t, err)
	otherPub, err := otherPriv.PublicKey()
	require.NoError(t, err)
	wrongDir := identity.NewMockDirectory()
	wrongDir.Insert(identity.Identity{
		DID:    syntax.DID(did),
		Handle: syntax.HandleInvalid,
		Keys: map[string]identity.VerificationMethod{
			"atproto": {Type: "Multikey", PublicKeyMultibase: otherPub.Multibase()},
		},
	})
	_, err = reposync.FetchVerifiedHead(ctx, client, fetcher, &wrongDir, did)
	require.Error(t, err)
	require.Contains(t, err.Error(), "signature")

	got := map[string]string{}
	walker := &reposync.Walker{Fetcher: fetcher}
	require.NoError(t, walker.WalkPrefix(ctx, head.Root, "place.stream.", func(path string, rcid cid.Cid, rec []byte) error {
		require.NoError(t, reposync.VerifyBlock(rcid, rec))
		got[path] = rcid.String()
		return nil
	}))
	require.Equal(t, want, got)

	// A walk that starts from a verified head must also see writes that
	// happen afterwards, once the head is re-fetched.
	commitTestRecord(t, cli, constants.PLACE_STREAM_MEDIA_ORIGIN, "babczxv3")
	head2, err := reposync.FetchVerifiedHead(ctx, client, fetcher, &dir, did)
	require.NoError(t, err)
	require.NotEqual(t, head.CID, head2.CID)
	require.Greater(t, head2.Rev, head.Rev)

	got2 := map[string]string{}
	require.NoError(t, walker.WalkPrefix(ctx, head2.Root, "place.stream.", func(path string, rcid cid.Cid, rec []byte) error {
		got2[path] = rcid.String()
		return nil
	}))
	require.Len(t, got2, len(want)+1)
	require.Contains(t, got2, constants.PLACE_STREAM_MEDIA_ORIGIN+"/babczxv3")
}
