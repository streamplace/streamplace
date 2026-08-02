package reposync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/bluesky-social/indigo/atproto/repo/mst"
	"github.com/ipfs/go-cid"
)

// ErrInvalidNode is returned when an MST node block is structurally bogus: keys
// out of order, keys outside the bounds the parent promised, unusable prefix
// compression. The block hashed correctly, so this means the repo itself is
// malformed rather than the transport being lossy.
var ErrInvalidNode = errors.New("invalid MST node")

// DefaultBatchSize is how many frontier entries a walk step resolves at once.
const DefaultBatchSize = 50

// KeyRange is a half-open bytewise key range [Lo, Hi).
//
// A nil Hi means "unbounded above". Lo is inclusive; an empty Lo is the smallest
// possible key, so it also means "unbounded below".
type KeyRange struct {
	Lo []byte
	Hi []byte
}

// PrefixRange is the range of all keys starting with prefix.
func PrefixRange(prefix string) KeyRange {
	lo := []byte(prefix)
	hi := append([]byte(nil), lo...)
	for i := len(hi) - 1; i >= 0; i-- {
		if hi[i] != 0xFF {
			hi[i]++
			return KeyRange{Lo: lo, Hi: hi[:i+1]}
		}
	}
	// All-0xFF (or empty) prefix: nothing sorts above it.
	return KeyRange{Lo: lo, Hi: nil}
}

func (r KeyRange) contains(key []byte) bool {
	if bytes.Compare(key, r.Lo) < 0 {
		return false
	}
	return r.Hi == nil || bytes.Compare(key, r.Hi) < 0
}

func (r KeyRange) String() string {
	if r.Hi == nil {
		return fmt.Sprintf("[%q, +inf)", r.Lo)
	}
	return fmt.Sprintf("[%q, %q)", r.Lo, r.Hi)
}

// normalizeRanges validates, sorts and merges overlapping/abutting ranges so the
// intersection tests below can stay simple.
func normalizeRanges(in []KeyRange) ([]KeyRange, error) {
	if len(in) == 0 {
		return nil, errors.New("no key ranges given")
	}
	out := make([]KeyRange, 0, len(in))
	for _, r := range in {
		if r.Hi != nil && bytes.Compare(r.Lo, r.Hi) >= 0 {
			return nil, fmt.Errorf("empty or inverted key range %s", r)
		}
		out = append(out, KeyRange{Lo: append([]byte(nil), r.Lo...), Hi: append([]byte(nil), r.Hi...)})
	}
	sort.Slice(out, func(i, j int) bool { return bytes.Compare(out[i].Lo, out[j].Lo) < 0 })
	merged := make([]KeyRange, 0, len(out))
	merged = append(merged, out[0])
	for _, r := range out[1:] {
		last := &merged[len(merged)-1]
		if last.Hi == nil {
			continue // last already runs to +inf, swallows everything after it
		}
		if bytes.Compare(r.Lo, last.Hi) <= 0 {
			if r.Hi == nil || bytes.Compare(r.Hi, last.Hi) > 0 {
				last.Hi = r.Hi
			}
			continue
		}
		merged = append(merged, r)
	}
	return merged, nil
}

func rangesContain(ranges []KeyRange, key []byte) bool {
	for _, r := range ranges {
		if r.contains(key) {
			return true
		}
	}
	return false
}

// rangesIntersectSubtree reports whether any wanted range could contain a key
// from a subtree whose keys are strictly between lo and hi (nil meaning
// unbounded). The test is conservative: it may say yes for a subtree that turns
// out to hold nothing in range, which costs a fetch but never loses a record.
func rangesIntersectSubtree(ranges []KeyRange, lo, hi []byte) bool {
	for _, r := range ranges {
		// Need some key k with lo < k < hi and r.Lo <= k < r.Hi.
		if r.Hi != nil && lo != nil && bytes.Compare(lo, r.Hi) >= 0 {
			continue
		}
		if hi != nil && bytes.Compare(r.Lo, hi) >= 0 {
			continue
		}
		return true
	}
	return false
}

// pendingEntry is one unresolved piece of a walk.
//
// If Key is nil it is an MST subtree that still needs to be fetched and
// expanded, and Lo/Hi are the exclusive bounds the tree structure guarantees for
// every key inside it. If Key is non-nil it is an in-range record at that key
// whose block has not been fetched and emitted yet, and Lo/Hi are unused.
type pendingEntry struct {
	CID cid.Cid
	Key []byte
	Lo  []byte
	Hi  []byte
}

func (p pendingEntry) isRecord() bool { return p.Key != nil }

// Frontier is the complete state of an in-progress walk, and is the unit of
// checkpointing. Pending is kept in ascending key order, which is what lets a
// walk emit records in key order without holding the whole tree in memory.
type Frontier struct {
	Root    cid.Cid
	Ranges  []KeyRange
	Pending []pendingEntry
}

// Done reports whether the walk this frontier describes has nothing left to do.
func (fr *Frontier) Done() bool { return len(fr.Pending) == 0 }

type rangeDTO struct {
	Lo []byte `json:"lo"`
	Hi []byte `json:"hi"`
}

type pendingDTO struct {
	CID string `json:"cid"`
	Key []byte `json:"key,omitempty"`
	Lo  []byte `json:"lo,omitempty"`
	Hi  []byte `json:"hi,omitempty"`
}

type frontierDTO struct {
	Root    string       `json:"root"`
	Ranges  []rangeDTO   `json:"ranges"`
	Pending []pendingDTO `json:"pending"`
}

func (fr Frontier) MarshalJSON() ([]byte, error) {
	dto := frontierDTO{
		Root:    fr.Root.String(),
		Ranges:  make([]rangeDTO, len(fr.Ranges)),
		Pending: make([]pendingDTO, len(fr.Pending)),
	}
	for i, r := range fr.Ranges {
		dto.Ranges[i] = rangeDTO(r)
	}
	for i, p := range fr.Pending {
		dto.Pending[i] = pendingDTO{CID: p.CID.String(), Key: p.Key, Lo: p.Lo, Hi: p.Hi}
	}
	return json.Marshal(dto)
}

func (fr *Frontier) UnmarshalJSON(b []byte) error {
	var dto frontierDTO
	if err := json.Unmarshal(b, &dto); err != nil {
		return err
	}
	root, err := cid.Decode(dto.Root)
	if err != nil {
		return fmt.Errorf("frontier root %q: %w", dto.Root, err)
	}
	out := Frontier{
		Root:    root,
		Ranges:  make([]KeyRange, len(dto.Ranges)),
		Pending: make([]pendingEntry, len(dto.Pending)),
	}
	for i, r := range dto.Ranges {
		out.Ranges[i] = KeyRange(r)
	}
	for i, p := range dto.Pending {
		c, err := cid.Decode(p.CID)
		if err != nil {
			return fmt.Errorf("frontier pending cid %q: %w", p.CID, err)
		}
		out.Pending[i] = pendingEntry{CID: c, Key: p.Key, Lo: p.Lo, Hi: p.Hi}
	}
	*fr = out
	return nil
}

// RecordVisitor is called once per in-range record, in ascending key order.
// path is the MST key ("collection/rkey"), rcid the record's CID and rec its
// verified dag-cbor bytes. Returning an error aborts the walk.
//
// Visitors must be idempotent: see the package docs on at-least-once emission.
type RecordVisitor func(path string, rcid cid.Cid, rec []byte) error

// Walker performs prefix-bounded MST walks against a [BlockFetcher].
type Walker struct {
	Fetcher BlockFetcher
	// BatchSize caps how many frontier entries are resolved per step, and so
	// how many blocks are requested per round trip. Zero means
	// [DefaultBatchSize].
	BatchSize int
	// Checkpoint, if set, is called with the frontier after every completed
	// step (that is, after that step's records have been emitted). Returning an
	// error aborts the walk and rolls the frontier back to before the step, so
	// a retried Resume re-emits the step's records; this makes it safe to
	// commit visitor effects and the frontier together inside Checkpoint.
	Checkpoint func(*Frontier) error
}

// WalkPrefix visits every record in the tree at root whose key starts with
// prefix.
func (w *Walker) WalkPrefix(ctx context.Context, root cid.Cid, prefix string, visit RecordVisitor) error {
	return w.WalkRanges(ctx, root, []KeyRange{PrefixRange(prefix)}, visit)
}

// WalkRanges visits every record in the tree at root whose key falls in any of
// ranges. Ranges need not be sorted or disjoint; they are normalized first.
func (w *Walker) WalkRanges(ctx context.Context, root cid.Cid, ranges []KeyRange, visit RecordVisitor) error {
	norm, err := normalizeRanges(ranges)
	if err != nil {
		return err
	}
	fr := &Frontier{
		Root:    root,
		Ranges:  norm,
		Pending: []pendingEntry{{CID: root}},
	}
	return w.Resume(ctx, fr, visit)
}

// Resume continues a walk from a checkpointed frontier. fr is updated in place
// as the walk progresses, so an aborted Resume leaves fr at the last completed
// step — where a step only counts as completed once its Checkpoint call (if
// any) has succeeded — and can be called again.
func (w *Walker) Resume(ctx context.Context, fr *Frontier, visit RecordVisitor) error {
	if fr == nil {
		return errors.New("nil frontier")
	}
	if len(fr.Ranges) == 0 {
		return errors.New("frontier has no key ranges")
	}
	for !fr.Done() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := w.step(ctx, fr, visit); err != nil {
			return err
		}
	}
	return nil
}

// step resolves the head of the frontier: expand any subtrees in it, then emit
// the run of records that is now known to sort before everything still
// unresolved. The frontier is only advanced after the visitor has accepted those
// records, which is what makes emission at-least-once rather than lossy.
func (w *Walker) step(ctx context.Context, fr *Frontier, visit RecordVisitor) error {
	batch := w.BatchSize
	if batch <= 0 {
		batch = DefaultBatchSize
	}
	if batch > len(fr.Pending) {
		batch = len(fr.Pending)
	}
	head := fr.Pending[:batch]
	rest := fr.Pending[batch:]

	var nodeCIDs []cid.Cid
	for _, p := range head {
		if !p.isRecord() {
			nodeCIDs = append(nodeCIDs, p.CID)
		}
	}
	if len(nodeCIDs) > 0 {
		blocks, err := w.Fetcher.GetBlocks(ctx, nodeCIDs)
		if err != nil {
			return fmt.Errorf("fetching %d MST nodes: %w", len(nodeCIDs), err)
		}
		expanded := make([]pendingEntry, 0, len(head))
		for _, p := range head {
			if p.isRecord() {
				expanded = append(expanded, p)
				continue
			}
			data, ok := blocks[p.CID]
			if !ok {
				return fmt.Errorf("%w: MST node %s", ErrMissingBlock, p.CID)
			}
			children, err := expandNode(p, data, fr.Ranges)
			if err != nil {
				return err
			}
			expanded = append(expanded, children...)
		}
		head = expanded
	}

	// Everything up to the first unresolved subtree is now known to be the next
	// records in key order.
	n := 0
	for n < len(head) && head[n].isRecord() {
		n++
	}
	emit := head[:n]

	next := make([]pendingEntry, 0, len(head)-n+len(rest))
	next = append(next, head[n:]...)
	next = append(next, rest...)

	if len(emit) > 0 {
		recCIDs := make([]cid.Cid, len(emit))
		for i, p := range emit {
			recCIDs[i] = p.CID
		}
		blocks, err := w.Fetcher.GetBlocks(ctx, recCIDs)
		if err != nil {
			return fmt.Errorf("fetching %d records: %w", len(recCIDs), err)
		}
		for _, p := range emit {
			data, ok := blocks[p.CID]
			if !ok {
				return fmt.Errorf("%w: record %s at %q", ErrMissingBlock, p.CID, p.Key)
			}
			if err := visit(string(p.Key), p.CID, data); err != nil {
				return fmt.Errorf("visiting %q: %w", p.Key, err)
			}
		}
	}

	prev := fr.Pending
	fr.Pending = next
	if w.Checkpoint != nil {
		if err := w.Checkpoint(fr); err != nil {
			// Roll back so a retried Resume re-emits this step's records. A
			// caller that commits visitor effects inside Checkpoint would
			// otherwise lose them: the failed commit discards the effects and
			// the advanced frontier would never emit those records again.
			fr.Pending = prev
			return fmt.Errorf("checkpointing frontier: %w", err)
		}
	}
	return nil
}

// expandNode turns one fetched MST node block into the frontier entries it
// contributes, in key order, dropping subtrees and values that cannot be in
// range.
func expandNode(p pendingEntry, data []byte, ranges []KeyRange) ([]pendingEntry, error) {
	nd, err := mst.NodeDataFromCBOR(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w %s: %w", ErrInvalidNode, p.CID, err)
	}
	if err := checkNodeData(nd); err != nil {
		return nil, fmt.Errorf("%w %s: %w", ErrInvalidNode, p.CID, err)
	}
	node := nd.Node(&p.CID)

	out := make([]pendingEntry, 0, len(node.Entries))
	// lo tracks the exclusive lower bound for the next child pointer: the key of
	// the value entry just before it, or this node's own inherited lower bound
	// for the leftmost child. The matching upper bound is the key of the value
	// entry just after it, or this node's inherited upper bound for the
	// rightmost child.
	lo := p.Lo
	var prevKey []byte
	for i, e := range node.Entries {
		if e.IsChild() {
			if e.ChildCID == nil {
				return nil, fmt.Errorf("%w %s: entry %d is a child with no CID", ErrInvalidNode, p.CID, i)
			}
			hi := p.Hi
			if i+1 < len(node.Entries) && node.Entries[i+1].IsValue() {
				hi = node.Entries[i+1].Key
			}
			if rangesIntersectSubtree(ranges, lo, hi) {
				out = append(out, pendingEntry{CID: *e.ChildCID, Lo: lo, Hi: hi})
			}
			continue
		}
		if !e.IsValue() {
			return nil, fmt.Errorf("%w %s: entry %d is neither value nor child", ErrInvalidNode, p.CID, i)
		}
		key := e.Key
		if prevKey != nil && bytes.Compare(key, prevKey) <= 0 {
			return nil, fmt.Errorf("%w %s: key %q does not sort after %q", ErrInvalidNode, p.CID, key, prevKey)
		}
		if p.Lo != nil && bytes.Compare(key, p.Lo) <= 0 {
			return nil, fmt.Errorf("%w %s: key %q below inherited bound %q", ErrInvalidNode, p.CID, key, p.Lo)
		}
		if p.Hi != nil && bytes.Compare(key, p.Hi) >= 0 {
			return nil, fmt.Errorf("%w %s: key %q above inherited bound %q", ErrInvalidNode, p.CID, key, p.Hi)
		}
		if rangesContain(ranges, key) {
			out = append(out, pendingEntry{CID: *e.Value, Key: key})
		}
		lo = key
		prevKey = key
	}
	return out, nil
}

// checkNodeData validates the prefix compression before mst.NodeData.Node
// expands it, because Node slices the previous key by PrefixLen without checking
// and would panic on a hostile (but correctly hashed) block.
func checkNodeData(nd *mst.NodeData) error {
	var prev []byte
	for i, e := range nd.Entries {
		if e.PrefixLen < 0 || e.PrefixLen > int64(len(prev)) {
			return fmt.Errorf("entry %d has prefix length %d, previous key is %d bytes", i, e.PrefixLen, len(prev))
		}
		key := make([]byte, 0, int(e.PrefixLen)+len(e.KeySuffix))
		key = append(key, prev[:e.PrefixLen]...)
		key = append(key, e.KeySuffix...)
		if len(key) == 0 || len(key) > mst.MAX_KEY_BYTES {
			return fmt.Errorf("entry %d has invalid key length %d", i, len(key))
		}
		prev = key
	}
	return nil
}
