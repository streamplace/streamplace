package media

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"sync"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// Video renditions as first-class MUXL tracks — the audio dual-codec path
// (finishTranscodedSegment) applied to video. A transcoder (Livepeer) hands
// back one MP4 per rendition of a source segment; each rendition's video
// track is relabelled to a fixed track id, signed as a c2pa.transcoded
// derivative of the source video track under the node's identity, and the
// signed tracks are appended into one bare canonical segment ("addendum").
// Canonical segments concatenate blindly, so the addendum folds into the
// same live window as the source segment: each rendition becomes a video
// track with its own catalog entry (codec, size), which the master playlist
// already renders as an HLS variant. Provenance travels with every
// rendition byte, exactly as with the transcoded audio.

// RenditionsChannel is the bus rendition name an addendum is published on
// for the streamer, beside the "source" channel the segment itself goes to,
// so syndication can ship renditions to peers as they're minted.
const RenditionsChannel = "renditions"

// Syndication frames. A source segment travels as its raw bytes (a bare
// canonical .m4s starts with a box size, which can never spell this); an
// addendum of rendition tracks is prefixed so the receiving node feeds it to
// its live window instead of validating it as a segment.
var renditionFrameMagic = []byte("SPRN")

// FrameRenditions wraps an addendum for the syndication websocket.
func FrameRenditions(addendum []byte) []byte {
	return append(append([]byte{}, renditionFrameMagic...), addendum...)
}

// UnframeRenditions reports whether a syndication message is an addendum
// and returns it.
func UnframeRenditions(msg []byte) ([]byte, bool) {
	if len(msg) > len(renditionFrameMagic) && string(msg[:len(renditionFrameMagic)]) == string(renditionFrameMagic) {
		return msg[len(renditionFrameMagic):], true
	}
	return nil, false
}

// RenditionInput is one transcoded rendition of a source segment as the
// transcoder returned it: an MP4 carrying the rendition's video (and, from
// Livepeer, the source audio muxed back in — ignored here).
type RenditionInput struct {
	Name string
	MP4  []byte
}

// renditionTrackBase + the rendition's index in the ladder is its track id.
// Fixed, not "next free", so a rendition keeps one id for the life of a
// stream whatever the source segment's own track set does (the audio
// completion adds a track), and the live window sees one continuous track
// per rendition.
const renditionTrackBase = 100

// RenditionTrackID is the track id rendition number i (in ladder order) is
// minted under.
func RenditionTrackID(i int) uint32 { return renditionTrackBase + uint32(i) }

// MintVideoRenditions signs the transcoded renditions of srcSeg (a bare
// canonical segment, as distributed) into one addendum of canonical video
// tracks. rs is in ladder order; a rendition that can't be minted is
// skipped with a log line rather than failing the rest. Returns nil, nil
// when nothing could be minted.
func (mm *MediaManager) MintVideoRenditions(ctx context.Context, srcSeg []byte, rs []RenditionInput) ([]byte, error) {
	cert, keyPEM, err := mm.transcodeSigner()
	if err != nil {
		return nil, err
	}
	return mm.mintVideoRenditions(ctx, srcSeg, rs, cert, keyPEM)
}

func (mm *MediaManager) mintVideoRenditions(ctx context.Context, srcSeg []byte, rs []RenditionInput, cert, keyPEM []byte) ([]byte, error) {
	events, err := unwrapMuxlEvents(ctx, srcSeg)
	if err != nil {
		return nil, fmt.Errorf("unwrap source segment: %w", err)
	}
	cat, tracks := catalogAndTracks(events)
	if cat == nil || cat.Video == nil {
		return nil, fmt.Errorf("source segment has no video track")
	}
	var srcVideoTID uint32
	for _, v := range cat.Video.Renditions {
		srcVideoTID = v.TrackID()
		break
	}
	sourceVideo := tracks[strconv.FormatUint(uint64(srcVideoTID), 10)]
	if len(sourceVideo) == 0 {
		return nil, fmt.Errorf("source video track %d missing", srcVideoTID)
	}
	manifest := transcodeManifestFor(mm.cli.BroadcasterDID(), "transcoded video")

	var addendum []byte
	minted := 0
	for i, r := range rs {
		if len(r.MP4) == 0 {
			continue
		}
		want := RenditionTrackID(i)
		pieces, err := mm.canonicalRenditionTrack(ctx, r, want)
		if err != nil {
			log.Warn(ctx, "rendition: canonicalize failed, skipping", "rendition", r.Name, "error", err)
			continue
		}
		// One signed asset per canonical segment (a rendition the transcoder
		// keyframed mid-segment is several): a signature over a run of
		// segments does not verify.
		ok := true
		var rendition []byte
		for _, piece := range pieces {
			signed, err := muxl.RunMuxlSignTranscode(ctx, muxl.TranscodeInput{
				Output:   piece,
				Source:   sourceVideo,
				CertPEM:  cert,
				KeyPEM:   keyPEM,
				Manifest: manifest,
			})
			if err != nil {
				log.Warn(ctx, "rendition: sign failed, skipping", "rendition", r.Name, "error", err)
				ok = false
				break
			}
			rendition = append(rendition, signed...)
		}
		if !ok {
			continue
		}
		addendum = append(addendum, rendition...)
		minted++
	}
	if minted == 0 {
		return nil, nil
	}
	return addendum, nil
}

// canonicalRenditionTrack canonicalizes a rendition MP4 with its video track
// relabelled to want and returns that track's bytes per canonical segment
// the input became (a rendition spanning several GoPs is several), in
// order. The transcoder's track layout is learned on first sight per
// rendition name and remembered, so later segments canonicalize once, not
// twice.
func (mm *MediaManager) canonicalRenditionTrack(ctx context.Context, r RenditionInput, want uint32) ([][]byte, error) {
	var remap map[uint32]uint32
	if v, ok := mm.renditionVideoTID.Load(r.Name); ok && v.(uint32) != want {
		remap = map[uint32]uint32{v.(uint32): want}
	}
	canon, err := muxl.RunMuxlCanonicalize(ctx, r.MP4, remap)
	if err != nil {
		return nil, fmt.Errorf("canonicalize: %w", err)
	}
	events, err := unwrapMuxlEvents(ctx, canon)
	if err != nil {
		return nil, fmt.Errorf("unwrap canonical: %w", err)
	}
	cat, _ := catalogAndTracks(events)
	if cat == nil || cat.Video == nil || len(cat.Video.Renditions) == 0 {
		return nil, fmt.Errorf("no video track in rendition")
	}
	var videoTID uint32
	for _, v := range cat.Video.Renditions {
		videoTID = v.TrackID()
		break
	}
	if videoTID != want {
		// First sight of this transcoder's layout: remember the id its
		// video comes out under and canonicalize again with the relabel.
		mm.renditionVideoTID.Store(r.Name, videoTID)
		canon, err = muxl.RunMuxlCanonicalize(ctx, r.MP4, map[uint32]uint32{videoTID: want})
		if err != nil {
			return nil, fmt.Errorf("canonicalize with relabel: %w", err)
		}
		events, err = unwrapMuxlEvents(ctx, canon)
		if err != nil {
			return nil, fmt.Errorf("unwrap relabelled: %w", err)
		}
	}
	key := strconv.FormatUint(uint64(want), 10)
	var pieces [][]byte
	for _, ev := range events {
		if (ev.Type == "segment" || ev.Type == "signed-segment") && len(ev.Tracks[key]) > 0 {
			pieces = append(pieces, ev.Tracks[key])
		}
	}
	if len(pieces) == 0 {
		return nil, fmt.Errorf("relabelled video track %d missing", want)
	}
	return pieces, nil
}

// renditionTIDCache is the per-rendition-name video track id a transcoder's
// MP4s come out under (see canonicalRenditionTrack).
type renditionTIDCache = sync.Map

// PresentationWithOpus is a segment as a flat MP4 with video plus its Opus
// audio only — what the transcoder is handed so the audio it muxes back
// into each rendition is the one the WebRTC packetizer can take. A
// single-codec AAC segment (none completed yet) comes back as is.
func PresentationWithOpus(ctx context.Context, seg []byte) ([]byte, error) {
	opus, err := filterSegmentToCodec(ctx, seg, true)
	if err != nil {
		return nil, err
	}
	var flat bytes.Buffer
	if err := muxl.RunMuxlWrap(ctx, bytes.NewReader(opus), "flat", &flat); err != nil {
		return nil, err
	}
	return flat.Bytes(), nil
}
