package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/rtmps"
)

func TestIngestProblems(t *testing.T) {
	ctx := context.Background()
	a := &StreamplaceAPI{IngestHosts: rtmps.NewIngestHosts([]string{"stream.place"})}
	const streamer = "did:plc:streamer"

	none := a.ingestProblems(ctx, streamer)
	require.Equal(t, "place.stream.ingest.defs#problems", none.LexiconTypeID)
	// an empty list, not null: it's what clears the dashboard
	require.NotNil(t, none.Problems)
	require.Empty(t, none.Problems)
	require.Equal(t, "", problemsKey(none))

	release := a.IngestHosts.Open(ctx, rtmps.ListenerRTMPS, "stream.place", "", streamer)
	probs := a.ingestProblems(ctx, streamer)
	require.Len(t, probs.Problems, 1)
	p := probs.Problems[0]
	require.Equal(t, ProblemDeprecatedIngestHost, p.Code)
	require.Equal(t, "warning", p.Severity)
	require.Contains(t, p.Message, "stream.place")
	require.NotNil(t, p.Link)
	require.NotEqual(t, problemsKey(none), problemsKey(probs))

	release()
	require.Empty(t, a.ingestProblems(ctx, streamer).Problems)
}

func TestIngestProblemsMoveTo(t *testing.T) {
	ctx := context.Background()
	a := &StreamplaceAPI{IngestHosts: rtmps.NewIngestHosts([]string{"stream.place"})}
	require.Equal(t, "to rtmps://rtmp.stream.place:1935/live", a.moveTo(ctx, "rtmps://rtmp.stream.place:1935/live"))
	// never from one retired name to another
	require.NotContains(t, a.moveTo(ctx, "rtmps://stream.place:1935/live"), "stream.place:1935")
	require.NotEmpty(t, a.moveTo(ctx, ""))
}
