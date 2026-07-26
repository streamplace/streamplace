package atproto

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/repo"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/ipfs/go-cid"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/reposync"
)

// placeStreamPrefix is the collection prefix shared by every streamplace
// lexicon. Because MST keys are "collection/rkey" sorted bytewise, all of those
// records live in one contiguous key range.
const placeStreamPrefix = "place.stream."

// backfillRanges is the set of MST key ranges a backfill walks: everything
// under place.stream., plus one range per non-streamplace collection the
// firehose accepts.
//
// It is derived from CollectionFilter at runtime so the backfill and the
// firehose can never drift apart about which records this node indexes.
func backfillRanges() []reposync.KeyRange {
	ranges := []reposync.KeyRange{reposync.PrefixRange(placeStreamPrefix)}
	for _, nsid := range CollectionFilter {
		if strings.HasPrefix(nsid, placeStreamPrefix) {
			continue
		}
		// The trailing slash keeps "app.bsky.feed.post" from also matching
		// "app.bsky.feed.postgate".
		ranges = append(ranges, reposync.PrefixRange(nsid+"/"))
	}
	return ranges
}

// backfillRepo indexes every record we care about from ident's repo. It returns
// the repo revision the index is now consistent with, and the MST root CID that
// revision committed to (empty if the fallback path was used, which never sees
// a verified root).
//
// The fast path walks only the subtrees holding records we index. Hosts that do
// not implement com.atproto.sync.getBlocks fall back to downloading the whole
// repo as a CAR.
func (atsync *ATProtoSynchronizer) backfillRepo(ctx context.Context, ident *identity.Identity, xrpcc *xrpc.Client) (string, string, error) {
	rev, root, err := atsync.walkBackfill(ctx, ident, xrpcc)
	if err == nil {
		return rev, root, nil
	}
	if !isMethodNotSupported(err) {
		// Anything else -- a bad signature, a malformed tree, a network
		// failure -- must propagate. Falling back on a verification failure
		// would make the verification decorative.
		return "", "", err
	}
	log.Warn(ctx, "host does not support sync.getBlocks, falling back to full getRepo",
		"pds", xrpcc.Host, "did", ident.DID.String(), "err", err)
	rev, err = atsync.legacyBackfill(ctx, ident, xrpcc)
	if err != nil {
		return "", "", err
	}
	return rev, "", nil
}

// walkBackfill does a verified, prefix-bounded walk of the remote repo, handing
// every record in range to the same indexing path the firehose uses.
func (atsync *ATProtoSynchronizer) walkBackfill(ctx context.Context, ident *identity.Identity, xrpcc *xrpc.Client) (string, string, error) {
	did := ident.DID.String()

	dir := atsync.PLCDirectory
	if dir == nil {
		// resolveIdent initializes this lazily, and every caller goes through
		// it first; be defensive rather than nil-panic. Note this is the
		// *uncached* directory on purpose: a signing key cached from before a
		// rotation would fail commit verification, and backfills are rare
		// enough that the extra lookup does not matter.
		dir = CustomDirectory(atsync.CLI.PLCURL)
	}

	fetcher := &reposync.CachedFetcher{
		// Bounded lifetime: one cache per backfill, so the head fetch and the
		// walk share blocks without holding a repo in memory afterwards.
		Cache: reposync.NewMemoryBlockCache(),
		Inner: &pdsLockedFetcher{
			lock:  pdsLocks.GetLock(ident.PDSEndpoint()),
			inner: &reposync.XRPCBlockFetcher{Client: xrpcc, DID: did},
		},
	}

	head, err := reposync.FetchVerifiedHead(ctx, xrpcc, fetcher, dir, did)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch verified head for %s from PDS %s: %w", did, xrpcc.Host, err)
	}

	walker := &reposync.Walker{Fetcher: fetcher}
	records := 0
	err = walker.WalkRanges(ctx, head.Root, backfillRanges(), func(path string, rcid cid.Cid, rec []byte) error {
		nsid, rkey, err := syntax.ParseRepoPath(path)
		if err != nil {
			log.Warn(ctx, "failed to parse repo path", "k", path, "err", err)
			return fmt.Errorf("could not parse repo path %s: %w", path, err)
		}
		log.Debug(ctx, "record type", "key", path, "type", nsid.String())

		bs := rec
		err = atsync.handleCreateUpdate(ctx, did, rkey, &bs, rcid.String(), nsid, false, true)
		if err != nil {
			log.Warn(ctx, "failed to handle create update", "err", err)
			// invalid CBOR and stuff should get ignored, so we don't return
		}
		records++
		return nil
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to walk repo for %s from PDS %s: %w", did, xrpcc.Host, err)
	}

	log.Log(ctx, "walked repo", "did", did, "rev", head.Rev, "root", head.Root.String(), "records", records)
	return head.Rev, head.Root.String(), nil
}

// legacyBackfill is the pre-walker path: download the entire repo as a CAR and
// feed every record in it to the indexer. It is kept as the fallback for hosts
// without com.atproto.sync.getBlocks -- notably streamplace's own PDS, whose
// did:web repos are synced through here for VOD origin indexing.
func (atsync *ATProtoSynchronizer) legacyBackfill(ctx context.Context, ident *identity.Identity, xrpcc *xrpc.Client) (string, error) {
	rev := ""
	pdsLock := pdsLocks.GetLock(ident.PDSEndpoint())
	pdsLock.Lock()
	repoBytes, err := SyncGetRepo(ctx, xrpcc, ident.DID.String(), rev)
	pdsLock.Unlock()
	if err != nil {
		return "", fmt.Errorf("failed to fetch repo for %s from PDS %s: %w", ident.DID.String(), xrpcc.Host, err)
	}

	log.Debug(ctx, "got diff", "bytes", len(repoBytes))

	r, err := repo.ReadRepoFromCar(ctx, bytes.NewReader(repoBytes))
	if err != nil {
		return "", fmt.Errorf("failed to parse repo CAR data for %s: %w", ident.DID.String(), err)
	}
	// extract DID from repo commit
	sc := r.SignedCommit()
	signerDID, err := syntax.ParseDID(sc.Did)
	if err != nil {
		return "", fmt.Errorf("invalid DID in repo commit for %s: %w", ident.DID.String(), err)
	}
	if signerDID != ident.DID {
		return "", fmt.Errorf("signer DID %s does not match identity %s", signerDID, ident.DID.String())
	}

	err = r.ForEach(ctx, "", func(k string, v cid.Cid) error {
		nsid, rkey, err := syntax.ParseRepoPath(k)
		if err != nil {
			log.Warn(ctx, "failed to parse repo path", "k", k, "err", err)
			return fmt.Errorf("could not parse repo path %s: %w", k, err)
		}
		_, bs, err := r.GetRecordBytes(ctx, k)
		if err != nil {
			log.Warn(ctx, "failed to get record bytes", "k", k, "rkey", rkey, "err", err)
			return fmt.Errorf("could not retrieve record bytes for %s (rkey: %s): %w", k, rkey, err)
		}
		log.Debug(ctx, "record type", "key", k, "type", nsid.String())

		err = atsync.handleCreateUpdate(ctx, signerDID.String(), rkey, bs, v.String(), nsid, false, true)
		if err != nil {
			log.Warn(ctx, "failed to handle create update", "err", err)
			// invalid CBOR and stuff should get ignored, so
			// return fmt.Errorf("failed to process record update for %s (type: %s): %w", k, nsid.String(), err)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to iterate over repo: %w", err)
	}

	return sc.Rev, nil
}

// pdsLockedFetcher serializes block fetches per PDS, the way the legacy path
// serializes its one big getRepo download.
//
// The lock is held only across the network call and never across a visitor
// callback: handleCreateUpdate can synchronously start a sync of another repo,
// which may live on the same host, and pdsLocks are plain mutexes.
type pdsLockedFetcher struct {
	lock  *sync.Mutex
	inner reposync.BlockFetcher
}

var _ reposync.BlockFetcher = (*pdsLockedFetcher)(nil)

func (f *pdsLockedFetcher) GetBlocks(ctx context.Context, cids []cid.Cid) (map[cid.Cid][]byte, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.inner.GetBlocks(ctx, cids)
}

// isMethodNotSupported reports whether err means "this host does not implement
// that XRPC method", which is the only failure the backfill is allowed to
// answer by falling back to a full getRepo.
//
// A host that has never heard of the route answers 404 (or 405 for the wrong
// verb); one that knows the lexicon but has not implemented it answers 501
// and/or the MethodNotImplemented error name. 401 is in the list because of
// streamplace's own PDS specifically: unregistered /xrpc/* methods land on its
// wildcard proxy handler, which needs an OAuth session and answers
// "oauth session not found" with a 401 to an anonymous sync request. Node to
// node repo sync (VOD origin indexing) runs through exactly that path, so
// without this the fallback would never fire for did:web streamplace repos.
// Errors are unwrapped because reposync wraps everything with %w.
//
// A false positive costs one wasted getRepo attempt, whose own error then
// propagates -- it can never turn a verification failure into a silent success,
// because verification failures are not HTTP errors.
func isMethodNotSupported(err error) bool {
	if err == nil {
		return false
	}
	var xe *xrpc.Error
	if errors.As(err, &xe) {
		switch xe.StatusCode {
		case http.StatusUnauthorized, http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusNotImplemented:
			return true
		}
	}
	var xrpcErr *xrpc.XRPCError
	if errors.As(err, &xrpcErr) && xrpcErr.ErrStr == "MethodNotImplemented" {
		return true
	}
	return false
}
