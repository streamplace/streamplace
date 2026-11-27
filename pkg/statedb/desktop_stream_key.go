package statedb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/xrpc"
	"github.com/google/uuid"
	glex "github.com/streamplace/glex/runtime"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/crypto/spkey"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
)

// DesktopStreamKey keeps the private key on the node, never in the renderer or OBS.
type DesktopStreamKey struct {
	DID        string `gorm:"primaryKey;column:did"`
	PrivateKey string
	SigningKey string
	RKey       string
	CreatedAt  time.Time
	Published  bool
}

// DesktopStreamKey returns one durable key per account, publishing and indexing it before use.
func (state *StatefulDB) DesktopStreamKey(ctx context.Context, did string) (string, error) {
	unlock, err := state.GetNamedLock("desktop-stream-key::" + did)
	if err != nil {
		return "", err
	}
	defer unlock()
	var key DesktopStreamKey
	err = state.DB.WithContext(ctx).First(&key, "did = ?", did).Error
	indexed := false
	if err == nil {
		signingKey, indexErr := state.model.GetSigningKey(ctx, key.SigningKey, did)
		if indexErr != nil {
			keys, listErr := state.model.GetSigningKeysForRepo(did)
			if listErr != nil {
				return "", listErr
			}
			revoked := false
			for _, signingKey := range keys {
				if signingKey.DID == key.SigningKey && signingKey.RevokedAt != nil {
					revoked = true
					break
				}
			}
			if !revoked {
				return "", indexErr
			}
			if err := state.DB.WithContext(ctx).Delete(&key).Error; err != nil {
				return "", err
			}
			err = gorm.ErrRecordNotFound
		} else {
			indexed = signingKey != nil
		}
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		priv, pub, err := spkey.GenerateStreamKeyForDID(did)
		if err != nil {
			return "", fmt.Errorf("generate desktop key: %w", err)
		}
		key = DesktopStreamKey{DID: did, PrivateKey: priv, SigningKey: pub.DIDKey(), RKey: uuid.NewString(), CreatedAt: time.Now().UTC()}
		if err := state.DB.WithContext(ctx).Create(&key).Error; err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	if !key.Published {
		var storedSession oatproxy.OAuthSession
		err := state.indexingSessions(ctx).Where("repo_did = ?", did).Order("updated_at DESC").First(&storedSession).Error
		if err != nil {
			return "", fmt.Errorf("get desktop session: %w", err)
		}
		session, err := state.OATProxy.RefreshIfNeeded(&storedSession)
		if err != nil {
			return "", fmt.Errorf("refresh desktop session: %w", err)
		}
		client, err := state.OATProxy.GetXrpcClient(session)
		if err != nil {
			return "", fmt.Errorf("get desktop client: %w", err)
		}
		createdBy := "Streamplace Desktop"
		record := &placestream.Key{SigningKey: key.SigningKey, CreatedAt: key.CreatedAt.Format(time.RFC3339Nano), CreatedBy: &createdBy}
		input := comatproto.RepoPutRecord_Input{Repo: did, Collection: "place.stream.key", Rkey: key.RKey, Record: &glex.LexiconTypeDecoder{Val: record}}
		var out comatproto.RepoPutRecord_Output
		if err := client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", nil, input, &out); err != nil {
			return "", fmt.Errorf("publish desktop key: %w", err)
		}
		if err := state.DB.WithContext(ctx).Model(&key).Update("published", true).Error; err != nil {
			return "", err
		}
	}
	if !indexed {
		if err := state.model.UpdateSigningKey(&model.SigningKey{DID: key.SigningKey, RepoDID: did, RKey: key.RKey, CreatedAt: key.CreatedAt}); err != nil {
			return "", fmt.Errorf("index desktop key: %w", err)
		}
	}
	return key.PrivateKey, nil
}

func (state *StatefulDB) DesktopIngestKey(ctx context.Context) (string, error) {
	dids, err := state.IndexingDIDs(ctx)
	if err != nil {
		return "", err
	}
	switch len(dids) {
	case 0:
		return "", fmt.Errorf("log in to Streamplace Desktop")
	case 1:
		return state.DesktopStreamKey(ctx, dids[0])
	default:
		return "", fmt.Errorf("more than one account logged in to Streamplace Desktop")
	}
}
