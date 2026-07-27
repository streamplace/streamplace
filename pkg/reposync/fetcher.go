package reposync

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	indigoat "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car"
	carutil "github.com/ipld/go-car/util"
	"github.com/multiformats/go-multihash"
)

// ErrMissingBlock is returned when a fetcher could not produce a block that was
// asked for. Callers must treat this as a hard error: silently dropping a block
// would turn a truncated response into an apparently-complete walk.
var ErrMissingBlock = errors.New("requested block missing from response")

// ErrBlockMismatch is returned when the bytes offered for a CID do not hash to
// that CID.
var ErrBlockMismatch = errors.New("block bytes do not match requested CID")

// DefaultChunkSize is how many CIDs [XRPCBlockFetcher] asks for per
// com.atproto.sync.getBlocks call.
//
// 20 is not a round number by accident: getBlocks takes its CIDs as repeated
// query parameters, and the reference PDS parses query strings with express's
// qs, whose default arrayLimit is 20. Ask for 21 CIDs at once and the array
// silently becomes an object, which the PDS then rejects with
// "cids/0 must be a cid string". TestIntegrationResumeOverNetwork probes this
// boundary against a real PDS.
const DefaultChunkSize = 20

// BlockFetcher retrieves repo blocks by CID.
//
// Implementations MUST verify that the bytes they return hash to the CID that
// was requested, and MUST return an error wrapping [ErrMissingBlock] if any
// requested CID is absent from the result. Returning a partial map is not
// allowed: the walker relies on "every requested block came back, verified" for
// its completeness guarantee.
type BlockFetcher interface {
	GetBlocks(ctx context.Context, cids []cid.Cid) (map[cid.Cid][]byte, error)
}

// VerifyBlock checks that data is the pre-image of c.
//
// Only CIDv1 with a SHA-256 multihash is accepted; that is what the atproto repo
// spec requires, and refusing anything else keeps a remote from handing us an
// identity- or weak-hashed CID that any bytes would satisfy.
func VerifyBlock(c cid.Cid, data []byte) error {
	if c.Version() != 1 {
		return fmt.Errorf("%w: %s is not a CIDv1", ErrBlockMismatch, c)
	}
	dec, err := multihash.Decode(c.Hash())
	if err != nil {
		return fmt.Errorf("%w: undecodable multihash in %s: %w", ErrBlockMismatch, c, err)
	}
	if dec.Code != multihash.SHA2_256 {
		return fmt.Errorf("%w: %s does not use sha2-256", ErrBlockMismatch, c)
	}
	got, err := c.Prefix().Sum(data)
	if err != nil {
		return fmt.Errorf("hashing %d bytes for %s: %w", len(data), c, err)
	}
	if !got.Equals(c) {
		return fmt.Errorf("%w: wanted %s, bytes hash to %s", ErrBlockMismatch, c, got)
	}
	return nil
}

// XRPCBlockFetcher fetches blocks from a remote repo host with
// com.atproto.sync.getBlocks.
type XRPCBlockFetcher struct {
	Client *xrpc.Client
	DID    string
	// ChunkSize caps how many CIDs go into a single getBlocks request.
	// Zero means [DefaultChunkSize].
	ChunkSize int
	// Retry bounds how hard each getBlocks call is retried after a transient
	// failure. The zero value means the package defaults.
	Retry RetryPolicy
}

var _ BlockFetcher = (*XRPCBlockFetcher)(nil)

func (f *XRPCBlockFetcher) GetBlocks(ctx context.Context, cids []cid.Cid) (map[cid.Cid][]byte, error) {
	want := dedupeCIDs(cids)
	out := make(map[cid.Cid][]byte, len(want))
	chunk := f.ChunkSize
	if chunk <= 0 {
		chunk = DefaultChunkSize
	}
	for start := 0; start < len(want); start += chunk {
		end := start + chunk
		if end > len(want) {
			end = len(want)
		}
		batch := want[start:end]
		strs := make([]string, len(batch))
		for i, c := range batch {
			strs[i] = c.String()
		}
		// Retried as a unit: a walk of a big repo makes hundreds of these calls
		// in a row, so a single 429 from a busy PDS must not end it. Parsing
		// happens outside the retry -- a CAR we cannot read is not transient.
		var raw []byte
		what := fmt.Sprintf("com.atproto.sync.getBlocks %s (%d cids)", f.DID, len(strs))
		err := f.Retry.do(ctx, what, func() error {
			var err error
			raw, err = indigoat.SyncGetBlocks(ctx, f.Client, strs, f.DID)
			return err
		})
		if err != nil {
			return nil, fmt.Errorf("com.atproto.sync.getBlocks for %s (%d cids): %w", f.DID, len(strs), err)
		}
		blocks, err := parseCAR(raw)
		if err != nil {
			return nil, fmt.Errorf("com.atproto.sync.getBlocks for %s: %w", f.DID, err)
		}
		for _, c := range batch {
			data, ok := blocks[c]
			if !ok {
				return nil, fmt.Errorf("%w: %s from %s", ErrMissingBlock, c, f.DID)
			}
			if err := VerifyBlock(c, data); err != nil {
				return nil, fmt.Errorf("block from %s: %w", f.DID, err)
			}
			out[c] = data
		}
	}
	return out, nil
}

// parseCAR reads every block out of a CARv1 stream.
//
// We do not use car.NewCarReader here: it rejects a CAR whose header declares no
// roots, and a getBlocks response is exactly that (a bag of blocks with no root).
func parseCAR(raw []byte) (map[cid.Cid][]byte, error) {
	br := bufio.NewReader(bytes.NewReader(raw))
	hdr, err := car.ReadHeader(br)
	if err != nil {
		return nil, fmt.Errorf("parsing CAR header: %w", err)
	}
	if hdr.Version != 1 {
		return nil, fmt.Errorf("unsupported CAR version %d", hdr.Version)
	}
	out := map[cid.Cid][]byte{}
	for {
		c, data, err := carutil.ReadNode(br)
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("reading CAR block: %w", err)
		}
		out[c] = data
	}
}

// BlockCache is a local store of blocks that have already been fetched and
// verified. Get reports found=false for a miss; a miss is never an error.
type BlockCache interface {
	Get(ctx context.Context, c cid.Cid) (data []byte, found bool, err error)
	Put(ctx context.Context, c cid.Cid, data []byte) error
}

// MemoryBlockCache is a trivial in-process [BlockCache].
type MemoryBlockCache struct {
	mu     sync.RWMutex
	blocks map[cid.Cid][]byte
}

func NewMemoryBlockCache() *MemoryBlockCache {
	return &MemoryBlockCache{blocks: map[cid.Cid][]byte{}}
}

var _ BlockCache = (*MemoryBlockCache)(nil)

func (m *MemoryBlockCache) Get(ctx context.Context, c cid.Cid) ([]byte, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.blocks[c]
	return data, ok, nil
}

func (m *MemoryBlockCache) Put(ctx context.Context, c cid.Cid, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blocks[c] = data
	return nil
}

// Len returns the number of cached blocks.
func (m *MemoryBlockCache) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.blocks)
}

// CachedFetcher serves blocks out of Cache when it can and writes everything it
// fetches from Inner back through. Blocks are immutable and content-addressed,
// so the cache never goes stale; this is what makes an interrupted walk cheap to
// restart.
type CachedFetcher struct {
	Cache BlockCache
	Inner BlockFetcher
}

var _ BlockFetcher = (*CachedFetcher)(nil)

func (f *CachedFetcher) GetBlocks(ctx context.Context, cids []cid.Cid) (map[cid.Cid][]byte, error) {
	want := dedupeCIDs(cids)
	out := make(map[cid.Cid][]byte, len(want))
	var miss []cid.Cid
	for _, c := range want {
		data, found, err := f.Cache.Get(ctx, c)
		if err != nil {
			return nil, fmt.Errorf("block cache get %s: %w", c, err)
		}
		// A cache that hands back the wrong bytes must not be able to poison the
		// walk, so re-verify and treat corruption as a miss.
		if !found || VerifyBlock(c, data) != nil {
			miss = append(miss, c)
			continue
		}
		out[c] = data
	}
	if len(miss) == 0 {
		return out, nil
	}
	fetched, err := f.Inner.GetBlocks(ctx, miss)
	if err != nil {
		return nil, err
	}
	for _, c := range miss {
		data, ok := fetched[c]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrMissingBlock, c)
		}
		if err := f.Cache.Put(ctx, c, data); err != nil {
			return nil, fmt.Errorf("block cache put %s: %w", c, err)
		}
		out[c] = data
	}
	return out, nil
}

func dedupeCIDs(in []cid.Cid) []cid.Cid {
	seen := make(map[cid.Cid]struct{}, len(in))
	out := make([]cid.Cid, 0, len(in))
	for _, c := range in {
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}
