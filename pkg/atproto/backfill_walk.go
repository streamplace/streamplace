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
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/reposync"
)

// placeStreamPrefix is the collection prefix shared by every streamplace
// lexicon. Because MST keys are "collection/rkey" sorted bytewise, all of those
// records live in one contiguous key range.
const placeStreamPrefix = "place.stream."

// windowedCollections are the collections a backfill reads by time window
// instead of all at once.
//
// The policy, because getting this list wrong is a correctness bug rather than a
// performance one: window a collection if and only if it is a high-volume
// append-only log whose rkeys are TIDs. Those sort chronologically, so "the last
// day of chat" is a key range (see [reposync.TIDForTime]), and they are the
// whole cost of a first sync -- an active account has tens of thousands of
// posts, follows and chat messages, and a few dozen records everywhere else. A
// production sweep measured an average "shallow" sync fetching 756 records
// because follows were being read in full; one account with 3516 follows took
// four minutes.
//
// Everything else must stay full, in particular every current-state registry:
// signing keys, chat gates, settings, delegations, the media catalog,
// app.bsky.graph.block (moderation). Windowing those would be wrong, not slow.
// A windowed collection's records below the floor are invisible until the
// deepening ladder reaches the genesis window, which is hours later -- and
// "hours without your block list" or "hours without your signing key" is not a
// tradeoff, it is a broken node.
//
// Windowing changes when a record is indexed, never whether: the ladder of
// windows in [nextBackfillWindow] bottoms out at the start of the collection,
// and until it does the repo row says so. A record whose rkey is not a TID is
// not skipped either -- the windows are key ranges, so it simply arrives with
// whichever window its rkey sorts into. That is the caveat on the TID
// requirement: a literal rkey that sorts below the floor ("!oldest", say) waits
// for the genesis window like any old record would, which is fine for a log and
// would not be fine for a registry keyed "self".
//
// Every entry must be a collection [backfillRanges] would otherwise walk whole
// -- either under place.stream. or in [CollectionFilter]. TestBackfillRanges
// checks that.
var windowedCollections = []string{
	constants.PLACE_STREAM_CHAT_MESSAGE,
	constants.APP_BSKY_FEED_POST,
	constants.APP_BSKY_GRAPH_FOLLOW,
}

// backfillRanges is the set of MST key ranges a backfill walks: everything
// under place.stream., plus one range per non-streamplace collection the
// firehose accepts.
//
// It is derived from CollectionFilter at runtime so the backfill and the
// firehose can never drift apart about which records this node indexes.
//
// floor is a TID watermark for the windowed collections: those are walked only
// from floor forward, with the rest of their key space cut out of the ranges
// that would otherwise cover it. An empty floor means "from the beginning",
// which is every range whole.
func backfillRanges(floor string) []reposync.KeyRange {
	ranges := []reposync.KeyRange{reposync.PrefixRange(placeStreamPrefix)}
	for _, nsid := range CollectionFilter {
		if strings.HasPrefix(nsid, placeStreamPrefix) {
			continue
		}
		// The trailing slash keeps "app.bsky.feed.post" from also matching
		// "app.bsky.feed.postgate".
		ranges = append(ranges, reposync.PrefixRange(nsid+"/"))
	}
	if floor == "" {
		return ranges
	}
	for _, nsid := range windowedCollections {
		whole := reposync.PrefixRange(nsid + "/")
		kept := make([]reposync.KeyRange, 0, len(ranges)+1)
		for _, r := range ranges {
			kept = append(kept, subtractRange(r, whole)...)
		}
		ranges = append(kept, reposync.KeyRange{Lo: []byte(nsid + "/" + floor), Hi: whole.Hi})
	}
	return ranges
}

// windowRanges covers [lo, hi) of every windowed collection and nothing else.
// It is what a deepening step walks. An empty lo starts at the first key of
// each collection; an empty hi runs to the last.
func windowRanges(lo, hi string) []reposync.KeyRange {
	out := make([]reposync.KeyRange, 0, len(windowedCollections))
	for _, nsid := range windowedCollections {
		r := reposync.PrefixRange(nsid + "/")
		if lo != "" {
			r.Lo = []byte(nsid + "/" + lo)
		}
		if hi != "" {
			r.Hi = []byte(nsid + "/" + hi)
		}
		out = append(out, r)
	}
	return out
}

// subtractRange returns r with cut removed: r itself when they do not overlap,
// the pieces of r on either side of cut when they do, and nothing when cut
// swallows r. Empty pieces are dropped, because a zero-width range is not a
// range the walker will accept.
func subtractRange(r, cut reposync.KeyRange) []reposync.KeyRange {
	if !rangesOverlap(r, cut) {
		return []reposync.KeyRange{r}
	}
	var out []reposync.KeyRange
	if bytes.Compare(r.Lo, cut.Lo) < 0 {
		out = append(out, reposync.KeyRange{Lo: r.Lo, Hi: cut.Lo})
	}
	if cut.Hi != nil && (r.Hi == nil || bytes.Compare(cut.Hi, r.Hi) < 0) {
		out = append(out, reposync.KeyRange{Lo: cut.Hi, Hi: r.Hi})
	}
	return out
}

func rangesOverlap(a, b reposync.KeyRange) bool {
	if a.Hi != nil && bytes.Compare(b.Lo, a.Hi) >= 0 {
		return false
	}
	if b.Hi != nil && bytes.Compare(a.Lo, b.Hi) >= 0 {
		return false
	}
	return true
}

// InitialWindow is how much history a first sync reads from the windowed
// collections. It is the whole cost difference between meeting an account and
// serving it: everything else in the repo is configuration-sized.
const InitialWindow = 24 * time.Hour

// backfillSpans is the ladder of windows the deepening sweep walks, each one
// reaching further back than the last. After the last rung the next window runs
// to the start of the collection, and the repo is complete.
//
// Spans are wall-clock ages, not window widths: a repo at the 7d rung has
// everything from seven days ago forward, and its next window is
// [30d ago, 7d ago).
var backfillSpans = []time.Duration{
	InitialWindow,
	7 * 24 * time.Hour,
	30 * 24 * time.Hour,
	180 * 24 * time.Hour,
}

// backfillWindow is one step of the ladder: the slice of the windowed
// collections a deepening walk should read next.
type backfillWindow struct {
	// Lo and Hi are TID bounds, empty meaning the start/end of the collection.
	Lo string
	Hi string
	// Genesis reports that this window reaches the start of the collection, so
	// a repo that finishes it has no history left to fetch.
	Genesis bool
	// Horizon is the wall-clock instant Lo encodes, for logging. Zero for a
	// genesis window, which has no horizon.
	Horizon time.Time
}

// nextBackfillWindow picks the next deepening window for a repo whose windowed
// collections are synced from floor forward.
//
// Which rung of the ladder a repo is on is read off the age of its floor rather
// than stored: a floor is a timestamp, and "how far back does this repo go" is
// the only thing that matters. That keeps the watermark a single self-describing
// column, and makes a row written by an older build (or a hand-edited one) land
// on a sensible rung by itself.
//
// An empty floor means nothing has been recorded, which is both a repo that has
// never been synced and every row written before this code existed: those start
// at the top of the ladder, with a window that is open-ended above.
func nextBackfillWindow(floor string, now time.Time) backfillWindow {
	window := func(span time.Duration, hi string) backfillWindow {
		horizon := now.Add(-span)
		return backfillWindow{Lo: reposync.TIDForTime(horizon), Hi: hi, Horizon: horizon}
	}
	if floor == "" {
		return window(backfillSpans[0], "")
	}
	floorTime, err := reposync.TimeForTID(floor)
	if err != nil {
		// Not a timestamp we can place on the ladder. One genesis-bounded
		// window finishes the repo off rather than looping on it forever.
		return backfillWindow{Hi: floor, Genesis: true}
	}
	age := now.Sub(floorTime)
	for _, span := range backfillSpans {
		if span > age {
			return window(span, floor)
		}
	}
	return backfillWindow{Hi: floor, Genesis: true}
}

// backfillResult is what a completed backfill knows about the repo it read.
type backfillResult struct {
	// Rev is the repo revision the index is now consistent with.
	Rev string
	// RootCID is the MST root that revision committed to, empty if the fallback
	// path was used, which never sees a verified root.
	RootCID string
	// Floor is the TID watermark for the windowed collections: their history is
	// synced from here forward. Empty means from the start of the collection.
	Floor string
	// Done reports that the windowed collections need no further deepening.
	Done bool
}

// backfillRepo indexes every record we care about from ident's repo.
//
// floor windows the high-volume collections: only their history from that TID
// forward is read, which is what makes first contact with a busy account cost
// seconds instead of minutes. An empty floor reads everything.
//
// The fast path walks only the subtrees holding records we index. Hosts that do
// not implement com.atproto.sync.getBlocks fall back to downloading the whole
// repo as a CAR -- which reads all of it, window or no window, so such a repo
// comes back complete.
func (atsync *ATProtoSynchronizer) backfillRepo(ctx context.Context, ident *identity.Identity, xrpcc *xrpc.Client, floor string) (backfillResult, error) {
	rev, root, err := atsync.walkBackfill(ctx, ident, xrpcc, backfillRanges(floor))
	if err == nil {
		return backfillResult{Rev: rev, RootCID: root, Floor: floor, Done: floor == ""}, nil
	}
	if isStaleWalkError(err) {
		// walkBackfill already exhausted its restart-from-a-new-head budget
		// on this. isMethodNotSupported is written not to claim these either,
		// but say it once here rather than depend on that ordering: answering
		// "the repo moved" with a full getRepo download would be absurd.
		return backfillResult{}, err
	}
	if !isMethodNotSupported(err) {
		// Anything else -- a bad signature, a malformed tree, a network
		// failure -- must propagate. Falling back on a verification failure
		// would make the verification decorative.
		return backfillResult{}, err
	}
	log.Warn(ctx, "host does not support sync.getBlocks, falling back to full getRepo",
		"pds", xrpcc.Host, "did", ident.DID.String(), "err", err)
	rev, err = atsync.legacyBackfill(ctx, ident, xrpcc)
	if err != nil {
		return backfillResult{}, err
	}
	return backfillResult{Rev: rev, Done: true}, nil
}

// walkBackfill does a verified, range-bounded walk of the remote repo, handing
// every record in range to the same indexing path the firehose uses.
func (atsync *ATProtoSynchronizer) walkBackfill(ctx context.Context, ident *identity.Identity, xrpcc *xrpc.Client, ranges []reposync.KeyRange) (string, string, error) {
	did := ident.DID.String()

	// The *uncached* directory on purpose: a signing key cached from before a
	// rotation would fail commit verification, and backfills are rare enough
	// that the extra lookup does not matter.
	dir := atsync.directory(false)

	// Every retry in this walk consults what the host has been telling us about
	// backing off; see [pdsBackoffHints]. It only works if the calls go through
	// a client with the hint-capturing transport installed, which is what
	// [SyncHTTPClient] is for -- callers build xrpcc with it.
	retry := reposync.RetryPolicy{Hints: pdsBackoffHints}

	fetcher := &reposync.CachedFetcher{
		// Bounded lifetime: one cache per backfill, so the head fetch and the
		// walk share blocks without holding a repo in memory afterwards.
		Cache: reposync.NewMemoryBlockCache(),
		Inner: &pdsLockedFetcher{
			lock:  pdsLocks.GetLock(ident.PDSEndpoint()),
			inner: &reposync.XRPCBlockFetcher{Client: xrpcc, DID: did, Retry: retry},
		},
	}

	fetchHead := func(ctx context.Context) (*reposync.Head, error) {
		head, err := reposync.FetchVerifiedHead(ctx, xrpcc, fetcher, dir, did, retry)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch verified head for %s from PDS %s: %w", did, xrpcc.Host, err)
		}
		return head, nil
	}

	records := 0
	walk := func(ctx context.Context, root cid.Cid) error {
		records = 0
		walker := &reposync.Walker{Fetcher: fetcher}
		err := walker.WalkRanges(ctx, root, ranges, func(path string, rcid cid.Cid, rec []byte) error {
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

// pdsBackoffHints is this process's memory of what PDS hosts have said about
// backing off, shared by every repo sync so that one repo's 429 slows down the
// next repo on that host too.
//
// It is fed by the transport on [SyncHTTPClient] and read by the retry policies
// in [walkBackfill]. It has to be a package-level singleton for the same reason
// pdsLocks is: the unit a rate limit applies to is the host, and the sweep works
// on thousands of repos across it.
var pdsBackoffHints = reposync.NewBackoffHints()

// SyncHTTPClient is the HTTP client every repo-sync XRPC call goes through.
//
// It is [aqhttp.Client] -- same SSRF-checking transport, same timeout, same
// redirect policy -- with one wrapper installed: the round tripper that notices
// throttled responses and records their Retry-After / ratelimit-reset headers.
// indigo's xrpc client discards response headers, so watching the transport is
// the only place those numbers can be read at all, and without them a retry is
// guessing at a wait the host already told us.
//
// The wrapper is scoped to sync traffic rather than installed on aqhttp.Client
// globally: it is only useful where something consults the registry, and the
// node makes plenty of unrelated HTTP requests that would otherwise pay for a
// map lookup and a header parse.
var SyncHTTPClient = newSyncHTTPClient()

func newSyncHTTPClient() *http.Client {
	// By value: aqhttp.Client is a struct of settings (no mutex), and copying it
	// keeps the connection-pooling transport shared with the rest of the node.
	c := aqhttp.Client
	c.Transport = pdsBackoffHints.Transport(c.Transport)
	return &c
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
// polite response -- all the more so now that the backoff is usually the number
// the host itself asked for ([pdsBackoffHints]) rather than a guess.
// reposync.DefaultRetryMaxDelay is what keeps that pause bounded.
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

// TerminalRepoError is a backfill failure that retrying cannot fix, because the
// account itself is deactivated, deleted, suspended, or taken down. Callers
// record Status on the repo row and stop; the firehose clears it if the account
// comes back.
type TerminalRepoError struct {
	// Status is a model.RepoStatus* value.
	Status string
	Err    error
}

func (e *TerminalRepoError) Error() string {
	return fmt.Sprintf("repo is %s: %v", e.Status, e.Err)
}

func (e *TerminalRepoError) Unwrap() error { return e.Err }

// repoStatusFromError maps the account-lifecycle errors an atproto host returns
// onto a [model.Repo] status, or "" for anything worth retrying.
//
// Only named lexicon errors count. A DNS failure, a timeout, an SSRF block or a
// 500 says nothing about the account -- parking a repo as "notfound" because its
// PDS was briefly unreachable would take it out of the index until it happened
// to commit again.
func repoStatusFromError(err error) string {
	switch xrpcErrorName(err) {
	case "RepoDeactivated":
		return model.RepoStatusDeactivated
	case "RepoNotFound":
		return model.RepoStatusNotFound
	case "RepoTakendown":
		return model.RepoStatusTakendown
	case "RepoSuspended":
		return model.RepoStatusSuspended
	}
	return ""
}

// parkTerminalRepo records a terminal account state on the repo row so that the
// boot-time sweep and every cached lookup stop asking about it.
//
// It returns the error the caller should return -- a [TerminalRepoError] if the
// failure was terminal, or nil if it was not, in which case the caller keeps its
// own error. Version and RootCID are deliberately left as they are: whatever we
// managed to index of this repo before it went away stays valid and stays
// served.
func parkTerminalRepo(ctx context.Context, mod model.Model, did string, err error) error {
	status := repoStatusFromError(err)
	if status == model.RepoStatusOK {
		return nil
	}
	log.Log(ctx, "repo is in a terminal account state", "did", did, "status", status, "err", err)
	if serr := mod.SetRepoStatus(ctx, did, status); serr != nil {
		return fmt.Errorf("failed to record %s status for %s: %w", status, did, serr)
	}
	return &TerminalRepoError{Status: status, Err: err}
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
