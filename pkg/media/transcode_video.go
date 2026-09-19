package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
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

// MintVideoRenditionsWith is MintVideoRenditions with an explicit signing
// identity (tests and offline harnesses, which have no node signer).
func (mm *MediaManager) MintVideoRenditionsWith(ctx context.Context, srcSeg []byte, rs []RenditionInput, cert, keyPEM []byte) ([]byte, error) {
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
	var srcVideoTID, srcTimescale uint32
	for _, v := range cat.Video.Renditions {
		srcVideoTID = v.TrackID()
		srcTimescale = v.Timescale()
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
		pieces, renTimescale, err := mm.canonicalRenditionTrack(ctx, r, want)
		if err != nil {
			log.Warn(ctx, "rendition: canonicalize failed, skipping", "rendition", r.Name, "error", err)
			continue
		}
		// The transcoder saw a segment that starts at zero (the TS we hand
		// it carries running time, not the stream's), so its output starts
		// at zero too: put every fragment back on the source's timeline.
		if delta, ok := retimeDelta(sourceVideo, srcTimescale, pieces[0], renTimescale); ok {
			for j := range pieces {
				pieces[j] = shiftTfdt(pieces[j], delta)
			}
		} else {
			log.Warn(ctx, "rendition: could not align timeline to source", "rendition", r.Name)
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
func (mm *MediaManager) canonicalRenditionTrack(ctx context.Context, r RenditionInput, want uint32) ([][]byte, uint32, error) {
	var remap map[uint32]uint32
	if v, ok := mm.renditionVideoTID.Load(r.Name); ok && v.(uint32) != want {
		remap = map[uint32]uint32{v.(uint32): want}
	}
	canon, err := muxl.RunMuxlCanonicalize(ctx, r.MP4, remap)
	if err != nil {
		return nil, 0, fmt.Errorf("canonicalize: %w", err)
	}
	events, err := unwrapMuxlEvents(ctx, canon)
	if err != nil {
		return nil, 0, fmt.Errorf("unwrap canonical: %w", err)
	}
	cat, _ := catalogAndTracks(events)
	if cat == nil || cat.Video == nil || len(cat.Video.Renditions) == 0 {
		return nil, 0, fmt.Errorf("no video track in rendition")
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
			return nil, 0, fmt.Errorf("canonicalize with relabel: %w", err)
		}
		events, err = unwrapMuxlEvents(ctx, canon)
		if err != nil {
			return nil, 0, fmt.Errorf("unwrap relabelled: %w", err)
		}
		// The catalog is the relabelled one from here on, or the lookup
		// below misses the track and the rendition is retimed in the
		// source's timescale.
		if cat, _ = catalogAndTracks(events); cat == nil || cat.Video == nil {
			return nil, 0, fmt.Errorf("no video track in relabelled rendition")
		}
	}
	var timescale uint32
	for _, v := range cat.Video.Renditions {
		if v.TrackID() == want {
			timescale = v.Timescale()
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
		return nil, 0, fmt.Errorf("relabelled video track %d missing", want)
	}
	return pieces, timescale, nil
}

// renditionTIDCache is the per-rendition-name video track id a transcoder's
// MP4s come out under (see canonicalRenditionTrack).
type renditionTIDCache = sync.Map

// PresentationWithOpus is a segment as a fragmented MP4 with video plus its
// Opus audio only — what the transcoder is handed so the audio it muxes
// back into each rendition is the one the WebRTC packetizer can take. A
// single-codec AAC segment (none completed yet) comes back as is.
//
// Fragmented, not flat: a flat MP4 carries a track duration, and qtdemux
// clips samples that present past it — with B-frames the last P frame's
// presentation time lands beyond the summed sample durations, so every
// segment lost one frame (49 of 50 from a real broadcast). A fragmented
// presentation has no duration to clip against and every sample gets
// through.
func PresentationWithOpus(ctx context.Context, seg []byte) ([]byte, error) {
	opus, err := filterSegmentToCodec(ctx, seg, true)
	if err != nil {
		return nil, err
	}
	var fmp4 bytes.Buffer
	if err := muxl.RunMuxlWrap(ctx, bytes.NewReader(opus), "fmp4", &fmp4); err != nil {
		return nil, err
	}
	return fmp4.Bytes(), nil
}

// firstTfdt returns the first fragment's baseMediaDecodeTime in a run of
// [moof][mdat] (or [uuid]…[moof][mdat]) boxes, and whether one was found.
func firstTfdt(b []byte) (uint64, bool) {
	var found bool
	var val uint64
	walkBoxes(b, func(typ string, body []byte) bool {
		if typ != "moof" {
			return true
		}
		walkBoxes(body, func(t2 string, traf []byte) bool {
			if t2 != "traf" {
				return true
			}
			walkBoxes(traf, func(t3 string, tfdt []byte) bool {
				if t3 != "tfdt" || len(tfdt) < 8 {
					return true
				}
				if tfdt[0] == 1 && len(tfdt) >= 12 {
					val = binary.BigEndian.Uint64(tfdt[4:12])
				} else {
					val = uint64(binary.BigEndian.Uint32(tfdt[4:8]))
				}
				found = true
				return false
			})
			return !found
		})
		return !found
	})
	return val, found
}

// shiftTfdt adds delta to every tfdt in b, in place, and returns b. A
// version-0 tfdt that would overflow 32 bits is left alone (the box can't
// grow without moving every mdat offset).
func shiftTfdt(b []byte, delta int64) []byte {
	walkBoxesOffsets(b, 0, func(typ string, start, end int) bool {
		if typ != "moof" {
			return true
		}
		walkBoxesOffsets(b[:end], start+8, func(t2 string, s2, e2 int) bool {
			if t2 != "traf" {
				return true
			}
			walkBoxesOffsets(b[:e2], s2+8, func(t3 string, s3, e3 int) bool {
				if t3 != "tfdt" || e3-s3 < 16 {
					return true
				}
				body := b[s3+8 : e3]
				if body[0] == 1 && len(body) >= 12 {
					v := int64(binary.BigEndian.Uint64(body[4:12])) + delta
					binary.BigEndian.PutUint64(body[4:12], uint64(v))
				} else {
					v := int64(binary.BigEndian.Uint32(body[4:8])) + delta
					if v >= 0 && v <= math.MaxUint32 {
						binary.BigEndian.PutUint32(body[4:8], uint32(v))
					}
				}
				return true
			})
			return true
		})
		return true
	})
	return b
}

// retimeDelta is what to add to a rendition's tfdts so its first fragment
// lands where the source segment starts, in the rendition's timescale.
func retimeDelta(sourceVideo []byte, srcTimescale uint32, rendition []byte, renTimescale uint32) (int64, bool) {
	srcBase, ok := firstTfdt(sourceVideo)
	if !ok || srcTimescale == 0 {
		return 0, false
	}
	renBase, ok := firstTfdt(rendition)
	if !ok {
		return 0, false
	}
	if renTimescale == 0 {
		renTimescale = srcTimescale
	}
	want := int64(float64(srcBase) * float64(renTimescale) / float64(srcTimescale))
	return want - int64(renBase), true
}

// walkBoxes calls fn(type, body) for each top-level box of b until fn
// returns false.
func walkBoxes(b []byte, fn func(typ string, body []byte) bool) {
	walkBoxesOffsets(b, 0, func(typ string, start, end int) bool {
		return fn(typ, b[start+8:end])
	})
}

// walkBoxesOffsets calls fn(type, start, end) for each box in b[off:] until
// fn returns false; start is the box header offset, end one past the box.
// 64-bit sizes are not expected in a canonical segment and stop the walk.
func walkBoxesOffsets(b []byte, off int, fn func(typ string, start, end int) bool) {
	for off+8 <= len(b) {
		size := int(binary.BigEndian.Uint32(b[off : off+4]))
		typ := string(b[off+4 : off+8])
		if size < 8 || off+size > len(b) {
			return
		}
		if !fn(typ, off, off+size) {
			return
		}
		off += size
	}
}
