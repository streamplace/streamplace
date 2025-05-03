package atproto

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	_ "github.com/bluesky-social/indigo/api/bsky"
	atcrypto "github.com/bluesky-social/indigo/atproto/crypto"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/repo"
	"github.com/bluesky-social/indigo/util"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

var SyncGetRepo = comatproto.SyncGetRepo

// handleLocks provides per-handle synchronization
var handleLocks = struct {
	sync.Mutex
	locks map[string]*sync.Mutex
}{
	locks: make(map[string]*sync.Mutex),
}

// getHandleLock returns a mutex for the given handle
func getHandleLock(handle string) *sync.Mutex {
	handleLocks.Lock()
	defer handleLocks.Unlock()

	if lock, exists := handleLocks.locks[handle]; exists {
		return lock
	}

	lock := &sync.Mutex{}
	handleLocks.locks[handle] = lock
	return lock
}

func (atsync *ATProtoSynchronizer) SyncBlueskyRepoCached(ctx context.Context, handle string, mod model.Model) (*model.Repo, error) {
	ctx, span := otel.Tracer("signer").Start(ctx, "SyncBlueskyRepoCached")
	defer span.End()
	repo, err := mod.GetRepoByHandleOrDID(handle)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo for %s: %w", handle, err)
	}
	if repo != nil {
		return repo, nil
	}

	return atsync.SyncBlueskyRepo(ctx, handle, mod)
}

type mstNode struct {
	rkey       syntax.RecordKey
	collection syntax.NSID
}

func (atsync *ATProtoSynchronizer) SyncBlueskyRepo(ctx context.Context, handle string, mod model.Model) (*model.Repo, error) {
	ctx = log.WithLogValues(ctx, "func", "SyncBlueskyRepo")
	// Get handle-specific lock and ensure synchronized access

	ident, err := ResolveIdent(ctx, handle)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Bluesky handle %s: %w", handle, err)
	}

	handleLock := getHandleLock(ident.DID.String())
	handleLock.Lock()
	defer handleLock.Unlock()

	rev := ""
	oldRepo, err := mod.GetRepo(ident.DID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get DID record for %s: %w", ident.DID.String(), err)
	}
	if oldRepo != nil {
		log.Log(ctx, "found existing DID record", "did", oldRepo.DID, "version", oldRepo.Version)
		return oldRepo, nil
	} else {
		// create an empty repo while we sync. this is useful because we'll start monitoring the firehose for
		// any new follows and such from this user while we're syncing, which can take a long time
		newRepo := model.Repo{
			DID:     ident.DID.String(),
			PDS:     ident.PDSEndpoint(),
			Version: "",
			RootCID: "",
			Handle:  ident.Handle.String(),
		}
		err = mod.UpdateRepo(&newRepo)
		if err != nil {
			return nil, fmt.Errorf("failed to create empty DID record for %s: %w", ident.DID.String(), err)
		}
	}

	log.Log(ctx, "resolved bluesky identity", "did", ident.DID, "handle", ident.Handle, "pds", ident.PDSEndpoint())
	xrpcc := xrpc.Client{
		Host:   ident.PDSEndpoint(),
		Client: &aqhttp.Client,
	}
	if xrpcc.Host == "" {
		return nil, fmt.Errorf("no PDS endpoint found for Bluesky identity %s", handle)
	}
	repoBytes, err := SyncGetRepo(ctx, &xrpcc, ident.DID.String(), rev)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repo for %s from PDS %s: %w", ident.DID.String(), xrpcc.Host, err)
	}

	// uncomment for saving new test cases:

	// timestamp := time.Now().Unix()
	// filename := fmt.Sprintf("%d.base64", timestamp)
	// encodedBytes := base64.URLEncoding.EncodeToString(repoBytes)
	// err = os.WriteFile(filename, []byte(encodedBytes), 0644)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to write encoded repo bytes to file: %w", err)
	// }

	log.Log(ctx, "got diff", "bytes", len(repoBytes))

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	root, err := repo.IngestRepo(ctx, bs, bytes.NewReader(repoBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to ingest repo for %s: %w", ident.DID.String(), err)
	}
	log.Log(ctx, "ingested repo", "root", root)
	if oldRepo != nil && oldRepo.RootCID != "" {
		oldRoot, err := cid.Decode(oldRepo.RootCID)
		if err != nil {
			return nil, fmt.Errorf("failed to decode old root CID for %s: %w", ident.DID.String(), err)
		}
		if oldRoot.Equals(root) {
			log.Debug(ctx, "no changes to repo", "root", root)
			return oldRepo, nil
		}
	}

	r, err := repo.ReadRepoFromCar(ctx, bytes.NewReader(repoBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to parse repo CAR data for %s: %w", ident.DID.String(), err)
	}

	mstNodes := map[string]mstNode{}
	err = r.ForEach(ctx, "", func(k string, v cid.Cid) error {
		nsid, rkey, err := syntax.ParseRepoPath(k)
		if err != nil {
			log.Warn(ctx, "failed to parse repo path", "k", k, "err", err)
			return err
		}
		hash := v.Hash().HexString()
		log.Debug(ctx, "got mst node", "cid", v, "rkey", rkey, "nsid", nsid, "hash", hash)
		mstNodes[hash] = mstNode{
			rkey:       rkey,
			collection: nsid,
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate over repo: %w", err)
	}

	// extract DID from repo commit
	sc := r.SignedCommit()
	signerDID, err := syntax.ParseDID(sc.Did)
	if err != nil {
		return nil, fmt.Errorf("invalid DID in repo commit for %s: %w", ident.DID.String(), err)
	}
	if signerDID != ident.DID {
		return nil, fmt.Errorf("signer DID %s does not match identity %s", signerDID, ident.DID.String())
	}

	bs = r.Blockstore()
	cst := util.CborStore(bs)
	allKeys, err := bs.AllKeysChan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all keys: %w", err)
	}
	for k := range allKeys {
		blk, err := bs.Get(ctx, k)
		if err != nil {
			return nil, fmt.Errorf("failed to get block for key %s: %w", k, err)
		}
		rec := map[string]any{}
		err = cst.Get(ctx, k, &rec)
		if err != nil {
			return nil, fmt.Errorf("failed to get block for key %s: %w", k, err)
		}
		typ, ok := rec["$type"]
		if !ok {
			log.Debug(ctx, "record type not found", "key", k)
			continue
		}
		log.Debug(ctx, "record type", "key", k, "type", typ)
		hash := k.Hash().HexString()
		node, ok := mstNodes[hash]
		if !ok {
			log.Warn(ctx, "no mst node found for record", "key", k, "hash", hash)
			continue
		}
		rawData := blk.RawData()
		err = atsync.handleCreateUpdate(ctx, signerDID.String(), node.rkey, &rawData, k.String(), node.collection)
		if err != nil {
			log.Warn(ctx, "failed to handle create update", "err", err)
		}
	}

	newRepo := model.Repo{
		DID:     ident.DID.String(),
		PDS:     ident.PDSEndpoint(),
		Version: sc.Rev,
		RootCID: root.String(),
		Handle:  ident.Handle.String(),
	}
	err = mod.UpdateRepo(&newRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to update DID record for %s: %w", sc.Did, err)
	}

	return &newRepo, nil
}

func parseSigningKey(ctx context.Context, key string) error {
	if !strings.HasPrefix(key, constants.DID_KEY_PREFIX) {
		return fmt.Errorf("invalid key format for DID key: %s", key)
	}
	_, err := atcrypto.ParsePublicDIDKey(key)
	if err != nil {
		return fmt.Errorf("failed to parse multibase key %s: %w", key, err)
	}
	return nil
}

var ResolveIdent = resolveIdent

func resolveIdent(ctx context.Context, arg string) (*identity.Identity, error) {
	id, err := syntax.ParseAtIdentifier(arg)
	if err != nil {
		return nil, err
	}

	dir := identity.DefaultDirectory()
	return dir.Lookup(ctx, *id)
}
