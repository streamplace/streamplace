package atproto

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"

	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/crypto/aqpub"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	comatproto "github.com/bluesky-social/indigo/api/atproto"
	atcrypto "github.com/bluesky-social/indigo/atproto/crypto"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/repo"
	"github.com/bluesky-social/indigo/util"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
)

var SyncGetRepo = comatproto.SyncGetRepo
var STREAMPLACE_COLLECTION = "place.stream.key"
var STREAMPLACE_SIGNING_KEY = "signingKey"

const DID_KEY_PREFIX = "did:key"
const ADDRESS_KEY_PREFIX = "0x"

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

func SyncBlueskyRepoCached(ctx context.Context, handle string, mod model.Model) (string, error) {
	repo, err := mod.GetRepoByHandle(handle)
	if err != nil {
		return "", fmt.Errorf("failed to get repo for %s: %w", handle, err)
	}
	if repo != nil {
		return repo.SigningKey, nil
	}
	return SyncBlueskyRepo(ctx, handle, mod)
}

func SyncBlueskyRepo(ctx context.Context, handle string, mod model.Model) (string, error) {
	// Get handle-specific lock and ensure synchronized access
	handleLock := getHandleLock(handle)
	handleLock.Lock()
	defer handleLock.Unlock()

	ident, err := ResolveIdent(ctx, handle)
	if err != nil {
		return "", fmt.Errorf("failed to resolve Bluesky handle %s: %w", handle, err)
	}

	rev := ""
	oldRepo, err := mod.GetRepo(ident.DID.String())
	if err != nil {
		return "", fmt.Errorf("failed to get DID record for %s: %w", ident.DID.String(), err)
	}
	if oldRepo != nil {
		log.Log(ctx, "found existing DID record", "did", oldRepo.DID, "version", oldRepo.Version)
		rev = oldRepo.Version
	}

	log.Log(ctx, "resolved bluesky identity", "did", ident.DID, "handle", ident.Handle, "pds", ident.PDSEndpoint())
	xrpcc := xrpc.Client{
		Host:   ident.PDSEndpoint(),
		Client: &aqhttp.Client,
	}
	if xrpcc.Host == "" {
		return "", fmt.Errorf("no PDS endpoint found for Bluesky identity %s", handle)
	}
	repoBytes, err := SyncGetRepo(ctx, &xrpcc, ident.DID.String(), rev)
	if err != nil {
		return "", fmt.Errorf("failed to fetch repo for %s from PDS %s: %w", ident.DID.String(), xrpcc.Host, err)
	}

	// uncomment for saving new test cases:

	// timestamp := time.Now().Unix()
	// filename := fmt.Sprintf("%d.base64", timestamp)
	// encodedBytes := base64.URLEncoding.EncodeToString(repoBytes)
	// err = os.WriteFile(filename, []byte(encodedBytes), 0644)
	// if err != nil {
	// 	return "", fmt.Errorf("failed to write encoded repo bytes to file: %w", err)
	// }

	log.Log(ctx, "got diff", "bytes", len(repoBytes))

	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	root, err := repo.IngestRepo(ctx, bs, bytes.NewReader(repoBytes))
	if err != nil {
		return "", fmt.Errorf("failed to ingest repo for %s: %w", ident.DID.String(), err)
	}
	log.Log(ctx, "ingested repo", "root", root)
	if oldRepo != nil {
		oldRoot, err := cid.Decode(oldRepo.RootCID)
		if err != nil {
			return "", fmt.Errorf("failed to decode old root CID for %s: %w", ident.DID.String(), err)
		}
		if oldRoot.Equals(root) {
			log.Log(ctx, "no changes to repo", "root", root)
			return oldRepo.SigningKey, nil
		}
	}

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

	processed := 0
	var key string
	if oldRepo != nil {
		key = oldRepo.SigningKey
	}
	bs = r.Blockstore()
	cst := util.CborStore(bs)
	allKeys, err := bs.AllKeysChan(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get all keys: %w", err)
	}
	for k := range allKeys {
		log.Log(ctx, "processing key", "key", k)
		rec := map[string]any{}
		err := cst.Get(ctx, k, &rec)
		if err != nil {
			return "", fmt.Errorf("failed to get block for key %s: %w", k, err)
		}
		log.Log(ctx, "got block", "key", k, "size", len(rec))
		typ, ok := rec["$type"]
		if !ok {
			continue
		}
		if typ != STREAMPLACE_COLLECTION {
			continue
		}
		processed += 1
		streamplaceKeyAny, ok := rec[STREAMPLACE_SIGNING_KEY]
		if !ok {
			continue
		}
		streamplaceKey, ok := streamplaceKeyAny.(string)
		if !ok {
			continue
		}
		key = streamplaceKey
	}
	log.Log(ctx, "processed new posts", "postCount", processed)

	var aqk aqpub.Pub
	if strings.HasPrefix(key, DID_KEY_PREFIX) {
		pubKey, err := atcrypto.ParsePublicDIDKey(key)
		if err != nil {
			return "", fmt.Errorf("failed to parse multibase key %s: %w", key, err)
		}
		aqk, err = aqpub.FromBytes(pubKey.UncompressedBytes())
		if err != nil {
			return "", fmt.Errorf("failed to parse public key for %s: %w", handle, err)
		}
	} else if strings.HasPrefix(key, ADDRESS_KEY_PREFIX) {
		aqk, err = aqpub.FromHexString(key)
		if err != nil {
			return "", fmt.Errorf("failed to parse public key for %s: %w", handle, err)
		}
	} else {
		return "", fmt.Errorf("invalid key format for %s: %s", handle, key)
	}
	if err != nil {
		return "", fmt.Errorf("failed to parse public key for %s: %w", handle, err)
	}
	addr := aqk.String()
	newRepo := model.Repo{
		DID:        ident.DID.String(),
		PDS:        ident.PDSEndpoint(),
		Version:    sc.Rev,
		SigningKey: addr,
		RootCID:    root.String(),
		Handle:     handle,
	}
	err = mod.UpdateRepo(&newRepo)
	if err != nil {
		return "", fmt.Errorf("failed to update DID record for %s: %w", sc.Did, err)
	}

	return addr, nil
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
