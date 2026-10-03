package media

import (
	"bytes"
	"context"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/muxl"
	"sync"
	"testing"
	"time"
)

func TestCaptionMasterCanonicalNamespaceSurvivesAudioCompletion(t *testing.T) {
	for _, late := range []bool{false, true} {
		name := "initial"
		if late {
			name = "late"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			signer := newBareSegmentSigner(t)
			key, err := signers.MarshalES256KPrivateKeyPEM(signer.Signer)
			require.NoError(t, err)
			// Three GoPs catch an unknown text track that qtdemux only rejects
			// when the following AV fragment arrives; a two-GoP input misses it.
			fixture := runSynthPipeline(t, ctx,
				"videotestsrc num-buffers=90 pattern=ball ! video/x-raw,width=320,height=240,framerate=30/1 ! x264enc key-int-max=30 tune=zerolatency speed-preset=ultrafast ! h264parse ! mp4mux name=mux fragment-duration=500 ! appsink name=sink "+
					"audiotestsrc num-buffers=141 samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! opusenc ! mux.")
			events := make(chan *muxl.MuxlEvent, 16)
			done := make(chan error, 1)
			go func() {
				done <- muxl.RunMuxlSignSegment(ctx, bytes.NewReader(fixture), muxl.SignerInput{
					CertPEM: signer.Cert, KeyPEM: key, TrackManifest: signer.PrebuiltManifest,
					TextFn: func(_ context.Context, r muxl.TextRequest) (*muxl.TextAttachment, error) {
						if late && r.StartMs == 0 {
							return nil, nil
						}
						return &muxl.TextAttachment{Tracks: []muxl.TextTrackAttachment{{TextTrack: muxl.TextTrack{TrackID: CaptionTrackIDBase, Language: "en-US", Label: "human"}, Cues: []muxl.TextCue{{ID: "human", Start: r.StartMs + 10, End: r.StartMs + 50, Text: "words survive AAC"}}}}}, nil
					},
				}, nil, nil, events)
				close(events)
			}()
			mm := NewOffline(&config.CLI{BroadcasterHost: "test.example.com"})
			var mu sync.Mutex
			var completed, sources [][]byte
			tr := mm.newStreamTranscoder(ctx, "aac", signer.Cert, key, func(_ any, b []byte) { mu.Lock(); completed = append(completed, b); mu.Unlock() })
			defer tr.Close()
			for event := range events {
				if event.Type == "signed-segment" {
					source := concatTracksByID(event.Tracks)
					sources = append(sources, source)
					require.NoError(t, tr.Feed(source, nil))
				}
			}
			require.NoError(t, <-done)
			require.NoError(t, tr.Close())
			mu.Lock()
			defer mu.Unlock()
			require.Len(t, completed, len(sources), "late track declarations must not terminate continuous audio completion")
			if late {
				require.GreaterOrEqual(t, len(sources), 3, "exercise another AV GoP after declaring text")
			}
			for index, segment := range completed {
				completedTracks := segmentTracks(t, ctx, segment)
				for id, run := range segmentTracks(t, ctx, sources[index]) {
					require.Equal(t, run, completedTracks[id], "audio completion must retain every signed source run (track %s)", id)
				}
				require.Equal(t, concatTracksByID(completedTracks), segment, "the added audio run keeps ascending track order, ahead of text")
				events, err := unwrapMuxlEvents(ctx, segment)
				require.NoError(t, err)
				catalog, tracks := catalogAndTracks(events)
				require.Contains(t, tracks, "1")
				require.Contains(t, tracks, "2")
				require.Contains(t, tracks, "3")
				require.NotNil(t, catalog.Audio)
				require.Len(t, catalog.Audio.Renditions, 2)
				cues, err := muxl.RunMuxlReadTextCues(ctx, bytes.NewReader(segment), CaptionTrackIDBase)
				if late && index == 0 {
					require.NotContains(t, tracks, "100")
				} else {
					require.Contains(t, tracks, "100")
					require.NoError(t, err)
					require.Len(t, cues, 1)
					require.Equal(t, "words survive AAC", cues[0].Text)
				}
				report, err := muxl.RunMuxlVerify(ctx, bytes.NewReader(segment))
				require.NoError(t, err)
				require.NotContains(t, report, `"validation_state":"Invalid"`)
				media, err := ValidateMP4Media(ctx, segment)
				require.NoError(t, err)
				require.Equal(t, 48000, media.MediaData.Audio[0].Rate, "the default audio rendition remains decodable")
			}
		})
	}
}
