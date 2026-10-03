// Package fmp4 reads the samples of a fragmented MP4 track: the
// [moof][mdat] runs of a canonical MUXL track, or a presentation fMP4 with
// explicit tfhd bases (or default-base-is-moof) and trun data offsets.
// It reads each sample's bytes with its decode and presentation times.
package fmp4

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/bits"
)

// Sample is one media sample of a track.
type Sample struct {
	Data []byte
	// DTS and PTS are in the track's timescale.
	DTS uint64
	PTS uint64
	// Sync is set for sync samples (keyframes).
	Sync bool
}

// Fragment is one movie fragment's worth of a track.
type Fragment struct {
	TrackID uint32
	// BaseDecodeTime is the fragment's tfdt.
	BaseDecodeTime uint64
	Samples        []Sample
}

var errTruncated = errors.New("fmp4: truncated box")

// Fragments parses every moof/mdat pair in b. Boxes before a moof (uuid,
// styp, ftyp, moov) are skipped, so a canonical track's [uuid][uuid][moof]
// [mdat] layout and a flat/fragmented presentation both work. When a moof
// holds several tracks, each traf yields its own Fragment.
func Fragments(b []byte) ([]Fragment, error) {
	var out []Fragment
	off := 0
	for off+8 <= len(b) {
		size, typ, hdr, err := boxHeader(b, off)
		if err != nil {
			return out, err
		}
		if typ == "moof" {
			moof := b[off : off+size]
			dataOff := off + size
			dataSize, dataType, dataHdr, err := boxHeader(b, dataOff)
			if err != nil {
				return out, err
			}
			if dataType != "mdat" {
				return out, fmt.Errorf("fmp4: moof must be followed by mdat")
			}
			frags, err := parseMoof(b, off, moof[hdr:], dataOff+dataHdr, dataOff+dataSize)
			if err != nil {
				return out, err
			}
			out = append(out, frags...)
		}
		off += size
	}
	return out, nil
}

// boxHeader reads a box header at off: its total size, type, and header
// length (8, or 16 for a 64-bit size).
func boxHeader(b []byte, off int) (size int, typ string, hdr int, err error) {
	if off+8 > len(b) {
		return 0, "", 0, errTruncated
	}
	size = int(binary.BigEndian.Uint32(b[off : off+4]))
	typ = string(b[off+4 : off+8])
	hdr = 8
	switch size {
	case 0:
		size = len(b) - off
	case 1:
		if off+16 > len(b) {
			return 0, "", 0, errTruncated
		}
		large := binary.BigEndian.Uint64(b[off+8 : off+16])
		if large > uint64(len(b)-off) {
			return 0, "", 0, errTruncated
		}
		size = int(large)
		hdr = 16
	}
	if size < hdr || off+size > len(b) {
		return 0, "", 0, fmt.Errorf("fmp4: bad %q box size %d at %d", typ, size, off)
	}
	return size, typ, hdr, nil
}

// children walks the boxes inside body, calling fn with each.
func children(body []byte, fn func(typ string, box, payload []byte) error) error {
	off := 0
	for off+8 <= len(body) {
		size, typ, hdr, err := boxHeader(body, off)
		if err != nil {
			return err
		}
		if err := fn(typ, body[off:off+size], body[off+hdr:off+size]); err != nil {
			return err
		}
		off += size
	}
	return nil
}

const (
	tfhdBaseDataOffset         = 0x000001
	tfhdSampleDescriptionIndex = 0x000002
	tfhdDefaultSampleDuration  = 0x000008
	tfhdDefaultSampleSize      = 0x000010
	tfhdDefaultSampleFlags     = 0x000020
	tfhdDefaultBaseIsMoof      = 0x020000

	trunDataOffset       = 0x000001
	trunFirstSampleFlags = 0x000004
	trunSampleDuration   = 0x000100
	trunSampleSize       = 0x000200
	trunSampleFlags      = 0x000400
	trunSampleCTO        = 0x000800
)

type traf struct {
	trackID     uint32
	baseOffset  int // absolute offset into the file of the data base
	defDuration uint32
	defSize     uint32
	defFlags    uint32
	tfdt        uint64
	hasTfdt     bool
	truns       [][]byte
}

// parseMoof returns a Fragment per traf. file is the whole buffer and
// moofOff the moof box's offset in it, for resolving data offsets.
func parseMoof(file []byte, moofOff int, body []byte, dataStart, dataEnd int) ([]Fragment, error) {
	var out []Fragment
	err := children(body, func(typ string, _, payload []byte) error {
		if typ != "traf" {
			return nil
		}
		t := traf{baseOffset: moofOff}
		err := children(payload, func(typ string, _, p []byte) error {
			switch typ {
			case "tfhd":
				return t.parseTfhd(p, moofOff, len(file), len(out) == 0)
			case "tfdt":
				if len(p) < 8 {
					return errTruncated
				}
				if p[0] == 1 {
					if len(p) < 12 {
						return errTruncated
					}
					t.tfdt = binary.BigEndian.Uint64(p[4:12])
				} else {
					t.tfdt = uint64(binary.BigEndian.Uint32(p[4:8]))
				}
				t.hasTfdt = true
			case "trun":
				t.truns = append(t.truns, p)
			}
			return nil
		})
		if err != nil {
			return err
		}
		frag, err := t.samples(file, dataStart, dataEnd)
		if err != nil {
			return err
		}
		out = append(out, frag)
		return nil
	})
	return out, err
}

func (t *traf) parseTfhd(p []byte, moofOff, fileSize int, first bool) error {
	if len(p) < 8 {
		return errTruncated
	}
	flags := binary.BigEndian.Uint32(p[0:4]) & 0xffffff
	t.trackID = binary.BigEndian.Uint32(p[4:8])
	i := 8
	need := func(n int) error {
		if i+n > len(p) {
			return errTruncated
		}
		return nil
	}
	if flags&tfhdBaseDataOffset != 0 {
		if err := need(8); err != nil {
			return err
		}
		offset := binary.BigEndian.Uint64(p[i : i+8])
		if offset > uint64(fileSize) {
			return fmt.Errorf("fmp4: tfhd base data offset outside the file")
		}
		t.baseOffset = int(offset)
		i += 8
	} else if flags&tfhdDefaultBaseIsMoof != 0 || first {
		t.baseOffset = moofOff
	} else {
		return fmt.Errorf("fmp4: unsupported implicit traf base")
	}
	if flags&tfhdSampleDescriptionIndex != 0 {
		if err := need(4); err != nil {
			return err
		}
		i += 4
	}
	if flags&tfhdDefaultSampleDuration != 0 {
		if err := need(4); err != nil {
			return err
		}
		t.defDuration = binary.BigEndian.Uint32(p[i : i+4])
		i += 4
	}
	if flags&tfhdDefaultSampleSize != 0 {
		if err := need(4); err != nil {
			return err
		}
		t.defSize = binary.BigEndian.Uint32(p[i : i+4])
		i += 4
	}
	if flags&tfhdDefaultSampleFlags != 0 {
		if err := need(4); err != nil {
			return err
		}
		t.defFlags = binary.BigEndian.Uint32(p[i : i+4])
	}
	return nil
}

func (t *traf) samples(file []byte, dataStart, dataEnd int) (Fragment, error) {
	frag := Fragment{TrackID: t.trackID, BaseDecodeTime: t.tfdt}
	dts := t.tfdt
	for _, p := range t.truns {
		if len(p) < 8 {
			return frag, errTruncated
		}
		version := p[0]
		flags := binary.BigEndian.Uint32(p[0:4]) & 0xffffff
		count := int(binary.BigEndian.Uint32(p[4:8]))
		i := 8
		dataOff := 0
		if flags&trunDataOffset != 0 {
			if i+4 > len(p) {
				return frag, errTruncated
			}
			dataOff = int(int32(binary.BigEndian.Uint32(p[i : i+4])))
			i += 4
		} else {
			return frag, fmt.Errorf("fmp4: trun missing data offset")
		}
		firstFlags := t.defFlags
		hasFirst := false
		if flags&trunFirstSampleFlags != 0 {
			if i+4 > len(p) {
				return frag, errTruncated
			}
			firstFlags = binary.BigEndian.Uint32(p[i : i+4])
			hasFirst = true
			i += 4
		}
		pos := t.baseOffset + dataOff
		if pos < dataStart || pos > dataEnd {
			return frag, fmt.Errorf("fmp4: trun data offset %d outside mdat", pos)
		}
		entrySize := 4 * bits.OnesCount32(flags&(trunSampleDuration|trunSampleSize|trunSampleFlags|trunSampleCTO))
		if entrySize > 0 && count > (len(p)-i)/entrySize {
			return frag, errTruncated
		}
		// Every supported sample consumes payload bytes. Default-only runs
		// must also fit before we allocate or iterate from an untrusted count.
		if count > dataEnd-pos || (flags&trunSampleSize == 0 && (t.defSize == 0 || uint64(count)*uint64(t.defSize) > uint64(dataEnd-pos))) {
			return frag, fmt.Errorf("fmp4: sample count exceeds mdat payload")
		}
		for s := range count {
			dur, size, sflags := t.defDuration, t.defSize, t.defFlags
			var cto int64
			if flags&trunSampleDuration != 0 {
				if i+4 > len(p) {
					return frag, errTruncated
				}
				dur = binary.BigEndian.Uint32(p[i : i+4])
				i += 4
			}
			if flags&trunSampleSize != 0 {
				if i+4 > len(p) {
					return frag, errTruncated
				}
				size = binary.BigEndian.Uint32(p[i : i+4])
				i += 4
			}
			if flags&trunSampleFlags != 0 {
				if i+4 > len(p) {
					return frag, errTruncated
				}
				sflags = binary.BigEndian.Uint32(p[i : i+4])
				i += 4
			} else if s == 0 && hasFirst {
				sflags = firstFlags
			}
			if flags&trunSampleCTO != 0 {
				if i+4 > len(p) {
					return frag, errTruncated
				}
				raw := binary.BigEndian.Uint32(p[i : i+4])
				if version == 0 {
					cto = int64(raw)
				} else {
					cto = int64(int32(raw))
				}
				i += 4
			}
			// Producers may explicitly encode a zero-duration terminal sample.
			if size == 0 || (flags&trunSampleDuration == 0 && dur == 0) {
				return frag, fmt.Errorf("fmp4: sample must have payload and an explicit or default duration")
			}
			end := pos + int(size)
			if end > dataEnd || end < pos {
				return frag, fmt.Errorf("fmp4: sample of %d bytes at %d runs past mdat", size, pos)
			}
			pts := int64(dts) + cto
			if pts < 0 {
				pts = 0
			}
			frag.Samples = append(frag.Samples, Sample{
				Data: file[pos:end],
				DTS:  dts,
				PTS:  uint64(pts),
				// sample_depends_on == 2 (I-frame) or !sample_is_non_sync_sample.
				Sync: sflags&0x10000 == 0,
			})
			pos = end
			dts += uint64(dur)
		}
	}
	return frag, nil
}

// TrackInfo is what the movie header says about a track.
type TrackInfo struct {
	ID        uint32
	Timescale uint32
	// Handler is the hdlr handler type: "vide", "soun", "text", "sbtl",
	// "subt".
	Handler string
}

// Tracks reads the trak entries of the moov box in b (an init segment or
// flat MP4). It returns nil when there is no moov.
func Tracks(b []byte) ([]TrackInfo, error) {
	var out []TrackInfo
	off := 0
	for off+8 <= len(b) {
		size, typ, hdr, err := boxHeader(b, off)
		if err != nil {
			return out, err
		}
		if typ == "moov" {
			err := children(b[off+hdr:off+size], func(typ string, _, trak []byte) error {
				if typ != "trak" {
					return nil
				}
				info, err := parseTrak(trak)
				if err != nil {
					return err
				}
				out = append(out, info)
				return nil
			})
			return out, err
		}
		off += size
	}
	return out, nil
}

func parseTrak(trak []byte) (TrackInfo, error) {
	var info TrackInfo
	err := children(trak, func(typ string, _, p []byte) error {
		switch typ {
		case "tkhd":
			if len(p) < 4 {
				return errTruncated
			}
			idOff := 12 // version 0: flags(4) creation(4) modification(4)
			if p[0] == 1 {
				idOff = 20
			}
			if len(p) < idOff+4 {
				return errTruncated
			}
			info.ID = binary.BigEndian.Uint32(p[idOff : idOff+4])
		case "mdia":
			return children(p, func(typ string, _, m []byte) error {
				switch typ {
				case "mdhd":
					tsOff := 12
					if len(m) > 0 && m[0] == 1 {
						tsOff = 20
					}
					if len(m) < tsOff+4 {
						return errTruncated
					}
					info.Timescale = binary.BigEndian.Uint32(m[tsOff : tsOff+4])
				case "hdlr":
					if len(m) < 12 {
						return errTruncated
					}
					info.Handler = string(m[8:12])
				}
				return nil
			})
		}
		return nil
	})
	return info, err
}
