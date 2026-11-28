package statedb

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spaces"
	"stream.place/streamplace/pkg/spaces/spacestest"
)

func openTestDrafts(t *testing.T, state *StatefulDB, did string, client spaces.Client) *Drafts {
	t.Helper()
	drafts, err := state.OpenDrafts(t.Context(), did, client)
	require.NoError(t, err)
	return drafts
}

type draftSpaceFault struct {
	spacestest.Fake
	intercept func(method string, call func() error) error
}

func (c *draftSpaceFault) Do(ctx context.Context, kind, enc, method string, params map[string]any, body, out any) error {
	call := func() error { return c.Fake.Do(ctx, kind, enc, method, params, body, out) }
	if c.intercept != nil {
		return c.intercept(method, call)
	}
	return call()
}

func TestPrivateDraftMigrationRetry(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx, did := t.Context(), "did:plc:retry"
		legacy := openTestDrafts(t, state, did, nil)
		first, err := legacy.Create(ctx, "first", newDraftRec("First SQL title", "processing"))
		require.NoError(t, err)
		second, err := legacy.Create(ctx, "second", newDraftRec("Second SQL title", "processing"))
		require.NoError(t, err)
		legacy.Close()

		space := &draftSpaceFault{}
		_, _, err = spaces.PutRecord(ctx, space, did, spaces.DraftMetadataCollection, draftRkey(first.URI), &placestream.VodDraftMetadata{Title: "Existing remote edit"})
		require.NoError(t, err)
		outage := errors.New("PDS temporarily unavailable")
		createCalls := 0
		space.intercept = func(method string, call func() error) error {
			if method == "com.atproto.space.createRecord" {
				createCalls++
				if createCalls == 2 {
					return outage
				}
			}
			return call()
		}
		_, err = state.OpenDrafts(ctx, did, space)
		require.ErrorIs(t, err, outage)
		_, err = state.OpenDrafts(ctx, did, nil)
		require.ErrorIs(t, err, ErrDraftReauthorize)

		// Only the completed row has been scrubbed. A worker transition after
		// that checkpoint must not copy the old SQL title back to the space.
		raw, err := state.getDraft(ctx, did, first.URI)
		require.NoError(t, err)
		rec, err := raw.Record()
		require.NoError(t, err)
		require.Empty(t, rec.Title)
		require.NoError(t, state.CreateUpload(ctx, &Upload{
			ID: "first", RepoDID: did, DurationMS: 2000, ContentCID: "content-first", ProcessingStatus: "done",
		}))
		require.NoError(t, state.syncDraftResult(ctx, "first"))
		drafts := openTestDrafts(t, state, did, space)
		defer drafts.Close()
		got, err := drafts.Get(ctx, first.URI)
		require.NoError(t, err)
		rec, err = got.Record()
		require.NoError(t, err)
		require.Equal(t, "Existing remote edit", rec.Title)
		require.Equal(t, "ready", rec.Status)
		got, err = drafts.Get(ctx, second.URI)
		require.NoError(t, err)
		rec, err = got.Record()
		require.NoError(t, err)
		require.Equal(t, "Second SQL title", rec.Title)
	})
}

func TestPrivateDraftLostCreateResponseDoesNotResurrectDeletedRecord(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx, did := t.Context(), "did:plc:lostresponse"
		legacy := openTestDrafts(t, state, did, nil)
		row, err := legacy.Create(ctx, "", newDraftRec("Legacy", "ready"))
		require.NoError(t, err)
		legacy.Close()
		space := &draftSpaceFault{}
		lost := errors.New("connection dropped after committing")
		space.intercept = func(method string, call func() error) error {
			err := call()
			if method == "com.atproto.space.createRecord" && err == nil {
				return lost
			}
			return err
		}
		_, err = state.OpenDrafts(ctx, did, space)
		require.ErrorIs(t, err, lost)
		require.NoError(t, spaces.DeleteRecord(ctx, space, did, spaces.DraftMetadataCollection, draftRkey(row.URI)))
		space.intercept = nil
		drafts := openTestDrafts(t, state, did, space)
		defer drafts.Close()
		_, err = drafts.Get(ctx, row.URI)
		require.ErrorIs(t, err, ErrDraftNotFound)
		require.Empty(t, space.Records)
		raw, err := state.getDraft(ctx, did, row.URI)
		require.NoError(t, err)
		rec, err := raw.Record()
		require.NoError(t, err)
		require.Empty(t, rec.Title)
	})
}

func TestPrivateDraftCRUDIgnoresForgedOperationalState(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx, did := t.Context(), "did:plc:private"
		const uploadID = "trusted-upload"
		require.NoError(t, state.CreateUpload(ctx, &Upload{ID: uploadID, RepoDID: did}))
		space := &draftSpaceFault{}
		drafts := openTestDrafts(t, state, did, space)
		rec := newDraftRec("Initial title", "processing")
		rec.Connections = []placestream.VodDraftVideo_Connections_Elem{{Video_Connection: &placestream.Video_Connection{
			Ref: &comatproto.RepoStrongRef{Uri: "at://" + did + "/place.stream.livestream/live", Cid: "trusted-connection"},
		}}}
		row, err := drafts.Create(ctx, uploadID, rec)
		require.NoError(t, err)
		key := draftRkey(row.URI)
		var written map[string]any
		require.NoError(t, json.Unmarshal(space.Records[key], &written))
		require.Equal(t, map[string]any{"$type": spaces.DraftMetadataCollection, "title": "Initial title"}, written)

		space.Records[key] = json.RawMessage(`{"$type":"place.stream.vod.draftMetadata","title":"PDS title","description":"PDS description","tags":["remove"],"status":"error","durationMs":999999,"source":false,"error":"forged","connections":false,"createdAt":"forged","originUploadId":"forged","contentCid":"forged"}`)
		updated, err := drafts.Update(ctx, &placestream.VodUpdateDraft_Input{Uri: row.URI, Tags: []string{}})
		require.NoError(t, err)
		merged, err := updated.Record()
		require.NoError(t, err)
		require.Equal(t, "PDS title", merged.Title)
		require.Equal(t, "PDS description", *merged.Description)
		require.Empty(t, merged.Tags)
		require.Equal(t, rec.CreatedAt, merged.CreatedAt)
		require.Equal(t, "trusted-connection", merged.Connections[0].Video_Connection.Ref.Cid)
		require.Equal(t, "processing", merged.Status)
		require.Nil(t, merged.Error)
		require.Nil(t, merged.Source)
		require.Nil(t, merged.DurationMs)
		require.Equal(t, uploadID, updated.OriginUploadID)
		require.Empty(t, updated.ContentCID)
		drafts.Close()

		require.NoError(t, state.SetUploadProcessed(ctx, uploadID, 1000, "trusted-content", "", "", 0))
		require.NoError(t, state.syncDraftResult(ctx, uploadID))
		raw, err := state.getDraft(ctx, did, row.URI)
		require.NoError(t, err)
		local, err := raw.Record()
		require.NoError(t, err)
		require.Empty(t, local.Title)
		require.Nil(t, local.Description)
		require.Equal(t, "trusted-content", raw.ContentCID)
		drafts = openTestDrafts(t, state, did, space)
		ready, err := drafts.Get(ctx, row.URI)
		require.NoError(t, err)
		merged, err = ready.Record()
		require.NoError(t, err)
		require.Equal(t, "PDS title", merged.Title)
		require.Equal(t, "ready", merged.Status)
		require.Equal(t, int64(1000), *merged.DurationMs)

		outage := errors.New("PDS unavailable")
		space.Err = outage
		_, err = drafts.Get(ctx, row.URI)
		require.ErrorIs(t, err, outage)
		_, _, err = drafts.List(ctx, 50, "")
		require.ErrorIs(t, err, outage)
		require.ErrorIs(t, drafts.Delete(ctx, row.URI), outage)
		space.Err = nil
		_, err = drafts.Get(ctx, row.URI)
		require.NoError(t, err, "a failed private delete must keep the local anchor")
		require.NoError(t, drafts.Delete(ctx, row.URI))
		drafts.Close()

		_, err = state.OpenDrafts(ctx, did, nil)
		require.ErrorIs(t, err, ErrDraftReauthorize, "deleting the last draft must not reset cutover")
		_, err = state.CreateUploadDraftTx(ctx, state.DB, did, uploadID, nil)
		require.NoError(t, err)
		drafts = openTestDrafts(t, state, did, space)
		defer drafts.Close()
		rows, _, err := drafts.List(ctx, 50, "")
		require.NoError(t, err)
		require.Empty(t, rows, "duplicate completion must not replace a deleted draft")
	})
}

func TestPrivateDraftListAndUpdateRespectDirectDeletes(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx, did := t.Context(), "did:plc:pages"
		space := &draftSpaceFault{}
		drafts := openTestDrafts(t, state, did, space)
		defer drafts.Close()
		var uris []string
		for _, title := range []string{"Earlier", "Later", "Deleted newest"} {
			row, err := drafts.Create(ctx, "", newDraftRec(title, "processing"))
			require.NoError(t, err)
			uris = append(uris, row.URI)
		}
		require.NoError(t, state.DB.Model(&DraftVideo{}).Where("uri IN ?", uris).
			Update("created_at", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)).Error)
		require.NoError(t, spaces.DeleteRecord(ctx, space, did, spaces.DraftMetadataCollection, draftRkey(uris[2])))
		rows, next, err := drafts.List(ctx, 1, "")
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, uris[1], rows[0].URI)
		rows, _, err = drafts.List(ctx, 1, next)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, uris[0], rows[0].URI)
		space.intercept = func(method string, call func() error) error {
			if method == "com.atproto.space.applyWrites" {
				// Simulate a direct deletion after updateDraft read the record.
				require.NoError(t, spaces.DeleteRecord(ctx, &space.Fake, did, spaces.DraftMetadataCollection, draftRkey(uris[0])))
			}
			return call()
		}
		title := "Do not resurrect"
		_, err = drafts.Update(ctx, &placestream.VodUpdateDraft_Input{Uri: uris[0], Title: &title})
		require.ErrorIs(t, err, ErrDraftNotFound)
		require.NotContains(t, space.Records, draftRkey(uris[0]))
	})
}

func TestMigratedFinalizeFailureLeavesOperationalPendingDraft(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx, did := t.Context(), "did:plc:pending"
		space := &spacestest.Fake{}
		openTestDrafts(t, state, did, space).Close()
		const id = "late-publish"
		require.NoError(t, state.CreateLivestreamUpload(ctx, id, did, "at://"+did+"/place.stream.livestream/live"))
		state.SetLivestreamVODFinalizer(func(ctx context.Context, task FinalizeLivestreamVODTask) (string, error) {
			return "finished-content", state.SetUploadProcessed(ctx, task.UploadID, 4500, "finished-content", "", "", 123)
		})
		state.SetVideoPublisher(func(context.Context, FinalizeLivestreamVODTask) (string, string, error) {
			return "", "", errBoom
		})
		task := enqueueClaimedTask(t, state, TaskFinalizeLivestreamVOD, FinalizeLivestreamVODTask{
			UploadID: id, RepoDID: did, Publish: &VideoDraft{Title: "Requested title", Tags: []string{"custom"}},
		}, WithMaxTries(1), WithTaskKey("finalize-vod:"+id))
		require.NoError(t, state.runClaimedTask(ctx, task, time.Hour))
		row, err := state.getDraftByUpload(ctx, id)
		require.NoError(t, err)
		require.NotNil(t, row)
		local, err := row.Record()
		require.NoError(t, err)
		require.Empty(t, local.Title)
		require.Empty(t, local.Tags)
		require.Equal(t, "ready", local.Status)
		require.Equal(t, "finished-content", row.ContentCID)
		_, err = state.OpenDrafts(ctx, did, nil)
		require.ErrorIs(t, err, ErrDraftReauthorize)

		drafts := openTestDrafts(t, state, did, space)
		defer drafts.Close()
		visible, err := drafts.Get(ctx, row.URI)
		require.NoError(t, err)
		rec, err := visible.Record()
		require.NoError(t, err)
		require.Equal(t, "Requested title", rec.Title)
		require.Equal(t, []string{"custom"}, rec.Tags)
		require.Equal(t, "ready", rec.Status)
		require.Equal(t, int64(4500), *rec.DurationMs)
		scrubbed, err := state.GetTask(ctx, task.ID)
		require.NoError(t, err)
		require.NotContains(t, string(scrubbed.Payload), "Requested title")
		require.NotContains(t, string(scrubbed.Payload), "custom")
		require.NotContains(t, string(scrubbed.Payload), `"publish"`)
	})
}

func TestMigratedImplicitUploadPreservesEditsWhileProcessing(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx, did := t.Context(), "did:plc:worker"
		const uploadID = "implicit"
		space := &spacestest.Fake{}
		openTestDrafts(t, state, did, space).Close()
		require.NoError(t, state.CreateUpload(ctx, &Upload{ID: uploadID, RepoDID: did, Backend: "file", Filename: "original.mp4"}))
		var uri string
		state.SetVODProcessor(func(ctx context.Context, task VODProcessTask) (string, error) {
			row, err := state.getDraftByUpload(ctx, task.UploadID)
			require.NoError(t, err)
			require.NotNil(t, row)
			uri = row.URI
			local, err := row.Record()
			require.NoError(t, err)
			require.Empty(t, local.Title, "pending worker anchor must not contain editable metadata")
			drafts := openTestDrafts(t, state, did, space)
			defer drafts.Close()
			visible, err := drafts.Get(ctx, uri)
			require.NoError(t, err)
			record, err := visible.Record()
			require.NoError(t, err)
			require.Equal(t, "original.mp4", record.Title)
			require.Equal(t, "processing", record.Status)
			title := "Edited before processing finished"
			_, err = drafts.Update(ctx, &placestream.VodUpdateDraft_Input{Uri: uri, Title: &title})
			require.NoError(t, err)
			return "finished-content", state.SetUploadProcessed(ctx, task.UploadID, 8000, "finished-content", "", "", 100)
		})
		task, err := state.EnqueueTask(ctx, TaskVODProcess, VODProcessTask{UploadID: uploadID, RepoDID: did})
		require.NoError(t, err)
		require.NoError(t, state.processVODProcessTask(ctx, task))
		drafts := openTestDrafts(t, state, did, space)
		defer drafts.Close()
		ready, err := drafts.Get(ctx, uri)
		require.NoError(t, err)
		record, err := ready.Record()
		require.NoError(t, err)
		require.Equal(t, "Edited before processing finished", record.Title)
		require.Equal(t, "ready", record.Status)
		require.Equal(t, int64(8000), *record.DurationMs)
	})
}

func TestScopedDraftAccessRequiresCompleteMigration(t *testing.T) {
	WithAllDatabases(t, func(state *StatefulDB) {
		ctx, did := t.Context(), "did:plc:scopeddraft"
		space := &draftSpaceFault{}
		legacy := openTestDrafts(t, state, did, nil)
		known, err := legacy.Create(ctx, "", newDraftRec("Original", "ready"))
		require.NoError(t, err)
		legacy.Close()
		_, err = state.OpenMigratedDrafts(ctx, did, space)
		require.ErrorIs(t, err, ErrDraftReauthorize)
		require.Empty(t, space.Calls)

		outage := errors.New("PDS write failed")
		space.intercept = func(method string, call func() error) error {
			if method == "com.atproto.space.createRecord" {
				return outage
			}
			return call()
		}
		_, err = state.OpenDrafts(ctx, did, space)
		require.ErrorIs(t, err, outage)
		space.Calls = nil
		_, err = state.OpenMigratedDrafts(ctx, did, space)
		require.ErrorIs(t, err, ErrDraftReauthorize)
		require.Empty(t, space.Calls)

		space.intercept = nil
		openTestDrafts(t, state, did, space).Close()
		const uploadID = "scoped-pending"
		require.NoError(t, state.CreateUpload(ctx, &Upload{ID: uploadID, RepoDID: did, Filename: "pending.mp4"}))
		pending, err := state.CreateUploadDraftTx(ctx, state.DB, did, uploadID, nil)
		require.NoError(t, err)
		require.NotNil(t, pending)
		// Restricted callers cannot materialize worker drafts, seed private
		// defaults, or fall back to a SQL read while carrying a read grant.
		space.Calls = nil
		drafts, err := state.OpenMigratedDrafts(ctx, did, space)
		require.NoError(t, err)
		defer drafts.Close()
		_, err = drafts.Get(ctx, pending.URI)
		require.ErrorIs(t, err, ErrDraftNotFound)
		require.Empty(t, space.Calls)
		rev := space.Rev
		got, err := drafts.Get(ctx, known.URI)
		require.NoError(t, err)
		record, err := got.Record()
		require.NoError(t, err)
		require.Equal(t, "Original", record.Title)
		rows, _, err := drafts.List(ctx, 10, "")
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, known.URI, rows[0].URI)
		require.Equal(t, rev, space.Rev)
		space.Calls = nil
		require.NoError(t, drafts.Delete(ctx, known.URI))
		require.Equal(t, []string{"com.atproto.space.applyWrites"}, space.Calls)
	})
}
