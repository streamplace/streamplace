package captions

import (
	"strings"

	"stream.place/streamplace/pkg/placestream"
)

// Canonical caption policy values from place.stream.metadata.captionPolicy.
const (
	CanonicalAuto   = "auto"
	CanonicalIngest = "ingest"
	CanonicalOff    = "off"
)

// Policy is a streamer's caption policy with the lexicon's defaults applied.
type Policy struct {
	Canonical         string // auto | ingest | off
	AllowNodeCaptions bool
	Languages         []string // recognition hints, most prominent first
}

// DefaultPolicy is what a stream with no captionPolicy gets.
func DefaultPolicy() Policy {
	return Policy{Canonical: CanonicalAuto, AllowNodeCaptions: true}
}

// PolicyFromMetadata reads the caption policy out of a segment's metadata
// configuration, applying the defaults for absent fields and unknown values.
func PolicyFromMetadata(cfg *placestream.MetadataConfiguration) Policy {
	p := DefaultPolicy()
	if cfg == nil || cfg.CaptionPolicy == nil {
		return p
	}
	cp := cfg.CaptionPolicy
	if cp.Canonical != nil {
		switch strings.ToLower(*cp.Canonical) {
		case CanonicalIngest:
			p.Canonical = CanonicalIngest
		case CanonicalOff:
			p.Canonical = CanonicalOff
		}
	}
	if cp.AllowNodeCaptions != nil {
		p.AllowNodeCaptions = *cp.AllowNodeCaptions
	}
	p.Languages = append([]string(nil), cp.Languages...)
	return p
}

// Mode is what a node does for a stream's captions.
type Mode string

const (
	// ModeNone: no caption source runs on this node for the stream.
	ModeNone Mode = "none"
	// ModeRecognize: run speech recognition over the stream's audio.
	ModeRecognize Mode = "recognize"
	// ModeIngest: the streamer supplies captions at ingest (CEA-608/708 in the
	// video, or pushCaptions); the node decodes and distributes those.
	ModeIngest Mode = "ingest"
)

// Situation is everything the policy engine needs to know about one stream
// on one node.
type Situation struct {
	Policy Policy
	// Origin is true on the node ingesting the stream (segments signed here),
	// false on a relay that validates replicated segments.
	Origin bool
	// NodeCaptions is the node default (cli.Captions).
	NodeCaptions bool
	// IncomingCaptions is true when the segments this node receives already
	// carry captions: a canonical MUXL text track or an upstream sidecar.
	IncomingCaptions bool
	// IngestCaptions is true once the streamer's own captions have been seen
	// at this node: CEA-608/708 cc_data in the H264 SEI or pushed cues.
	IngestCaptions bool
}

// Decision is the policy engine's answer for one stream.
type Decision struct {
	Mode Mode
	// Origin of the track this node would publish; meaningful when Mode is
	// not ModeNone.
	Origin Origin
	// Languages are the recognition hints, when Mode is ModeRecognize.
	Languages []string
}

// Recognize reports whether the node should transcribe speech.
func (d Decision) Recognize() bool { return d.Mode == ModeRecognize }

// Decide applies the behavior matrix:
//
//   - Origin, canonical=auto: recognize into the canonical track; if the
//     streamer's own captions turn up at ingest they are better than
//     recognition and take over as the canonical track.
//   - Origin, canonical=ingest: only the streamer's ingest captions, canonical.
//   - canonical=off with node captions allowed and enabled: recognize (or,
//     at the origin, decode ingest captions) as a sidecar track.
//   - allowNodeCaptions=false: nothing node-generated; a relay passes through
//     whatever is incoming, the origin still masters ingest captions when the
//     policy asks for them.
//   - Relay with incoming captions: pass through, nothing generated.
//   - Relay, nothing incoming, allowed, node enabled: recognize as a sidecar.
func Decide(s Situation) Decision {
	nodeMayCaption := s.Policy.AllowNodeCaptions && s.NodeCaptions
	langs := s.Policy.Languages

	if s.Origin {
		switch s.Policy.Canonical {
		case CanonicalIngest:
			return Decision{Mode: ModeIngest, Origin: OriginCanonical}
		case CanonicalAuto:
			if s.IngestCaptions {
				return Decision{Mode: ModeIngest, Origin: OriginCanonical}
			}
			return Decision{Mode: ModeRecognize, Origin: OriginCanonical, Languages: langs}
		}
		// canonical=off: the signed stream carries no captions. Node captions
		// are a sidecar when allowed.
		if !nodeMayCaption {
			return Decision{Mode: ModeNone}
		}
		if s.IngestCaptions {
			return Decision{Mode: ModeIngest, Origin: OriginSidecar}
		}
		return Decision{Mode: ModeRecognize, Origin: OriginSidecar, Languages: langs}
	}

	// Relay.
	if s.IncomingCaptions || !nodeMayCaption {
		return Decision{Mode: ModeNone}
	}
	if s.IngestCaptions {
		// The origin left the embedded captions in the video without
		// mastering them; decoding them beats transcribing over them.
		return Decision{Mode: ModeIngest, Origin: OriginSidecar}
	}
	return Decision{Mode: ModeRecognize, Origin: OriginSidecar, Languages: langs}
}

// PushedOrigin is the origin of a track of cues pushed by the streamer
// (pushCaptions): canonical whenever the policy lets the streamer's own
// captions into the signed stream, otherwise a sidecar when node captions are
// allowed, otherwise local to this node's viewers.
func PushedOrigin(p Policy) Origin {
	switch p.Canonical {
	case CanonicalAuto, CanonicalIngest:
		return OriginCanonical
	}
	if p.AllowNodeCaptions {
		return OriginSidecar
	}
	return OriginLocal
}
