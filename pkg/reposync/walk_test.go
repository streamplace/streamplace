package reposync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/bluesky-social/indigo/atproto/repo/mst"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// synthetic repo helpers
// ---------------------------------------------------------------------------

// recordBytes builds a minimal but genuinely valid dag-cbor record: the map
// {"t": <path>}. Content doesn't matter to the walker, only that it is stable
// and distinct per path.
func recordBytes(path string) []byte {
	v := []byte(path)
	if len(v) >= 256 {
		panic("test record path too long")
	}
	out := []byte{0xA1, 0x61, 't'} // map(1), text(1) "t"
	if len(v) < 24 {
		out = append(out, byte(0x60|len(v)))
	} else {
		out = append(out, 0x78, byte(len(v)))
	}
	return append(out, v...)
}

func dagCBORCID(t *testing.T, data []byte) cid.Cid {
	t.Helper()
	c, err := cid.NewPrefixV1(cid.DagCBOR, multihash.SHA2_256).Sum(data)
	require.NoError(t, err)
	return c
}

type testRepo struct {
	// blocks is every block of the repo: MST nodes and records.
	blocks map[cid.Cid][]byte
	// records maps MST key -> record CID.
	records map[string]cid.Cid
	// paths maps record CID -> MST key, for assertions about what got fetched.
	paths map[cid.Cid]string
	root  cid.Cid
}

func buildRepo(t *testing.T, paths []string) *testRepo {
	t.Helper()
	tr := &testRepo{
		blocks:  map[cid.Cid][]byte{},
		records: map[string]cid.Cid{},
		paths:   map[cid.Cid]string{},
	}
	tree := mst.NewEmptyTree()
	for _, p := range paths {
		data := recordBytes(p)
		c := dagCBORCID(t, data)
		tr.blocks[c] = data
		tr.records[p] = c
		tr.paths[c] = p
		_, err := tree.Insert([]byte(p), c)
		require.NoError(t, err, "inserting %q", p)
	}
	root, err := tree.RootCID()
	require.NoError(t, err)
	tr.root = *root
	writeNodeBlocks(t, tree.Root, tr.blocks)
	return tr
}

// writeNodeBlocks serializes every node of a fully-computed tree into blocks.
// Tree.RootCID must have been called first so child CIDs are populated.
func writeNodeBlocks(t *testing.T, n *mst.Node, out map[cid.Cid][]byte) {
	t.Helper()
	for _, e := range n.Entries {
		if e.IsChild() {
			require.NotNil(t, e.ChildCID, "child CID not computed")
		}
	}
	nd := n.NodeData()
	data, c, err := nd.Bytes()
	require.NoError(t, err)
	out[*c] = data
	for _, e := range n.Entries {
		if e.Child != nil {
			writeNodeBlocks(t, e.Child, out)
		}
	}
}

// expectedInRange returns the paths of a repo matching prefix, sorted.
func expectedInRange(paths []string, prefix string) []string {
	r := PrefixRange(prefix)
	var out []string
	for _, p := range paths {
		if r.contains([]byte(p)) {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// test fetcher
// ---------------------------------------------------------------------------

var errFetcherBoom = errors.New("simulated fetcher failure")

// testFetcher serves blocks out of a map with the same guarantees a real
// BlockFetcher must offer, plus knobs for the adversarial cases.
type testFetcher struct {
	blocks map[cid.Cid][]byte
	// log records every CID ever requested, in order.
	log []cid.Cid
	// calls counts GetBlocks invocations.
	calls int
	// failAfter, when > 0, makes every call past that number fail.
	failAfter int
	// tamper substitutes wrong bytes for these CIDs, as a lying host would.
	tamper map[cid.Cid]bool
	// omit drops these CIDs from responses entirely.
	omit map[cid.Cid]bool
}

func newTestFetcher(tr *testRepo) *testFetcher {
	return &testFetcher{
		blocks: tr.blocks,
		tamper: map[cid.Cid]bool{},
		omit:   map[cid.Cid]bool{},
	}
}

func (f *testFetcher) GetBlocks(ctx context.Context, cids []cid.Cid) (map[cid.Cid][]byte, error) {
	f.calls++
	f.log = append(f.log, cids...)
	if f.failAfter > 0 && f.calls > f.failAfter {
		return nil, errFetcherBoom
	}
	out := map[cid.Cid][]byte{}
	for _, c := range dedupeCIDs(cids) {
		if f.omit[c] {
			continue
		}
		data, ok := f.blocks[c]
		if !ok {
			return nil, fmt.Errorf("%w: %s not in test repo", ErrMissingBlock, c)
		}
		if f.tamper[c] {
			data = append([]byte("tampered:"), data...)
		}
		if err := VerifyBlock(c, data); err != nil {
			return nil, err
		}
		out[c] = data
	}
	for _, c := range cids {
		if _, ok := out[c]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrMissingBlock, c)
		}
	}
	return out, nil
}

// distinctFetched returns the set of CIDs the fetcher was ever asked for.
func (f *testFetcher) distinctFetched() map[cid.Cid]bool {
	out := map[cid.Cid]bool{}
	for _, c := range f.log {
		out[c] = true
	}
	return out
}

type emission struct {
	path string
	cid  cid.Cid
	data []byte
}

func collectVisitor(dst *[]emission) RecordVisitor {
	return func(path string, rcid cid.Cid, rec []byte) error {
		*dst = append(*dst, emission{path: path, cid: rcid, data: append([]byte(nil), rec...)})
		return nil
	}
}

func emittedPaths(es []emission) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.path
	}
	return out
}

// ---------------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------------

// exactnessPaths mixes in-range place.stream.* records with out-of-range
// records on both sides, including keys immediately adjacent to the
// ["place.stream.", "place.stream/") boundaries.
func exactnessPaths() []string {
	return []string{
		// well below the range
		"app.bsky.feed.post/3lbaaaaaaaa22",
		"app.bsky.feed.post/3lbaaaaaaaa23",
		"app.bsky.graph.follow/3lbaaaaaaaa24",
		// immediately below "place.stream."
		"place.strea.thing/3lbaaaaaaaa25",
		"place.stream-adjacent/3lbaaaaaaaa26",
		// in range
		"place.stream.a/3lbaaaaaaaa27",
		"place.stream.chat.message/3lbaaaaaaaa28",
		"place.stream.chat.message/3lbaaaaaaaa29",
		"place.stream.chat.message/3lbaaaaaaaa2a",
		"place.stream.chat.profile/self",
		"place.stream.livestream/3lbaaaaaaaa2b",
		"place.stream.media.origin/3lbaaaaaaaa2c",
		"place.stream.media.origin/3lbaaaaaaaa2d",
		"place.stream.media.origin/3lbaaaaaaaa2e",
		"place.stream.zzzzzzzzzz/3lbaaaaaaaa2f",
		// immediately above "place.stream/"
		"place.stream0.thing/3lbaaaaaaaa2g",
		"place.streamx.thing/3lbaaaaaaaa2h",
		// well above
		"xyz.example.thing/3lbaaaaaaaa2i",
		"zzz.last.thing/3lbaaaaaaaa2j",
	}
}

func TestPrefixRange(t *testing.T) {
	r := PrefixRange("place.stream.")
	require.Equal(t, "place.stream.", string(r.Lo))
	require.Equal(t, "place.stream/", string(r.Hi))

	require.True(t, r.contains([]byte("place.stream.a/1")))
	require.True(t, r.contains([]byte("place.stream.")))
	require.False(t, r.contains([]byte("place.stream-adjacent/1")))
	require.False(t, r.contains([]byte("place.stream/")))
	require.False(t, r.contains([]byte("place.stream0.thing/1")))

	// 0xFF rollover: the prefix is unbounded above.
	roll := PrefixRange("a\xff\xff")
	require.Equal(t, "a", string(roll.Lo[:1]))
	require.Equal(t, "b", string(roll.Hi))

	all := PrefixRange("")
	require.Nil(t, all.Hi)
	require.True(t, all.contains([]byte("anything")))
}

// Case 1: the walk emits exactly the in-range records, with correct bytes, in
// key order.
func TestWalkPrefixExactness(t *testing.T) {
	ctx := context.Background()
	paths := exactnessPaths()
	tr := buildRepo(t, paths)
	f := newTestFetcher(tr)

	for _, batch := range []int{1, 2, 50} {
		t.Run(fmt.Sprintf("batch%d", batch), func(t *testing.T) {
			var got []emission
			w := &Walker{Fetcher: f, BatchSize: batch}
			require.NoError(t, w.WalkPrefix(ctx, tr.root, "place.stream.", collectVisitor(&got)))

			want := expectedInRange(paths, "place.stream.")
			require.Len(t, want, 10, "fixture should have 10 in-range records")
			require.Equal(t, want, emittedPaths(got), "emitted paths, in key order")
			for _, e := range got {
				require.Equal(t, tr.records[e.path], e.cid, "record cid for %q", e.path)
				require.Equal(t, recordBytes(e.path), e.data, "record bytes for %q", e.path)
			}
		})
	}
}

// Case 2: a repo whose out-of-range half is deep gets pruned; we touch only a
// small fraction of the blocks and never pull an out-of-range record.
func TestWalkPrefixPrunes(t *testing.T) {
	ctx := context.Background()
	paths := exactnessPaths()
	for i := 0; i < 2000; i++ {
		paths = append(paths, fmt.Sprintf("app.bsky.feed.post/3lbpost%06d", i))
	}
	tr := buildRepo(t, paths)
	f := newTestFetcher(tr)

	var got []emission
	w := &Walker{Fetcher: f}
	require.NoError(t, w.WalkPrefix(ctx, tr.root, "place.stream.", collectVisitor(&got)))
	require.Equal(t, expectedInRange(paths, "place.stream."), emittedPaths(got))

	fetched := f.distinctFetched()
	t.Logf("fetched %d distinct blocks of %d in the repo", len(fetched), len(tr.blocks))
	require.Less(t, len(fetched), len(tr.blocks)/10,
		"fetched %d of %d blocks; pruning is not working", len(fetched), len(tr.blocks))

	inRange := PrefixRange("place.stream.")
	for c := range fetched {
		path, isRecord := tr.paths[c]
		if !isRecord {
			continue // MST node; walking out-of-range-adjacent nodes is expected
		}
		require.True(t, inRange.contains([]byte(path)), "fetched out-of-range record %q", path)
	}
}

// Case 3: nothing in range, and a completely empty repo.
func TestWalkPrefixEmptyResults(t *testing.T) {
	ctx := context.Background()

	t.Run("no matching records", func(t *testing.T) {
		paths := []string{
			"app.bsky.feed.post/3lbaaaaaaaa22",
			"app.bsky.feed.post/3lbaaaaaaaa23",
			"xyz.example.thing/3lbaaaaaaaa24",
		}
		tr := buildRepo(t, paths)
		var got []emission
		w := &Walker{Fetcher: newTestFetcher(tr)}
		require.NoError(t, w.WalkPrefix(ctx, tr.root, "place.stream.", collectVisitor(&got)))
		require.Empty(t, got)
	})

	t.Run("empty repo", func(t *testing.T) {
		tr := buildRepo(t, nil)
		require.Len(t, tr.blocks, 1, "an empty repo is one empty MST node")
		var got []emission
		w := &Walker{Fetcher: newTestFetcher(tr)}
		require.NoError(t, w.WalkPrefix(ctx, tr.root, "place.stream.", collectVisitor(&got)))
		require.Empty(t, got)
	})
}

// Case 4: a host that returns the wrong bytes for a CID is caught, whether the
// block is an MST node or a record.
func TestWalkTamperedBlockFails(t *testing.T) {
	ctx := context.Background()
	paths := exactnessPaths()
	tr := buildRepo(t, paths)

	// Learn which blocks the walk actually touches, then corrupt each in turn.
	probe := newTestFetcher(tr)
	var probed []emission
	require.NoError(t, (&Walker{Fetcher: probe}).WalkPrefix(ctx, tr.root, "place.stream.", collectVisitor(&probed)))

	var touchedNodes, touchedRecords []cid.Cid
	for c := range probe.distinctFetched() {
		if _, isRecord := tr.paths[c]; isRecord {
			touchedRecords = append(touchedRecords, c)
		} else {
			touchedNodes = append(touchedNodes, c)
		}
	}
	require.NotEmpty(t, touchedNodes)
	require.NotEmpty(t, touchedRecords)

	for _, tc := range []struct {
		name string
		cid  cid.Cid
	}{
		{"mst node", touchedNodes[0]},
		{"record", touchedRecords[0]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newTestFetcher(tr)
			f.tamper[tc.cid] = true
			var got []emission
			err := (&Walker{Fetcher: f}).WalkPrefix(ctx, tr.root, "place.stream.", collectVisitor(&got))
			require.Error(t, err)
			require.ErrorIs(t, err, ErrBlockMismatch)
			require.NotEqual(t, expectedInRange(paths, "place.stream."), emittedPaths(got))
		})
	}
}

// Case 5: a host that quietly drops a block must not produce a successful walk
// over a subset.
func TestWalkOmittedBlockFails(t *testing.T) {
	ctx := context.Background()
	paths := exactnessPaths()
	tr := buildRepo(t, paths)

	probe := newTestFetcher(tr)
	var probed []emission
	require.NoError(t, (&Walker{Fetcher: probe}).WalkPrefix(ctx, tr.root, "place.stream.", collectVisitor(&probed)))

	var candidates []cid.Cid
	for c := range probe.distinctFetched() {
		candidates = append(candidates, c)
	}
	require.NotEmpty(t, candidates)
	// deterministic ordering so failures are reproducible
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].String() < candidates[j].String() })

	for _, c := range candidates {
		f := newTestFetcher(tr)
		f.omit[c] = true
		var got []emission
		err := (&Walker{Fetcher: f}).WalkPrefix(ctx, tr.root, "place.stream.", collectVisitor(&got))
		require.Error(t, err, "omitting %s produced a successful walk", c)
		require.ErrorIs(t, err, ErrMissingBlock)
		require.NotEqual(t, expectedInRange(paths, "place.stream."), emittedPaths(got),
			"omitting %s still emitted the full set", c)
	}
}

// Case 6: a walk interrupted mid-flight resumes from its last checkpoint with no
// records lost (emission is at-least-once, so duplicates are allowed).
func TestWalkResume(t *testing.T) {
	ctx := context.Background()
	paths := exactnessPaths()
	for i := 0; i < 300; i++ {
		paths = append(paths, fmt.Sprintf("place.stream.media.origin/3lbmedia%06d", i))
		paths = append(paths, fmt.Sprintf("app.bsky.feed.post/3lbpost%06d", i))
	}
	tr := buildRepo(t, paths)
	want := expectedInRange(paths, "place.stream.")

	var checkpointJSON []byte
	broken := newTestFetcher(tr)
	broken.failAfter = 12
	var first []emission
	w := &Walker{
		Fetcher:   broken,
		BatchSize: 4,
		Checkpoint: func(fr *Frontier) error {
			b, err := json.Marshal(fr)
			require.NoError(t, err)
			checkpointJSON = b
			return nil
		},
	}
	err := w.WalkPrefix(ctx, tr.root, "place.stream.", collectVisitor(&first))
	require.ErrorIs(t, err, errFetcherBoom)
	require.NotEmpty(t, checkpointJSON, "walk should have checkpointed before failing")
	require.NotEmpty(t, first, "the aborted walk should have made some progress")
	require.NotEqual(t, want, emittedPaths(first), "the aborted walk should be incomplete")
	t.Logf("aborted walk emitted %d of %d records before failing", len(first), len(want))

	var resumed Frontier
	require.NoError(t, json.Unmarshal(checkpointJSON, &resumed))
	require.Equal(t, tr.root, resumed.Root)
	require.False(t, resumed.Done())

	healthy := newTestFetcher(tr)
	var second []emission
	require.NoError(t, (&Walker{Fetcher: healthy, BatchSize: 4}).Resume(ctx, &resumed, collectVisitor(&second)))
	require.True(t, resumed.Done())

	union := map[string]cid.Cid{}
	for _, e := range append(append([]emission{}, first...), second...) {
		if prev, ok := union[e.path]; ok {
			require.Equal(t, prev, e.cid, "duplicate emission for %q disagreed", e.path)
		}
		union[e.path] = e.cid
	}
	got := make([]string, 0, len(union))
	for p := range union {
		got = append(got, p)
	}
	sort.Strings(got)
	require.Equal(t, want, got)
	for _, p := range want {
		require.Equal(t, tr.records[p], union[p])
	}
}

// A warm cache makes a repeat walk entirely local.
func TestWalkCachedFetcherWarmCacheDoesNoRemoteWork(t *testing.T) {
	ctx := context.Background()
	paths := exactnessPaths()
	tr := buildRepo(t, paths)
	want := expectedInRange(paths, "place.stream.")

	inner := newTestFetcher(tr)
	cached := &CachedFetcher{Cache: NewMemoryBlockCache(), Inner: inner}

	var cold []emission
	require.NoError(t, (&Walker{Fetcher: cached}).WalkPrefix(ctx, tr.root, "place.stream.", collectVisitor(&cold)))
	require.Equal(t, want, emittedPaths(cold))
	require.Greater(t, inner.calls, 0)

	callsAfterCold := inner.calls
	var warm []emission
	require.NoError(t, (&Walker{Fetcher: cached}).WalkPrefix(ctx, tr.root, "place.stream.", collectVisitor(&warm)))
	require.Equal(t, want, emittedPaths(warm))
	require.Equal(t, callsAfterCold, inner.calls, "warm walk hit the network")
}

// Case 7: two disjoint ranges in one pass.
func TestWalkRangesMultipleRanges(t *testing.T) {
	ctx := context.Background()
	paths := exactnessPaths()
	tr := buildRepo(t, paths)

	ranges := []KeyRange{PrefixRange("place.stream."), PrefixRange("app.bsky.graph.follow/")}
	var got []emission
	require.NoError(t, (&Walker{Fetcher: newTestFetcher(tr), BatchSize: 2}).
		WalkRanges(ctx, tr.root, ranges, collectVisitor(&got)))

	want := append(expectedInRange(paths, "app.bsky.graph.follow/"), expectedInRange(paths, "place.stream.")...)
	sort.Strings(want)
	require.Equal(t, want, emittedPaths(got))
	require.Contains(t, want, "app.bsky.graph.follow/3lbaaaaaaaa24")
}

func TestNormalizeRanges(t *testing.T) {
	_, err := normalizeRanges(nil)
	require.Error(t, err)

	_, err = normalizeRanges([]KeyRange{{Lo: []byte("b"), Hi: []byte("a")}})
	require.Error(t, err)

	got, err := normalizeRanges([]KeyRange{
		{Lo: []byte("m"), Hi: []byte("n")},
		{Lo: []byte("a"), Hi: []byte("c")},
		{Lo: []byte("b"), Hi: []byte("d")},
	})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "a", string(got[0].Lo))
	require.Equal(t, "d", string(got[0].Hi))
	require.Equal(t, "m", string(got[1].Lo))

	got, err = normalizeRanges([]KeyRange{
		{Lo: []byte("a"), Hi: nil},
		{Lo: []byte("m"), Hi: []byte("n")},
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Nil(t, got[0].Hi)
}

func TestCollectAndDiff(t *testing.T) {
	ctx := context.Background()
	paths := exactnessPaths()
	tr := buildRepo(t, paths)
	w := &Walker{Fetcher: newTestFetcher(tr)}

	before, err := w.CollectPrefix(ctx, tr.root, "place.stream.")
	require.NoError(t, err)
	require.Len(t, before, 10)

	// Mutate: add one record, delete one, change one.
	var next []string
	for _, p := range paths {
		if p == "place.stream.chat.message/3lbaaaaaaaa29" {
			continue // deleted
		}
		next = append(next, p)
	}
	next = append(next, "place.stream.chat.message/3lbaaaaaaaa2z") // created
	tr2 := buildRepo(t, next)
	// simulate an update in place by pointing an existing path at other bytes
	updatedPath := "place.stream.chat.profile/self"
	otherData := recordBytes("place.stream.chat.profile/self#v2")
	otherCID := dagCBORCID(t, otherData)
	tr2.blocks[otherCID] = otherData

	w2 := &Walker{Fetcher: newTestFetcher(tr2)}
	after, err := w2.CollectPrefix(ctx, tr2.root, "place.stream.")
	require.NoError(t, err)
	after[updatedPath] = otherCID

	d := DiffCollections(before, after)
	require.Equal(t, []string{"place.stream.chat.message/3lbaaaaaaaa2z"}, d.Created)
	require.Equal(t, []string{"place.stream.chat.message/3lbaaaaaaaa29"}, d.Deleted)
	require.Equal(t, []string{updatedPath}, d.Updated)
	require.False(t, d.Empty())

	require.True(t, DiffCollections(before, before).Empty())
}

func TestVisitorErrorAborts(t *testing.T) {
	ctx := context.Background()
	tr := buildRepo(t, exactnessPaths())
	boom := errors.New("visitor said no")
	err := (&Walker{Fetcher: newTestFetcher(tr)}).WalkPrefix(ctx, tr.root, "place.stream.",
		func(path string, rcid cid.Cid, rec []byte) error { return boom })
	require.ErrorIs(t, err, boom)
}

func TestCheckNodeDataRejectsBadPrefixLen(t *testing.T) {
	// A correctly-hashed but hostile node whose prefix compression points past
	// the previous key would panic inside mst.NodeData.Node.
	nd := &mst.NodeData{Entries: []mst.EntryData{{PrefixLen: 5, KeySuffix: []byte("x")}}}
	require.Error(t, checkNodeData(nd))

	nd = &mst.NodeData{Entries: []mst.EntryData{{PrefixLen: -1, KeySuffix: []byte("x")}}}
	require.Error(t, checkNodeData(nd))

	nd = &mst.NodeData{Entries: []mst.EntryData{{PrefixLen: 0, KeySuffix: []byte("abc")}, {PrefixLen: 2, KeySuffix: []byte("z")}}}
	require.NoError(t, checkNodeData(nd))
}

// A node block can hash correctly and still be nonsense; the walker must not
// trust the key order or bounds a node claims.
func TestExpandNodeRejectsMalformedNodes(t *testing.T) {
	val := dagCBORCID(t, recordBytes("whatever"))
	encode := func(t *testing.T, entries []mst.EntryData) (pendingEntry, []byte) {
		t.Helper()
		nd := &mst.NodeData{Entries: entries}
		data, c, err := nd.Bytes()
		require.NoError(t, err)
		return pendingEntry{CID: *c}, data
	}
	all := []KeyRange{PrefixRange("")}

	t.Run("keys out of order", func(t *testing.T) {
		p, data := encode(t, []mst.EntryData{
			{PrefixLen: 0, KeySuffix: []byte("b/1"), Value: val},
			{PrefixLen: 0, KeySuffix: []byte("a/1"), Value: val},
		})
		_, err := expandNode(p, data, all)
		require.ErrorIs(t, err, ErrInvalidNode)
	})

	t.Run("duplicate keys", func(t *testing.T) {
		p, data := encode(t, []mst.EntryData{
			{PrefixLen: 0, KeySuffix: []byte("a/1"), Value: val},
			{PrefixLen: 2, KeySuffix: []byte("1"), Value: val},
		})
		_, err := expandNode(p, data, all)
		require.ErrorIs(t, err, ErrInvalidNode)
	})

	t.Run("key below inherited bound", func(t *testing.T) {
		p, data := encode(t, []mst.EntryData{{PrefixLen: 0, KeySuffix: []byte("m/1"), Value: val}})
		p.Lo = []byte("z/1")
		_, err := expandNode(p, data, all)
		require.ErrorIs(t, err, ErrInvalidNode)
	})

	t.Run("key above inherited bound", func(t *testing.T) {
		p, data := encode(t, []mst.EntryData{{PrefixLen: 0, KeySuffix: []byte("m/1"), Value: val}})
		p.Hi = []byte("a/1")
		_, err := expandNode(p, data, all)
		require.ErrorIs(t, err, ErrInvalidNode)
	})

	t.Run("in bounds is fine", func(t *testing.T) {
		p, data := encode(t, []mst.EntryData{{PrefixLen: 0, KeySuffix: []byte("m/1"), Value: val}})
		p.Lo, p.Hi = []byte("a/1"), []byte("z/1")
		out, err := expandNode(p, data, all)
		require.NoError(t, err)
		require.Len(t, out, 1)
		require.Equal(t, "m/1", string(out[0].Key))
	})

	t.Run("undecodable block", func(t *testing.T) {
		_, err := expandNode(pendingEntry{}, []byte("not cbor at all"), all)
		require.ErrorIs(t, err, ErrInvalidNode)
	})
}

// Child bounds must bracket a subtree by the neighbouring value keys, so a
// subtree that cannot hold an in-range key is never fetched.
func TestExpandNodeBracketsChildren(t *testing.T) {
	val := dagCBORCID(t, recordBytes("whatever"))
	left := dagCBORCID(t, []byte("left"))
	mid := dagCBORCID(t, []byte("mid"))
	right := dagCBORCID(t, []byte("right"))

	nd := &mst.NodeData{
		Left: &left,
		Entries: []mst.EntryData{
			{PrefixLen: 0, KeySuffix: []byte("d/1"), Value: val, Right: &mid},
			{PrefixLen: 0, KeySuffix: []byte("k/1"), Value: val, Right: &right},
		},
	}
	data, c, err := nd.Bytes()
	require.NoError(t, err)
	p := pendingEntry{CID: *c, Lo: []byte("a/1"), Hi: []byte("z/1")}

	out, err := expandNode(p, data, []KeyRange{PrefixRange("")})
	require.NoError(t, err)
	require.Len(t, out, 5)
	require.Equal(t, []pendingEntry{
		{CID: left, Lo: []byte("a/1"), Hi: []byte("d/1")},
		{CID: val, Key: []byte("d/1")},
		{CID: mid, Lo: []byte("d/1"), Hi: []byte("k/1")},
		{CID: val, Key: []byte("k/1")},
		{CID: right, Lo: []byte("k/1"), Hi: []byte("z/1")},
	}, out)

	// A range that only touches the middle subtree keeps just that child.
	out, err = expandNode(p, data, []KeyRange{{Lo: []byte("e/1"), Hi: []byte("f/1")}})
	require.NoError(t, err)
	require.Equal(t, []pendingEntry{{CID: mid, Lo: []byte("d/1"), Hi: []byte("k/1")}}, out)

	// A range entirely below the node's own lower bound keeps nothing.
	out, err = expandNode(p, data, []KeyRange{{Lo: []byte("A/1"), Hi: []byte("B/1")}})
	require.NoError(t, err)
	require.Empty(t, out)

	// A range just above the node's lower bound still needs the leftmost child:
	// "a/11" would live there.
	out, err = expandNode(p, data, []KeyRange{{Lo: []byte("a/1"), Hi: []byte("a/2")}})
	require.NoError(t, err)
	require.Equal(t, []pendingEntry{{CID: left, Lo: []byte("a/1"), Hi: []byte("d/1")}}, out)
}

func TestFrontierJSONRoundTrip(t *testing.T) {
	tr := buildRepo(t, exactnessPaths())
	fr := &Frontier{
		Root:   tr.root,
		Ranges: []KeyRange{{Lo: []byte("place.stream."), Hi: []byte("place.stream/")}, {Lo: []byte("z"), Hi: nil}},
		Pending: []pendingEntry{
			{CID: tr.root, Lo: []byte("a"), Hi: []byte("b")},
			{CID: tr.records["place.stream.chat.profile/self"], Key: []byte("place.stream.chat.profile/self")},
		},
	}
	b, err := json.Marshal(fr)
	require.NoError(t, err)

	var back Frontier
	require.NoError(t, json.Unmarshal(b, &back))
	require.Equal(t, *fr, back)
}
