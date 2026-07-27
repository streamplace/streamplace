package atproto

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

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
	if isStaleWalkError(err) {
		// walkBackfill already exhausted its restart-from-a-new-head budget
		// on this. isMethodNotSupported is written not to claim these either,
		// but say it once here rather than depend on that ordering: answering
		// "the repo moved" with a full getRepo download would be absurd.
		return "", "", err
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

	fetchHead := func(ctx context.Context) (*reposync.Head, error) {
		head, err := reposync.FetchVerifiedHead(ctx, xrpcc, fetcher, dir, did)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch verified head for %s from PDS %s: %w", did, xrpcc.Host, err)
		}
		return head, nil
	}

	records := 0
	walk := func(ctx context.Context, root cid.Cid) error {
		records = 0
		walker := &reposync.Walker{Fetcher: fetcher}
		err := walker.WalkRanges(ctx, root, backfillRanges(), func(path string, rcid cid.Cid, rec []byte) error {
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
			return fmt.Errorf("failed to walk repo for %s from PDS %s: %w", did, xrpcc.Host, err)
		}
		return nil
	}

	head, err := walkWithHeadRetry(ctx, maxWalkAttempts, walkRetryDelay, fetchHead, walk)
	if err != nil {
		return "", "", err
	}

	log.Log(ctx, "walked repo", "did", did, "rev", head.Rev, "root", head.Root.String(), "records", records)
	return head.Rev, head.Root.String(), nil
}

// maxWalkAttempts bounds how many times a backfill restarts its walk against a
// freshly read head.
const maxWalkAttempts = 3

// walkRetryDelay is the pause before re-reading the head, so a repo in the
// middle of a burst of writes gets a moment to settle.
const walkRetryDelay = 1500 * time.Millisecond

// walkWithHeadRetry walks the repo at the current head, restarting against a
// new head when the walk discovers the repo moved underneath it.
//
// A walk pins one root and then makes hundreds of sequential getBlocks calls
// against it, while the PDS garbage-collects the blocks that only superseded
// commits referenced. A repo that commits while we are reading it can therefore
// leave us asking for blocks the host no longer has. That is a race, not
// corruption: read the head again and walk the new tree. The [reposync.CachedFetcher]
// is deliberately reused across attempts, so the second walk pays only for the
// churned path and whatever records are new.
//
// If the head did not move, the blocks really are gone: the repo is incomplete
// and we fail rather than record a version whose contents we could not read.
//
// Records emitted by an abandoned attempt are emitted again by the next one.
// That is the walker's documented at-least-once contract; the indexing visitor
// is idempotent, keyed by (path, record cid).
func walkWithHeadRetry(
	ctx context.Context,
	attempts int,
	delay time.Duration,
	fetchHead func(context.Context) (*reposync.Head, error),
	walk func(context.Context, cid.Cid) error,
) (*reposync.Head, error) {
	head, err := fetchHead(ctx)
	if err != nil {
		return nil, err
	}
	for attempt := 1; ; attempt++ {
		err := walk(ctx, head.Root)
		if err == nil {
			return head, nil
		}
		if !isStaleWalkError(err) {
			return nil, err
		}
		if attempt >= attempts {
			// Giving up leaves the repo row at Version="", and the boot-time
			// Migrate sweep re-runs backfills for those. That safety net is
			// what makes a bounded number of attempts here acceptable.
			return nil, fmt.Errorf("gave up after %d walk attempts: %w", attempt, err)
		}
		if serr := sleepCtx(ctx, delay); serr != nil {
			return nil, errors.Join(err, serr)
		}
		next, ferr := fetchHead(ctx)
		if ferr != nil {
			return nil, errors.Join(err, ferr)
		}
		if next.Rev == head.Rev && next.Root == head.Root {
			return nil, fmt.Errorf("repo is missing blocks at rev %s, which is still the head: %w", head.Rev, err)
		}
		log.Warn(ctx, "repo moved during backfill walk, restarting from the new head",
			"rev", head.Rev, "newRev", next.Rev, "attempt", attempt, "err", err)
		head = next
	}
}

// sleepCtx waits for d, or returns the context's error as soon as it is done.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
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
//
// It is held across the fetcher's retry backoff, though, which is what we want:
// a 429 applies to the whole host, so pausing every backfill against it is the
// polite response. reposync.DefaultRetryMaxDelay is what keeps that pause
// bounded.
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

// isStaleWalkError reports whether err means "the repo moved while we were
// reading it", which a backfill answers by re-reading the head and walking
// again rather than by giving up.
//
// The three shapes it has to recognize:
//
//   - [reposync.ErrMissingBlock], our own client-side check, when a host
//     answers a getBlocks call with a short bag of blocks.
//   - The BlockNotFound error name. streamplace's own PDS fails the whole
//     request that way rather than returning a partial CAR.
//   - A 400 InvalidRequest whose message contains "Could not find cids". That
//     is what the TypeScript reference PDS says -- observed from both a
//     bsky.network mothership and a self-hosted instance -- and it has no error
//     name of its own, so matching the message string is the only option.
func isStaleWalkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, reposync.ErrMissingBlock) {
		return true
	}
	if xrpcErrorName(err) == "BlockNotFound" {
		return true
	}
	var xe *xrpc.XRPCError
	if errors.As(err, &xe) && strings.Contains(xe.Message, "Could not find cids") {
		return true
	}
	return false
}

// xrpcErrorName pulls the lexicon error name out of an XRPC failure.
//
// The reference implementation puts it in the response body's "error" field,
// which indigo decodes into XRPCError.ErrStr. streamplace's own PDS answers
// through echo's default error handler, which emits only {"message": "..."} --
// so its BlockNotFound/RepoNotFound/InvalidRequest names arrive at the front of
// Message instead. Accept both, but only when the leading token actually looks
// like a lexicon error name, so that prose like "oauth session not found" is
// not mistaken for one.
func xrpcErrorName(err error) string {
	var xe *xrpc.XRPCError
	if !errors.As(err, &xe) {
		return ""
	}
	if xe.ErrStr != "" {
		return xe.ErrStr
	}
	name, _, _ := strings.Cut(xe.Message, ":")
	if !isLexiconErrorName(name) {
		return ""
	}
	return name
}

// isLexiconErrorName reports whether s has the shape of an atproto error name:
// UpperCamelCase, letters and digits only.
func isLexiconErrorName(s string) bool {
	if s == "" || s[0] < 'A' || s[0] > 'Z' {
		return false
	}
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
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
// A named lexicon error disqualifies all of that, whatever the status code: a
// host that answers BlockNotFound or RepoNotFound plainly does implement the
// method, and is telling us something about this repo. That distinction is not
// academic -- streamplace's own getBlocks returns 404 BlockNotFound for a block
// it has garbage collected, and treating that as "unsupported" would answer a
// mid-walk race against a peer with a full-repo CAR download instead of a cheap
// re-walk.
//
// A false positive costs one wasted getRepo attempt, whose own error then
// propagates -- it can never turn a verification failure into a silent success,
// because verification failures are not HTTP errors.
func isMethodNotSupported(err error) bool {
	if err == nil {
		return false
	}
	switch name := xrpcErrorName(err); name {
	case "":
		// No name to go on; fall through to the status code.
	case "MethodNotImplemented", "XRPCNotSupported":
		return true
	default:
		return false
	}
	var xe *xrpc.Error
	if errors.As(err, &xe) {
		switch xe.StatusCode {
		case http.StatusUnauthorized, http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusNotImplemented:
			return true
		}
	}
	return false
}
