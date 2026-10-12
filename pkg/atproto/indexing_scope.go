package atproto

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"stream.place/streamplace/pkg/log"
)

// ErrRepoNotLocal prevents upstream-mode discovery or scraping of accounts
// that have not successfully authenticated to this node.
var ErrRepoNotLocal = errors.New("repo has no authenticated local session")

func (atsync *ATProtoSynchronizer) upstreamMode() bool {
	return atsync.UpstreamStation != nil || (atsync.CLI != nil && atsync.CLI.UpstreamStation != "")
}

// indexingDID authorizes before identity resolution or a cache hit. Resolve
// local handles to the persisted DID so a handle reassignment cannot authorize
// scraping a different remote account.
func (atsync *ATProtoSynchronizer) indexingDID(ctx context.Context, identifier string) (string, error) {
	if !atsync.upstreamMode() {
		return identifier, nil
	}
	if atsync.StatefulDB == nil {
		return "", fmt.Errorf("%w: %s", ErrRepoNotLocal, identifier)
	}
	did, err := atsync.StatefulDB.IndexingDID(ctx, identifier)
	if err != nil {
		return "", fmt.Errorf("failed to check local OAuth session: %w", err)
	}
	// Identity events can rename an already indexed user before the OAuth
	// session's cached handle changes. The index may supply the DID, but only
	// the persisted session can authorize it.
	if did == "" && !strings.HasPrefix(identifier, "did:") && atsync.Model != nil {
		repo, err := atsync.Model.GetRepoByHandleOrDID(identifier)
		if err != nil {
			return "", fmt.Errorf("failed to look up local handle: %w", err)
		}
		if repo != nil {
			did, err = atsync.StatefulDB.IndexingDID(ctx, repo.DID)
			if err != nil {
				return "", fmt.Errorf("failed to check local OAuth session: %w", err)
			}
		}
	}
	if did == "" {
		return "", fmt.Errorf("%w: %s", ErrRepoNotLocal, identifier)
	}
	return did, nil
}

func (atsync *ATProtoSynchronizer) indexEventAllowed(ctx context.Context, did string) bool {
	_, err := atsync.indexingDID(ctx, did)
	if err == nil {
		return true
	}
	if !errors.Is(err, ErrRepoNotLocal) {
		log.Error(ctx, "failed to authorize indexed event", "did", did, "err", err)
	}
	return false
}
