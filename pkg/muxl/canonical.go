package muxl

import (
	"encoding/binary"
	"fmt"
)

// StripFlatMP4WrapperToBundle converts a signed muxl flat MP4
// (`ftyp + c2pa-uuid + moov + mdat-envelope { canonical segments }`)
// to its canonical-segment-bundle equivalent — just the bytes inside
// the outer mdat envelope.
//
// The returned bundle is a concatenation of signed canonical segments
// (`[c2pa-uuid + muxl-uuid + moof+mdat]*` per track per GoP) and is
// the new canonical Streamplace storage shape: smaller than the
// flat-MP4 wrapper, no synthesizable header bytes inside the signed
// surface, and each canonical segment inside is an independently
// c2pa-verifiable `.m4s` asset.
//
// Playback requires re-synthesizing a flat-MP4 wrapper from the
// catalogs embedded in each canonical segment's muxl-uuid (see
// `muxl::build_synth_flat_header`).
func StripFlatMP4WrapperToBundle(buf []byte) ([]byte, error) {
	const mdatBoxType = "mdat"
	off := 0
	for off+8 <= len(buf) {
		size32 := binary.BigEndian.Uint32(buf[off : off+4])
		kind := string(buf[off+4 : off+8])
		var boxSize int
		var headerSize int
		switch size32 {
		case 1:
			if off+16 > len(buf) {
				return nil, fmt.Errorf("muxl: truncated largesize header for box %q at %d", kind, off)
			}
			boxSize = int(binary.BigEndian.Uint64(buf[off+8 : off+16]))
			headerSize = 16
		case 0:
			boxSize = len(buf) - off
			headerSize = 8
		default:
			boxSize = int(size32)
			headerSize = 8
		}
		if off+boxSize > len(buf) {
			return nil, fmt.Errorf("muxl: box %q at %d extends past asset (size %d, remaining %d)", kind, off, boxSize, len(buf)-off)
		}
		if kind == mdatBoxType {
			return buf[off+headerSize : off+boxSize], nil
		}
		off += boxSize
	}
	return nil, fmt.Errorf("muxl: no mdat envelope found in signed flat MP4 (asset is %d bytes)", len(buf))
}
