//go:build stationslate && linux && cgo

package statedb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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

const stationTestTargetDID = "did:plc:stationtargets"

func stationTargetFixture(t *testing.T) (*StatefulDB, func() *stationshard.Store) {
	t.Helper()
	cfg := stationtest.Config(t)
	prefix := "station-targets-test/" + uuid.NewString()
	objects := blob.NewS3Store(s3.NewClient(cfg), cfg.Bucket)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		keys, err := objects.List(ctx, prefix+"/")
		require.NoError(t, err)
		for _, key := range keys {
			require.True(t, strings.HasPrefix(key, prefix+"/"))
			require.NoError(t, objects.Delete(ctx, key))
		}
	})
	open := func() *stationshard.Store {
		store, err := stationshard.Open(stationshard.Config{S3: cfg, Prefix: prefix, Owner: "did:plc:node"})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, store.Close()) })
		return store
	}
	state, err := MakeDB(t.Context(), &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	db, err := state.DB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	state.stationTargetsCipher, err = newStationTargetCipher(prefix, bytes.Repeat([]byte{1}, 32))
	require.NoError(t, err)
	state.StationShards = open()
	return state, open
}

func stationTestTarget(key string, active bool) placestream.MultistreamTarget {
	return placestream.MultistreamTarget{Url: "rtmp://example.com/live/" + key, Active: active, CreatedAt: "2026-11-11T00:00:00Z"}
}

func stationLegacyTarget(t *testing.T, did, rkey string, target placestream.MultistreamTarget) MultistreamTarget {
	t.Helper()
	var record bytes.Buffer
	require.NoError(t, target.MarshalCBOR(&record))
	cid, err := spid.GetCIDFromBytes(record.Bytes())
	require.NoError(t, err)
	return MultistreamTarget{URI: spaces.TargetURI(did, rkey), CID: cid.String(), RepoDID: did, Active: target.Active, MultistreamTarget: record.Bytes()}
}

func stationTestRkey(view *placestream.MultistreamDefs_TargetView) string {
	return view.Uri[strings.LastIndexByte(view.Uri, '/')+1:]
}

func TestStationTargetCipher(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	c, err := newStationTargetCipher("stations/test", key)
	require.NoError(t, err)
	doc := stationTargetDocument{Phase: stationTargetsLocal, Targets: map[string]placestream.MultistreamTarget{"first": stationTestTarget("private-test-credential", true)}}
	first, err := c.seal(stationTestTargetDID, doc)
	require.NoError(t, err)
	second, err := c.seal(stationTestTargetDID, doc)
	require.NoError(t, err)
	require.NotEqual(t, first, second)
	require.False(t, bytes.Contains(first, []byte("private-test-credential")))
	decoded, err := c.open(stationTestTargetDID, first)
	require.NoError(t, err)
	require.Equal(t, doc.Targets["first"].Url, decoded.Targets["first"].Url)
	_, err = c.open("did:plc:other", first)
	require.Error(t, err)
	other, err := newStationTargetCipher("stations/other", key)
	require.NoError(t, err)
	_, err = other.open(stationTestTargetDID, first)
	require.Error(t, err)
	_, err = c.aead.Open(nil, nil, first, []byte("stations/test\x00"+stationTestTargetDID+"\x00another-key"))
	require.Error(t, err)
	first[len(first)-1] ^= 1
	_, err = c.open(stationTestTargetDID, first)
	require.Error(t, err)
}

func TestStationTargetImportCrashAndResume(t *testing.T) {
	state, reopen := stationTargetFixture(t)
	ctx, did := t.Context(), stationTestTargetDID
	original := stationTestTarget("legacy-secret", true)
	first := stationLegacyTarget(t, did, "alpha", original)
	last := stationLegacyTarget(t, did, "zulu", stationTestTarget("another-secret", false))
	require.NoError(t, state.DB.Create(&[]MultistreamTarget{last, first}).Error)
	moved, err := state.StationTargetsMoved(ctx, did)
	require.NoError(t, err)
	require.False(t, moved)
	_, err = state.StationShards.Get(ctx, did, stationTargetsKey)
	require.ErrorIs(t, err, stationshard.ErrNotFound, "checking an absent marker must not import SQL rows")
	require.NoError(t, state.DB.Exec(`CREATE TRIGGER interrupt_target_import BEFORE DELETE ON multistream_targets BEGIN SELECT RAISE(ABORT, 'interrupted cleanup'); END`).Error)
	_, err = state.StationListTargets(ctx, did)
	require.ErrorContains(t, err, "interrupted cleanup")
	var sqlCount int64
	require.NoError(t, state.DB.Model(&MultistreamTarget{}).Count(&sqlCount).Error)
	require.EqualValues(t, 2, sqlCount)
	value, err := state.StationShards.Get(ctx, did, stationTargetsKey)
	require.NoError(t, err)
	require.False(t, bytes.Contains(value, []byte(original.Url)))
	moved, err = state.StationTargetsMoved(ctx, did)
	require.NoError(t, err)
	require.False(t, moved)
	unchanged, err := state.StationShards.Get(ctx, did, stationTargetsKey)
	require.NoError(t, err)
	require.Equal(t, value, unchanged, "checking the imported phase must not finish the import")

	require.NoError(t, state.StationShards.Close())
	state.StationShards = reopen()
	require.NoError(t, state.DB.Exec(`DROP TRIGGER interrupt_target_import`).Error)
	require.NoError(t, state.CreateMultistreamEvent(first.URI, "running", "test-status"))
	views, err := state.StationListTargets(ctx, did)
	require.NoError(t, err)
	require.Len(t, views, 2)
	require.Equal(t, first.URI, views[0].Uri)
	require.Equal(t, first.CID, views[0].Cid)
	require.Equal(t, last.URI, views[1].Uri)
	require.NotNil(t, views[0].LatestEvent)
	require.Equal(t, "running", views[0].LatestEvent.Message)
	require.Nil(t, views[1].LatestEvent)
	require.NoError(t, state.DB.Model(&MultistreamTarget{}).Count(&sqlCount).Error)
	require.Zero(t, sqlCount)

	changed := stationTestTarget("different-secret", true)
	view, err := state.StationUpdateTarget(ctx, did, "alpha", &changed)
	require.NoError(t, err)
	require.Equal(t, first.URI, view.Uri)
	require.NotEqual(t, first.CID, view.Cid)
	views, err = state.StationListTargets(ctx, did)
	require.NoError(t, err)
	require.Equal(t, changed.Url, views[0].Record.Val.(*placestream.MultistreamTarget).Url)
	_, err = state.StationUpdateTarget(ctx, "did:plc:someoneelse", "alpha", &changed)
	require.ErrorContains(t, err, "not found")
	require.ErrorContains(t, state.StationDeleteTarget(ctx, did, "bad/key"), "rkey")
	_, err = state.StationListTargets(ctx, did+"/not-a-did")
	require.ErrorContains(t, err, "DID")

	require.NoError(t, state.StationDeleteTarget(ctx, did, "alpha"))
	require.NoError(t, state.DB.Create(&first).Error) // Simulate a stale bootstrap row.
	views, err = state.StationListTargets(ctx, did)
	require.NoError(t, err)
	require.Len(t, views, 1)
	require.Equal(t, last.URI, views[0].Uri)

	bad := stationLegacyTarget(t, did, "wrong-authority", original)
	bad.RepoDID = "did:plc:wrong"
	require.NoError(t, state.DB.Create(&bad).Error)
	_, err = state.StationListTargets(ctx, bad.RepoDID)
	require.ErrorContains(t, err, "does not match its DID")
}

func TestStationTargetConcurrentQuotas(t *testing.T) {
	state, _ := stationTargetFixture(t)
	ctx, did := t.Context(), stationTestTargetDID
	active := stationTestTarget("active", true)
	inactive := stationTestTarget("inactive", false)
	const seeded = MAX_MULTISTREAM_TARGETS - 2
	rows := make([]MultistreamTarget, 0, seeded)
	for i := range seeded {
		target := inactive
		if i < MAX_ACTIVE_MULTISTREAM_TARGETS-1 {
			target = active
		}
		rows = append(rows, stationLegacyTarget(t, did, fmt.Sprintf("seed-%03d", i), target))
	}
	require.NoError(t, state.DB.Create(&rows).Error)

	race := func(n int, write func(int) error, wantError string) int {
		start := make(chan struct{})
		results := make(chan error, n)
		for i := range n {
			go func() {
				<-start
				results <- write(i)
			}()
		}
		close(start)
		var succeeded int
		for range n {
			if err := <-results; err == nil {
				succeeded++
			} else {
				require.ErrorContains(t, err, wantError)
			}
		}
		return succeeded
	}
	require.Equal(t, 1, race(8, func(i int) error {
		if i%2 == 0 {
			_, err := state.StationCreateTarget(ctx, did, &active)
			return err
		}
		_, err := state.StationUpdateTarget(ctx, did, fmt.Sprintf("seed-%03d", i+10), &active)
		return err
	}, "maximum number of active"))
	views, err := state.StationListTargets(ctx, did)
	require.NoError(t, err)
	before := len(views)
	require.Equal(t, MAX_MULTISTREAM_TARGETS-before, race(4, func(int) error {
		_, err := state.StationCreateTarget(ctx, did, &inactive)
		return err
	}, "maximum number of multistream targets"))
	views, err = state.StationListTargets(ctx, did)
	require.NoError(t, err)
	require.Len(t, views, MAX_MULTISTREAM_TARGETS)
	var activeCount int
	for _, view := range views {
		if view.Record.Val.(*placestream.MultistreamTarget).Active {
			activeCount++
		}
	}
	require.Equal(t, MAX_ACTIVE_MULTISTREAM_TARGETS, activeCount)
	rotated := stationTestTarget("rotated", true)
	_, err = state.StationUpdateTarget(ctx, did, "seed-000", &rotated)
	require.NoError(t, err, "an edit that uses no additional quotas must succeed")
	var sqlCount int64
	require.NoError(t, state.DB.Model(&MultistreamTarget{}).Count(&sqlCount).Error)
	require.Zero(t, sqlCount, "station creates and edits must not write SQL target rows")
}

type stationTargetClient func(context.Context, string, map[string]any, any, any) error

func (c stationTargetClient) Do(ctx context.Context, _, _, method string, params map[string]any, body, out any) error {
	return c(ctx, method, params, body, out)
}

func stationTestSpaceError(code string) error { return &xrpc.XRPCError{ErrStr: code} }

func TestStationTargetMoveResumesWithoutOverwritingSpace(t *testing.T) {
	state, reopen := stationTargetFixture(t)
	ctx, did := t.Context(), stationTestTargetDID
	local := stationTestTarget("station-key", true)
	var rkeys []string
	for range 3 {
		view, err := state.StationCreateTarget(ctx, did, &local)
		require.NoError(t, err)
		rkeys = append(rkeys, stationTestRkey(view))
	}
	slices.Sort(rkeys)
	// The first key already belongs to the PDS. The second create commits but loses its response.
	changed := stationTestTarget("pds-rotated-key", true)
	records := map[string]placestream.MultistreamTarget{rkeys[0]: changed}
	spaceExists, calls, lost := false, 0, errors.New("PDS response lost")
	client := stationTargetClient(func(_ context.Context, method string, _ map[string]any, body, out any) error {
		calls++
		switch method {
		case "com.atproto.space.createRecord":
			if !spaceExists {
				return stationTestSpaceError("SpaceNotFound")
			}
			in := body.(map[string]any)
			require.Equal(t, spaces.PersonalSpaceURI(did), in["space"])
			require.Equal(t, did, in["repo"])
			require.Equal(t, spaces.TargetCollection, in["collection"])
			rkey := in["rkey"].(string)
			if _, exists := records[rkey]; exists {
				return stationTestSpaceError("RecordAlreadyExists")
			}
			records[rkey] = *in["record"].(*placestream.MultistreamTarget)
			if rkey == rkeys[1] {
				return lost
			}
			return nil
		case "com.atproto.space.createSpace":
			spaceExists = true
			return nil
		case "com.atproto.space.listRepoOps":
			reply := fmt.Sprintf(`{"ops":[{"collection":%q,"rkey":%q,"cid":null}]}`, spaces.TargetCollection, rkeys[1])
			return json.Unmarshal([]byte(reply), out)
		default:
			return fmt.Errorf("unexpected RPC: %s", method)
		}
	})
	require.ErrorIs(t, state.StationMoveTargets(ctx, client, did), lost)
	moved, err := state.StationTargetsMoved(ctx, did)
	require.NoError(t, err)
	require.False(t, moved, "a partial move is not a completed cutover")
	_, err = state.StationListTargets(ctx, did)
	require.ErrorIs(t, err, ErrStationTargetsInSpace)
	_, err = state.StationCreateTarget(ctx, did, &local)
	require.ErrorIs(t, err, ErrStationTargetsInSpace)
	_, err = state.StationUpdateTarget(ctx, did, rkeys[2], &local)
	require.ErrorIs(t, err, ErrStationTargetsInSpace)
	require.ErrorIs(t, state.StationDeleteTarget(ctx, did, rkeys[2]), ErrStationTargetsInSpace)

	delete(records, rkeys[1]) // Independent delete before the station can checkpoint the create.
	require.NoError(t, state.StationShards.Close())
	state.StationShards = reopen()
	require.NoError(t, state.StationMoveTargets(ctx, client, did))
	require.Equal(t, changed.Url, records[rkeys[0]].Url)
	require.NotContains(t, records, rkeys[1])
	require.Equal(t, local.Url, records[rkeys[2]].Url)
	delete(records, rkeys[2])
	before := calls
	require.NoError(t, state.StationMoveTargets(ctx, client, did))
	marker, err := state.StationShards.Get(ctx, did, stationTargetsKey)
	require.NoError(t, err)
	moved, err = state.StationTargetsMoved(ctx, did)
	require.NoError(t, err)
	require.True(t, moved)
	unchanged, err := state.StationShards.Get(ctx, did, stationTargetsKey)
	require.NoError(t, err)
	require.Equal(t, marker, unchanged)
	require.Equal(t, before, calls)
	require.NotContains(t, records, rkeys[2])
	_, err = state.StationListTargets(ctx, did)
	require.ErrorIs(t, err, ErrStationTargetsInSpace)
}

func TestStationTargetMoveFencesDelayedOwner(t *testing.T) {
	state, _ := stationTargetFixture(t)
	ctx, did := t.Context(), stationTestTargetDID
	target := stationTestTarget("secret", true)
	_, err := state.StationCreateTarget(ctx, did, &target)
	require.NoError(t, err)
	entered, unblock, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	var once sync.Once
	defer once.Do(func() { close(unblock) })
	old := stationTargetClient(func(ctx context.Context, method string, _ map[string]any, _, _ any) error {
		if method != "com.atproto.space.createRecord" {
			return fmt.Errorf("stale owner sent RPC: %s", method)
		}
		close(entered)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-unblock:
			return stationTestSpaceError("SpaceNotFound")
		}
	})
	go func() { done <- state.StationMoveTargets(ctx, old, did) }()
	select {
	case <-entered:
	case err := <-done:
		t.Fatalf("move did not reach PDS: %v", err)
	case <-time.After(15 * time.Second):
		t.Fatal("move did not reach PDS")
	}
	current := stationTargetClient(func(_ context.Context, method string, _ map[string]any, _, out any) error {
		switch method {
		case "com.atproto.space.listRepoOps":
			return json.Unmarshal([]byte(`{"ops":[]}`), out)
		case "com.atproto.space.createRecord":
			return nil
		default:
			return fmt.Errorf("unexpected RPC: %s", method)
		}
	})
	require.ErrorIs(t, state.StationMoveTargets(ctx, current, did), stationshard.ErrHeld)
	_, lease, err := state.StationShards.ReadRoleState(ctx, did, stationTargetsMoveRole)
	require.NoError(t, err)
	require.NoError(t, state.StationShards.ReleaseRole(ctx, lease))
	require.NoError(t, state.StationMoveTargets(ctx, current, did))
	once.Do(func() { close(unblock) })
	require.ErrorIs(t, <-done, stationshard.ErrFenced)
}
