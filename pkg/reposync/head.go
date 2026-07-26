package reposync

import (
	"bytes"
	"context"
	"fmt"

	indigoat "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/repo"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/ipfs/go-cid"
)

// Head is a repo's current commit, after signature verification. Everything
// reachable from Root is authenticated by Commit.Sig.
type Head struct {
	// Commit is the decoded, structurally valid, correctly signed commit.
	Commit *repo.Commit
	// CID is the CID of the commit block itself.
	CID cid.Cid
	// Root is the CID of the root MST node (Commit.Data).
	Root cid.Cid
	// Rev is the commit revision (a TID).
	Rev string
}

// FetchVerifiedHead resolves a repo's current commit and proves it belongs to
// did.
//
// It asks the host for the latest commit CID, pulls that block through f (which
// verifies the bytes against the CID), decodes it, and checks the commit's
// structure, DID, rev and signature against the account's atproto signing key
// from dir. On success, nothing the host says about the repo below Head.Root can
// be forged.
func FetchVerifiedHead(ctx context.Context, client *xrpc.Client, f BlockFetcher, dir identity.Directory, did string) (*Head, error) {
	parsedDID, err := syntax.ParseDID(did)
	if err != nil {
		return nil, fmt.Errorf("invalid did %q: %w", did, err)
	}

	latest, err := indigoat.SyncGetLatestCommit(ctx, client, did)
	if err != nil {
		return nil, fmt.Errorf("com.atproto.sync.getLatestCommit for %s: %w", did, err)
	}
	commitCID, err := cid.Decode(latest.Cid)
	if err != nil {
		return nil, fmt.Errorf("undecodable commit cid %q for %s: %w", latest.Cid, did, err)
	}

	blocks, err := f.GetBlocks(ctx, []cid.Cid{commitCID})
	if err != nil {
		return nil, fmt.Errorf("fetching commit block for %s: %w", did, err)
	}
	raw, ok := blocks[commitCID]
	if !ok {
		return nil, fmt.Errorf("%w: commit %s for %s", ErrMissingBlock, commitCID, did)
	}
	// Don't take the fetcher's word for it: the commit CID is the anchor for the
	// whole walk, so re-verify it here regardless of which fetcher we were given.
	if err := VerifyBlock(commitCID, raw); err != nil {
		return nil, fmt.Errorf("commit block for %s: %w", did, err)
	}

	var commit repo.Commit
	if err := commit.UnmarshalCBOR(bytes.NewReader(raw)); err != nil {
		return nil, fmt.Errorf("decoding commit %s for %s: %w", commitCID, did, err)
	}
	if err := commit.VerifyStructure(); err != nil {
		return nil, fmt.Errorf("commit %s for %s: %w", commitCID, did, err)
	}
	if commit.DID != did {
		return nil, fmt.Errorf("commit %s is for repo %s, not %s", commitCID, commit.DID, did)
	}
	if commit.Rev != latest.Rev {
		return nil, fmt.Errorf("commit %s has rev %q, host reported %q for %s", commitCID, commit.Rev, latest.Rev, did)
	}

	ident, err := dir.LookupDID(ctx, parsedDID)
	if err != nil {
		return nil, fmt.Errorf("resolving identity for %s: %w", did, err)
	}
	pubkey, err := ident.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("no atproto signing key for %s: %w", did, err)
	}
	if err := commit.VerifySignature(pubkey); err != nil {
		return nil, fmt.Errorf("commit %s signature for %s: %w", commitCID, did, err)
	}

	return &Head{
		Commit: &commit,
		CID:    commitCID,
		Root:   commit.Data,
		Rev:    commit.Rev,
	}, nil
}
