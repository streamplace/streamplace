package reposync

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/repo"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car"
	carutil "github.com/ipld/go-car/util"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
)

const testDID = "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"

// signedRepo is a synthetic repo plus a signed commit over its MST root.
type signedRepo struct {
	*testRepo
	priv      atcrypto.PrivateKey
	commit    *repo.Commit
	commitCID cid.Cid
}

func buildSignedRepo(t *testing.T, did string, paths []string) *signedRepo {
	t.Helper()
	tr := buildRepo(t, paths)
	priv, err := atcrypto.GeneratePrivateKeyP256()
	require.NoError(t, err)
	sr := &signedRepo{testRepo: tr, priv: priv}
	sr.commit, sr.commitCID = sr.signCommit(t, did, tr.root, syntax.NewTIDNow(0).String(), priv)
	return sr
}

// signCommit builds, signs and stores a commit block, returning it and its CID.
func (sr *signedRepo) signCommit(t *testing.T, did string, root cid.Cid, rev string, priv atcrypto.PrivateKey) (*repo.Commit, cid.Cid) {
	t.Helper()
	c := &repo.Commit{
		DID:     did,
		Version: 3,
		Data:    root,
		Rev:     rev,
	}
	require.NoError(t, c.Sign(priv))
	buf := new(bytes.Buffer)
	require.NoError(t, c.MarshalCBOR(buf))
	data := buf.Bytes()
	cc, err := cid.NewPrefixV1(cid.DagCBOR, multihash.SHA2_256).Sum(data)
	require.NoError(t, err)
	sr.blocks[cc] = data
	return c, cc
}

func (sr *signedRepo) directory(t *testing.T, did string, pub atcrypto.PublicKey) identity.Directory {
	t.Helper()
	dir := identity.NewMockDirectory()
	dir.Insert(identity.Identity{
		DID:    syntax.DID(did),
		Handle: syntax.HandleInvalid,
		Keys: map[string]identity.VerificationMethod{
			"atproto": {Type: "Multikey", PublicKeyMultibase: pub.Multibase()},
		},
	})
	return &dir
}

// ---------------------------------------------------------------------------
// a minimal com.atproto.sync.* host
// ---------------------------------------------------------------------------

type fakeHost struct {
	blocks map[cid.Cid][]byte
	head   cid.Cid
	rev    string
	// omit is dropped from getBlocks responses.
	omit map[cid.Cid]bool
	// tamper is served with wrong bytes.
	tamper map[cid.Cid]bool
	// requests counts getBlocks calls.
	requests int
}

func newFakeHost(sr *signedRepo) *fakeHost {
	return &fakeHost{
		blocks: sr.blocks,
		head:   sr.commitCID,
		rev:    sr.commit.Rev,
		omit:   map[cid.Cid]bool{},
		tamper: map[cid.Cid]bool{},
	}
}

func (h *fakeHost) start(t *testing.T) *xrpc.Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/xrpc/com.atproto.sync.getLatestCommit", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"cid": h.head.String(), "rev": h.rev})
	})
	mux.HandleFunc("/xrpc/com.atproto.sync.getBlocks", func(w http.ResponseWriter, r *http.Request) {
		h.requests++
		buf := new(bytes.Buffer)
		// Real getBlocks responses carry an empty roots list.
		if err := car.WriteHeader(&car.CarHeader{Roots: nil, Version: 1}, buf); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		for _, s := range r.URL.Query()["cids"] {
			c, err := cid.Decode(s)
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			if h.omit[c] {
				continue
			}
			data, ok := h.blocks[c]
			if !ok {
				continue
			}
			if h.tamper[c] {
				data = append([]byte("tampered:"), data...)
			}
			if err := carutil.LdWrite(buf, c.Bytes(), data); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		w.Header().Set("Content-Type", "application/vnd.ipld.car")
		_, _ = w.Write(buf.Bytes())
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &xrpc.Client{Host: srv.URL, Client: srv.Client()}
}

// ---------------------------------------------------------------------------
// tests
// ---------------------------------------------------------------------------

// Case 8: head verification accepts a correctly signed commit and rejects the
// obvious forgeries.
func TestFetchVerifiedHead(t *testing.T) {
	ctx := context.Background()
	sr := buildSignedRepo(t, testDID, exactnessPaths())
	pub, err := sr.priv.PublicKey()
	require.NoError(t, err)

	t.Run("valid", func(t *testing.T) {
		host := newFakeHost(sr)
		client := host.start(t)
		f := &XRPCBlockFetcher{Client: client, DID: testDID}
		head, err := FetchVerifiedHead(ctx, client, f, sr.directory(t, testDID, pub), testDID)
		require.NoError(t, err)
		require.Equal(t, sr.commitCID, head.CID)
		require.Equal(t, sr.root, head.Root)
		require.Equal(t, sr.commit.Rev, head.Rev)
		require.Equal(t, testDID, head.Commit.DID)
	})

	t.Run("wrong signing key", func(t *testing.T) {
		other, err := atcrypto.GeneratePrivateKeyP256()
		require.NoError(t, err)
		otherPub, err := other.PublicKey()
		require.NoError(t, err)

		host := newFakeHost(sr)
		client := host.start(t)
		f := &XRPCBlockFetcher{Client: client, DID: testDID}
		_, err = FetchVerifiedHead(ctx, client, f, sr.directory(t, testDID, otherPub), testDID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "signature")
	})

	t.Run("commit signed by an impostor", func(t *testing.T) {
		// The host serves a commit over a repo root it made up, signed with a key
		// that is not the account's.
		impostor, err := atcrypto.GeneratePrivateKeyP256()
		require.NoError(t, err)
		forged := buildSignedRepo(t, testDID, exactnessPaths()[:3])
		_, forgedCID := forged.signCommit(t, testDID, forged.root, syntax.NewTIDNow(0).String(), impostor)

		host := newFakeHost(forged)
		host.head = forgedCID
		client := host.start(t)
		f := &XRPCBlockFetcher{Client: client, DID: testDID}
		_, err = FetchVerifiedHead(ctx, client, f, sr.directory(t, testDID, pub), testDID)
		require.Error(t, err)
	})

	t.Run("tampered commit block", func(t *testing.T) {
		host := newFakeHost(sr)
		host.tamper[sr.commitCID] = true
		client := host.start(t)
		f := &XRPCBlockFetcher{Client: client, DID: testDID}
		_, err := FetchVerifiedHead(ctx, client, f, sr.directory(t, testDID, pub), testDID)
		require.ErrorIs(t, err, ErrBlockMismatch)
	})

	t.Run("missing commit block", func(t *testing.T) {
		host := newFakeHost(sr)
		host.omit[sr.commitCID] = true
		client := host.start(t)
		f := &XRPCBlockFetcher{Client: client, DID: testDID}
		_, err := FetchVerifiedHead(ctx, client, f, sr.directory(t, testDID, pub), testDID)
		require.ErrorIs(t, err, ErrMissingBlock)
	})

	t.Run("did mismatch", func(t *testing.T) {
		otherDID := "did:plc:bbbbbbbbbbbbbbbbbbbbbbbb"
		host := newFakeHost(sr)
		client := host.start(t)
		f := &XRPCBlockFetcher{Client: client, DID: otherDID}
		_, err := FetchVerifiedHead(ctx, client, f, sr.directory(t, otherDID, pub), otherDID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "is for repo")
	})

	t.Run("rev mismatch", func(t *testing.T) {
		host := newFakeHost(sr)
		host.rev = syntax.NewTIDNow(1).String()
		client := host.start(t)
		f := &XRPCBlockFetcher{Client: client, DID: testDID}
		_, err := FetchVerifiedHead(ctx, client, f, sr.directory(t, testDID, pub), testDID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "host reported")
	})

	t.Run("unknown did", func(t *testing.T) {
		host := newFakeHost(sr)
		client := host.start(t)
		f := &XRPCBlockFetcher{Client: client, DID: testDID}
		empty := identity.NewMockDirectory()
		_, err := FetchVerifiedHead(ctx, client, f, &empty, testDID)
		require.ErrorIs(t, err, identity.ErrDIDNotFound)
	})
}

// The real getBlocks/CAR path: chunking, verification and missing-block
// detection against an HTTP host.
func TestXRPCBlockFetcher(t *testing.T) {
	ctx := context.Background()
	paths := exactnessPaths()
	sr := buildSignedRepo(t, testDID, paths)
	pub, err := sr.priv.PublicKey()
	require.NoError(t, err)

	t.Run("walk over http", func(t *testing.T) {
		host := newFakeHost(sr)
		client := host.start(t)
		f := &XRPCBlockFetcher{Client: client, DID: testDID, ChunkSize: 2}
		head, err := FetchVerifiedHead(ctx, client, f, sr.directory(t, testDID, pub), testDID)
		require.NoError(t, err)

		var got []emission
		require.NoError(t, (&Walker{Fetcher: f, BatchSize: 3}).
			WalkPrefix(ctx, head.Root, "place.stream.", collectVisitor(&got)))
		require.Equal(t, expectedInRange(paths, "place.stream."), emittedPaths(got))
		for _, e := range got {
			require.Equal(t, recordBytes(e.path), e.data)
		}
		require.Greater(t, host.requests, 1, "ChunkSize 2 should force several requests")
	})

	t.Run("unknown cid is a missing block", func(t *testing.T) {
		host := newFakeHost(sr)
		client := host.start(t)
		f := &XRPCBlockFetcher{Client: client, DID: testDID}
		bogus, err := cid.NewPrefixV1(cid.DagCBOR, multihash.SHA2_256).Sum([]byte("not in this repo"))
		require.NoError(t, err)
		_, err = f.GetBlocks(ctx, []cid.Cid{bogus})
		require.ErrorIs(t, err, ErrMissingBlock)
	})

	t.Run("tampered block is rejected", func(t *testing.T) {
		host := newFakeHost(sr)
		host.tamper[sr.root] = true
		client := host.start(t)
		f := &XRPCBlockFetcher{Client: client, DID: testDID}
		_, err := f.GetBlocks(ctx, []cid.Cid{sr.root})
		// The CAR framing carries the CID the host claims; tampered bytes either
		// arrive under a different CID (missing) or fail verification.
		require.Error(t, err)
	})
}

func TestVerifyBlock(t *testing.T) {
	data := recordBytes("place.stream.chat.profile/self")
	c, err := cid.NewPrefixV1(cid.DagCBOR, multihash.SHA2_256).Sum(data)
	require.NoError(t, err)
	require.NoError(t, VerifyBlock(c, data))
	require.ErrorIs(t, VerifyBlock(c, append(data, 'x')), ErrBlockMismatch)

	// An identity-multihash CID would "verify" trivially; refuse it.
	idCID, err := cid.NewPrefixV1(cid.DagCBOR, multihash.IDENTITY).Sum(data)
	require.NoError(t, err)
	require.ErrorIs(t, VerifyBlock(idCID, data), ErrBlockMismatch)
}
