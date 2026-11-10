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

func TestMultistreamMigration(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		legacy, err := state.CreateMultistreamTarget(ctx, nil, multistreamTestInput(), multistreamTestDID)
		require.NoError(t, err)
		require.NoError(t, state.DB.Migrator().DropColumn(&MultistreamTarget{}, "in_space"))
		require.NoError(t, state.DB.AutoMigrate(&MultistreamTarget{}))
		require.NoError(t, state.CreateMultistreamEvent(legacy.Uri, "connected", "active"))
		require.NoError(t, state.CreateMultistreamEvent(legacy.Uri, "disconnected", "error"))
		rkey := legacy.Uri[strings.LastIndex(legacy.Uri, "/")+1:]
		original, err := state.GetMultistreamTarget(multistreamTestDID, rkey)
		require.NoError(t, err)
		require.False(t, original.InSpace)
		var originalEvents []MultistreamEvent
		require.NoError(t, state.DB.Order("id").Find(&originalEvents).Error)
		calls := 0
		client := multistreamSpaceClient{call: func(method string, body map[string]any, out any) error {
			calls++
			require.Equal(t, "com.atproto.space.putRecord", method)
			require.Equal(t, multistreamTestDID, body["repo"])
			require.Equal(t, spaces.PersonalSpaceURI(multistreamTestDID), body["space"])
			require.Equal(t, multistreamTestInput().MultistreamTarget.Url, body["record"].(map[string]any)["url"])
			require.Equal(t, rkey, body["rkey"])
			require.Equal(t, multistreamCollection, body["record"].(map[string]any)["$type"])
			return multistreamSuccess(out)
		}}
		require.NoError(t, state.MigrateMultistreamTargets(ctx, client, multistreamTestDID))
		migrated, err := state.GetMultistreamTarget(multistreamTestDID, rkey)
		require.NoError(t, err)
		require.Equal(t, legacy.Uri, migrated.URI)
		require.True(t, migrated.InSpace)
		require.Equal(t, "upstream-cid", migrated.CID)
		require.Equal(t, original.MultistreamTarget, migrated.MultistreamTarget)
		var events []MultistreamEvent
		require.NoError(t, state.DB.Order("id").Find(&events).Error)
		require.Equal(t, originalEvents, events)
		listed, err := state.ListMultistreamTargets(multistreamTestDID, 100, 0, nil)
		require.NoError(t, err)
		require.Len(t, listed, 1)
		require.Equal(t, migrated.URI, listed[0].Uri)
		require.Equal(t, migrated.CID, listed[0].Cid)
		require.NotNil(t, listed[0].LatestEvent)
		require.NoError(t, state.MigrateMultistreamTargets(ctx, client, multistreamTestDID))
		require.Equal(t, 1, calls)
		rerun, err := state.GetMultistreamTarget(multistreamTestDID, rkey)
		require.NoError(t, err)
		require.Equal(t, migrated, rerun)
	})
}

func TestMultistreamMigrationFailure(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		legacy, err := state.CreateMultistreamTarget(ctx, nil, multistreamTestInput(), multistreamTestDID)
		require.NoError(t, err)
		require.NoError(t, state.CreateMultistreamEvent(legacy.Uri, "connected", "active"))
		_, err = state.CreateMultistreamTarget(ctx, nil, multistreamTestInput(), multistreamTestDID)
		require.NoError(t, err)
		var original []MultistreamTarget
		require.NoError(t, state.DB.Order("uri").Find(&original).Error)
		calls := 0
		client := multistreamSpaceClient{call: func(string, map[string]any, any) error {
			calls++
			return errors.New("unsupported PDS")
		}}
		require.ErrorContains(t, state.MigrateMultistreamTargets(ctx, client, multistreamTestDID), "unsupported PDS")
		require.Equal(t, 1, calls)
		var unchanged []MultistreamTarget
		require.NoError(t, state.DB.Order("uri").Find(&unchanged).Error)
		require.Equal(t, original, unchanged)
		var event MultistreamEvent
		require.NoError(t, state.DB.First(&event).Error)
		require.Equal(t, legacy.Uri, event.TargetURI)
		client.call = func(_ string, _ map[string]any, out any) error { return multistreamSuccess(out) }
		require.NoError(t, state.MigrateMultistreamTargets(ctx, client, multistreamTestDID))
		var migrated MultistreamTarget
		require.NoError(t, state.DB.First(&migrated).Error)
		require.True(t, migrated.InSpace)
	})
}

func TestMultistreamCreateSpaceFallback(t *testing.T) {
	for _, success := range []bool{false, true} {
		t.Run(map[bool]string{false: "fallback", true: "space"}[success], func(t *testing.T) {
			WithAllDatabases(t, func(state *StatefulDB) {
				client := multistreamSpaceClient{call: func(method string, body map[string]any, out any) error {
					require.Equal(t, "com.atproto.space.putRecord", method)
					if !success {
						return errors.New("unsupported PDS")
					}
					return multistreamSuccess(out)
				}}
				view, err := state.CreateMultistreamTarget(t.Context(), client, multistreamTestInput(), multistreamTestDID)
				require.NoError(t, err)
				var row MultistreamTarget
				require.NoError(t, state.DB.First(&row).Error)
				require.Equal(t, success, row.InSpace)
				rkey := view.Uri[strings.LastIndex(view.Uri, "/")+1:]
				require.Equal(t, "at://"+multistreamTestDID+"/"+multistreamCollection+"/"+rkey, view.Uri)
				require.Equal(t, view.Uri, row.URI)
				require.Equal(t, view.Cid, row.CID)
			})
		})
	}
}

func TestMultistreamSpaceWrites(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx := t.Context()
		methods := []string{}
		client := multistreamSpaceClient{call: func(method string, body map[string]any, out any) error {
			methods = append(methods, method)
			if out != nil {
				return multistreamSuccess(out)
			}
			return nil
		}}
		view, err := state.CreateMultistreamTarget(ctx, client, multistreamTestInput(), multistreamTestDID)
		require.NoError(t, err)
		rkey := view.Uri[strings.LastIndex(view.Uri, "/")+1:]
		original, err := state.GetMultistreamTarget(multistreamTestDID, rkey)
		require.NoError(t, err)
		require.True(t, original.InSpace)
		_, err = state.GetMultistreamTarget("did:plc:other", rkey)
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
		input := placestream.MultistreamPutTarget_Input{MultistreamTarget: multistreamTestInput().MultistreamTarget, Rkey: &rkey}
		name := "renamed"
		input.MultistreamTarget.Name = &name
		failure := errors.New("PDS write failed")
		failing := multistreamSpaceClient{call: func(string, map[string]any, any) error { return failure }}
		for _, c := range []spaces.Client{nil, failing} {
			_, err = state.UpdateMultistreamTarget(ctx, c, view.Uri, input)
			require.Error(t, err)
			require.Error(t, state.DeleteMultistreamTarget(ctx, c, view.Uri))
			unchanged, err := state.GetMultistreamTarget(multistreamTestDID, rkey)
			require.NoError(t, err)
			require.Equal(t, original, unchanged)
		}
		updated, err := state.UpdateMultistreamTarget(ctx, client, view.Uri, input)
		require.NoError(t, err)
		require.Equal(t, view.Uri, updated.Uri)
		require.Equal(t, "upstream-cid", updated.Cid)
		listed, err := state.ListMultistreamTargets(multistreamTestDID, 100, 0, nil)
		require.NoError(t, err)
		require.Equal(t, name, *listed[0].Record.Val.(*placestream.MultistreamTarget).Name)
		require.NoError(t, state.DeleteMultistreamTarget(ctx, client, view.Uri))
		_, err = state.GetMultistreamTarget(multistreamTestDID, rkey)
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
		require.Equal(t, []string{"com.atproto.space.putRecord", "com.atproto.space.putRecord", "com.atproto.space.deleteRecord"}, methods)
	})
}

func TestMultistreamCreateLimitsBeforePDS(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		for range MAX_ACTIVE_MULTISTREAM_TARGETS {
			input := multistreamTestInput()
			input.MultistreamTarget.Active = true
			_, err := state.CreateMultistreamTarget(t.Context(), nil, input, multistreamTestDID)
			require.NoError(t, err)
		}
		client := multistreamSpaceClient{call: func(string, map[string]any, any) error {
			t.Fatal("limit must be enforced before writing the PDS")
			return nil
		}}
		input := multistreamTestInput()
		input.MultistreamTarget.Active = true
		_, err := state.CreateMultistreamTarget(t.Context(), client, input, multistreamTestDID)
		require.ErrorContains(t, err, "maximum number of active")
		for i := MAX_ACTIVE_MULTISTREAM_TARGETS; i < MAX_MULTISTREAM_TARGETS; i++ {
			_, err := state.CreateMultistreamTarget(t.Context(), nil, multistreamTestInput(), multistreamTestDID)
			require.NoError(t, err)
		}
		_, err = state.CreateMultistreamTarget(t.Context(), client, multistreamTestInput(), multistreamTestDID)
		require.ErrorContains(t, err, "maximum number of multistream targets")
	})
}
