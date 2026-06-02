package model

import (
	"context"
	"testing"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/streamplace"
)

const testServerDID = "did:web:us.example.com"

// putTrackVideo writes a place.stream.media.track (backed by blobCID) and a
// place.stream.video whose sourceTracks references it. Returns nothing; the
// video is addressable at videoURI.
func putTrackVideo(t *testing.T, m Model, videoURI, trackURI, blobCID string) {
	t.Helper()
	ctx := context.Background()

	track := &streamplace.MediaTrack{
		LexiconTypeID: "place.stream.media.track",
		Track: &streamplace.MediaTrack_Track{
			MediaDefs_MuxlTrack: &streamplace.MediaDefs_MuxlTrack{
				LexiconTypeID: "place.stream.media.defs#muxlTrack",
				Blob:          blobCID,
				TrackId:       "1",
				MediaType:     "video",
			},
		},
	}
	require.NoError(t, m.UpsertMediaTrack(ctx, track, parseURI(t, trackURI)))

	video := &streamplace.Video{
		LexiconTypeID: "place.stream.video",
		Title:         videoURI,
		Source: &streamplace.Video_Source{
			MediaDefs_SourceTracks: &streamplace.MediaDefs_SourceTracks{
				LexiconTypeID: "place.stream.media.defs#sourceTracks",
				Tracks: []*comatproto.RepoStrongRef{
					{LexiconTypeID: "com.atproto.repo.strongRef", Uri: trackURI, Cid: "bafytrackcid"},
				},
			},
		},
	}
	require.NoError(t, m.UpsertVideo(ctx, video, parseURI(t, videoURI)))
}

// putClipVideo writes a sourceClip video referencing parentURI.
func putClipVideo(t *testing.T, m Model, clipURI, parentURI string) {
	t.Helper()
	video := &streamplace.Video{
		LexiconTypeID: "place.stream.video",
		Title:         clipURI,
		Source: &streamplace.Video_Source{
			MediaDefs_SourceClip: &streamplace.MediaDefs_SourceClip{
				LexiconTypeID: "place.stream.media.defs#sourceClip",
				Video:         parentURI,
				Start:         1000,
				End:           2000,
			},
		},
	}
	require.NoError(t, m.UpsertVideo(context.Background(), video, parseURI(t, clipURI)))
}

// putOrigin attests that serverDID hosts blobCID.
func putOrigin(t *testing.T, m Model, serverDID, blobCID string) {
	t.Helper()
	rec := &streamplace.MediaOrigin{
		LexiconTypeID: "place.stream.media.origin",
		Blob:          blobCID,
		Size:          123,
		MimeType:      "video/mp4",
	}
	aturi := parseURI(t, "at://"+serverDID+"/place.stream.media.origin/"+blobCID)
	require.NoError(t, m.UpsertMediaOrigin(context.Background(), rec, aturi))
}

func uriSet(out *streamplace.MediaGetVideoList_Output) map[string]bool {
	s := map[string]bool{}
	for _, v := range out.Videos {
		s[v.Uri] = true
	}
	return s
}

// TestGetVideoList_HostedFilter verifies that passing hostedByServerDID
// restricts the listing to videos whose content blob this node actually
// hosts, while passing "" returns everything (old behavior).
func TestGetVideoList_HostedFilter(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	// Two videos we host, two we only know about from the firehose.
	putTrackVideo(t, m, "at://did:plc:alice/place.stream.video/have1", "at://did:plc:alice/place.stream.media.track/t1", "blobHAVE1")
	putTrackVideo(t, m, "at://did:plc:alice/place.stream.video/missing1", "at://did:plc:alice/place.stream.media.track/t2", "blobMISS1")
	putTrackVideo(t, m, "at://did:plc:bob/place.stream.video/have2", "at://did:plc:bob/place.stream.media.track/t3", "blobHAVE2")
	putTrackVideo(t, m, "at://did:plc:bob/place.stream.video/missing2", "at://did:plc:bob/place.stream.media.track/t4", "blobMISS2")

	putOrigin(t, m, testServerDID, "blobHAVE1")
	putOrigin(t, m, testServerDID, "blobHAVE2")
	// An origin from a *different* node must not make us advertise it.
	putOrigin(t, m, "did:web:other.example.com", "blobMISS1")

	// Unfiltered: everything indexed shows up.
	all, err := m.GetVideoList(ctx, "", 25, "", "")
	require.NoError(t, err)
	require.Len(t, all.Videos, 4)

	// Filtered to our node: only the two we host.
	hosted, err := m.GetVideoList(ctx, "", 25, "", testServerDID)
	require.NoError(t, err)
	got := uriSet(hosted)
	require.Equal(t, map[string]bool{
		"at://did:plc:alice/place.stream.video/have1": true,
		"at://did:plc:bob/place.stream.video/have2":   true,
	}, got)
}

// TestGetVideoList_HostedClip verifies a clip is advertised iff the node
// hosts its *parent's* content blob.
func TestGetVideoList_HostedClip(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	putTrackVideo(t, m, "at://did:plc:alice/place.stream.video/parentHave", "at://did:plc:alice/place.stream.media.track/p1", "blobParentHave")
	putTrackVideo(t, m, "at://did:plc:alice/place.stream.video/parentMiss", "at://did:plc:alice/place.stream.media.track/p2", "blobParentMiss")
	putClipVideo(t, m, "at://did:plc:alice/place.stream.video/clipHave", "at://did:plc:alice/place.stream.video/parentHave")
	putClipVideo(t, m, "at://did:plc:alice/place.stream.video/clipMiss", "at://did:plc:alice/place.stream.video/parentMiss")

	putOrigin(t, m, testServerDID, "blobParentHave")

	hosted, err := m.GetVideoList(ctx, "", 25, "", testServerDID)
	require.NoError(t, err)
	got := uriSet(hosted)
	require.True(t, got["at://did:plc:alice/place.stream.video/parentHave"], "parent we host should be listed")
	require.True(t, got["at://did:plc:alice/place.stream.video/clipHave"], "clip of a hosted parent should be listed")
	require.False(t, got["at://did:plc:alice/place.stream.video/parentMiss"], "parent we don't host should be hidden")
	require.False(t, got["at://did:plc:alice/place.stream.video/clipMiss"], "clip of an unhosted parent should be hidden")
}

// TestGetVideoList_HostedPagination walks every page with a small limit and
// confirms the hosted set is returned exactly once across pages, with the
// firehose-only videos skipped, and that paging terminates.
func TestGetVideoList_HostedPagination(t *testing.T) {
	m, err := MakeDB(":memory:")
	require.NoError(t, err)
	ctx := context.Background()

	wantHosted := map[string]bool{}
	// Interleave hosted and unhosted so page boundaries straddle both.
	for i := 0; i < 6; i++ {
		hi := "at://did:plc:alice/place.stream.video/h" + string(rune('a'+i))
		mi := "at://did:plc:alice/place.stream.video/m" + string(rune('a'+i))
		putTrackVideo(t, m, hi, "at://did:plc:alice/place.stream.media.track/ht"+string(rune('a'+i)), "blobH"+string(rune('a'+i)))
		putTrackVideo(t, m, mi, "at://did:plc:alice/place.stream.media.track/mt"+string(rune('a'+i)), "blobM"+string(rune('a'+i)))
		putOrigin(t, m, testServerDID, "blobH"+string(rune('a'+i)))
		wantHosted[hi] = true
	}

	seen := map[string]int{}
	cursor := ""
	pages := 0
	for {
		page, err := m.GetVideoList(ctx, "", 2, cursor, testServerDID)
		require.NoError(t, err)
		for _, v := range page.Videos {
			seen[v.Uri]++
		}
		pages++
		require.Less(t, pages, 50, "pagination did not terminate")
		if page.Cursor == nil {
			break
		}
		cursor = *page.Cursor
	}

	require.Len(t, seen, len(wantHosted), "every hosted video returned exactly once")
	for uri := range wantHosted {
		require.Equal(t, 1, seen[uri], "hosted video %s seen exactly once", uri)
	}
}
