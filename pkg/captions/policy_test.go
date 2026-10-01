package captions

import (
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/placestream"
)

func TestPolicyFromMetadataDefaults(t *testing.T) {
	require.Equal(t, DefaultPolicy(), PolicyFromMetadata(nil))
	require.Equal(t, DefaultPolicy(), PolicyFromMetadata(&placestream.MetadataConfiguration{}))

	p := PolicyFromMetadata(&placestream.MetadataConfiguration{CaptionPolicy: &placestream.MetadataCaptionPolicy{
		Canonical: new("INGEST"), AllowNodeCaptions: new(false), Languages: []string{"en", "es"},
	}})
	require.Equal(t, Policy{Canonical: CanonicalIngest, AllowNodeCaptions: false, Languages: []string{"en", "es"}}, p)

	unknown := PolicyFromMetadata(&placestream.MetadataConfiguration{CaptionPolicy: &placestream.MetadataCaptionPolicy{Canonical: new("someday")}})
	require.Equal(t, CanonicalAuto, unknown.Canonical, "unknown values fall back to auto")
	require.True(t, unknown.AllowNodeCaptions)
}

func TestDecideMatrix(t *testing.T) {
	pol := func(canonical string, allow bool) Policy {
		return Policy{Canonical: canonical, AllowNodeCaptions: allow, Languages: []string{"en"}}
	}
	cases := []struct {
		name string
		in   Situation
		want Decision
	}{
		{"origin auto recognizes canonical",
			Situation{Policy: pol(CanonicalAuto, true), Origin: true, NodeCaptions: true},
			Decision{Mode: ModeRecognize, Origin: OriginCanonical, Languages: []string{"en"}}},
		{"origin auto with node captions off still recognizes (streamer asked)",
			Situation{Policy: pol(CanonicalAuto, true), Origin: true, NodeCaptions: false},
			Decision{Mode: ModeRecognize, Origin: OriginCanonical, Languages: []string{"en"}}},
		{"origin auto prefers the streamer's own ingest captions",
			Situation{Policy: pol(CanonicalAuto, true), Origin: true, NodeCaptions: true, IngestCaptions: true},
			Decision{Mode: ModeIngest, Origin: OriginCanonical}},
		{"origin ingest never recognizes, even before captions arrive",
			Situation{Policy: pol(CanonicalIngest, true), Origin: true, NodeCaptions: true},
			Decision{Mode: ModeIngest, Origin: OriginCanonical}},
		{"origin ingest with node captions disallowed still masters ingest captions",
			Situation{Policy: pol(CanonicalIngest, false), Origin: true, NodeCaptions: false},
			Decision{Mode: ModeIngest, Origin: OriginCanonical}},
		{"origin off, node allowed and enabled: sidecar recognition",
			Situation{Policy: pol(CanonicalOff, true), Origin: true, NodeCaptions: true},
			Decision{Mode: ModeRecognize, Origin: OriginSidecar, Languages: []string{"en"}}},
		{"origin off, node allowed and enabled, ingest captions present: sidecar ingest",
			Situation{Policy: pol(CanonicalOff, true), Origin: true, NodeCaptions: true, IngestCaptions: true},
			Decision{Mode: ModeIngest, Origin: OriginSidecar}},
		{"origin off, node disabled: nothing",
			Situation{Policy: pol(CanonicalOff, true), Origin: true, NodeCaptions: false},
			Decision{Mode: ModeNone}},
		{"origin off, node disallowed: nothing",
			Situation{Policy: pol(CanonicalOff, false), Origin: true, NodeCaptions: true},
			Decision{Mode: ModeNone}},
		{"relay with incoming canonical text track passes through",
			Situation{Policy: pol(CanonicalAuto, true), NodeCaptions: true, IncomingCaptions: true},
			Decision{Mode: ModeNone}},
		{"relay with incoming captions and ingest captions still passes through",
			Situation{Policy: pol(CanonicalOff, true), NodeCaptions: true, IncomingCaptions: true, IngestCaptions: true},
			Decision{Mode: ModeNone}},
		{"relay, nothing incoming, allowed, enabled: sidecar recognition",
			Situation{Policy: pol(CanonicalAuto, true), NodeCaptions: true},
			Decision{Mode: ModeRecognize, Origin: OriginSidecar, Languages: []string{"en"}}},
		{"relay, nothing incoming, ingest policy at origin never produced a track: sidecar recognition",
			Situation{Policy: pol(CanonicalIngest, true), NodeCaptions: true},
			Decision{Mode: ModeRecognize, Origin: OriginSidecar, Languages: []string{"en"}}},
		{"relay sees embedded captions the origin did not master: sidecar ingest",
			Situation{Policy: pol(CanonicalOff, true), NodeCaptions: true, IngestCaptions: true},
			Decision{Mode: ModeIngest, Origin: OriginSidecar}},
		{"relay, node disabled: nothing",
			Situation{Policy: pol(CanonicalAuto, true), NodeCaptions: false},
			Decision{Mode: ModeNone}},
		{"relay, node disallowed: nothing",
			Situation{Policy: pol(CanonicalAuto, false), NodeCaptions: true},
			Decision{Mode: ModeNone}},
		{"relay, disallowed, ingest captions visible: nothing",
			Situation{Policy: pol(CanonicalOff, false), NodeCaptions: true, IngestCaptions: true},
			Decision{Mode: ModeNone}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, Decide(c.in))
		})
	}
}

func TestDecisionString(t *testing.T) {
	require.Equal(t, "none", Decision{Mode: ModeNone}.String())
	require.Equal(t, "recognize(sidecar)", Decision{Mode: ModeRecognize, Origin: OriginSidecar}.String())
	require.Equal(t, "ingest(canonical)", Decision{Mode: ModeIngest, Origin: OriginCanonical}.String())
}

func TestPushedOrigin(t *testing.T) {
	require.Equal(t, OriginCanonical, PushedOrigin(Policy{Canonical: CanonicalAuto, AllowNodeCaptions: true}))
	require.Equal(t, OriginCanonical, PushedOrigin(Policy{Canonical: CanonicalIngest, AllowNodeCaptions: false}))
	require.Equal(t, OriginSidecar, PushedOrigin(Policy{Canonical: CanonicalOff, AllowNodeCaptions: true}))
	require.Equal(t, OriginLocal, PushedOrigin(Policy{Canonical: CanonicalOff, AllowNodeCaptions: false}))
}
