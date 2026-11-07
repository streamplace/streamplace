package livehls

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/muxl"
)

// A stream with stems groups its audio: the full mix (the lowest group, here
// AAC + Opus) is what HLS plays, the stem (a higher group) stays out of the
// master playlist's audio group but is still served as a track.
func TestMasterPlaylistPlaysTheFullMixOnly(t *testing.T) {
	audio := func(id uint32, codec string, group uint32, label string) muxl.MuxlAudioConfig {
		return muxl.MuxlAudioConfig{Codec: codec, Container: muxl.MuxlContainer{Kind: "cmaf", Timescale: 48000, TrackID: id},
			SampleRate: 48000, NumberOfChannels: 2, AlternateGroup: group, Label: label}
	}
	w := NewWriter()
	require.NoError(t, w.Observe(&muxl.MuxlEvent{
		Type:       "init",
		TrackInits: map[string][]byte{"1": []byte("V"), "2": []byte("A"), "3": []byte("B"), "4": []byte("C"), "5": []byte("D")},
		Catalog: &muxl.MuxlCatalog{
			Video: &muxl.MuxlCatalogVideo{Renditions: map[string]muxl.MuxlVideoConfig{
				"v": {Codec: "avc1.640028", Container: muxl.MuxlContainer{Kind: "cmaf", Timescale: 90000, TrackID: 1}, CodedWidth: 1280, CodedHeight: 720},
			}},
			Audio: &muxl.MuxlCatalogAudio{Renditions: map[string]muxl.MuxlAudioConfig{
				// The stem's Opus track has the lowest id: order must not decide.
				"stem-opus": audio(2, "opus", 2, "Music"),
				"mix-opus":  audio(3, "opus", 1, "Mix"),
				"mix-aac":   audio(4, "mp4a.40.2", 1, "Mix"),
				"stem-aac":  audio(5, "mp4a.40.2", 2, "Music"),
			}},
		},
	}))
	seg := bytes.Repeat([]byte{1}, 10)
	require.NoError(t, w.Observe(&muxl.MuxlEvent{
		Type:         "segment",
		Tracks:       map[string][]byte{"1": seg, "2": seg, "3": seg, "4": seg, "5": seg},
		Durations:    map[string]uint64{"1": 90000, "2": 48000, "3": 48000, "4": 48000, "5": 48000},
		SampleCounts: map[string]uint32{"1": 30, "2": 50, "3": 50, "4": 50, "5": 50},
	}))

	require.Equal(t, "4", w.PrimaryAudioTrackID(), "the mix's AAC is the default audio")
	master := w.MasterPlaylist(func(tid string) string { return "track" + tid + ".m3u8" })
	require.Equal(t, 2, strings.Count(master, "#EXT-X-MEDIA:TYPE=AUDIO"), "the mix's two codecs, no stems:\n%s", master)
	require.Contains(t, master, `DEFAULT=YES,AUTOSELECT=YES,CHANNELS="2",URI="track4.m3u8"`)
	require.Contains(t, master, `DEFAULT=NO,AUTOSELECT=YES,CHANNELS="2",URI="track3.m3u8"`)
	require.NotContains(t, master, "track2.m3u8")
	require.NotContains(t, master, "track5.m3u8")
	require.Contains(t, master, `CODECS="avc1.640028,mp4a.40.2"`)
	require.NotEmpty(t, w.MediaPlaylist("2", "init", func(uint64) string { return "s.m4s" }), "the stem is still a served track")
}
