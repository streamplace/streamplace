package atproto

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"aquareum.tv/aquareum/pkg/log"
	"aquareum.tv/aquareum/pkg/model"
	comatproto "github.com/bluesky-social/indigo/api/atproto"
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

func SyncBlueskyRepo(ctx context.Context, handle string, mod model.Model) (string, error) {
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
		Host: ident.PDSEndpoint(),
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
			return oldRepo.AquareumKey, nil
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
		key = oldRepo.AquareumKey
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
		byts, err := json.Marshal(rec)
		if err != nil {
			return "", fmt.Errorf("failed to marshal block for key %s: %w", k, err)
		}
		typ, ok := rec["$type"]
		if !ok {
			continue
		}
		if typ != "app.bsky.feed.post" {
			continue
		}
		fmt.Println(string(byts))
		processed += 1
		aquareumKeyAny, ok := rec["aquareumKey"]
		if !ok {
			continue
		}
		aquareumKey, ok := aquareumKeyAny.(string)
		if !ok {
			continue
		}
		key = aquareumKey
	}
	// err = r.ForEach(ctx, "app.bsky.feed.post", func(k string, v cid.Cid) error {
	// 	log.Log(ctx, "processing record", "key", k, "cid", v)
	// 	_, recBytes, err := r.GetRecordBytes(ctx, k)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to get record bytes for key %s: %w", k, err)
	// 	}

	// 	rec, err := data.UnmarshalCBOR(*recBytes)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to unmarshal CBOR for record %s: %w", k, err)
	// 	}
	// 	typ, ok := rec["$type"]
	// 	if !ok {
	// 		return nil
	// 	}
	// 	if typ != "app.bsky.feed.post" {
	// 		return nil
	// 	}
	// 	processed += 1
	// 	aquareumKeyAny, ok := rec["aquareumKey"]
	// 	if !ok {
	// 		return nil
	// 	}
	// 	aquareumKey, ok := aquareumKeyAny.(string)
	// 	if !ok {
	// 		return nil
	// 	}
	// 	key = aquareumKey
	// 	return nil
	// })
	// if err != nil {
	// 	return "", fmt.Errorf("error processing repo records for %s: %w", ident.DID.String(), err)
	// }
	log.Log(ctx, "processed new posts", "postCount", processed)
	newRepo := model.Repo{
		DID:         ident.DID.String(),
		PDS:         ident.PDSEndpoint(),
		Version:     sc.Rev,
		AquareumKey: key,
		RootCID:     root.String(),
	}
	err = mod.UpdateRepo(&newRepo)
	if err != nil {
		return "", fmt.Errorf("failed to update DID record for %s: %w", sc.Did, err)
	}

	return key, nil
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
