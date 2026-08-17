package media

import (
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/muxl"
)

// catalogVideos builds a *muxl.MuxlCatalog (aliased to upstream.Catalog) whose
// Video.Renditions holds the given entries.
func catalogVideos(trackID uint32, w, h int) muxl.MuxlCatalogVideo {
	return muxl.MuxlCatalogVideo{
		Renditions: map[string]muxl.MuxlVideoConfig{
			"source": {Codec: "h264", Container: muxl.MuxlContainer{Kind: "cmaf", TrackID: trackID}, CodedWidth: uint32(w), CodedHeight: uint32(h)},
		},
	}
}

// multi catalog: highest is 1080p (track 3), then 720p (track 2), then 480p (1).
func multiVideoCatalog() muxl.MuxlCatalogVideo {
	return muxl.MuxlCatalogVideo{
		Renditions: map[string]muxl.MuxlVideoConfig{
			"480p":  {Codec: "h264", Container: muxl.MuxlContainer{Kind: "cmaf", TrackID: 1}, CodedWidth: 854, CodedHeight: 480},
			"720p":  {Codec: "h264", Container: muxl.MuxlContainer{Kind: "cmaf", TrackID: 2}, CodedWidth: 1280, CodedHeight: 720},
			"1080p": {Codec: "h264", Container: muxl.MuxlContainer{Kind: "cmaf", TrackID: 3}, CodedWidth: 1920, CodedHeight: 1080},
		},
	}
}

func TestSelectBestVideoTrackID(t *testing.T) {
	// Default (preferHeight=0): the highest-quality rendition wins.
	mv := multiVideoCatalog()
	cat := &muxl.MuxlCatalog{Video: &mv, Audio: nil}
	require.Equal(t, uint32(3), selectBestVideoTrackID(cat, 0), "default should pick the 1080p (largest-area) track")

	// Preference at-or-below: prefer the tallest track that fits.
	require.Equal(t, uint32(2), selectBestVideoTrackID(cat, 720), "prefer 720 when asking for 720")
	require.Equal(t, uint32(3), selectBestVideoTrackID(cat, 1080), "asking for 1080 takes the tallest at-or-below 1080 = 1080")

	// A preference below the smallest available track falls back to the best.
	require.Equal(t, uint32(3), selectBestVideoTrackID(cat, 240), "no track fits 240, should fall back to best quality")

	// A single-track catalog with a muxl "legacy"-kind container (zero TrackID)
	// is not addressable, so the selector can't pick it.
	legacyMuxlV := catalogVideos(0, 1920, 1080)
	legacyMuxl := &muxl.MuxlCatalog{Video: &legacyMuxlV}
	require.Equal(t, uint32(0), selectBestVideoTrackID(legacyMuxl, 0), "muxl legacy container (no CMAF track id) is not selectable")

	// A single usable track always wins.
	singleV := catalogVideos(7, 640, 360)
	single := &muxl.MuxlCatalog{Video: &singleV}
	require.Equal(t, uint32(7), selectBestVideoTrackID(single, 0))
	require.Equal(t, uint32(7), selectBestVideoTrackID(single, 480))
	require.Equal(t, uint32(7), selectBestVideoTrackID(single, 1080))
}

func TestSelectBestVideoTrackIDTieBreak(t *testing.T) {
	source := muxl.MuxlCatalogVideo{Renditions: map[string]muxl.MuxlVideoConfig{
		"narrow": {Codec: "h264", Container: muxl.MuxlContainer{Kind: "cmaf", TrackID: 1}, CodedWidth: 960, CodedHeight: 1080},
		"wide":   {Codec: "h264", Container: muxl.MuxlContainer{Kind: "cmaf", TrackID: 2}, CodedWidth: 1920, CodedHeight: 540},
		"small":  {Codec: "h264", Container: muxl.MuxlContainer{Kind: "cmaf", TrackID: 3}, CodedWidth: 1280, CodedHeight: 720},
	}}
	cat := &muxl.MuxlCatalog{Video: &source}
	// Equal area (1920x540 == 960x1080): larger width wins deterministically
	// (track 2, not whichever the map happens to hand out first).
	require.Equal(t, uint32(2), selectBestVideoTrackID(cat, 0))
}

func TestChooseAudioTrackID(t *testing.T) {
	aac := muxl.MuxlCatalogAudio{Renditions: map[string]muxl.MuxlAudioConfig{
		"aac": {Codec: "mp4a.40.2", Container: muxl.MuxlContainer{Kind: "cmaf", TrackID: 4}},
	}}
	opus := muxl.MuxlCatalogAudio{Renditions: map[string]muxl.MuxlAudioConfig{
		"opus": {Codec: "opus", Container: muxl.MuxlContainer{Kind: "cmaf", TrackID: 5}},
	}}
	both := muxl.MuxlCatalogAudio{Renditions: map[string]muxl.MuxlAudioConfig{
		"opus": {Codec: "opus", Container: muxl.MuxlContainer{Kind: "cmaf", TrackID: 5}},
		"aac":  {Codec: "mp4a.40.2", Container: muxl.MuxlContainer{Kind: "cmaf", TrackID: 4}},
	}}

	catAAC := &muxl.MuxlCatalog{Audio: &aac}
	require.Equal(t, uint32(4), chooseAudioTrackID(catAAC, false), "RTMP source wants AAC")
	require.Equal(t, uint32(4), chooseAudioTrackID(catAAC, true), "no opus — degrade to the only audio track")

	catOpus := &muxl.MuxlCatalog{Audio: &opus}
	require.Equal(t, uint32(5), chooseAudioTrackID(catOpus, true), "wants opus — picks opus")
	require.Equal(t, uint32(5), chooseAudioTrackID(catOpus, false), "no AAC — degrade to the only audio track")

	catBoth := &muxl.MuxlCatalog{Audio: &both}
	require.Equal(t, uint32(4), chooseAudioTrackID(catBoth, false), "wants AAC — picks AAC track")
	require.Equal(t, uint32(5), chooseAudioTrackID(catBoth, true), "wants opus — picks opus track")

	require.Equal(t, uint32(0), chooseAudioTrackID(&muxl.MuxlCatalog{}, false), "no audio → 0")
}
