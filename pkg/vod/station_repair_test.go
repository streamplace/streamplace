package vod

import (
	"context"
	"testing"

	glex "github.com/streamplace/glex/runtime"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/placestream"
)

func TestStationAuditsLatePutAfterWithdrawalACK(t *testing.T) {
	ctx := t.Context()
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	did, cid := stationFederationTestDID, bdasl.CID([]byte("late-put"))
	prefix := stationOriginPrefix(did)
	uri := "at://" + did + "/" + constants.PLACE_STREAM_MEDIA_ORIGIN + "/" + cid
	poison := bdasl.CID([]byte("corrupt public intent"))
	require.NoError(t, writeStationMarker(ctx, store, prefix+"public/"+poison, []byte("{")))
	require.NoError(t, WithdrawStationOrigin(ctx, store, did, cid))
	require.NoError(t, writeStationMarker(ctx, store, prefix+"acked/withdrawn/"+cid, []byte("{}")))
	// Old A's request arrives after B got a successful delete, persisted its
	// ACK and restarted. No new local write/retry exists to wake B; a PDS
	// page containing the resurrected record must independently repair it.
	exists := true
	var methods []string
	client := stationXRPCFunc(func(_ context.Context, _, _, method string, params map[string]any, body, out any) error {
		methods = append(methods, method)
		switch method {
		case "com.atproto.repo.listRecords":
			require.Equal(t, 100, params["limit"])
			page := out.(*comatproto.RepoListRecords_Output)
			if params["cursor"] == nil {
				next := "second-page"
				page.Cursor = &next
			} else {
				require.Equal(t, "second-page", params["cursor"])
				page.Records = []comatproto.RepoListRecords_Record{
					{Uri: "at://" + did + "/" + constants.PLACE_STREAM_MEDIA_ORIGIN + "/" + poison},
					{Uri: uri},
				}
			}
		case "com.atproto.repo.getRecord":
			require.True(t, exists)
			*out.(*comatproto.RepoGetRecord_Output) = comatproto.RepoGetRecord_Output{Uri: uri, Value: &glex.LexiconTypeDecoder{Val: &placestream.MediaOrigin{Blob: cid}}}
		case "com.atproto.repo.deleteRecord":
			require.Equal(t, cid, body.(*comatproto.RepoDeleteRecord_Input).Rkey)
			exists = false
		default:
			t.Fatalf("unexpected operation: %s", method)
		}
		return nil
	})
	cursor, err := repairStationWithdrawals(ctx, store, client, did, "")
	require.NoError(t, err)
	require.Equal(t, "second-page", cursor)
	cursor, err = repairStationWithdrawals(ctx, store, client, did, cursor)
	require.NoError(t, err)
	require.Empty(t, cursor)
	require.False(t, exists)
	require.Equal(t, []string{"com.atproto.repo.listRecords", "com.atproto.repo.listRecords", "com.atproto.repo.getRecord", "com.atproto.repo.deleteRecord"}, methods)
}
