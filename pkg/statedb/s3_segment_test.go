package statedb

import (
	"context"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

// Objects recorded under several livestream records come back in one
// recording order; an object still uploading is left out.
func TestListS3SegmentsForLivestreams(t *testing.T) {
	ctx := context.Background()
	state, err := MakeDB(ctx, &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	t0 := time.Date(2026, 9, 16, 7, 12, 34, 0, time.UTC)
	first, second := "at://did:plc:x/place.stream.livestream/first", "at://did:plc:x/place.stream.livestream/second"
	rec := func(uri, key string, at time.Time, complete bool) {
		id, err := RecordStartHelper(ctx, state, uri, key, at)
		require.NoError(t, err)
		if complete {
			require.NoError(t, state.RecordComplete(ctx, id, 1, 100))
		}
	}
	rec(first, "07-12-34-1.m4s", t0, true)
	rec(first, "07-22-47-2.m4s", t0.Add(10*time.Minute), true)
	rec(second, "07-25-15-3.m4s", t0.Add(13*time.Minute), true)
	rec(second, "07-35-23-4.m4s", t0.Add(23*time.Minute), true)
	rec(second, "07-45-34-5.m4s", t0.Add(33*time.Minute), false) // still uploading
	rec("at://did:plc:x/place.stream.livestream/other", "x.m4s", t0.Add(5*time.Minute), true)

	segs, err := state.ListS3SegmentsForLivestreams(ctx, []string{second, first}) // caller order irrelevant
	require.NoError(t, err)
	keys := make([]string, len(segs))
	for i, s := range segs {
		keys[i] = path.Base(s.Key)
	}
	require.Equal(t, []string{"07-12-34-1.m4s", "07-22-47-2.m4s", "07-25-15-3.m4s", "07-35-23-4.m4s"}, keys)

	one, err := state.ListS3SegmentsForLivestream(ctx, first)
	require.NoError(t, err)
	require.Len(t, one, 2)
	none, err := state.ListS3SegmentsForLivestreams(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, none)
}

// RecordStartHelper records an object start for tests.
func RecordStartHelper(ctx context.Context, state *StatefulDB, uri, key string, at time.Time) (string, error) {
	return state.RecordStart(ctx, "did:plc:x", "bucket", "live-rec/did:plc:x/"+key, uri, at)
}
