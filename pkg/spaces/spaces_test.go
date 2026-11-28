package spaces

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/bluesky-social/indigo/xrpc"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/lexicons"
	"stream.place/streamplace/pkg/placestream"
)

type fakeClient struct {
	call func(method string, body map[string]any, out any) error
}

func (c fakeClient) Do(ctx context.Context, kind, inpenc, method string, params map[string]any, body, out any) error {
	if kind != "POST" || inpenc != "application/json" || params != nil {
		return fmt.Errorf("unexpected request: %s %s %v", kind, inpenc, params)
	}
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

func rpcError(name string) error {
	return fmt.Errorf("upstream: %w", &xrpc.Error{StatusCode: 400, Wrapped: &xrpc.XRPCError{ErrStr: name}})
}

func TestPutRecordCreatesSpace(t *testing.T) {
	for _, concurrent := range []bool{false, true} {
		t.Run(fmt.Sprint(concurrent), func(t *testing.T) {
			var methods []string
			puts := 0
			did, collection, rkey := "did:plc:example", "place.stream.multistream.target", "original-key"
			record := map[string]any{"$type": collection, "active": false}
			recordURI := PersonalSpaceURI(did) + "/" + did + "/" + collection + "/" + rkey
			client := fakeClient{call: func(method string, body map[string]any, out any) error {
				methods = append(methods, method)
				switch method {
				case "com.atproto.space.putRecord":
					puts++
					require.Equal(t, map[string]any{"space": PersonalSpaceURI(did), "repo": did, "collection": collection, "rkey": rkey, "record": record}, body)
					if puts == 1 {
						return rpcError("SpaceNotFound")
					}
					return json.Unmarshal([]byte(`{"uri":"`+recordURI+`","cid":"space-cid"}`), out)
				case "com.atproto.space.createSpace":
					require.Equal(t, map[string]any{"spaceType": PersonalSpaceType, "skey": PersonalSpaceKey, "policy": map[string]any{"$type": "com.atproto.personalspace.policy"}}, body)
					if concurrent {
						return rpcError("SpaceAlreadyExists")
					}
					return nil
				default:
					t.Fatalf("unexpected method %s", method)
					return nil
				}
			}}
			uri, cid, err := PutRecord(t.Context(), client, did, collection, rkey, record)
			require.NoError(t, err)
			require.Equal(t, recordURI, uri)
			require.Equal(t, "space-cid", cid)
			require.Equal(t, []string{"com.atproto.space.putRecord", "com.atproto.space.createSpace", "com.atproto.space.putRecord"}, methods)
		})
	}
}

func TestPutRecordStopsOnErrors(t *testing.T) {
	for _, failAt := range []string{"put", "create", "retry"} {
		t.Run(failAt, func(t *testing.T) {
			failure := errors.New("PDS unavailable")
			calls := 0
			client := fakeClient{call: func(method string, _ map[string]any, _ any) error {
				calls++
				if failAt == "put" || (failAt == "create" && calls == 2) {
					return failure
				}
				if method == "com.atproto.space.createSpace" {
					return nil
				}
				return rpcError("SpaceNotFound")
			}}
			_, _, err := PutRecord(t.Context(), client, "did:plc:example", "place.stream.multistream.target", "abc", map[string]any{})
			require.Error(t, err)
			if failAt == "retry" {
				require.Equal(t, 3, calls)
			} else {
				require.ErrorIs(t, err, failure)
			}
		})
	}
}

func TestDeleteRecord(t *testing.T) {
	failure := errors.New("PDS unavailable")
	for _, upstreamErr := range []error{nil, failure} {
		client := fakeClient{call: func(method string, body map[string]any, _ any) error {
			require.Equal(t, "com.atproto.space.deleteRecord", method)
			require.Equal(t, map[string]any{"space": PersonalSpaceURI("did:plc:example"), "repo": "did:plc:example", "collection": "place.stream.multistream.target", "rkey": "abc"}, body)
			return upstreamErr
		}}
		require.ErrorIs(t, DeleteRecord(t.Context(), client, "did:plc:example", "place.stream.multistream.target", "abc"), upstreamErr)
	}
}

func TestLexiconOverlay(t *testing.T) {
	docs, err := LexiconOverlay()
	require.NoError(t, err)
	require.Len(t, docs, 2)
	var declaration struct {
		Defs struct {
			Main struct {
				Type        string   `json:"type"`
				Key         string   `json:"key"`
				Collections []string `json:"collections"`
			} `json:"main"`
		} `json:"defs"`
	}
	require.NoError(t, json.Unmarshal(docs[PersonalSpaceType], &declaration))
	require.Equal(t, "space", declaration.Defs.Main.Type)
	require.Equal(t, "literal:self", declaration.Defs.Main.Key)
	require.Contains(t, declaration.Defs.Main.Collections, DraftMetadataCollection)
	var auth map[string]any
	require.NoError(t, json.Unmarshal(docs[lexicons.AuthFullID], &auth))
	require.NotContains(t, auth, "$type")
	main := auth["defs"].(map[string]any)["main"].(map[string]any)
	permissions := main["permissions"].([]any)
	require.Len(t, permissions, 2)
	require.NotContains(t, permissions[0].(map[string]any)["collection"], DraftMetadataCollection)
	want := map[string]any{"type": "permission", "resource": "space", "spaceType": PersonalSpaceType, "authority": "self", "collection": declaration.Defs.Main.Collections, "action": []string{"read", "create", "update", "delete"}, "manage": []string{"create"}}
	bs, err := json.Marshal(want)
	require.NoError(t, err)
	got, err := json.Marshal(permissions[1])
	require.NoError(t, err)
	require.JSONEq(t, string(bs), string(got))
	main["permissions"] = permissions[:1]
	withoutSpace, err := json.Marshal(auth)
	require.NoError(t, err)
	base, err := lexicons.AllFiles.ReadFile("place/stream/authFull.json")
	require.NoError(t, err)
	require.JSONEq(t, string(base), string(withoutSpace))
}

// pagedClient serves listRecords one page per call.
type pagedClient struct {
	pages []string
	err   error
}

func (c *pagedClient) Do(_ context.Context, _, _, _ string, params map[string]any, _, out any) error {
	if c.err != nil {
		return c.err
	}
	// The cursor a page hands out is the index of the next one.
	cursor, _ := params["cursor"].(string)
	page, _ := strconv.Atoi(cursor)
	return json.Unmarshal([]byte(c.pages[page]), out)
}

func TestListMultistreamTargets(t *testing.T) {
	did := "did:plc:example"
	value := `{"$type":"place.stream.multistream.target","url":"rtmp://localhost/live/key","createdAt":"2026-11-11T00:00:00Z","active":true}`
	client := &pagedClient{pages: []string{
		`{"cursor":"1","records":[{"rkey":"b","cid":"cid-b","value":` + value + `},{"rkey":"bad","cid":"cid-bad","value":"not a record"}]}`,
		`{"records":[{"rkey":"a","cid":"cid-a","value":` + value + `}]}`,
	}}
	targets, err := ListMultistreamTargets(t.Context(), client, did)
	require.NoError(t, err)
	require.Len(t, targets, 2)
	require.Equal(t, TargetURI(did, "a"), targets[0].Uri)
	require.Equal(t, "cid-b", targets[1].Cid)
	require.True(t, targets[1].Record.Val.(*placestream.MultistreamTarget).Active)

	// A space that does not exist yet, or was never written to, is empty.
	for _, name := range []string{"SpaceNotFound", "RepoNotFound"} {
		client.err = rpcError(name)
		targets, err = ListMultistreamTargets(t.Context(), client, did)
		require.NoError(t, err)
		require.Empty(t, targets)
		rev, err := LatestRev(t.Context(), client, did)
		require.NoError(t, err)
		require.Empty(t, rev)
	}
	client.err = errors.New("PDS unavailable")
	_, err = ListMultistreamTargets(t.Context(), client, did)
	require.ErrorIs(t, err, client.err)
	_, err = LatestRev(t.Context(), client, did)
	require.ErrorIs(t, err, client.err)
}

func TestDraftGrantUsesExpandedScope(t *testing.T) {
	const did = "did:plc:example"
	const base = "space:place.stream.personal?authority=" + did
	const collection = "&collection=place.stream.vod.draftMetadata"
	for _, scope := range []string{
		"atproto " + base + "&collection=place.stream.multistream.target" + collection + "&manage=create",
		base + collection + "&skey=self&action=read_self&action=create&action=update&action=delete",
		"space:place.stream.personal?authority=did%3Aplc%3Aexample" + collection,
		base + collection + "&action=read " + base + collection + "&action=create&action=update&action=delete",
	} {
		require.True(t, CollectionGrant(scope, did, DraftMetadataCollection), scope)
	}
	for _, scope := range []string{
		"atproto include:place.stream.authFull",
		"space:place.stream.personal",
		base + "&collection=place.stream.multistream.target",
		base + "&collection=place.stream.vod.draftMetadata.other",
		base + collection + "&action=read&action=delete",
		base + collection + "&skey=other",
		base + collection + "&authority=*",
		"space:place.stream.personal?authority=did:plc:other" + collection,
		"space:place.stream.personal.other?authority=" + did + collection,
		"other:" + base + collection,
	} {
		require.False(t, CollectionGrant(scope, did, DraftMetadataCollection), scope)
	}
}

func TestCollectionActionsGrant(t *testing.T) {
	const did = "did:plc:example"
	const base = "space:place.stream.personal?authority=" + did + "&skey=self&collection=place.stream.vod.draftMetadata"
	for _, tc := range []struct {
		name, scope string
		actions     []string
		want        bool
	}{
		{"read only", base + "&action=read_self", []string{"read"}, true},
		{"read cannot write", base + "&action=read", []string{"update"}, false},
		{"write cannot read", base + "&action=delete", []string{"read"}, false},
		{"all requested actions must be granted", base + "&action=delete", []string{"read", "delete"}, false},
		{"actions can span grants", base + "&action=read " + base + "&action=delete", []string{"read", "delete"}, true},
		{"other collection cannot supply delete", base + "&action=read space:place.stream.personal?authority=" + did + "&collection=place.stream.multistream.target&action=delete", []string{"read", "delete"}, false},
		{"unknown requested action", base, []string{"manage"}, false},
		{"empty action request", base, nil, false},
		{"unknown granted action", base + "&action=read&action=manage", []string{"read"}, false},
		{"explicit empty granted action", base + "&action=", []string{"read"}, false},
		{"unexpanded authority", "space:place.stream.personal?authority=self&skey=self&collection=place.stream.vod.draftMetadata", []string{"read"}, false},
		{"empty authority", "space:place.stream.personal?authority=&collection=place.stream.vod.draftMetadata", []string{"read"}, false},
		{"ambiguous authority", base + "&authority=" + did, []string{"read"}, false},
		{"empty key", "space:place.stream.personal?authority=" + did + "&skey=&collection=place.stream.vod.draftMetadata", []string{"read"}, false},
		{"ambiguous key", base + "&skey=self", []string{"read"}, false},
		{"empty collection", base + "&collection=", []string{"read"}, false},
		{"valid grant after malformed token", base + "&action=% " + base + "&action=read", []string{"read"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, CollectionActionsGrant(tc.scope, did, DraftMetadataCollection, tc.actions...))
		})
	}
}
