package media

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/muxl"
)

func TestReplicatedRenditionBindsItsSource(t *testing.T) {
	ctx := context.Background()
	makeSource := func() ([]byte, string) {
		signer := newBareSegmentSigner(t)
		did := signer.DID()
		signer.PrebuiltManifest = []byte(fmt.Sprintf(`{"title":"MoQ source","assertions":[{"label":"c2pa.actions","data":{"actions":[{"action":"c2pa.created"},{"action":"c2pa.published"}]}},{"label":"cawg.metadata","data":{"@context":{"dc":"http://purl.org/dc/elements/1.1/"},"dc:creator":%q,"dc:title":"MoQ source","dc:date":"1970-01-01T00:00:00.000Z"}}]}`, did))
		fixture, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
		require.NoError(t, err)
		events := make(chan *muxl.MuxlEvent, 16)
		done := make(chan error, 1)
		go func() { done <- signer.SignSegmentStream(ctx, bytes.NewReader(fixture), events); close(events) }()
		var source []byte
		for event := range events {
			if event.Type == "signed-segment" && source == nil {
				source = concatTracksByID(event.Tracks)
			}
		}
		require.NoError(t, <-done)
		return source, did
	}
	source, did := makeSource()
	wrongSource, wrongDID := makeSource()
	manager := &MediaManager{cli: &config.CLI{BroadcasterHost: "node.test", AllowedStreams: []string{did, wrongDID}}}
	cert, key := nodeSignerForTest(t)
	addendum, err := manager.MintVideoRenditionsWith(ctx, source, []RenditionInput{{Name: "160p", MP4: renditionMP4(t, 160, 120)}}, cert, key)
	require.NoError(t, err)
	require.NoError(t, manager.ValidateReplicatedRenditions(ctx, did, source, addendum))
	require.ErrorContains(t, manager.ValidateReplicatedRenditions(ctx, wrongDID, wrongSource, addendum), "no ingredient bound")
	require.Nil(t, manager.GetLiveWindow(did), "archive validation must not publish live media")
}
