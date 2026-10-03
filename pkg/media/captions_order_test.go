package media

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/muxl"
)

func TestCaptionMasterCanonicalNumericInterleave(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)
	fixture, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)
	in := muxl.SignerInput{CertPEM: ms.Cert, Sign: muxl.SignerToCallback(ms.Signer, 32), TrackManifest: captionManifest("ingest"), TextFn: func(_ context.Context, req muxl.TextRequest) (*muxl.TextAttachment, error) {
		return &muxl.TextAttachment{Tracks: []muxl.TextTrackAttachment{
			{TextTrack: muxl.TextTrack{TrackID: 3, Language: "en", Label: "human"}, Cues: []muxl.TextCue{{Start: req.StartMs, End: req.EndMs, Text: "third"}}},
			{TextTrack: muxl.TextTrack{TrackID: 10, Language: "es", Label: "human"}, Cues: []muxl.TextCue{{Start: req.StartMs, End: req.EndMs, Text: "tenth"}}},
		}}, nil
	}}
	events := make(chan *muxl.MuxlEvent, 16)
	require.NoError(t, muxl.RunMuxlSignSegment(ctx, bytes.NewReader(fixture), in, nil, nil, events))
	close(events)
	for event := range events {
		if event.Type == "signed-segment" {
			segment := concatTracksByID(event.Tracks)
			for _, id := range []string{"1", "2", "3", "10"} {
				require.True(t, bytes.HasPrefix(segment, event.Tracks[id]), "canonical byte order at track %s", id)
				segment = segment[len(event.Tracks[id]):]
			}
		}
	}
}
