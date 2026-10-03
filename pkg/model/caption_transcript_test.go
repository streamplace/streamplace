package model

import (
	"context"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/placestream"
)

func captionRec(subject, language, source string, startMs int64, text string) placestream.CaptionTranscript {
	return placestream.CaptionTranscript{
		LexiconTypeID: "place.stream.caption.transcript",
		Subject:       comatproto.RepoStrongRef{LexiconTypeID: "com.atproto.repo.strongRef", Uri: subject, Cid: "bafysubject"},
		Language:      language,
		Source:        source,
		StartMs:       startMs,
		Text:          text,
		Timings:       []int64{100},
		CreatedAt:     "2026-09-30T12:00:00.000Z",
	}
}

func TestCaptionTranscriptIndex(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const (
		video = "at://did:plc:alice/place.stream.video/v1"
		live  = "at://did:plc:alice/place.stream.livestream/l1"
	)
	put := func(repo, rkey string, rec placestream.CaptionTranscript) string {
		uri := "at://" + repo + "/place.stream.caption.transcript/" + rkey
		require.NoError(t, m.UpsertCaptionTranscript(ctx, rec, syntax.ATURI(uri)))
		return uri
	}

	late := put("did:plc:alice", "b", captionRec(video, "en", "human", 60_000, "later"))
	early := put("did:plc:alice", "a", captionRec(video, "en", "human", 0, "earlier"))
	node := put("did:web:node.example", "n", captionRec(video, "en", "auto", 0, "from the node"))
	put("did:plc:alice", "other", captionRec("at://did:plc:alice/place.stream.video/v2", "en", "human", 0, "another video"))

	t.Run("subject query returns every author's chunks in order", func(t *testing.T) {
		rows, err := m.GetCaptionTranscriptsBySubject(ctx, video)
		require.NoError(t, err)
		var uris []string
		for _, r := range rows {
			uris = append(uris, r.URI)
		}
		require.Equal(t, []string{early, late, node}, uris)
		require.Equal(t, "did:plc:alice", rows[0].RepoDID)
		require.Equal(t, "bafysubject", rows[0].SubjectCID)
	})

	t.Run("a repo's own chunks for a subject", func(t *testing.T) {
		rows, err := m.GetCaptionTranscriptsForRepoSubject(ctx, "did:web:node.example", video)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, node, rows[0].URI)
	})

	t.Run("kind defaults to captions and the record decodes back", func(t *testing.T) {
		rows, err := m.GetCaptionTranscriptsForRepoSubject(ctx, "did:plc:alice", video)
		require.NoError(t, err)
		require.Equal(t, "captions", rows[0].Kind)
		require.Nil(t, rows[0].MediaStart)
		rec, err := rows[0].ToRecord()
		require.NoError(t, err)
		require.Equal(t, "earlier", rec.Text)
		require.Equal(t, []int64{100}, rec.Timings)
	})

	t.Run("media start of a live session is indexed as a time", func(t *testing.T) {
		rec := captionRec(live, "en", "auto", 5000, "live words")
		start := "2026-09-30T12:34:56.789Z"
		kind := "subtitles"
		rec.MediaStart, rec.Kind = &start, &kind
		uri := put("did:plc:alice", "live1", rec)
		rows, err := m.GetCaptionTranscriptsBySubject(ctx, live)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, uri, rows[0].URI)
		require.Equal(t, "subtitles", rows[0].Kind)
		require.NotNil(t, rows[0].MediaStart)
		require.True(t, rows[0].MediaStart.Equal(time.Date(2026, 9, 30, 12, 34, 56, 789_000_000, time.UTC)))
	})

	t.Run("an update replaces the row", func(t *testing.T) {
		put("did:plc:alice", "a", captionRec(video, "en", "human", 0, "corrected"))
		rows, err := m.GetCaptionTranscriptsBySubject(ctx, video)
		require.NoError(t, err)
		require.Len(t, rows, 3)
		rec, err := rows[0].ToRecord()
		require.NoError(t, err)
		require.Equal(t, "corrected", rec.Text)
	})

	t.Run("delete removes only that chunk", func(t *testing.T) {
		require.NoError(t, m.DeleteCaptionTranscript(ctx, late))
		rows, err := m.GetCaptionTranscriptsBySubject(ctx, video)
		require.NoError(t, err)
		require.Len(t, rows, 2)
		require.Equal(t, []string{early, node}, []string{rows[0].URI, rows[1].URI})
	})

	t.Run("a bad media start is refused", func(t *testing.T) {
		rec := captionRec(live, "en", "auto", 0, "x")
		bad := "yesterday"
		rec.MediaStart = &bad
		require.Error(t, m.UpsertCaptionTranscript(ctx, rec, syntax.ATURI("at://did:plc:alice/place.stream.caption.transcript/bad")))
	})
}
