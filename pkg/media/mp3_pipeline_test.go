package media

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/log"
)

// TestRunMP3Pipeline exercises RunMP3Pipeline end-to-end against
// pre-synthesized 2-second 440 Hz sine-tone fixtures in three different
// container/codec shapes — one per pipeline branch:
//
//   - synth-2s-440hz.mp3 — MP3 in MP3 (passthrough via mpegaudioparse)
//   - synth-2s-440hz.m4a — AAC in MP4 (transcode via avdec_aac + lamemp3enc)
//   - synth-2s-440hz.mka — Opus in Matroska (transcode via opusdec + lamemp3enc)
//
// The fixtures are committed under test/fixtures/ so the tests stay
// hermetic. To regenerate (e.g. after changing the inputs):
//
//	ffmpeg -y -f lavfi -i "sine=frequency=440:duration=2:sample_rate=48000" \
//	  -ac 2 -c:a libmp3lame -b:a 128k test/fixtures/synth-2s-440hz.mp3
//	ffmpeg -y -f lavfi -i "sine=frequency=440:duration=2:sample_rate=48000" \
//	  -ac 2 -c:a aac      -b:a 128k test/fixtures/synth-2s-440hz.m4a
//	ffmpeg -y -f lavfi -i "sine=frequency=440:duration=2:sample_rate=48000" \
//	  -ac 2 -c:a libopus  -b:a 96k  test/fixtures/synth-2s-440hz.mka
func TestRunMP3Pipeline(t *testing.T) {
	gstinit.InitGST()

	cases := []struct {
		name           string
		fixture        string
		wantTranscoded bool
	}{
		{
			name:           "mp3_passthrough",
			fixture:        "synth-2s-440hz.mp3",
			wantTranscoded: false,
		},
		{
			name:           "aac_in_m4a_transcode",
			fixture:        "synth-2s-440hz.m4a",
			wantTranscoded: true,
		},
		{
			name:           "opus_in_mka_transcode",
			fixture:        "synth-2s-440hz.mka",
			wantTranscoded: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx = log.WithLogValues(ctx, "test", "TestRunMP3Pipeline", "case", tc.name)

			src, err := os.ReadFile(getFixture(tc.fixture))
			require.NoError(t, err)
			require.NotEmpty(t, src)

			out := &bytes.Buffer{}
			result, err := RunMP3Pipeline(ctx, bytes.NewReader(src), int64(len(src)), out)
			require.NoError(t, err)
			require.Greater(t, out.Len(), 1024, "expected non-trivial MP3 output, got %d bytes", out.Len())

			// Output should be raw MP3 frames (sync byte 0xFF) or
			// occasionally start with an ID3v2 tag ("ID3"). lamemp3enc
			// emits raw frames by default; mpegaudioparse passthrough
			// preserves whatever the demuxer handed us (with the ID3
			// tag stripped by id3demux).
			head := out.Bytes()[:3]
			isMP3Frame := head[0] == 0xFF && head[1]&0xE0 == 0xE0
			isID3v2 := head[0] == 'I' && head[1] == 'D' && head[2] == '3'
			require.True(t, isMP3Frame || isID3v2,
				"expected output to start with MP3 sync 0xFF or ID3v2 tag, got %x", head)

			require.Equal(t, tc.wantTranscoded, result.Transcoded, "transcoded flag mismatch")
			require.NotNil(t, result.Audio, "expected audio probe metadata")
			require.NotZero(t, result.Audio.Rate, "expected non-zero sample rate")
			require.NotZero(t, result.Audio.Channels, "expected non-zero channel count")
			require.NotZero(t, result.DurationMS, "expected non-zero duration")
		})
	}
}
