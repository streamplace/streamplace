package atproto

import (
	"context"

	"github.com/bluesky-social/indigo/atproto/identity"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/indexdb"
)

// RepoIdentity is the repo-synchronization and identity-resolution role
// of ATProtoSynchronizer: the read-facing half of the atproto node
// logic. The other half is the firehose indexer, driven by cmd as its
// own component. Consumers (api, spxrpc) depend on this role rather
// than the whole synchronizer.
type RepoIdentity interface {
	// SyncBlueskyRepoCached returns the indexdb repo for a handle or
	// DID, syncing from the PDS on first sight.
	SyncBlueskyRepoCached(ctx context.Context, handle string) (*indexdb.Repo, error)
	// ResolveAuthorHandle resolves a DID to its current handle.
	ResolveAuthorHandle(ctx context.Context, did string) string
	// RefreshIdentity re-resolves a DID document against the directory.
	RefreshIdentity(ctx context.Context, did string) (*identity.Identity, error)
	// FetchUserProfile fetches an actor's app.bsky profile view.
	FetchUserProfile(ctx context.Context, username string) (*appbsky.ActorDefs_ProfileViewDetailed, error)
}

var _ RepoIdentity = (*ATProtoSynchronizer)(nil)
