package media

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers"
)

func TestValidateMP4PublishesEarlyOpusAndCompletesCanonicalAudio(t *testing.T) {
	for _, tt := range []struct {
		name      string
		published bool
		bitrate   int
		wantEarly bool
	}{
		{name: "published", published: true, wantEarly: true},
		{name: "preview", wantEarly: true},
		{name: "bitrate admission waits for completion", published: true, bitrate: 10},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ms, source := earlyWebRTCSource(t, tt.published)
			mm := earlyWebRTCManager(t, ms)
			mm.cli.MaximumLiveBitrate = tt.bitrate
			private := mm.bus.SubscribeSegment(ctx, ms.Streamer(), WebRTCSourceRendition)
			defer mm.bus.UnsubscribeSegment(ctx, ms.Streamer(), WebRTCSourceRendition, private)
			canonical := mm.newSegmentSubs[0].queue

			require.NoError(t, mm.ValidateMP4(ctx, bytes.NewReader(source), true))
			if tt.wantEarly {
				select {
				case segment := <-private.C:
					require.Equal(t, tt.published, segment.Published, "preview media must remain private")
					require.NotNil(t, segment.PacketizedData)
					require.NotEmpty(t, segment.PacketizedData.Video)
					require.NotEmpty(t, segment.PacketizedData.Audio)
					require.Positive(t, segment.PacketizedData.Duration)
				case <-time.After(5 * time.Second):
					t.Fatal("Opus source was withheld until AAC completion")
				}
			} else {
				select {
				case <-private.C:
					t.Fatal("early playback bypassed configured bitrate admission")
				case <-time.After(100 * time.Millisecond):
				}
			}
			select {
			case <-canonical:
				t.Fatal("canonical consumers received incomplete single-codec media")
			case <-time.After(100 * time.Millisecond):
			}
			require.Nil(t, mm.GetLiveWindow(ms.Streamer()), "HLS must wait for canonical completion")

			// One source GOP has no successor to flush its continuous AAC tail.
			// Closing the real transcoder supplies EOS and completes that GOP.
			mm.transcodersMu.Lock()
			tr := mm.transcoders[ms.Streamer()]
			mm.transcodersMu.Unlock()
			require.NotNil(t, tr)
			require.NoError(t, tr.Close())
			select {
			case notification := <-canonical:
				codecs := audioCodecsOf(t, context.Background(), notification.Muxl)
				require.Contains(t, codecs, "opus")
				require.Contains(t, codecs, "mp4a.40.2")
				require.Equal(t, tt.published, notification.Metadata.Published)
				require.Equal(t, tt.published, notification.Segment.Published)
				require.Equal(t, tt.wantEarly, notification.WebRTCPublished,
					"completion must tell the director whether playback already received this GOP")
			case <-time.After(5 * time.Second):
				t.Fatal("AAC completion did not reach canonical consumers")
			}
			require.Eventually(t, func() bool {
				window := mm.GetLiveWindow(ms.Streamer())
				if window == nil {
					return false
				}
				playlist := window.MasterPlaylist(func(tid string) string { return tid + ".m3u8" })
				return strings.Contains(playlist, "opus") && strings.Contains(playlist, "mp4a.40.2")
			}, 5*time.Second, 10*time.Millisecond, "HLS must retain both completed audio renditions")
			require.Equal(t, tt.published, mm.LiveWindowPublished(ms.Streamer()))
		})
	}
}

func TestValidateMP4RejectsSourceBeforeEarlyPlayback(t *testing.T) {
	ms, source := earlyWebRTCSource(t, true)
	for _, name := range []string{"malformed", "streamer denied", "content filtered"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			mm := earlyWebRTCManager(t, ms)
			input := source
			switch name {
			case "malformed":
				input = source[:7]
			case "streamer denied":
				mm.cli.AllowedStreams = []string{"did:key:another-streamer"}
			case "content filtered":
				mm.cli.ContentFilters = &config.ContentFilters{}
				mm.cli.ContentFilters.ContentWarnings.Enabled = true
				mm.cli.ContentFilters.ContentWarnings.BlockedWarnings = []string{"early-test-warning"}
			}
			private := mm.bus.SubscribeSegment(ctx, ms.Streamer(), WebRTCSourceRendition)
			defer mm.bus.UnsubscribeSegment(ctx, ms.Streamer(), WebRTCSourceRendition, private)
			canonical := mm.newSegmentSubs[0].queue
			err := mm.ValidateMP4(ctx, bytes.NewReader(input), true)
			require.Error(t, err)
			if name == "streamer denied" {
				require.ErrorContains(t, err, "not allowed")
			} else if name == "content filtered" {
				require.ErrorContains(t, err, "content warning blocked")
			}
			select {
			case <-private.C:
				t.Fatal("rejected source reached private playback")
			case <-canonical:
				t.Fatal("rejected source reached canonical consumers")
			case <-time.After(100 * time.Millisecond):
			}
			require.Empty(t, mm.transcoders, "rejected sources must not start codec completion")
			require.Nil(t, mm.GetLiveWindow(ms.Streamer()))
		})
	}
}

func earlyWebRTCSource(t *testing.T, published bool) (*MediaSignerLocal, []byte) {
	t.Helper()
	ms := newBareSegmentSigner(t)
	pub, err := atproto.ParsePubKey(ms.Signer.Public())
	require.NoError(t, err)
	ms.StreamerName = pub.DIDKey()
	var manifest map[string]any
	require.NoError(t, json.Unmarshal(ms.PrebuiltManifest, &manifest))
	assertions := manifest["assertions"].([]any)
	metadata := assertions[1].(map[string]any)["data"].(map[string]any)
	metadata["dc:creator"] = ms.Streamer()
	metadata["@context"].(map[string]any)["Iptc4xmpExt"] = "http://iptc.org/std/Iptc4xmpExt/2008-02-29/"
	metadata["Iptc4xmpExt:ContentWarning"] = []string{"early-test-warning"}
	if published {
		assertions[0].(map[string]any)["data"].(map[string]any)["actions"] = []obj{
			{"action": "c2pa.created"}, {"action": "c2pa.published"},
		}
	}
	ms.PrebuiltManifest, err = json.Marshal(manifest)
	require.NoError(t, err)
	segments := allSignedBareSegments(t, context.Background(), ms, getFixture("h264-opus-frag.mp4"))
	require.NotEmpty(t, segments)
	require.Equal(t, []string{"opus"}, audioCodecsOf(t, context.Background(), segments[0]))
	return ms, segments[0]
}

func earlyWebRTCManager(t *testing.T, ms *MediaSignerLocal) *MediaManager {
	t.Helper()
	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)
	mm := NewOffline(&config.CLI{
		BroadcasterHost: "test.example.com", AllowedStreams: []string{ms.Streamer()}, DataDir: t.TempDir(),
	})
	mm.bus = bus.NewBus()
	mm.transcoders = map[string]*streamTranscoder{}
	// Observe real canonical distribution without a forwarding goroutine.
	mm.newSegmentSubs = []*segmentSubscriber{{queue: make(chan *NewSegmentNotification, 1)}}
	mm.nodeSignerOnce.Do(func() { mm.nodeCert, mm.nodeKeyPEM = ms.Cert, keyPEM })
	t.Cleanup(func() {
		mm.transcodersMu.Lock()
		tr := mm.transcoders[ms.Streamer()]
		delete(mm.transcoders, ms.Streamer())
		mm.transcodersMu.Unlock()
		if tr != nil {
			require.NoError(t, tr.Close())
		}
	})
	return mm
}
