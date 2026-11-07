package localdb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The segment record lists every audio track with its real codec, and the
// group and label of a stream that distinguishes its tracks; a row written
// before the codec was recorded still reads as the Opus it was.
func TestToStreamplaceSegmentListsEveryAudioTrack(t *testing.T) {
	seg := &Segment{
		ID: "seg", RepoDID: "did:plc:test", StartTime: time.Now(),
		MediaData: &SegmentMediaData{
			Video: []*SegmentMediadataVideo{{Width: 1280, Height: 720, FPSNum: 30, FPSDen: 1}},
			Audio: []*SegmentMediadataAudio{
				{Codec: "aac", Rate: 48000, Channels: 2, AlternateGroup: 1, Label: "Mix"},
				{Codec: "opus", Rate: 48000, Channels: 2, AlternateGroup: 1, Label: "Mix"},
				{Codec: "opus", Rate: 48000, Channels: 2, AlternateGroup: 2, Label: "Music"},
			},
		},
	}
	record, err := seg.ToStreamplaceSegment()
	require.NoError(t, err)
	require.Len(t, record.Audio, 3)
	require.Equal(t, "aac", record.Audio[0].Codec)
	require.Equal(t, "opus", record.Audio[2].Codec)
	require.Equal(t, int64(48000), record.Audio[2].Rate)
	require.Equal(t, int64(2), record.Audio[2].Channels)
	require.NotNil(t, record.Audio[2].AlternateGroup)
	require.Equal(t, int64(2), *record.Audio[2].AlternateGroup)
	require.NotNil(t, record.Audio[2].Label)
	require.Equal(t, "Music", *record.Audio[2].Label)

	seg.MediaData.Audio = []*SegmentMediadataAudio{{Rate: 48000, Channels: 2}}
	record, err = seg.ToStreamplaceSegment()
	require.NoError(t, err)
	require.Len(t, record.Audio, 1)
	require.Equal(t, "opus", record.Audio[0].Codec, "a legacy row's audio was Opus")
	require.Nil(t, record.Audio[0].AlternateGroup, "an ungrouped track carries no group")
	require.Nil(t, record.Audio[0].Label)
}
