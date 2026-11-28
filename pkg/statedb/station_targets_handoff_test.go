//go:build stationslate && linux && cgo

package statedb

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/xrpc"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/internal/stationtest"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/s3"
	"stream.place/streamplace/pkg/spaces"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/stationshard"
)

func TestStationTargetsHandoffS3(t *testing.T) {
	cfg := stationtest.Config(t)

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	namespace := "targets-handoff-" + uuid.NewString()
	prefix := "stations/" + namespace
	objects := blob.NewS3Store(s3.NewClient(cfg), cfg.Bucket)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		keys, err := objects.List(ctx, prefix+"/")
		require.NoError(t, err)
		for _, key := range keys {
			require.True(t, strings.HasPrefix(key, prefix+"/"), "refusing to delete outside the test namespace")
			require.NoError(t, objects.Delete(ctx, key))
		}
	})

	var key [32]byte
	_, err := rand.Read(key[:])
	require.NoError(t, err)
	keyFile := filepath.Join(t.TempDir(), "station.key")
	require.NoError(t, os.WriteFile(keyFile, key[:], 0600))
	open := func(owner string) (*StatefulDB, func()) {
		t.Helper()
		cli := &config.CLI{DBURL: "station://" + namespace, StationStateKeyFile: keyFile}
		cli.SetS3Config(cfg)
		state, err := MakeDB(ctx, cli, nil, nil)
		require.NoError(t, err)
		sqlDB, err := state.DB.DB()
		require.NoError(t, err)
		closeSQL := sync.OnceValue(sqlDB.Close)
		t.Cleanup(func() { require.NoError(t, closeSQL()) })
		shard, err := stationshard.Open(stationshard.Config{S3: cfg, Prefix: prefix + "/shards", Owner: owner, TTL: 2 * time.Minute})
		require.NoError(t, err)
		state.StationShards = shard
		t.Cleanup(func() { require.NoError(t, shard.Close()) })
		return state, func() {
			t.Helper()
			require.NoError(t, shard.Close())
			require.NoError(t, closeSQL())
		}
	}

	// 257 DIDs in 256 slots guarantee a collision without fixing the hash output.
	var did, other string
	var slots [256]string
	for i := range len(slots) + 1 {
		candidate := fmt.Sprintf("did:web:creator-%d.example", i)
		slot := stationshard.Slot(candidate)
		if slots[slot] != "" {
			did, other = slots[slot], candidate
			break
		}
		slots[slot] = candidate
	}
	require.NotEmpty(t, other)
	const nodeA = "did:web:node-a.example"
	const nodeB = "did:web:node-b.example"
	a, closeA := open(nodeA)
	b, closeB := open(nodeB)

	target := func(name string, active bool) *placestream.MultistreamTarget {
		return &placestream.MultistreamTarget{
			Url: "rtmps://ingest.example/live/" + name, Name: &name,
			CreatedAt: "2026-11-11T00:00:00Z", Active: active,
		}
	}
	checkTarget := func(view placestream.MultistreamDefs_TargetView, uri string, want *placestream.MultistreamTarget) {
		t.Helper()
		require.Equal(t, uri, view.Uri)
		cid, err := spid.GetCID(want)
		require.NoError(t, err)
		require.Equal(t, cid.String(), view.Cid)
		require.NotNil(t, view.Record)
		value, ok := view.Record.Val.(*placestream.MultistreamTarget)
		require.True(t, ok, "unexpected target payload type: %T", view.Record.Val)
		require.NotNil(t, value)
		got := *value
		got.LexiconTypeID = want.LexiconTypeID
		require.Equal(t, *want, got)
	}
	list := func(state *StatefulDB, did string) []placestream.MultistreamDefs_TargetView {
		t.Helper()
		views, err := state.StationListTargets(ctx, did)
		require.NoError(t, err)
		return views
	}
	noSQLTargets := func(state *StatefulDB, did string) {
		t.Helper()
		var count int64
		require.NoError(t, state.DB.WithContext(ctx).Model(&MultistreamTarget{}).Where("repo_did = ?", did).Count(&count).Error)
		require.Zero(t, count, "station target writes must not leave SQL rows")
	}
	ownerIs := func(state *StatefulDB, want string) {
		t.Helper()
		owner, err := state.StationShards.Owner(ctx, did)
		require.NoError(t, err)
		require.Equal(t, want, owner)
	}

	original := target("primary", true)
	created, err := a.StationCreateTarget(ctx, did, original)
	require.NoError(t, err)
	require.NotNil(t, created)
	rkey, found := strings.CutPrefix(created.Uri, spaces.TargetURI(did, ""))
	require.True(t, found && rkey != "" && !strings.Contains(rkey, "/"), "expected a target URI containing the creator DID and rkey")
	checkTarget(*created, spaces.TargetURI(did, rkey), original)
	noSQLTargets(b, did)
	ownerIs(b, nodeA)

	legacy := target("legacy", false)
	legacyView, err := a.CreateMultistreamTarget(placestream.MultistreamCreateTarget_Input{MultistreamTarget: *legacy}, other)
	require.NoError(t, err)
	imported := list(a, other)
	require.Len(t, imported, 1)
	checkTarget(imported[0], legacyView.Uri, legacy)
	noSQLTargets(b, other)

	// The other creator cannot edit or delete the first creator's rkey in the same slot.
	_, err = a.StationUpdateTarget(ctx, other, rkey, target("cross-creator", false))
	require.Error(t, err)
	require.Error(t, a.StationDeleteTarget(ctx, other, rkey))

	failure := errors.New("test PDS unavailable for target creation")
	space := &targetHandoffSpace{did: other, records: make(map[string]spaces.Record), unavailable: failure}
	require.ErrorIs(t, a.StationMoveTargets(ctx, space, other), failure)

	closeA()
	_, err = b.StationShards.Owner(ctx, did)
	require.ErrorIs(t, err, stationshard.ErrNotFound, "A must release its slot on close")
	recovered := list(b, did)
	require.Len(t, recovered, 1)
	checkTarget(recovered[0], created.Uri, original)
	ownerIs(b, nodeB)
	noSQLTargets(b, did)

	edited := target("edited-on-b", false)
	updated, err := b.StationUpdateTarget(ctx, did, rkey, edited)
	require.NoError(t, err)
	require.NotNil(t, updated)
	checkTarget(*updated, created.Uri, edited)
	removed, err := b.StationCreateTarget(ctx, did, target("deleted-on-b", false))
	require.NoError(t, err)
	require.NotNil(t, removed)
	removedKey, found := strings.CutPrefix(removed.Uri, spaces.TargetURI(did, ""))
	require.True(t, found && removedKey != "")
	require.NoError(t, b.StationDeleteTarget(ctx, did, removedKey))
	noSQLTargets(b, did)

	space.unavailable = nil
	_, err = b.StationListTargets(ctx, other)
	require.ErrorIs(t, err, ErrStationTargetsInSpace, "the failed move must remain pending after handoff")
	require.NoError(t, b.StationMoveTargets(ctx, space, other))
	moved, err := spaces.ListMultistreamTargets(ctx, space, other)
	require.NoError(t, err)
	require.Len(t, moved, 1, "the successor must recover and move the imported target")
	checkTarget(moved[0], legacyView.Uri, legacy)
	closeB()

	restarted, _ := open(nodeA)
	views, err := restarted.StationListTargets(ctx, other)
	require.ErrorIs(t, err, ErrStationTargetsInSpace)
	require.Empty(t, views, "the migrated creator must not have local targets")
	_, err = restarted.StationCreateTarget(ctx, other, legacy)
	require.ErrorIs(t, err, ErrStationTargetsInSpace)
	legacyKey, found := strings.CutPrefix(legacyView.Uri, spaces.TargetURI(other, ""))
	require.True(t, found && legacyKey != "")
	_, err = restarted.StationUpdateTarget(ctx, other, legacyKey, legacy)
	require.ErrorIs(t, err, ErrStationTargetsInSpace)
	require.ErrorIs(t, restarted.StationDeleteTarget(ctx, other, legacyKey), ErrStationTargetsInSpace)
	space.unavailable = failure
	require.NoError(t, restarted.StationMoveTargets(ctx, space, other), "completed migration must not need the PDS again")
	ownerIs(restarted, nodeA)
	noSQLTargets(restarted, did)
	noSQLTargets(restarted, other)
	isolated := list(restarted, did)
	require.Len(t, isolated, 1, "the moved marker must not apply to a colliding DID")
	checkTarget(isolated[0], created.Uri, edited)
}

// targetHandoffSpace is one existing personal space. The pinned spaces PDS
// supports createRecord, not putRecord with a create-only precondition.
type targetHandoffSpace struct {
	did         string
	records     map[string]spaces.Record
	unavailable error
}

func (s *targetHandoffSpace) Do(_ context.Context, _, _, method string, params map[string]any, body, out any) error {
	if s.unavailable != nil {
		return s.unavailable
	}
	space := spaces.PersonalSpaceURI(s.did)
	var response any
	switch method {
	case "com.atproto.space.listRepoOps":
		if params["space"] != space || params["repo"] != s.did {
			return fmt.Errorf("incorrect space repo query: %v", params)
		}
		if len(s.records) == 0 {
			return &xrpc.Error{StatusCode: 400, Wrapped: &xrpc.XRPCError{ErrStr: "RepoNotFound"}}
		}
		ops := make([]map[string]any, 0, len(s.records))
		for rkey, record := range s.records {
			ops = append(ops, map[string]any{"collection": spaces.TargetCollection, "rkey": rkey, "cid": record.CID, "prev": nil})
		}
		response = map[string]any{"ops": ops}
	case "com.atproto.space.listRecords":
		if params["space"] != space || params["repo"] != s.did || params["collection"] != spaces.TargetCollection {
			return fmt.Errorf("incorrect space record query: %v", params)
		}
		records := make([]spaces.Record, 0, len(s.records))
		for _, record := range s.records {
			records = append(records, record)
		}
		response = map[string]any{"records": records}
	case "com.atproto.space.createRecord":
		var in struct {
			Space      string                        `json:"space"`
			Repo       string                        `json:"repo"`
			Collection string                        `json:"collection"`
			Rkey       string                        `json:"rkey"`
			Record     placestream.MultistreamTarget `json:"record"`
		}
		bs, err := json.Marshal(body)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(bs, &in); err != nil {
			return err
		}
		if in.Space != space || in.Repo != s.did || in.Collection != spaces.TargetCollection || in.Rkey == "" {
			return errors.New("incorrect space record address")
		}
		if _, exists := s.records[in.Rkey]; exists {
			return &xrpc.Error{StatusCode: 400, Wrapped: &xrpc.XRPCError{ErrStr: "RecordAlreadyExists"}}
		}
		cid, err := spid.GetCID(&in.Record)
		if err != nil {
			return err
		}
		value, err := json.Marshal(&in.Record)
		if err != nil {
			return err
		}
		s.records[in.Rkey] = spaces.Record{Rkey: in.Rkey, CID: cid.String(), Value: value}
		response = map[string]any{"uri": space + "/" + in.Repo + "/" + in.Collection + "/" + in.Rkey, "cid": cid.String()}
	default:
		return fmt.Errorf("unexpected personal space method %q", method)
	}
	if out == nil {
		return nil
	}
	bs, err := json.Marshal(response)
	if err != nil {
		return err
	}
	return json.Unmarshal(bs, out)
}
