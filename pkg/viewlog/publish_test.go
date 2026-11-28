package viewlog

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/placestream"
)

const (
	countStation = "did:web:station.example"
	countVideoA  = "at://did:web:alice.example/place.stream.video/3k6hd4i7ks323"
	countVideoB  = "at://did:web:bob.example/place.stream.video/3k6hd4i7ks323"
)

type countRPC func(context.Context, string, string, string, map[string]any, any, any) error

func (f countRPC) Do(ctx context.Context, kind, contentType, method string, params map[string]any, body, out any) error {
	return f(ctx, kind, contentType, method, params, body, out)
}

type countListStore struct {
	blob.Store
	list func(context.Context, string) ([]string, error)
}

func (s countListStore) List(ctx context.Context, prefix string) ([]string, error) {
	return s.list(ctx, prefix)
}

func countTestCID(t *testing.T, name string) string {
	t.Helper()
	c, err := (cid.Prefix{Version: 1, Codec: cid.DagCBOR, MhType: 0x12, MhLength: 32}).Sum([]byte(name))
	require.NoError(t, err)
	return c.String()
}

// testCountPDS applies CAS before changing repo state. afterPut can drop or
// corrupt the acknowledgement after the write has already taken effect.
type testCountPDS struct {
	t        *testing.T
	head     string
	heads    int
	puts     int
	records  map[string]placestream.MediaViewCount
	afterPut func(*comatproto.RepoPutRecord_Output) error
}

func (p *testCountPDS) Do(ctx context.Context, kind, contentType, method string, params map[string]any, body, out any) error {
	_, bounded := ctx.Deadline()
	require.True(p.t, bounded)
	switch method {
	case "com.atproto.sync.getLatestCommit":
		require.Equal(p.t, xrpc.Query, kind)
		require.Equal(p.t, countStation, params["did"])
		p.heads++
		out.(*comatproto.SyncGetLatestCommit_Output).Cid = p.head
	case "com.atproto.repo.putRecord":
		require.Equal(p.t, xrpc.Procedure, kind)
		require.Equal(p.t, "application/json", contentType)
		in := body.(*comatproto.RepoPutRecord_Input)
		require.Equal(p.t, countStation, in.Repo)
		require.Equal(p.t, constants.PLACE_STREAM_MEDIA_VIEW_COUNT, in.Collection)
		require.NotNil(p.t, in.SwapCommit)
		_, err := syntax.ParseRecordKey(in.Rkey)
		require.NoError(p.t, err)
		p.puts++
		if *in.SwapCommit != p.head {
			return &xrpc.XRPCError{ErrStr: "InvalidSwap", Message: "private PDS response"}
		}
		uri := "at://" + in.Repo + "/" + in.Collection + "/" + in.Rkey
		p.records[uri] = *in.Record.Val.(*placestream.MediaViewCount)
		p.head = countTestCID(p.t, fmt.Sprint(p.puts))
		ack := out.(*comatproto.RepoPutRecord_Output)
		*ack = comatproto.RepoPutRecord_Output{
			Uri: uri, Cid: countTestCID(p.t, uri), Commit: &comatproto.RepoDefs_CommitMeta{Cid: p.head},
		}
		if p.afterPut != nil {
			return p.afterPut(ack)
		}
	default:
		p.t.Fatalf("unexpected XRPC method %s", method)
	}
	return nil
}

func countTestInput(t *testing.T) (RunAggregationInput, *testCountPDS, string) {
	t.Helper()
	root := t.TempDir()
	store, err := blob.NewFileStore(root)
	require.NoError(t, err)
	start := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	pds := &testCountPDS{t: t, head: countTestCID(t, "initial"), records: make(map[string]placestream.MediaViewCount)}
	return RunAggregationInput{
		Store: store, StationClient: pds,
		CLI:         &config.CLI{StationDID: countStation, StationOriginPublisher: true},
		WindowStart: start, WindowEnd: start.Add(time.Minute),
	}, pds, root
}

func writeCountTestLogs(t *testing.T, root, node string, at time.Time, videos ...string) {
	t.Helper()
	ctx := t.Context()
	w := newAggTestWriter(t, root, node, at)
	for i, video := range videos {
		sid := fmt.Sprintf("%s-%d", node, i)
		w.Log(ctx, Event{Ts: at, Type: EventTypeManifestRequest, VideoURI: video, SID: sid, ManifestKind: ManifestKindMaster})
		w.Log(ctx, Event{Ts: at.Add(time.Millisecond), Type: EventTypeSegmentRequest, SID: sid})
	}
	runOneAndClose(t, ctx, w)
}

func TestStationViewCountRejectsStaleSnapshot(t *testing.T) {
	in, pds, root := countTestInput(t)
	writeCountTestLogs(t, root, "did:web:node1.example", in.WindowStart, countVideoA)
	old := in
	old.Store = countListStore{Store: in.Store, list: func(ctx context.Context, prefix string) ([]string, error) {
		require.Equal(t, 1, pds.heads, "station head must be captured before listing or reading logs")
		keys, err := in.Store.List(ctx, prefix)
		require.NoError(t, err)
		// A separate late-data task publishes from the new log set while the
		// first attempt still owns only its old listing.
		writeCountTestLogs(t, root, "did:web:node2.example", in.WindowStart.Add(time.Second), countVideoA)
		require.NoError(t, RunAggregation(ctx, in))
		return keys, nil
	}}
	err := RunAggregation(t.Context(), old)
	require.ErrorContains(t, err, "commit changed")
	require.NotContains(t, err.Error(), "private PDS response")
	require.Equal(t, 2, pds.heads, "stale attempt must not fetch another CAS token")
	require.Equal(t, 2, pds.puts)
	require.Len(t, pds.records, 1)
	for _, rec := range pds.records {
		require.EqualValues(t, 2, rec.Count)
	}
}

func TestStationViewCountReplayAfterUncertainWrite(t *testing.T) {
	for _, tc := range []struct {
		name     string
		afterPut func(*comatproto.RepoPutRecord_Output) error
	}{
		{
			name: "timeout",
			afterPut: func(*comatproto.RepoPutRecord_Output) error {
				return fmt.Errorf("private PDS response: %w", context.DeadlineExceeded)
			},
		},
		{
			name: "missing commit",
			afterPut: func(out *comatproto.RepoPutRecord_Output) error {
				out.Commit = nil
				return nil
			},
		},
		{
			name: "wrong acknowledgement URI",
			afterPut: func(out *comatproto.RepoPutRecord_Output) error {
				out.Uri = "at://did:web:other.example/place.stream.media.viewCount/other"
				return nil
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, pds, root := countTestInput(t)
			writeCountTestLogs(t, root, "did:web:node1.example", in.WindowStart, countVideoA, countVideoB)
			pds.afterPut = tc.afterPut
			err := RunAggregation(t.Context(), in)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "private PDS response")
			require.Equal(t, 1, pds.heads)
			require.Equal(t, 1, pds.puts, "stop the batch on an uncertain acknowledgement")
			require.Len(t, pds.records, 1)
			var firstURI string
			for uri, rec := range pds.records {
				firstURI = uri
				require.Equal(t, countVideoA, rec.Video)
				require.EqualValues(t, 1, rec.Count)
			}

			writeCountTestLogs(t, root, "did:web:node2.example", in.WindowStart.Add(time.Second), countVideoA)
			pds.afterPut = nil
			require.NoError(t, RunAggregation(t.Context(), in))
			require.Equal(t, 2, pds.heads, "one head read per attempt, including the two-record replay")
			require.Equal(t, 3, pds.puts)
			require.Len(t, pds.records, 2, "two creators sharing a video rkey must not overwrite one another")
			require.Equal(t, countVideoA, pds.records[firstURI].Video)
			require.EqualValues(t, 2, pds.records[firstURI].Count, "replay must recompute and update the original URI")
			for uri, rec := range pds.records {
				if uri != firstURI {
					require.Equal(t, countVideoB, rec.Video)
					require.EqualValues(t, 1, rec.Count)
				}
			}
		})
	}
}

func TestStationViewCountNoLocalFallback(t *testing.T) {
	in, _, root := countTestInput(t)
	writeCountTestLogs(t, root, "did:web:node.example", in.WindowStart, countVideoA)
	in.StationClient = countRPC(func(context.Context, string, string, string, map[string]any, any, any) error {
		t.Fatal("non-publisher contacted the PDS")
		return nil
	})
	in.CLI.StationOriginPublisher = false
	require.ErrorContains(t, RunAggregation(t.Context(), in), "--station-origin-publisher")

	in.CLI.StationOriginPublisher = true
	in.StationClient = nil
	require.ErrorContains(t, RunAggregation(t.Context(), in), "authenticated station PDS client")

	in.StationClient = countRPC(func(_ context.Context, _, _, method string, _ map[string]any, _, _ any) error {
		require.Equal(t, "com.atproto.sync.getLatestCommit", method)
		return fmt.Errorf("private PDS response")
	})
	err := RunAggregation(t.Context(), in)
	require.ErrorContains(t, err, "could not read station PDS commit")
	require.NotContains(t, err.Error(), "private PDS response")
}
