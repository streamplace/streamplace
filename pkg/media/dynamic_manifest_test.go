package media

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/muxl"
)

// stubManifester returns one manifest for the first BuildManifest call and a
// different one for every call after — the smallest possible model of a
// pre-live → live transition for SignSegmentStream to react to.
type stubManifester struct {
	calls   atomic.Int32
	preLive []byte
	live    []byte
}

func (s *stubManifester) BuildManifest(_ context.Context, _ string, _ int64) ([]byte, error) {
	if s.calls.Add(1) == 1 {
		return s.preLive, nil
	}
	return s.live, nil
}

// TestSignSegmentStreamRefreshesManifestPerSegment is the pre-live → live
// regression test: a long-lived SignSegmentStream call must consult the
// manifest builder for EVERY GoP, so a livestream record flipping from
// EndedAt-set (no c2pa.published) to EndedAt-nil (c2pa.published) lands in
// subsequent segments without restarting the wasm signer.
//
// Pre-fix, manifestBs was built once at SignSegmentStream entry and frozen for
// the whole RTMP session: this test would observe BuildManifest fire exactly
// once, and every signed segment would carry the pre-live manifest.
func TestSignSegmentStreamRefreshesManifestPerSegment(t *testing.T) {
	ctx := context.Background()
	ms := newBareSegmentSigner(t)
	// Drop PrebuiltManifest so the stub manifestBuilder wins the dispatch in
	// buildManifest (the production code path the fix targets).
	ms.PrebuiltManifest = nil
	stub := &stubManifester{
		preLive: []byte(`{
			"title":"pre-live",
			"assertions":[
				{"label":"c2pa.actions","data":{"actions":[{"action":"c2pa.created"}]}},
				{"label":"cawg.metadata","data":{
					"@context":{"dc":"http://purl.org/dc/elements/1.1/"},
					"dc:creator":"did:example","dc:title":"pre-live",
					"dc:date":"1970-01-01T00:00:00.000Z"
				}}
			]
		}`),
		live: []byte(`{
			"title":"live",
			"assertions":[
				{"label":"c2pa.actions","data":{"actions":[
					{"action":"c2pa.created"},
					{"action":"c2pa.published"}
				]}},
				{"label":"cawg.metadata","data":{
					"@context":{"dc":"http://purl.org/dc/elements/1.1/"},
					"dc:creator":"did:example","dc:title":"live",
					"dc:date":"1970-01-01T00:00:00.000Z"
				}}
			]
		}`),
	}
	ms.manifestBuilder = stub

	frag, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)

	eventCh := make(chan *muxl.MuxlEvent, 16)
	errCh := make(chan error, 1)
	go func() {
		err := ms.SignSegmentStream(ctx, bytes.NewReader(frag), eventCh)
		close(eventCh)
		errCh <- err
	}()

	var gops [][]byte // per-GoP bare .m4s
	for ev := range eventCh {
		if ev.Type != "signed-segment" {
			continue
		}
		gops = append(gops, concatTracksSorted(ev.Tracks))
	}
	require.NoError(t, <-errCh)
	require.GreaterOrEqual(t, len(gops), 2,
		"fixture must produce at least two GoPs to exercise the per-segment refresh")

	// Two of the three buildManifest paths matter here:
	//   - one call per GoP (the property the fix introduces);
	//   - the WrapperManifestFn is wired in muxl too, but the wasm signer
	//     currently doesn't ask the host for kind=1 (wrapper). So we expect
	//     exactly len(gops) calls — same as the GoP count.
	require.Equal(t, int32(len(gops)), stub.calls.Load(),
		"BuildManifest must be called once per GoP")

	// First GoP: pre-live (c2pa.created only). Later GoPs: live (c2pa.published
	// added). The actions live inside the SIGNED claim, so we re-verify each
	// bare .m4s in-wasm and read them back from the verify JSON.
	for i, m4s := range gops {
		out, err := muxl.RunMuxlVerify(ctx, bytes.NewReader(m4s))
		require.NoError(t, err, "GoP %d verify", i)
		require.NotContains(t, out, `"validation_state":"Invalid"`,
			"GoP %d must validate (verify JSON: %s)", i, out)

		var doc struct {
			Segments []struct {
				Manifest struct {
					Assertions []struct {
						Label string `json:"label"`
						Data  struct {
							Actions []struct {
								Action string `json:"action"`
							} `json:"actions"`
						} `json:"data"`
					} `json:"assertions"`
				} `json:"manifest"`
			} `json:"segments"`
		}
		require.NoError(t, json.Unmarshal([]byte(out), &doc), "GoP %d JSON", i)
		require.NotEmpty(t, doc.Segments, "GoP %d has segments", i)

		hasPublished := false
		for _, seg := range doc.Segments {
			for _, a := range seg.Manifest.Assertions {
				if !strings.HasPrefix(a.Label, "c2pa.actions") {
					continue
				}
				for _, act := range a.Data.Actions {
					if act.Action == "c2pa.published" {
						hasPublished = true
					}
				}
			}
		}
		wantPublished := i > 0
		require.Equal(t, wantPublished, hasPublished,
			"GoP %d: published=%t expected %t — manifest did not refresh between segments",
			i, hasPublished, wantPublished)
	}
}
