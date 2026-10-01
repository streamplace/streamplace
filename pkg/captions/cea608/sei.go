package cea608

import "encoding/binary"

// H264 NAL unit types that carry closed captions.
const (
	nalSEI = 6
)

// seiUserDataRegistered is the SEI payload type for
// user_data_registered_itu_t_t35, the ATSC A/53 carrier of cc_data.
const seiUserDataRegistered = 4

// ExtractCCData returns the cc_data triplets (cc_count * 3 bytes: a marker
// byte with cc_valid and cc_type, then two data bytes) carried in the closed
// caption SEI messages of one H264 access unit, in order. The sample may be
// length-prefixed (AVC, 4-byte sizes, as in MP4) or Annex B byte-stream.
// Samples without captions return nil.
func ExtractCCData(sample []byte) []byte {
	var out []byte
	forEachNAL(sample, func(nal []byte) {
		if len(nal) < 2 || nal[0]&0x1f != nalSEI {
			return
		}
		out = append(out, ccDataFromSEI(unescapeRBSP(nal[1:]))...)
	})
	return out
}

// forEachNAL calls fn with each NAL unit of an access unit. A sample whose
// first four bytes are a plausible NAL length is read as length-prefixed;
// otherwise it is scanned for Annex B start codes.
func forEachNAL(sample []byte, fn func(nal []byte)) {
	if isAnnexB(sample) {
		forEachAnnexBNAL(sample, fn)
		return
	}
	for len(sample) >= 4 {
		n := int(binary.BigEndian.Uint32(sample[:4]))
		sample = sample[4:]
		if n <= 0 || n > len(sample) {
			return
		}
		fn(sample[:n])
		sample = sample[n:]
	}
}

func isAnnexB(b []byte) bool {
	return len(b) >= 3 && b[0] == 0 && b[1] == 0 && (b[2] == 1 || (b[2] == 0 && len(b) >= 4 && b[3] == 1))
}

func forEachAnnexBNAL(b []byte, fn func(nal []byte)) {
	start := -1
	i := 0
	for i+2 < len(b) {
		if b[i] == 0 && b[i+1] == 0 && b[i+2] == 1 {
			if start >= 0 {
				end := i
				if end > start && b[end-1] == 0 {
					end--
				}
				if end > start {
					fn(b[start:end])
				}
			}
			i += 3
			start = i
			continue
		}
		i++
	}
	if start >= 0 && start < len(b) {
		fn(b[start:])
	}
}

// unescapeRBSP removes emulation prevention bytes (00 00 03 → 00 00).
func unescapeRBSP(b []byte) []byte {
	var out []byte
	zeros := 0
	for i := 0; i < len(b); i++ {
		c := b[i]
		if zeros >= 2 && c == 3 {
			if out == nil {
				out = append(make([]byte, 0, len(b)), b[:i]...)
			}
			zeros = 0
			continue
		}
		if out != nil {
			out = append(out, c)
		}
		if c == 0 {
			zeros++
		} else {
			zeros = 0
		}
	}
	if out == nil {
		return b
	}
	return out
}

// ccDataFromSEI walks the SEI messages of an RBSP and returns the cc_data
// bytes of every ATSC A/53 closed-caption message.
func ccDataFromSEI(rbsp []byte) []byte {
	var out []byte
	i := 0
	for i < len(rbsp) {
		if rbsp[i] == 0x80 { // rbsp_trailing_bits
			break
		}
		ptype := 0
		for i < len(rbsp) && rbsp[i] == 0xff {
			ptype += 255
			i++
		}
		if i >= len(rbsp) {
			break
		}
		ptype += int(rbsp[i])
		i++
		psize := 0
		for i < len(rbsp) && rbsp[i] == 0xff {
			psize += 255
			i++
		}
		if i >= len(rbsp) {
			break
		}
		psize += int(rbsp[i])
		i++
		if psize > len(rbsp)-i {
			psize = len(rbsp) - i
		}
		if ptype == seiUserDataRegistered {
			out = append(out, ccDataFromT35(rbsp[i:i+psize])...)
		}
		i += psize
	}
	return out
}

// ccDataFromT35 parses one user_data_registered_itu_t_t35 payload and
// returns its cc_data triplets when it is an ATSC A/53 caption payload.
func ccDataFromT35(p []byte) []byte {
	// itu_t_t35_country_code (0xB5 United States), provider code 0x0031
	// (ATSC), user_identifier "GA94", user_data_type_code 3 (cc_data).
	if len(p) < 9 || p[0] != 0xb5 || p[1] != 0x00 || p[2] != 0x31 {
		return nil
	}
	if string(p[3:7]) != "GA94" || p[7] != 0x03 {
		return nil
	}
	flags := p[8]
	if flags&0x40 == 0 { // process_cc_data_flag
		return nil
	}
	count := int(flags & 0x1f)
	// p[9] is em_data; triplets follow.
	start := 10
	n := count * 3
	if start+n > len(p) {
		n = (len(p) - start) / 3 * 3
		if n <= 0 {
			return nil
		}
	}
	return p[start : start+n]
}
