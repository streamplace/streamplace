package statedb

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	glex "github.com/streamplace/glex/runtime"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spaces"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/stationshard"
)

var ErrStationTargetsInSpace = errors.New("station multistream targets are migrating to or have moved to personal space")

var errStationTargetsUnchanged = errors.New("station multistream targets unchanged")
var errStationTargetsNeedImport = errors.New("station multistream targets need import")

const (
	stationTargetsKey      = "multistream-targets/v1"
	stationTargetsMoveRole = "multistream-targets-move"
)

type stationTargetsPhase uint8

const (
	stationTargetsUninitialized stationTargetsPhase = iota
	stationTargetsImported
	stationTargetsLocal
	stationTargetsMoving
	stationTargetsMoved
)

type stationTargetDocument struct {
	Phase     stationTargetsPhase                      `json:"phase"`
	Targets   map[string]placestream.MultistreamTarget `json:"targets,omitempty"`
	MoveEpoch uint64                                   `json:"moveEpoch,omitempty"`
	Attempt   string                                   `json:"attempt,omitempty"`
}

type stationTargetCipher struct {
	aead      cipher.AEAD
	namespace string
}

func newStationTargetCipher(namespace string, stationKey []byte) (stationTargetCipher, error) {
	if len(stationKey) != 32 {
		return stationTargetCipher{}, errors.New("station target encryption requires a 32-byte station key")
	}
	mac := hmac.New(sha256.New, stationKey)
	_, _ = mac.Write([]byte("streamplace/station/" + stationTargetsKey))
	block, err := aes.NewCipher(mac.Sum(nil))
	if err != nil {
		return stationTargetCipher{}, err
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	return stationTargetCipher{aead: aead, namespace: namespace}, err
}

func (c stationTargetCipher) aad(did string) []byte {
	return []byte(c.namespace + "\x00" + did + "\x00" + stationTargetsKey)
}

func (c stationTargetCipher) open(did string, value []byte) (stationTargetDocument, error) {
	var doc stationTargetDocument
	if value == nil {
		return doc, nil
	}
	plain, err := c.aead.Open(nil, nil, value, c.aad(did))
	if err != nil {
		return doc, fmt.Errorf("decrypt station multistream targets: %w", err)
	}
	if err := json.Unmarshal(plain, &doc); err != nil {
		return doc, fmt.Errorf("decode station multistream targets: %w", err)
	}
	if doc.Phase < stationTargetsImported || doc.Phase > stationTargetsMoved {
		return doc, errors.New("invalid station multistream target phase")
	}
	return doc, nil
}

func (c stationTargetCipher) seal(did string, doc stationTargetDocument) ([]byte, error) {
	plain, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("encode station multistream targets: %w", err)
	}
	return c.aead.Seal(nil, nil, plain, c.aad(did)), nil
}

func (state *StatefulDB) updateStationTargets(ctx context.Context, did string, change func(*stationTargetDocument) error) (stationTargetDocument, error) {
	var doc stationTargetDocument
	err := state.StationShards.Update(ctx, did, stationTargetsKey, func(value []byte) ([]byte, error) {
		var err error
		doc, err = state.stationTargetsCipher.open(did, value)
		if err != nil {
			return nil, err
		}
		if err := change(&doc); err != nil {
			return nil, err
		}
		return state.stationTargetsCipher.seal(did, doc)
	})
	if errors.Is(err, errStationTargetsUnchanged) {
		err = nil
	}
	return doc, err
}

func (state *StatefulDB) validateStationTargetAuthority(did string) error {
	if _, err := syntax.ParseDID(did); err != nil {
		return fmt.Errorf("invalid multistream target DID: %w", err)
	}
	if state.StationShards == nil || state.stationTargetsCipher.aead == nil {
		return errors.New("station multistream target storage is not configured")
	}
	return nil
}

func (state *StatefulDB) readStationTargets(ctx context.Context, did string) (stationTargetDocument, error) {
	if err := state.validateStationTargetAuthority(did); err != nil {
		return stationTargetDocument{}, err
	}
	value, err := state.StationShards.Get(ctx, did, stationTargetsKey)
	if errors.Is(err, stationshard.ErrNotFound) {
		return stationTargetDocument{}, nil
	}
	if err != nil {
		return stationTargetDocument{}, err
	}
	return state.stationTargetsCipher.open(did, value)
}

// StationTargetsMoved checks the durable cutover marker without importing legacy targets.
func (state *StatefulDB) StationTargetsMoved(ctx context.Context, did string) (bool, error) {
	doc, err := state.readStationTargets(ctx, did)
	return err == nil && doc.Phase == stationTargetsMoved, err
}

func (state *StatefulDB) loadStationTargets(ctx context.Context, did string) (stationTargetDocument, error) {
	doc, err := state.readStationTargets(ctx, did)
	if err != nil || doc.Phase >= stationTargetsLocal {
		return doc, err
	}
	return state.importStationTargets(ctx, did, doc)
}

// A durable import phase precedes SQL deletion and permanently prevents reimport.
func (state *StatefulDB) importStationTargets(ctx context.Context, did string, doc stationTargetDocument) (stationTargetDocument, error) {
	db := state.DB.WithContext(ctx)
	var targets map[string]placestream.MultistreamTarget
	if doc.Phase == stationTargetsUninitialized {
		var rows []MultistreamTarget
		if err := db.Where("repo_did = ?", did).Find(&rows).Error; err != nil {
			return doc, fmt.Errorf("read legacy multistream targets: %w", err)
		}
		targets = make(map[string]placestream.MultistreamTarget, len(rows))
		for _, row := range rows {
			rkey, ok := strings.CutPrefix(row.URI, spaces.TargetURI(did, ""))
			if !ok || row.RepoDID != did {
				return doc, errors.New("legacy multistream target URI does not match its DID and collection")
			}
			if _, err := syntax.ParseRecordKey(rkey); err != nil {
				return doc, fmt.Errorf("invalid legacy multistream target rkey: %w", err)
			}
			var target placestream.MultistreamTarget
			if err := target.UnmarshalCBOR(bytes.NewReader(row.MultistreamTarget)); err != nil {
				return doc, fmt.Errorf("decode legacy multistream target: %w", err)
			}
			targets[rkey] = target
		}
	}

	doc, err := state.updateStationTargets(ctx, did, func(current *stationTargetDocument) error {
		switch current.Phase {
		case stationTargetsUninitialized:
			current.Targets = targets
			current.Phase = stationTargetsLocal
			if len(targets) != 0 {
				current.Phase = stationTargetsImported
			}
		case stationTargetsImported:
			// Reconfirm WAL durability before deleting SQL after an unknown write outcome.
		default:
			return errStationTargetsUnchanged
		}
		return nil
	})
	if err != nil || doc.Phase != stationTargetsImported {
		return doc, err
	}
	// Bootstrap is offline; no station target operation inserts or edits SQL rows.
	if err := db.Where("repo_did = ?", did).Delete(&MultistreamTarget{}).Error; err != nil {
		return doc, fmt.Errorf("remove imported multistream targets: %w", err)
	}
	return state.updateStationTargets(ctx, did, func(current *stationTargetDocument) error {
		if current.Phase != stationTargetsImported {
			return errStationTargetsUnchanged
		}
		current.Phase = stationTargetsLocal
		return nil
	})
}

func (state *StatefulDB) changeLocalStationTargets(ctx context.Context, did string, change func(*stationTargetDocument) error) error {
	if err := state.validateStationTargetAuthority(did); err != nil {
		return err
	}
	apply := func(current *stationTargetDocument) error {
		switch current.Phase {
		case stationTargetsLocal:
			return change(current)
		case stationTargetsMoving, stationTargetsMoved:
			return ErrStationTargetsInSpace
		default:
			return errStationTargetsNeedImport
		}
	}
	doc, err := state.updateStationTargets(ctx, did, apply)
	if errors.Is(err, errStationTargetsNeedImport) {
		if _, err := state.importStationTargets(ctx, did, doc); err != nil {
			return err
		}
		_, err = state.updateStationTargets(ctx, did, apply)
	}
	return err
}

func (state *StatefulDB) StationCreateTarget(ctx context.Context, did string, input *placestream.MultistreamTarget) (*placestream.MultistreamDefs_TargetView, error) {
	return state.stationPutTarget(ctx, did, spid.TIDClock.Next().String(), input, true)
}

func (state *StatefulDB) StationUpdateTarget(ctx context.Context, did, rkey string, input *placestream.MultistreamTarget) (*placestream.MultistreamDefs_TargetView, error) {
	return state.stationPutTarget(ctx, did, rkey, input, false)
}

func stationTargetView(did, rkey string, target *placestream.MultistreamTarget) (placestream.MultistreamDefs_TargetView, error) {
	cid, err := spid.GetCID(target)
	if err != nil {
		return placestream.MultistreamDefs_TargetView{}, fmt.Errorf("get multistream target CID: %w", err)
	}
	return placestream.MultistreamDefs_TargetView{
		Uri:    spaces.TargetURI(did, rkey),
		Cid:    cid.String(),
		Record: &glex.LexiconTypeDecoder{Val: target},
	}, nil
}

func (state *StatefulDB) stationPutTarget(ctx context.Context, did, rkey string, input *placestream.MultistreamTarget, create bool) (*placestream.MultistreamDefs_TargetView, error) {
	if _, err := syntax.ParseRecordKey(rkey); err != nil {
		return nil, fmt.Errorf("invalid multistream target rkey: %w", err)
	}
	var view placestream.MultistreamDefs_TargetView
	err := state.changeLocalStationTargets(ctx, did, func(current *stationTargetDocument) error {
		if input == nil {
			return errors.New("missing multistream target")
		}
		target := *input
		previous, exists := current.Targets[rkey]
		if create {
			if exists {
				return errors.New("multistream target rkey already exists")
			}
			if len(current.Targets) >= MAX_MULTISTREAM_TARGETS {
				return fmt.Errorf("maximum number of multistream targets (%d) reached", MAX_MULTISTREAM_TARGETS)
			}
		} else if !exists {
			return errors.New("multistream target not found")
		}
		if target.Active && !previous.Active {
			active := 0
			for _, record := range current.Targets {
				if record.Active {
					active++
				}
			}
			if active >= MAX_ACTIVE_MULTISTREAM_TARGETS {
				return fmt.Errorf("maximum number of active multistream targets (%d) reached", MAX_ACTIVE_MULTISTREAM_TARGETS)
			}
		}
		var err error
		view, err = stationTargetView(did, rkey, &target)
		if err != nil {
			return err
		}
		if current.Targets == nil {
			current.Targets = make(map[string]placestream.MultistreamTarget)
		}
		current.Targets[rkey] = target
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &view, nil
}

func (state *StatefulDB) StationDeleteTarget(ctx context.Context, did, rkey string) error {
	if _, err := syntax.ParseRecordKey(rkey); err != nil {
		return fmt.Errorf("invalid multistream target rkey: %w", err)
	}
	return state.changeLocalStationTargets(ctx, did, func(current *stationTargetDocument) error {
		if _, exists := current.Targets[rkey]; !exists {
			return errors.New("multistream target not found")
		}
		delete(current.Targets, rkey)
		return nil
	})
}

func (state *StatefulDB) StationListTargets(ctx context.Context, did string) ([]placestream.MultistreamDefs_TargetView, error) {
	doc, err := state.loadStationTargets(ctx, did)
	if err != nil {
		return nil, err
	}
	if doc.Phase >= stationTargetsMoving {
		return nil, ErrStationTargetsInSpace
	}
	views := make([]placestream.MultistreamDefs_TargetView, 0, len(doc.Targets))
	if len(doc.Targets) == 0 {
		return views, nil
	}
	uris := make([]string, 0, len(doc.Targets))
	for _, rkey := range slices.Sorted(maps.Keys(doc.Targets)) {
		target := doc.Targets[rkey]
		view, err := stationTargetView(did, rkey, &target)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
		uris = append(uris, view.Uri)
	}
	events, err := state.LatestMultistreamEvents(uris)
	if err != nil {
		return nil, err
	}
	for i := range views {
		views[i].LatestEvent = events[views[i].Uri]
	}
	return views, nil
}

func (state *StatefulDB) StationMoveTargets(ctx context.Context, client spaces.Client, did string) error {
	doc, err := state.loadStationTargets(ctx, did)
	if err != nil || doc.Phase == stationTargetsMoved {
		return err
	}
	if client == nil {
		return errors.New("missing personal space client")
	}
	store := state.StationShards
	lease, err := store.ClaimRole(ctx, did, stationTargetsMoveRole)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), store.TTL()/2)
		defer cancel()
		_ = store.ReleaseRole(cleanup, lease)
	}()

	var rkeys []string
	var resume bool
	doc, err = state.updateStationTargets(ctx, did, func(current *stationTargetDocument) error {
		if current.Phase == stationTargetsMoved {
			return errStationTargetsUnchanged
		}
		if current.MoveEpoch >= lease.Epoch {
			return stationshard.ErrFenced
		}
		if len(current.Targets) == 0 {
			*current = stationTargetDocument{Phase: stationTargetsMoved}
		} else {
			current.Phase = stationTargetsMoving
			current.MoveEpoch = lease.Epoch
			rkeys = slices.Sorted(maps.Keys(current.Targets))
			resume = current.Attempt != ""
			current.Attempt = rkeys[0]
		}
		return nil
	})
	if err != nil || doc.Phase == stationTargetsMoved {
		return err
	}

	// Bound every external call to less than a renewed lease, including lazy space creation.
	// The PDS has no fencing token; a request already accepted there cannot be recalled.
	call := func(kind, method string, params map[string]any, body, out any) error {
		requestCtx, cancel := context.WithTimeout(ctx, store.TTL()/2)
		defer cancel()
		if err := store.RenewRole(requestCtx, lease); err != nil {
			return err
		}
		if err := requestCtx.Err(); err != nil {
			return err
		}
		encoding := ""
		if kind == "POST" {
			encoding = "application/json"
		}
		return client.Do(requestCtx, kind, encoding, method, params, body, out)
	}
	for i, rkey := range rkeys {
		seen := false
		if i == 0 && resume {
			// An unacknowledged create may already have landed and been deleted at the PDS.
			seen, err = stationSpaceTargetSeen(call, did, rkey)
		}
		if err != nil {
			return err
		}
		if !seen {
			target := doc.Targets[rkey]
			if err := createStationSpaceTarget(call, did, rkey, &target); err != nil {
				return err
			}
		}
		if err := store.RenewRole(ctx, lease); err != nil {
			return err
		}
		_, err := state.updateStationTargets(ctx, did, func(current *stationTargetDocument) error {
			if current.Phase != stationTargetsMoving || current.MoveEpoch != lease.Epoch || current.Attempt != rkey {
				return stationshard.ErrFenced
			}
			if i+1 == len(rkeys) {
				*current = stationTargetDocument{Phase: stationTargetsMoved}
			} else {
				delete(current.Targets, rkey)
				current.Attempt = rkeys[i+1]
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

type stationSpaceCall func(kind, method string, params map[string]any, body, out any) error

func stationSpaceError(err error, code string) bool {
	var rpcErr *xrpc.XRPCError
	return errors.As(err, &rpcErr) && rpcErr.ErrStr == code
}

func createStationSpaceTarget(call stationSpaceCall, did, rkey string, target *placestream.MultistreamTarget) error {
	body := map[string]any{
		"space": spaces.PersonalSpaceURI(did), "repo": did, "collection": spaces.TargetCollection,
		"rkey": rkey, "record": target,
	}
	create := func() error {
		return call("POST", "com.atproto.space.createRecord", nil, body, nil)
	}
	err := create()
	if stationSpaceError(err, "SpaceNotFound") {
		err = call("POST", "com.atproto.space.createSpace", nil, map[string]any{
			"spaceType": spaces.PersonalSpaceType, "skey": spaces.PersonalSpaceKey,
			"policy": map[string]string{"$type": "com.atproto.personalspace.policy"},
		}, nil)
		if err != nil && !stationSpaceError(err, "SpaceAlreadyExists") {
			return fmt.Errorf("create multistream personal space: %w", err)
		}
		err = create()
	}
	if err != nil && !stationSpaceError(err, "RecordAlreadyExists") {
		return fmt.Errorf("create multistream target in personal space: %w", err)
	}
	return nil
}

func stationSpaceTargetSeen(call stationSpaceCall, did, rkey string) (bool, error) {
	params := map[string]any{
		"space": spaces.PersonalSpaceURI(did), "repo": did, "excludeValues": true, "limit": 1000,
	}
	for {
		var out struct {
			Cursor string `json:"cursor"`
			Ops    []struct {
				Collection string `json:"collection"`
				Rkey       string `json:"rkey"`
			} `json:"ops"`
		}
		err := call("GET", "com.atproto.space.listRepoOps", params, nil, &out)
		if stationSpaceError(err, "SpaceNotFound") || stationSpaceError(err, "RepoNotFound") {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("read multistream personal space history: %w", err)
		}
		for _, op := range out.Ops {
			if op.Collection == spaces.TargetCollection && op.Rkey == rkey {
				return true, nil
			}
		}
		if out.Cursor == "" {
			return false, nil
		}
		params["cursor"] = out.Cursor
	}
}
