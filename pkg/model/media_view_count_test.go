package model

import (
	"context"
	"testing"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/placestream"
)

func parseURI(t *testing.T, s string) syntax.ATURI {
	t.Helper()
	u, err := syntax.ParseATURI(s)
	require.NoError(t, err)
	return u
}

// putViewCount writes one place.stream.media.viewCount record under
// the given reporter repo. Tracks slice is what shows up in the
// summary's bytes / durationMs totals. rkey is a per-call discriminator
// — same reporter + same rkey upserts in place.
func putViewCount(t *testing.T, m Model, reporter, rkey, videoURI string, count int64, tracks []placestream.MediaViewCount_TrackUsage) {
	t.Helper()
	rec := placestream.MediaViewCount{
		LexiconTypeID: "place.stream.media.viewCount",
		Video:         videoURI,
		Count:         count,
		WindowStart:   "2026-05-17T12:00:00Z",
		WindowEnd:     "2026-05-17T12:05:00Z",
		IndexedAt:     "2026-05-17T12:15:00Z",
		Tracks:        tracks,
	}
	aturi := parseURI(t, "at://"+reporter+"/place.stream.media.viewCount/"+rkey)
	require.NoError(t, m.UpsertMediaViewCount(context.Background(), rec, aturi))
}

func TestGetVideoView_NoViewCounts(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const (
		owner    = "did:plc:alice"
		videoURI = "at://did:plc:alice/place.stream.video/v1"
	)
	video := placestream.Video{
		LexiconTypeID: "place.stream.video",
		Title:         "hello",
		Source: placestream.Video_Source{
			MediaDefs_SourceTracks: placestream.MediaDefs_SourceTracks{
				LexiconTypeID: "place.stream.media.defs#sourceTracks",
				Tracks:        []*comatproto.RepoStrongRef{},
			},
		},
	}
	require.NoError(t, m.UpsertVideo(ctx, video, parseURI(t, videoURI)))

	view, err := m.GetVideoView(ctx, videoURI)
	require.NoError(t, err)
	require.NotNil(t, view)
	require.Equal(t, videoURI, view.Uri)
	require.Equal(t, owner, view.Author.Did)
	require.NotNil(t, view.ViewCounts, "summary is always present; zero-valued when no records")
	require.Equal(t, int64(0), view.ViewCounts.Count)
	require.Equal(t, int64(0), view.ViewCounts.Bytes)
	require.Equal(t, int64(0), view.ViewCounts.DurationMs)
	require.Equal(t, int64(0), view.ViewCounts.Reporters)
}

func TestGetVideoView_SumsAcrossReporters(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const videoURI = "at://did:plc:alice/place.stream.video/v1"
	video := placestream.Video{
		LexiconTypeID: "place.stream.video",
		Title:         "popular",
		Source: placestream.Video_Source{
			MediaDefs_SourceTracks: placestream.MediaDefs_SourceTracks{
				LexiconTypeID: "place.stream.media.defs#sourceTracks",
			},
		},
	}
	require.NoError(t, m.UpsertVideo(ctx, video, parseURI(t, videoURI)))

	trackRef := &comatproto.RepoStrongRef{
		LexiconTypeID: "com.atproto.repo.strongRef",
		Uri:           "at://did:plc:alice/place.stream.media.track/t1",
		Cid:           "bafytrack",
	}
	// Three reporters, one record each.
	putViewCount(t, m, "did:plc:node1", "w1", videoURI, 5, []placestream.MediaViewCount_TrackUsage{
		{Track: trackRef, Bytes: 100, DurationMs: 1000},
	})
	putViewCount(t, m, "did:plc:node2", "w1", videoURI, 7, []placestream.MediaViewCount_TrackUsage{
		{Track: trackRef, Bytes: 200, DurationMs: 2000},
	})
	putViewCount(t, m, "did:plc:node3", "w1", videoURI, 3, []placestream.MediaViewCount_TrackUsage{
		{Track: trackRef, Bytes: 50, DurationMs: 500},
		{Track: trackRef, Bytes: 25, DurationMs: 250},
	})

	view, err := m.GetVideoView(ctx, videoURI)
	require.NoError(t, err)
	require.NotNil(t, view)
	require.NotNil(t, view.ViewCounts)
	require.Equal(t, int64(15), view.ViewCounts.Count, "5+7+3")
	require.Equal(t, int64(375), view.ViewCounts.Bytes, "100+200+(50+25)")
	require.Equal(t, int64(3750), view.ViewCounts.DurationMs, "1000+2000+(500+250)")
	require.Equal(t, int64(3), view.ViewCounts.Reporters)
}

func TestGetVideoView_NotFound(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)

	view, err := m.GetVideoView(context.Background(), "at://did:plc:nobody/place.stream.video/v404")
	require.NoError(t, err)
	require.Nil(t, view, "missing video returns (nil, nil) so callers can surface a 404 without sniffing errors")
}

func TestUpsertMediaViewCountIdempotent(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const (
		reporter = "did:plc:node1"
		videoURI = "at://did:plc:alice/place.stream.video/v1"
	)
	rec := placestream.MediaViewCount{
		LexiconTypeID: "place.stream.media.viewCount",
		Video:         videoURI,
		Count:         5,
		WindowStart:   "2026-05-17T12:00:00Z",
		WindowEnd:     "2026-05-17T12:05:00Z",
		IndexedAt:     "2026-05-17T12:15:00Z",
	}
	aturi := parseURI(t, "at://"+reporter+"/place.stream.media.viewCount/win-1")

	require.NoError(t, m.UpsertMediaViewCount(ctx, rec, aturi))
	// Second call with the same URI replaces, doesn't append.
	rec.Count = 8
	require.NoError(t, m.UpsertMediaViewCount(ctx, rec, aturi))

	got, err := m.GetMediaViewCountByURI(ctx, aturi.String())
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, int64(8), got.Count, "upsert replaces by URI")
}

func TestDeleteMediaViewCount(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	const videoURI = "at://did:plc:alice/place.stream.video/v1"
	rec := placestream.MediaViewCount{
		LexiconTypeID: "place.stream.media.viewCount",
		Video:         videoURI,
		Count:         5,
		WindowStart:   "2026-05-17T12:00:00Z",
		WindowEnd:     "2026-05-17T12:05:00Z",
		IndexedAt:     "2026-05-17T12:15:00Z",
	}
	aturi := parseURI(t, "at://did:plc:node1/place.stream.media.viewCount/win-1")
	require.NoError(t, m.UpsertMediaViewCount(ctx, rec, aturi))

	require.NoError(t, m.DeleteMediaViewCount(ctx, aturi.String()))
	got, err := m.GetMediaViewCountByURI(ctx, aturi.String())
	require.NoError(t, err)
	require.Nil(t, got)
}
