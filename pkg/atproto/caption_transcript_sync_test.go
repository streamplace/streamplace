package atproto

import (
	"bytes"
	"context"
	"testing"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
)

// Transcript records arrive from streamers' repos and from nodes' server repos
// alike, are indexed by the subject they caption, and an update to a chunk
// replaces it.
func TestHandleCreateUpdateIndexesCaptionTranscripts(t *testing.T) {
	ctx := context.Background()
	atsync, mod, _ := offlineSynchronizer(t)

	const (
		streamer = "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
		node     = "did:web:node.example"
		subject  = "at://did:plc:aaaaaaaaaaaaaaaaaaaaaaaa/place.stream.livestream/3live000000000"
	)
	record := func(text string) *placestream.CaptionTranscript {
		start := "2026-09-30T12:00:00.000Z"
		return &placestream.CaptionTranscript{
			LexiconTypeID: "place.stream.caption.transcript",
			Subject:       comatproto.RepoStrongRef{LexiconTypeID: "com.atproto.repo.strongRef", Uri: subject, Cid: "bafylive"},
			MediaStart:    &start,
			StartMs:       1500,
			Text:          text,
			Timings:       []int64{300, 300},
			Language:      "en",
			Source:        "auto",
			CreatedAt:     "2026-09-30T12:01:00.000Z",
		}
	}
	index := func(repo, rkey string, rec *placestream.CaptionTranscript) {
		t.Helper()
		var buf bytes.Buffer
		require.NoError(t, rec.MarshalCBOR(&buf))
		cid, err := spid.GetCID(rec)
		require.NoError(t, err)
		bs := buf.Bytes()
		require.NoError(t, atsync.handleCreateUpdate(ctx, repo, syntax.RecordKey(rkey), &bs, cid.String(),
			syntax.NSID("place.stream.caption.transcript"), false, false, ""))
	}

	index(streamer, "3tr000000000a", record("hello world"))
	index(node, "3tr000000000b", record("hello world"))
	// Delivered twice (cursor replay, two relays) and then updated.
	index(streamer, "3tr000000000a", record("hello world"))
	index(streamer, "3tr000000000a", record("hello there"))

	rows, err := mod.GetCaptionTranscriptsBySubject(ctx, subject)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	byRepo := map[string]*model.CaptionTranscript{}
	for _, r := range rows {
		byRepo[r.RepoDID] = r
	}
	rec, err := byRepo[streamer].ToRecord()
	require.NoError(t, err)
	require.Equal(t, "hello there", rec.Text, "the update replaced the chunk")
	require.Equal(t, "auto", byRepo[node].Source)
	require.NotNil(t, byRepo[node].MediaStart)
}
