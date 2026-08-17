package reposync

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/util"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/cenkalti/backoff"
	"github.com/ipfs/go-cid"
	glex "github.com/streamplace/glex/runtime"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/devenv"
	"stream.place/streamplace/pkg/placestream"
)

// These tests run the walker against the Bluesky reference PDS booted by
// pkg/devenv, so they exercise the real com.atproto.sync.* surface: real CAR
// framing, real MST shapes, real did:plc key resolution.

// Collections used for the out-of-range writes. Their names matter more than
// their contents: they must bracket ["place.stream.", "place.stream/") on both
// sides, including immediately adjacent to the boundaries.
const (
	// "app.bsky.feed.post" is a lexicon the PDS knows, so it is fully validated.
	collBelowFar = "app.bsky.feed.post"
	// "place.strea." sorts immediately below "place.stream.".
	collBelowNear = "place.strea.thing"
	// 'x' > '/', so this sorts immediately above the range's exclusive end.
	collAboveNear = "place.streamx.thing"
	collAboveFar  = "zzz.example.thing"
)

func nowISO() string { return time.Now().UTC().Format(util.ISO8601) }

// devRecord is a record we wrote to the dev PDS: where it landed and what the
// PDS said its CID is. The CID is the assertion anchor -- a walk must produce
// exactly these.
type devRecord struct {
	path string
	cid  cid.Cid
}

// createRecord writes one record and returns its MST key and CID. rkey may be
// empty to let the PDS mint a TID.
func createRecord(ctx context.Context, t *testing.T, acct *devenv.DevEnvAccount, collection, rkey string, rec glex.Record) devRecord {
	t.Helper()
	in := &comatproto.RepoCreateRecord_Input{
		Collection: collection,
		Repo:       acct.DID,
		Record:     &glex.LexiconTypeDecoder{Val: rec},
	}
	if rkey != "" {
		in.Rkey = &rkey
	}
	out, err := comatproto.RepoCreateRecord(ctx, acct.XRPC, in)
	require.NoError(t, err, "creating %s record", collection)
	c, err := cid.Decode(out.Cid)
	require.NoError(t, err, "decoding cid %q returned for %s", out.Cid, out.Uri)
	// at://<did>/<collection>/<rkey>
	gotRkey := out.Uri[strings.LastIndex(out.Uri, "/")+1:]
	return devRecord{path: collection + "/" + gotRkey, cid: c}
}

// rawRecord is an arbitrary JSON record for a lexicon nothing in this repo
// generates code for, used for the out-of-range collections.
func rawRecord(t *testing.T, typ string, fields map[string]any) glex.Record {
	t.Helper()
	m := map[string]any{"$type": typ}
	for k, v := range fields {
		m[k] = v
	}
	rec, err := glex.RawJSON(m)
	require.NoError(t, err)
	return rec
}

func chatMessage(streamerDID, text string) *placestream.ChatMessage {
	return &placestream.ChatMessage{
		LexiconTypeID: "place.stream.chat.message",
		Text:          text,
		CreatedAt:     nowISO(),
		Streamer:      streamerDID,
	}
}

// retryUntilNoError is the untilNoErrors pattern from pkg/atproto's devenv
// tests: the PDS commits asynchronously, so anything that reads back a write
// needs a bounded retry rather than a sleep.
func retryUntilNoError(t *testing.T, f func() error) error {
	t.Helper()
	ticker := backoff.NewTicker(devenv.NewExponentialBackOff())
	defer ticker.Stop()
	var err error
	for i := 0; i < 10; i++ {
		err = f()
		if err == nil {
			return nil
		}
		if i < 9 {
			<-ticker.C
		}
	}
	return err
}

// fetchHead resolves and verifies the head, retrying while the PDS catches up.
// If afterRev is non-empty the head must also have advanced past it, which is
// how a test waits for its own writes to land.
func fetchHead(ctx context.Context, t *testing.T, client *xrpc.Client, f BlockFetcher, dir identity.Directory, did, afterRev string) *Head {
	t.Helper()
	var head *Head
	err := retryUntilNoError(t, func() error {
		h, err := FetchVerifiedHead(ctx, client, f, dir, did)
		if err != nil {
			return err
		}
		// revs are TIDs: fixed-length base32-sortable, so bytewise compare works.
		if afterRev != "" && h.Rev <= afterRev {
			return fmt.Errorf("rev %q has not advanced past %q", h.Rev, afterRev)
		}
		head = h
		return nil
	})
	require.NoError(t, err, "fetching verified head for %s", did)
	return head
}

// countingFetcher counts calls and remembers every CID asked for, so tests can
// talk about how much of a remote repo a walk actually touched.
type countingFetcher struct {
	inner BlockFetcher
	calls int
	seen  map[cid.Cid]bool
}

func newCountingFetcher(inner BlockFetcher) *countingFetcher {
	return &countingFetcher{inner: inner, seen: map[cid.Cid]bool{}}
}

func (f *countingFetcher) GetBlocks(ctx context.Context, cids []cid.Cid) (map[cid.Cid][]byte, error) {
	f.calls++
	for _, c := range cids {
		f.seen[c] = true
	}
	return f.inner.GetBlocks(ctx, cids)
}

// failAfterFetcher simulates the process dying (or the network going away)
// partway through a walk.
type failAfterFetcher struct {
	inner     BlockFetcher
	failAfter int
	calls     int
}

func (f *failAfterFetcher) GetBlocks(ctx context.Context, cids []cid.Cid) (map[cid.Cid][]byte, error) {
	f.calls++
	if f.calls > f.failAfter {
		return nil, errFetcherBoom
	}
	return f.inner.GetBlocks(ctx, cids)
}

func devClient(dev *devenv.DevEnv) *xrpc.Client {
	return &xrpc.Client{Host: dev.PDSURL, Client: &aqhttp.Client}
}

func sortedPaths(recs []devRecord) []string {
	out := make([]string, len(recs))
	for i, r := range recs {
		out[i] = r.path
	}
	sort.Strings(out)
	return out
}

func recordCIDs(recs []devRecord) map[string]cid.Cid {
	out := map[string]cid.Cid{}
	for _, r := range recs {
		out[r.path] = r.cid
	}
	return out
}

// TestIntegrationRangeSyncAgainstPDS is the whole story against a real PDS:
// verify the head, walk the place.stream. range exactly, diff two walks across a
// create+delete, resume a walk that died mid-flight, and handle an account with
// nothing in range. One dev env is shared by the subtests because booting it
// costs seconds; they run in order and later ones build on earlier writes.
func TestIntegrationRangeSyncAgainstPDS(t *testing.T) {
	ctx := context.Background()
	dev := devenv.WithDevEnv(t)
	t.Logf("dev env: pds=%s plc=%s", dev.PDSURL, dev.PLCURL)
	dir := dev.TestDirectory()
	client := devClient(dev)

	acctA := dev.CreateAccount(t)
	t.Logf("account A: %s (%s)", acctA.DID, acctA.Handle)

	// ---- account A: in-range records ------------------------------------
	profile := &placestream.ChatProfile{
		LexiconTypeID: "place.stream.chat.profile",
		SelfLabels:    []string{"bot"},
	}
	inRange := []devRecord{createRecord(ctx, t, acctA, "place.stream.chat.profile", "self", profile)}
	texts := map[string]string{}
	for i := 0; i < 3; i++ {
		text := fmt.Sprintf("integration message %d", i)
		rec := createRecord(ctx, t, acctA, "place.stream.chat.message", "", chatMessage(acctA.DID, text))
		inRange = append(inRange, rec)
		texts[rec.path] = text
	}

	// ---- account A: out-of-range records on both sides -------------------
	outOfRange := []devRecord{
		createRecord(ctx, t, acctA, collBelowFar, "", rawRecord(t, collBelowFar, map[string]any{
			"text":      "a post that sorts below place.stream.",
			"createdAt": nowISO(),
		})),
		createRecord(ctx, t, acctA, collBelowNear, "", rawRecord(t, collBelowNear, map[string]any{
			"createdAt": nowISO(),
		})),
		createRecord(ctx, t, acctA, collAboveNear, "", rawRecord(t, collAboveNear, map[string]any{
			"createdAt": nowISO(),
		})),
		createRecord(ctx, t, acctA, collAboveFar, "", rawRecord(t, collAboveFar, map[string]any{
			"createdAt": nowISO(),
		})),
	}
	for _, r := range outOfRange {
		require.False(t, PrefixRange("place.stream.").contains([]byte(r.path)),
			"fixture bug: %q is inside the walked range", r.path)
	}
	t.Logf("account A: %d in-range records, %d out-of-range", len(inRange), len(outOfRange))

	fetcherA := &XRPCBlockFetcher{Client: client, DID: acctA.DID, ChunkSize: 10}
	counting := newCountingFetcher(fetcherA)

	var headA *Head
	var firstWalk map[string]cid.Cid

	t.Run("verified head and exact prefix walk", func(t *testing.T) {
		headA = fetchHead(ctx, t, client, fetcherA, dir, acctA.DID, "")
		t.Logf("head: cid=%s rev=%s root=%s", headA.CID, headA.Rev, headA.Root)
		require.Equal(t, acctA.DID, headA.Commit.DID)
		require.NotEmpty(t, headA.Commit.Sig)

		var got []emission
		w := &Walker{Fetcher: counting, BatchSize: 5}
		require.NoError(t, w.WalkPrefix(ctx, headA.Root, "place.stream.", collectVisitor(&got)))

		require.Equal(t, sortedPaths(inRange), emittedPaths(got), "emitted paths, in key order")
		want := recordCIDs(inRange)
		firstWalk = map[string]cid.Cid{}
		for _, e := range got {
			require.Equal(t, want[e.path], e.cid, "record cid for %q", e.path)
			firstWalk[e.path] = e.cid
			if text, ok := texts[e.path]; ok {
				msg, err := glex.CborDecodeAs[placestream.ChatMessage](e.data)
				require.NoError(t, err, "decoding record at %q", e.path)
				require.Equal(t, text, msg.Text)
				require.Equal(t, acctA.DID, msg.Streamer)
			}
		}
		// The profile round-trips as the record we wrote.
		profileData := ""
		for _, e := range got {
			if strings.HasPrefix(e.path, "place.stream.chat.profile/") {
				p, err := glex.CborDecodeAs[placestream.ChatProfile](e.data)
				require.NoError(t, err)
				require.Equal(t, []string{"bot"}, p.SelfLabels)
				profileData = e.path
			}
		}
		require.Equal(t, "place.stream.chat.profile/self", profileData)

		t.Logf("walk made %d fetcher calls (each chunked into <=%d cids per request), %d distinct blocks",
			counting.calls, DefaultChunkSize, len(counting.seen))
		require.Greater(t, counting.calls, 1, "ChunkSize 10 with a batched walk should take several calls")
	})

	t.Run("create and delete are visible as a diff", func(t *testing.T) {
		require.NotNil(t, headA, "previous subtest must have run")

		added := createRecord(ctx, t, acctA, "place.stream.chat.message", "", chatMessage(acctA.DID, "added later"))
		deleted := inRange[1] // the first chat message
		slash := strings.LastIndex(deleted.path, "/")
		_, err := comatproto.RepoDeleteRecord(ctx, acctA.XRPC, &comatproto.RepoDeleteRecord_Input{
			Collection: deleted.path[:slash],
			Repo:       acctA.DID,
			Rkey:       deleted.path[slash+1:],
		})
		require.NoError(t, err)

		head2 := fetchHead(ctx, t, client, fetcherA, dir, acctA.DID, headA.Rev)
		require.Greater(t, head2.Rev, headA.Rev, "rev must advance after writes")
		require.NotEqual(t, headA.Root, head2.Root, "MST root must change after writes")

		w := &Walker{Fetcher: fetcherA, BatchSize: 5}
		after, err := w.CollectPrefix(ctx, head2.Root, "place.stream.")
		require.NoError(t, err)

		d := DiffCollections(firstWalk, after)
		require.Equal(t, []string{added.path}, d.Created)
		require.Equal(t, []string{deleted.path}, d.Deleted)
		require.Empty(t, d.Updated)
		require.Equal(t, added.cid, after[added.path])

		// And the second walk is itself exact.
		want := recordCIDs(inRange)
		delete(want, deleted.path)
		want[added.path] = added.cid
		require.Equal(t, want, after)

		headA = head2
		firstWalk = after
	})

	t.Run("updating a record in place shows as an update", func(t *testing.T) {
		require.NotNil(t, headA)
		updated := &placestream.ChatProfile{
			LexiconTypeID: "place.stream.chat.profile",
			SelfLabels:    []string{"bot", "test"},
		}
		out, err := comatproto.RepoPutRecord(ctx, acctA.XRPC, &comatproto.RepoPutRecord_Input{
			Collection: "place.stream.chat.profile",
			Repo:       acctA.DID,
			Rkey:       "self",
			Record:     &glex.LexiconTypeDecoder{Val: updated},
		})
		require.NoError(t, err)
		newCID, err := cid.Decode(out.Cid)
		require.NoError(t, err)

		head3 := fetchHead(ctx, t, client, fetcherA, dir, acctA.DID, headA.Rev)
		after, err := (&Walker{Fetcher: fetcherA}).CollectPrefix(ctx, head3.Root, "place.stream.")
		require.NoError(t, err)
		d := DiffCollections(firstWalk, after)
		require.Empty(t, d.Created)
		require.Empty(t, d.Deleted)
		require.Equal(t, []string{"place.stream.chat.profile/self"}, d.Updated)
		require.Equal(t, newCID, after["place.stream.chat.profile/self"])

		headA = head3
		firstWalk = after
	})

	t.Run("account with nothing in range", func(t *testing.T) {
		acctB := dev.CreateAccount(t)
		t.Logf("account B: %s", acctB.DID)
		fetcherB := &XRPCBlockFetcher{Client: client, DID: acctB.DID}

		// A brand new repo: verified head, empty walk.
		headB := fetchHead(ctx, t, client, fetcherB, dir, acctB.DID, "")
		require.Equal(t, acctB.DID, headB.Commit.DID)
		var got []emission
		require.NoError(t, (&Walker{Fetcher: fetcherB}).
			WalkPrefix(ctx, headB.Root, "place.stream.", collectVisitor(&got)))
		require.Empty(t, got, "empty repo emitted records")

		// Still empty once the repo has records, just none in range.
		createRecord(ctx, t, acctB, collBelowFar, "", rawRecord(t, collBelowFar, map[string]any{
			"text":      "not a streamplace record",
			"createdAt": nowISO(),
		}))
		createRecord(ctx, t, acctB, collAboveFar, "", rawRecord(t, collAboveFar, map[string]any{
			"createdAt": nowISO(),
		}))
		headB2 := fetchHead(ctx, t, client, fetcherB, dir, acctB.DID, headB.Rev)
		got = nil
		require.NoError(t, (&Walker{Fetcher: fetcherB}).
			WalkPrefix(ctx, headB2.Root, "place.stream.", collectVisitor(&got)))
		require.Empty(t, got, "out-of-range records leaked into the walk")
	})
}

// applyWrites bulk-creates records. The generated pkg/comatproto has no
// applyWrites binding, so this goes over the raw XRPC client. Returns the paths
// and CIDs the PDS assigned, in request order.
func applyWrites(ctx context.Context, t *testing.T, acct *devenv.DevEnvAccount, collection string, rkeys []string, value func(rkey string) map[string]any) []devRecord {
	t.Helper()
	writes := make([]map[string]any, 0, len(rkeys))
	for _, rkey := range rkeys {
		rec := map[string]any{"$type": collection}
		for k, v := range value(rkey) {
			rec[k] = v
		}
		writes = append(writes, map[string]any{
			"$type":      "com.atproto.repo.applyWrites#create",
			"collection": collection,
			"rkey":       rkey,
			"value":      rec,
		})
	}
	var out struct {
		Results []struct {
			URI string `json:"uri"`
			CID string `json:"cid"`
		} `json:"results"`
	}
	err := acct.XRPC.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.applyWrites", nil, map[string]any{
		"repo": acct.DID,
		// The padding records are shaped like the real thing but are not worth
		// validating; skipping it keeps the bulk writes cheap.
		"validate": false,
		"writes":   writes,
	}, &out)
	require.NoError(t, err, "applyWrites %d %s records", len(rkeys), collection)
	require.Len(t, out.Results, len(rkeys))
	recs := make([]devRecord, len(out.Results))
	for i, r := range out.Results {
		c, err := cid.Decode(r.CID)
		require.NoError(t, err)
		recs[i] = devRecord{path: collection + "/" + r.URI[strings.LastIndex(r.URI, "/")+1:], cid: c}
	}
	return recs
}

// TestIntegrationResumeOverNetwork walks a repo big enough to have a real
// multi-node MST, kills the fetcher partway through, and resumes from the
// checkpoint against the live PDS. It also shows the walk pruning: the
// out-of-range half of the repo is never fetched.
func TestIntegrationResumeOverNetwork(t *testing.T) {
	ctx := context.Background()
	dev := devenv.WithDevEnv(t)
	dir := dev.TestDirectory()
	client := devClient(dev)
	acct := dev.CreateAccount(t)
	t.Logf("account C: %s", acct.DID)

	// Enough records for the MST to be several nodes deep, half of them outside
	// the range.
	const n = 150
	var inRange, outOfRange []devRecord
	for start := 0; start < n; start += 50 {
		var keys []string
		for i := start; i < start+50 && i < n; i++ {
			keys = append(keys, fmt.Sprintf("pad%06d", i))
		}
		// Every record gets distinct content so that every path has a distinct
		// CID: identical records would collapse to one block and hide both
		// mis-addressed records and the real cost of a walk.
		inRange = append(inRange, applyWrites(ctx, t, acct, "place.stream.chat.message", keys, func(rkey string) map[string]any {
			return map[string]any{"text": "padding " + rkey, "createdAt": nowISO(), "streamer": acct.DID}
		})...)
		outOfRange = append(outOfRange, applyWrites(ctx, t, acct, collBelowFar, keys, func(rkey string) map[string]any {
			return map[string]any{"text": "padding " + rkey, "createdAt": nowISO()}
		})...)
		outOfRange = append(outOfRange, applyWrites(ctx, t, acct, collAboveFar, keys, func(rkey string) map[string]any {
			return map[string]any{"note": "padding " + rkey, "createdAt": nowISO()}
		})...)
	}
	t.Logf("wrote %d in-range and %d out-of-range records", len(inRange), len(outOfRange))

	live := &XRPCBlockFetcher{Client: client, DID: acct.DID, ChunkSize: 10}
	head := fetchHead(ctx, t, client, live, dir, acct.DID, "")
	want := recordCIDs(inRange)

	// Baseline: a healthy walk, and a count of the round trips it takes.
	counting := newCountingFetcher(live)
	full, err := (&Walker{Fetcher: counting, BatchSize: 5}).CollectPrefix(ctx, head.Root, "place.stream.")
	require.NoError(t, err)
	require.Equal(t, want, full)
	t.Logf("full walk: %d fetcher calls, %d distinct blocks for %d records",
		counting.calls, len(counting.seen), len(full))
	require.Greater(t, counting.calls, 2, "fixture is too small to test resume")

	// Pruning: no out-of-range record block was ever requested, and the walk
	// touched far fewer blocks than the repo holds.
	for _, r := range outOfRange {
		require.False(t, counting.seen[r.cid], "walk fetched out-of-range record %q", r.path)
	}
	require.Less(t, len(counting.seen), len(inRange)+len(outOfRange),
		"walk touched as many blocks as the repo has records; pruning is not working")

	// How many CIDs may one getBlocks carry? The PDS parses query strings with
	// express's qs, whose default arrayLimit is 20: past that the repeated
	// ?cids= parameters arrive as an object and the request is rejected with
	// "cids/0 must be a cid string". DefaultChunkSize sits at that limit; assert
	// the default works and log what happens just above it, so a future PDS that
	// lifts the limit shows up in the test output instead of silently.
	bigChunk := newCountingFetcher(&XRPCBlockFetcher{Client: client, DID: acct.DID})
	fullBig, err := (&Walker{Fetcher: bigChunk, BatchSize: 200}).CollectPrefix(ctx, head.Root, "place.stream.")
	require.NoError(t, err, "getBlocks with the default chunk size of %d cids", DefaultChunkSize)
	require.Equal(t, want, fullBig)
	t.Logf("full walk at chunk size %d: %d fetcher calls, %d distinct blocks",
		DefaultChunkSize, bigChunk.calls, len(bigChunk.seen))

	for _, chunk := range []int{DefaultChunkSize + 1, 100} {
		over := &XRPCBlockFetcher{Client: client, DID: acct.DID, ChunkSize: chunk}
		_, err := (&Walker{Fetcher: over, BatchSize: 200}).CollectPrefix(ctx, head.Root, "place.stream.")
		t.Logf("getBlocks with chunk size %d: %v", chunk, err)
	}

	// Now kill the fetcher as early as we can while still having made real
	// progress: the earliest failure point that produced both a checkpoint and
	// some emissions.
	var checkpointJSON []byte
	var first []emission
	var walkErr error
	failAfter := 0
	for failAfter = 1; failAfter < counting.calls; failAfter++ {
		checkpointJSON, first = nil, nil
		broken := &failAfterFetcher{inner: live, failAfter: failAfter}
		w := &Walker{
			Fetcher:   broken,
			BatchSize: 5,
			Checkpoint: func(fr *Frontier) error {
				// The walker mutates this frontier in place on the next step, so
				// the checkpoint has to be a snapshot.
				b, err := json.Marshal(fr)
				if err != nil {
					return err
				}
				checkpointJSON = b
				return nil
			},
		}
		walkErr = w.WalkPrefix(ctx, head.Root, "place.stream.", collectVisitor(&first))
		if walkErr != nil && len(checkpointJSON) > 0 && len(first) > 0 {
			break
		}
	}
	require.ErrorIs(t, walkErr, errFetcherBoom)
	require.NotEmpty(t, checkpointJSON, "aborted walk never checkpointed")
	require.NotEmpty(t, first, "aborted walk made no progress")
	require.Less(t, len(first), len(want), "aborted walk should be incomplete")
	t.Logf("aborted after %d fetcher calls: %d of %d records emitted, checkpoint %d bytes",
		failAfter, len(first), len(want), len(checkpointJSON))

	var resumed Frontier
	require.NoError(t, json.Unmarshal(checkpointJSON, &resumed))
	require.Equal(t, head.Root, resumed.Root)
	require.False(t, resumed.Done())

	var second []emission
	require.NoError(t, (&Walker{Fetcher: live, BatchSize: 5}).Resume(ctx, &resumed, collectVisitor(&second)))
	require.True(t, resumed.Done())

	// Emission is at-least-once, so union-and-dedupe rather than counting.
	union := map[string]cid.Cid{}
	for _, e := range append(append([]emission{}, first...), second...) {
		if prev, ok := union[e.path]; ok {
			require.Equal(t, prev, e.cid, "duplicate emission for %q disagreed", e.path)
		}
		union[e.path] = e.cid
	}
	require.Equal(t, want, union, "resumed walk did not cover the full range")
	t.Logf("resume emitted %d records (%d unique across both halves)", len(second), len(union))

	// A warm cache makes the whole walk local: a restart costs nothing remote.
	cache := NewMemoryBlockCache()
	cached := &CachedFetcher{Cache: cache, Inner: newCountingFetcher(live)}
	_, err = (&Walker{Fetcher: cached}).CollectPrefix(ctx, head.Root, "place.stream.")
	require.NoError(t, err)
	inner := newCountingFetcher(live)
	cached = &CachedFetcher{Cache: cache, Inner: inner}
	warm, err := (&Walker{Fetcher: cached}).CollectPrefix(ctx, head.Root, "place.stream.")
	require.NoError(t, err)
	require.Equal(t, want, warm)
	require.Zero(t, inner.calls, "warm walk hit the network")
}
