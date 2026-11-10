package statedb

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spaces"
)

const multistreamTestDID = "did:plc:multistreamtest"
const multistreamCollection = "place.stream.multistream.target"

type multistreamSpaceClient struct {
	call func(method string, body map[string]any, out any) error
}

func (c multistreamSpaceClient) Do(_ context.Context, _, _, method string, _ map[string]any, body, out any) error {
	bs, err := json.Marshal(body)
	if err != nil {
		return err
	}
	var obj map[string]any
	if err := json.Unmarshal(bs, &obj); err != nil {
		return err
	}
	return c.call(method, obj, out)
}

func multistreamTestInput() placestream.MultistreamCreateTarget_Input {
	name := "original target"
	return placestream.MultistreamCreateTarget_Input{MultistreamTarget: placestream.MultistreamTarget{
		Url: "rtmp://localhost/live/key", Name: &name, CreatedAt: "2026-11-09T00:00:00Z",
	}}
}

func multistreamSuccess(out any) error {
	return json.Unmarshal([]byte(`{"cid":"upstream-cid"}`), out)
}

func TestMultistreamSync(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		var original []MultistreamTarget
		for range 4 {
			_, err := state.CreateMultistreamTarget(multistreamTestInput(), multistreamTestDID)
			require.NoError(t, err)
		}
		// AutoMigrate must backfill rows that predate both space flags.
		for _, column := range []string{"in_space", "space_stale"} {
			require.NoError(t, state.DB.Migrator().DropColumn(&MultistreamTarget{}, column))
		}
		require.NoError(t, state.DB.AutoMigrate(&MultistreamTarget{}))
		require.NoError(t, state.DB.Order("uri").Find(&original).Error)
		for _, row := range original {
			require.False(t, row.InSpace)
			require.False(t, row.SpaceStale)
			require.NoError(t, state.CreateMultistreamEvent(row.URI, "connected", "active"))
		}
		require.NoError(t, state.DB.Model(&MultistreamTarget{}).Where("uri = ?", original[1].URI).
			Update("space_stale", true).Error)
		require.NoError(t, state.DB.Model(&MultistreamTarget{}).Where("uri = ?", original[2].URI).
			Updates(map[string]any{"in_space": true, "space_stale": true}).Error)
		require.NoError(t, state.DB.Model(&MultistreamTarget{}).Where("uri = ?", original[3].URI).
			Update("in_space", true).Error)
		other, err := state.CreateMultistreamTarget(multistreamTestInput(), "did:plc:other")
		require.NoError(t, err)
		var originalEvents []MultistreamEvent
		require.NoError(t, state.DB.Order("id").Find(&originalEvents).Error)
		var keys []string
		client := multistreamSpaceClient{call: func(method string, body map[string]any, out any) error {
			require.Equal(t, "com.atproto.space.putRecord", method)
			require.Equal(t, multistreamTestDID, body["repo"])
			require.Equal(t, spaces.PersonalSpaceURI(multistreamTestDID), body["space"])
			require.Equal(t, multistreamTestInput().MultistreamTarget.Url, body["record"].(map[string]any)["url"])
			require.Equal(t, multistreamCollection, body["record"].(map[string]any)["$type"])
			keys = append(keys, body["rkey"].(string))
			return multistreamSuccess(out)
		}}
		require.NoError(t, state.SyncMultistreamTargets(ctx, client, multistreamTestDID))
		var expectedKeys []string
		for i, before := range original {
			rkey := before.URI[strings.LastIndex(before.URI, "/")+1:]
			row, err := state.GetMultistreamTarget(multistreamTestDID, rkey)
			require.NoError(t, err)
			require.Equal(t, before.URI, row.URI)
			require.Equal(t, before.MultistreamTarget, row.MultistreamTarget)
			require.True(t, row.InSpace)
			require.False(t, row.SpaceStale)
			if i < 3 {
				expectedKeys = append(expectedKeys, rkey)
				require.Equal(t, "upstream-cid", row.CID)
			} else {
				require.Equal(t, before.CID, row.CID)
			}
		}
		require.ElementsMatch(t, expectedKeys, keys)
		var events []MultistreamEvent
		require.NoError(t, state.DB.Order("id").Find(&events).Error)
		require.Equal(t, originalEvents, events)
		var otherRow MultistreamTarget
		require.NoError(t, state.DB.Where("uri = ?", other.Uri).First(&otherRow).Error)
		require.False(t, otherRow.InSpace)
		listed, err := state.ListMultistreamTargets(multistreamTestDID, 100, 0, nil)
		require.NoError(t, err)
		require.Len(t, listed, 4)
		require.Equal(t, original[0].URI, listed[0].Uri)
		require.Equal(t, "upstream-cid", listed[0].Cid)
		require.NotNil(t, listed[0].LatestEvent)
		var synced, rerun []MultistreamTarget
		require.NoError(t, state.DB.Order("uri").Find(&synced).Error)
		require.NoError(t, state.SyncMultistreamTargets(ctx, client, multistreamTestDID))
		require.Len(t, keys, 3)
		require.NoError(t, state.DB.Order("uri").Find(&rerun).Error)
		require.Equal(t, synced, rerun)
	})
}

func TestMultistreamLocalWritesAndSyncFailure(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		view, err := state.CreateMultistreamTarget(multistreamTestInput(), multistreamTestDID)
		require.NoError(t, err)
		_, err = state.CreateMultistreamTarget(multistreamTestInput(), multistreamTestDID)
		require.NoError(t, err)
		calls := 0
		failing := multistreamSpaceClient{call: func(string, map[string]any, any) error {
			calls++
			return errors.New("PDS unavailable")
		}}
		for _, client := range []spaces.Client{nil, failing} {
			var before, after []MultistreamTarget
			require.NoError(t, state.DB.Order("uri").Find(&before).Error)
			err := state.SyncMultistreamTargets(ctx, client, multistreamTestDID)
			if client == nil {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "PDS unavailable")
			}
			require.NoError(t, state.DB.Order("uri").Find(&after).Error)
			require.Equal(t, before, after)
			_, err = state.UpdateMultistreamTarget(view.Uri, placestream.MultistreamPutTarget_Input{
				MultistreamTarget: multistreamTestInput().MultistreamTarget,
			})
			require.NoError(t, err)
			_, err = state.CreateMultistreamTarget(multistreamTestInput(), multistreamTestDID)
			require.NoError(t, err)
		}
		require.Equal(t, 1, calls)
		client := multistreamSpaceClient{call: func(_ string, _ map[string]any, out any) error {
			return multistreamSuccess(out)
		}}
		require.NoError(t, state.SyncMultistreamTargets(ctx, client, multistreamTestDID))
		name := "new record"
		input := multistreamTestInput().MultistreamTarget
		input.Name, input.Url = &name, "rtmp://localhost/live/new-key"
		updated, err := state.UpdateMultistreamTarget(view.Uri, placestream.MultistreamPutTarget_Input{MultistreamTarget: input})
		require.NoError(t, err)
		require.Equal(t, view.Uri, updated.Uri)
		var pending MultistreamTarget
		require.NoError(t, state.DB.Where("uri = ?", view.Uri).First(&pending).Error)
		require.True(t, pending.InSpace)
		require.True(t, pending.SpaceStale)
		require.Equal(t, updated.Cid, pending.CID)
		require.ErrorContains(t, state.SyncMultistreamTargets(ctx, failing, multistreamTestDID), "PDS unavailable")
		client.call = func(_ string, body map[string]any, out any) error {
			require.Equal(t, name, body["record"].(map[string]any)["name"])
			require.Equal(t, input.Url, body["record"].(map[string]any)["url"])
			return multistreamSuccess(out)
		}
		require.NoError(t, state.SyncMultistreamTargets(ctx, client, multistreamTestDID))
		var synced MultistreamTarget
		require.NoError(t, state.DB.Where("uri = ?", view.Uri).First(&synced).Error)
		require.True(t, synced.InSpace)
		require.False(t, synced.SpaceStale)
		require.Equal(t, "upstream-cid", synced.CID)
	})
}

func TestMultistreamSpaceDelete(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		view, err := state.CreateMultistreamTarget(multistreamTestInput(), multistreamTestDID)
		require.NoError(t, err)
		client := multistreamSpaceClient{call: func(_ string, _ map[string]any, out any) error {
			return multistreamSuccess(out)
		}}
		require.NoError(t, state.SyncMultistreamTargets(ctx, client, multistreamTestDID))
		rkey := view.Uri[strings.LastIndex(view.Uri, "/")+1:]
		original, err := state.GetMultistreamTarget(multistreamTestDID, rkey)
		require.NoError(t, err)
		failing := multistreamSpaceClient{call: func(string, map[string]any, any) error {
			return errors.New("PDS delete failed")
		}}
		for _, c := range []spaces.Client{nil, failing} {
			require.Error(t, state.DeleteMultistreamTarget(ctx, c, view.Uri))
			unchanged, err := state.GetMultistreamTarget(multistreamTestDID, rkey)
			require.NoError(t, err)
			require.Equal(t, original, unchanged)
		}
		client.call = func(method string, body map[string]any, _ any) error {
			require.Equal(t, "com.atproto.space.deleteRecord", method)
			require.Equal(t, rkey, body["rkey"])
			require.Equal(t, multistreamTestDID, body["repo"])
			return nil
		}
		require.NoError(t, state.DeleteMultistreamTarget(ctx, client, view.Uri))
		_, err = state.GetMultistreamTarget(multistreamTestDID, rkey)
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
		local, err := state.CreateMultistreamTarget(multistreamTestInput(), multistreamTestDID)
		require.NoError(t, err)
		require.NoError(t, state.DeleteMultistreamTarget(ctx, nil, local.Uri))
	})
}

func TestMultistreamSQLRetry(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		view, err := state.CreateMultistreamTarget(multistreamTestInput(), multistreamTestDID)
		require.NoError(t, err)
		puts, deletes := 0, 0
		client := multistreamSpaceClient{call: func(method string, _ map[string]any, out any) error {
			if method == "com.atproto.space.putRecord" {
				puts++
				return multistreamSuccess(out)
			}
			deletes++
			return nil
		}}
		fail := func(db *gorm.DB) { _ = db.AddError(errors.New("SQL unavailable")) }
		require.NoError(t, state.DB.Callback().Update().Before("gorm:update").Register("test:fail", fail))
		require.ErrorContains(t, state.SyncMultistreamTargets(ctx, client, multistreamTestDID), "SQL unavailable")
		require.NoError(t, state.DB.Callback().Update().Remove("test:fail"))
		var row MultistreamTarget
		require.NoError(t, state.DB.Where("uri = ?", view.Uri).First(&row).Error)
		require.False(t, row.InSpace)
		require.NoError(t, state.SyncMultistreamTargets(ctx, client, multistreamTestDID))
		require.Equal(t, 2, puts)
		require.NoError(t, state.DB.Callback().Delete().Before("gorm:delete").Register("test:fail", fail))
		require.ErrorContains(t, state.DeleteMultistreamTarget(ctx, client, view.Uri), "SQL unavailable")
		require.NoError(t, state.DB.Callback().Delete().Remove("test:fail"))
		require.NoError(t, state.DeleteMultistreamTarget(ctx, client, view.Uri))
		require.Equal(t, 2, deletes)
		require.ErrorIs(t, state.DB.Where("uri = ?", view.Uri).First(&MultistreamTarget{}).Error, gorm.ErrRecordNotFound)
	})
}
