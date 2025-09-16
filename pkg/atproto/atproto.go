package atproto

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	_ "github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/repo"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/ipfs/go-cid"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

var SyncGetRepo = comatproto.SyncGetRepo

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
	ident, err := atsync.resolveIdent(ctx, handle, true)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Bluesky handle %s: %w", handle, err)
	}

	ctx = log.WithLogValues(ctx, "did", ident.DID.String(), "handle", ident.Handle.String())

	handleLock := handleLocks.GetLock(ident.DID.String())
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
			Handle:  ident.Handle.String(),
		}
		err = mod.UpdateRepo(&newRepo)
		if err != nil {
			return nil, fmt.Errorf("failed to create empty DID record for %s: %w", ident.DID.String(), err)
		}
		err = atsync.StatefulDB.AddRepo(ident.DID.String())
		if err != nil {
			return nil, fmt.Errorf("failed to add repo to stateful DB for %s: %w", ident.DID.String(), err)
		}
	}

	log.Log(ctx, "resolved bluesky identity", "did", ident.DID, "handle", ident.Handle, "pds", ident.PDSEndpoint())
	pdsLock := pdsLocks.GetLock(ident.PDSEndpoint())
	xrpcc := xrpc.Client{
		Host:   ident.PDSEndpoint(),
		Client: &aqhttp.Client,
	}
	if xrpcc.Host == "" {
		return nil, fmt.Errorf("no PDS endpoint found for Bluesky identity %s", handle)
	}
	pdsLock.Lock()
	repoBytes, err := SyncGetRepo(ctx, &xrpcc, ident.DID.String(), rev)
	pdsLock.Unlock()
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

	log.Debug(ctx, "got diff", "bytes", len(repoBytes))

	r, err := repo.ReadRepoFromCar(ctx, bytes.NewReader(repoBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to parse repo CAR data for %s: %w", ident.DID.String(), err)
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

	err = r.ForEach(ctx, "", func(k string, v cid.Cid) error {
		nsid, rkey, err := syntax.ParseRepoPath(k)
		if err != nil {
			log.Warn(ctx, "failed to parse repo path", "k", k, "err", err)
			return fmt.Errorf("could not parse repo path %s: %w", k, err)
		}
		_, bs, err := r.GetRecordBytes(ctx, k)
		if err != nil {
			log.Warn(ctx, "failed to get record bytes", "k", k, "rkey", rkey, "err", err)
			return fmt.Errorf("could not retrieve record bytes for %s (rkey: %s): %w", k, rkey, err)
		}
		log.Debug(ctx, "record type", "key", k, "type", nsid.String())

		err = atsync.handleCreateUpdate(ctx, signerDID.String(), rkey, bs, v.String(), nsid, false, true)
		if err != nil {
			log.Warn(ctx, "failed to handle create update", "err", err)
			// invalid CBOR and stuff should get ignored, so
			// return fmt.Errorf("failed to process record update for %s (type: %s): %w", k, nsid.String(), err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate over repo: %w", err)
	}

	newRepo := model.Repo{
		DID:     ident.DID.String(),
		PDS:     ident.PDSEndpoint(),
		Version: sc.Rev,
		Handle:  ident.Handle.String(),
	}
	err = mod.UpdateRepo(&newRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to update DID record for %s: %w", sc.Did, err)
	}
	err = atsync.StatefulDB.AddRepo(ident.DID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to add repo to stateful DB for %s: %w", ident.DID.String(), err)
	}

	return &newRepo, nil
}

func (atsync *ATProtoSynchronizer) RefreshIdentity(ctx context.Context, did string) (*identity.Identity, error) {
	id, err := atsync.resolveIdent(ctx, did, false)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ident: %w", err)
	}
	newRepo := model.Repo{
		DID:    id.DID.String(),
		PDS:    id.PDSEndpoint(),
		Handle: id.Handle.String(),
	}
	err = atsync.Model.UpdateRepo(&newRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to update repo: %w", err)
	}
	return id, nil
}

func (atsync *ATProtoSynchronizer) resolveIdent(ctx context.Context, arg string, cached bool) (*identity.Identity, error) {
	if atsync.PLCDirectory == nil {
		atsync.PLCDirectory = CustomDirectory(atsync.CLI.PLCURL)
	}
	if atsync.CachedPLCDirectory == nil {
		cachedDir := identity.NewCacheDirectory(atsync.PLCDirectory, 250_000, time.Hour*24, time.Minute*2, time.Minute*5)
		atsync.CachedPLCDirectory = &cachedDir
	}
	dir := atsync.PLCDirectory
	if cached {
		dir = atsync.CachedPLCDirectory
	}
	id, err := syntax.ParseAtIdentifier(arg)
	if err != nil {
		return nil, err
	}

	resolvedID, err := dir.Lookup(ctx, *id)
	if err != nil {
		return nil, err
	}
	log.Log(ctx, "resolved ident", "id", resolvedID.DID.String(), "handle", resolvedID.Handle.String())

	return resolvedID, nil
}
func CustomDirectory(plcURL string) identity.Directory {
	base := identity.BaseDirectory{
		PLCURL:     plcURL,
		HTTPClient: aqhttp.Client,
		Resolver: net.Resolver{
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: time.Second * 3}
				return d.DialContext(ctx, network, address)
			},
		},
		TryAuthoritativeDNS: true,
		// primary Bluesky PDS instance only supports HTTP resolution method
		SkipDNSDomainSuffixes: []string{".bsky.social"},
	}
	return &base
}

func DIDDoc(host string) map[string]any {
	return map[string]any{
		"@context": []string{
			"https://www.w3.org/ns/did/v1",
			"https://w3id.org/security/multikey/v1",
			"https://w3id.org/security/suites/secp256k1-2019/v1",
		},
		"id": fmt.Sprintf("did:web:%s", host),
		"alsoKnownAs": []string{
			fmt.Sprintf("at://%s", host),
		},
		"service": []map[string]any{
			{
				"id":              "#bsky_fg",
				"type":            "BskyFeedGenerator",
				"serviceEndpoint": fmt.Sprintf("https://%s", host),
			},
			{
				"id":              "#atproto_pds",
				"type":            "AtprotoPersonalDataServer",
				"serviceEndpoint": fmt.Sprintf("https://%s", host),
			},
		},
		"verificationMethod": []map[string]any{
			{
				"id":                 fmt.Sprintf("did:web:%s#atproto", host),
				"type":               "Multikey",
				"controller":         fmt.Sprintf("did:web:%s", host),
				"publicKeyMultibase": LexiconPubMultibase,
			},
		},
	}
}
