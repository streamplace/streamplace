package vod

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/bits"
)

// tfhd and trun flag bits (ISO/IEC 14496-12).
const (
	tfhdBaseDataOffset  = 0x000001
	tfhdSampleDescIndex = 0x000002
	tfhdDefaultDuration = 0x000008
	tfhdDefaultSize     = 0x000010

	trunDataOffset       = 0x000001
	trunFirstSampleFlags = 0x000004
	trunSampleDuration   = 0x000100
	trunSampleSize       = 0x000200
	trunPerSampleFields  = 0x000f00
)

var errSplit = errors.New("split first sample")

// nextBox splits the first box off b, returning it whole and what follows.
// 64-bit and to-end-of-file sizes don't occur in MUXL segments.
func nextBox(b []byte) (box, tail []byte, err error) {
	if len(b) < 8 {
		return nil, nil, fmt.Errorf("%w: truncated box header", errSplit)
	}
	size := uint64(binary.BigEndian.Uint32(b))
	if size < 8 || size > uint64(len(b)) {
		return nil, nil, fmt.Errorf("%w: bad size %d for %q box", errSplit, size, b[4:8])
	}
	return b[:size], b[size:], nil
}

// childBoxes maps a container's children by type, refusing a second `single`.
func childBoxes(payload []byte, single string) (map[string][]byte, error) {
	out := map[string][]byte{}
	for len(payload) > 0 {
		box, tail, err := nextBox(payload)
		if err != nil {
			return nil, err
		}
		typ := string(box[4:8])
		if _, dup := out[typ]; dup && typ == single {
			return nil, fmt.Errorf("%w: more than one %s", errSplit, typ)
		}
		out[typ] = box
		payload = tail
	}
	return out, nil
}

func u32(b []byte, off int) (uint32, error) {
	if off+4 > len(b) {
		return 0, fmt.Errorf("%w: truncated box", errSplit)
	}
	return binary.BigEndian.Uint32(b[off:]), nil
}

// fragmentHead is what both halves of a split fragment share.
type fragmentHead struct {
	mfhd, tfhd  []byte // whole boxes, copied through
	tfdtV1      bool
	trunVersion byte
}

// build assembles a moof+mdat for one run of samples.
func (h fragmentHead) build(decodeTime uint64, trunFlags uint32, firstSampleFlags []byte, count uint32, entries, data []byte) []byte {
	var tfdt []byte
	if h.tfdtV1 || decodeTime > math.MaxUint32 {
		tfdt = binary.BigEndian.AppendUint64([]byte{1, 0, 0, 0}, decodeTime)
	} else {
		tfdt = binary.BigEndian.AppendUint32(make([]byte, 4, 8), uint32(decodeTime))
	}
	trunLen := 8 + 12 + len(firstSampleFlags) + len(entries)
	trafLen := 8 + len(h.tfhd) + 8 + len(tfdt) + trunLen
	moofLen := 8 + len(h.mfhd) + trafLen

	header := func(out []byte, size int, typ string) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(size)), typ...)
	}
	out := make([]byte, 0, moofLen+8+len(data))
	out = header(out, moofLen, "moof")
	out = append(out, h.mfhd...)
	out = header(out, trafLen, "traf")
	out = append(out, h.tfhd...)
	out = header(out, 8+len(tfdt), "tfdt")
	out = append(out, tfdt...)
	out = header(out, trunLen, "trun")
	out = binary.BigEndian.AppendUint32(out, uint32(h.trunVersion)<<24|trunFlags)
	out = binary.BigEndian.AppendUint32(out, count)
	// data_offset is from the start of the moof to the mdat payload.
	out = binary.BigEndian.AppendUint32(out, uint32(moofLen+8))
	out = append(out, firstSampleFlags...)
	out = append(out, entries...)
	out = header(out, 8+len(data), "mdat")
	return append(out, data...)
}

// SplitFirstSample extracts sample 0 from a single-track segment. MUXL GOPs
// may carry several moof+mdat pairs; later pairs remain in rest.
func SplitFirstSample(segment []byte) (key, rest []byte, err error) {
	firstEnd := 0
	awaitingData := false
	for b := segment; len(b) > 0; {
		box, tail, err := nextBox(b)
		if err != nil {
			return nil, nil, err
		}
		switch string(box[4:8]) {
		case "uuid":
			if firstEnd != 0 || awaitingData {
				return nil, nil, fmt.Errorf("%w: uuid after fragment start", errSplit)
			}
		case "moof":
			if awaitingData {
				return nil, nil, fmt.Errorf("%w: moof without mdat", errSplit)
			}
			awaitingData = true
		case "mdat":
			if !awaitingData {
				return nil, nil, fmt.Errorf("%w: mdat without moof", errSplit)
			}
			awaitingData = false
			if firstEnd == 0 {
				firstEnd = len(segment) - len(tail)
			}
		default:
			return nil, nil, fmt.Errorf("%w: unexpected box %q", errSplit, box[4:8])
		}
		b = tail
	}
	if firstEnd == 0 || awaitingData {
		return nil, nil, fmt.Errorf("%w: incomplete moof+mdat", errSplit)
	}
	key, rest, err = splitFirstFragment(segment[:firstEnd])
	if err != nil {
		return nil, nil, err
	}
	if firstEnd < len(segment) {
		if rest == nil {
			rest = segment[firstEnd:]
		} else {
			rest = append(rest, segment[firstEnd:]...)
		}
	}
	return key, rest, nil
}

func splitFirstFragment(segment []byte) (key, rest []byte, err error) {
	var moof, mdat []byte
	moofStart, mdatStart := 0, 0
	for b := segment; len(b) > 0; {
		box, tail, err := nextBox(b)
		if err != nil {
			return nil, nil, err
		}
		switch string(box[4:8]) {
		case "moof":
			if moof != nil {
				return nil, nil, fmt.Errorf("%w: more than one moof", errSplit)
			}
			moof, moofStart = box, len(segment)-len(b)
		case "mdat":
			if moof != nil && mdat == nil {
				mdat, mdatStart = box, len(segment)-len(b)
			}
		}
		b = tail
	}
	if moof == nil || mdat == nil {
		return nil, nil, fmt.Errorf("%w: no moof+mdat", errSplit)
	}
	moofBoxes, err := childBoxes(moof[8:], "traf")
	if err != nil {
		return nil, nil, err
	}
	traf := moofBoxes["traf"]
	if traf == nil {
		return nil, nil, fmt.Errorf("%w: no traf", errSplit)
	}
	trafBoxes, err := childBoxes(traf[8:], "trun")
	if err != nil {
		return nil, nil, err
	}
	tfhd, tfdt, trun := trafBoxes["tfhd"], trafBoxes["tfdt"], trafBoxes["trun"]
	if len(tfhd) < 16 || len(tfdt) < 16 || len(trun) < 16 {
		return nil, nil, fmt.Errorf("%w: traf lacks tfhd, tfdt or trun", errSplit)
	}

	// tfhd: the defaults for samples the trun doesn't spell out.
	tfhdFlags := binary.BigEndian.Uint32(tfhd[8:]) & 0xffffff
	if tfhdFlags&tfhdBaseDataOffset != 0 {
		return nil, nil, fmt.Errorf("%w: tfhd base-data-offset is not supported", errSplit)
	}
	off := 16
	if tfhdFlags&tfhdSampleDescIndex != 0 {
		off += 4
	}
	var defaultDuration, defaultSize uint32
	if tfhdFlags&tfhdDefaultDuration != 0 {
		if defaultDuration, err = u32(tfhd, off); err != nil {
			return nil, nil, err
		}
		off += 4
	}
	if tfhdFlags&tfhdDefaultSize != 0 {
		if defaultSize, err = u32(tfhd, off); err != nil {
			return nil, nil, err
		}
		off += 4
	}
	if tfhdFlags&0x000020 != 0 {
		if _, err := u32(tfhd, off); err != nil {
			return nil, nil, err
		}
	}

	head := fragmentHead{mfhd: moofBoxes["mfhd"], tfhd: tfhd, tfdtV1: tfdt[8] == 1, trunVersion: trun[8]}
	var decodeTime uint64
	switch {
	case head.tfdtV1 && len(tfdt) >= 20:
		decodeTime = binary.BigEndian.Uint64(tfdt[12:])
	case tfdt[8] == 0:
		decodeTime = uint64(binary.BigEndian.Uint32(tfdt[12:]))
	default:
		return nil, nil, fmt.Errorf("%w: bad tfdt", errSplit)
	}

	// trun: where the samples are and how long and large each is.
	trunFlags := binary.BigEndian.Uint32(trun[8:]) & 0xffffff
	count := binary.BigEndian.Uint32(trun[12:])
	if count == 0 {
		return nil, nil, fmt.Errorf("%w: no samples", errSplit)
	}
	off = 16
	dataStart := mdatStart + 8
	if trunFlags&trunDataOffset != 0 {
		v, err := u32(trun, off)
		if err != nil {
			return nil, nil, err
		}
		dataStart = moofStart + int(int32(v))
		off += 4
	}
	var firstSampleFlags []byte
	if trunFlags&trunFirstSampleFlags != 0 {
		if _, err := u32(trun, off); err != nil {
			return nil, nil, err
		}
		firstSampleFlags = trun[off : off+4]
		off += 4
	}
	entrySize := 4 * bits.OnesCount32(trunFlags&trunPerSampleFields)
	entries := trun[off:]
	if uint64(len(entries)) != uint64(count)*uint64(entrySize) {
		return nil, nil, fmt.Errorf("%w: trun holds %d bytes for %d samples", errSplit, len(entries), count)
	}

	duration, size, total := defaultDuration, uint64(defaultSize), uint64(defaultSize)*uint64(count)
	sizeAt := 0
	if trunFlags&trunSampleDuration != 0 {
		duration = binary.BigEndian.Uint32(entries)
		sizeAt = 4
	} else if tfhdFlags&tfhdDefaultDuration == 0 {
		return nil, nil, fmt.Errorf("%w: sample duration only in the init segment", errSplit)
	}
	if trunFlags&trunSampleSize != 0 {
		size, total = uint64(binary.BigEndian.Uint32(entries[sizeAt:])), 0
		for i := 0; i < len(entries); i += entrySize {
			total += uint64(binary.BigEndian.Uint32(entries[i+sizeAt:]))
		}
	} else if tfhdFlags&tfhdDefaultSize == 0 {
		return nil, nil, fmt.Errorf("%w: sample size only in the init segment", errSplit)
	}
	if dataStart < mdatStart+8 || uint64(dataStart)+total > uint64(mdatStart+len(mdat)) {
		return nil, nil, fmt.Errorf("%w: samples fall outside the mdat", errSplit)
	}
	data := segment[dataStart : dataStart+int(total)]

	if count > 1 && decodeTime > math.MaxUint64-uint64(duration) {
		return nil, nil, fmt.Errorf("%w: decode time overflow", errSplit)
	}
	trunFlags |= trunDataOffset
	key = head.build(decodeTime, trunFlags, firstSampleFlags, 1, entries[:entrySize], data[:size])
	if count > 1 {
		// first-sample-flags described the keyframe; the rest use their own.
		rest = head.build(decodeTime+uint64(duration), trunFlags&^trunFirstSampleFlags, nil, count-1, entries[entrySize:], data[size:])
	}
	return key, rest, nil
}
