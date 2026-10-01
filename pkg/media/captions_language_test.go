package media

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/muxl"
)

func TestCaptionMasterPushedLanguagesIgnoreRecognitionHints(t *testing.T) {
	m := newCaptionMaster(context.Background(), "streamer", &config.CLI{}, nil)
	m.setManifest(captionManifest("auto"))
	m.clock(time.UnixMilli(0))
	m.mediaFinished = true
	for _, language := range []string{"es", "fr"} {
		track := captions.Track{Language: language, Source: captions.SourceHuman}
		require.NoError(t, m.push(track, []captions.Cue{{ID: "line", Start: m.arrival.Add(100 * time.Millisecond), End: m.arrival.Add(500 * time.Millisecond), Text: language, Final: true}}))
	}
	got, err := m.text(context.Background(), muxl.TextRequest{EndMs: 1000})
	require.NoError(t, err)
	var languages []string
	for _, track := range got.Tracks {
		languages = append(languages, track.Language)
		require.Equal(t, "human", track.Label)
	}
	require.ElementsMatch(t, []string{"es", "fr"}, languages, "author languages must remain distinct despite en-US recognition hints")
}
