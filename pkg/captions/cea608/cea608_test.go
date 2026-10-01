package cea608

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// odd adds odd parity to a 7-bit CEA-608 byte, as the wire carries it.
func odd(b byte) byte {
	n := 0
	for i := range 7 {
		if b&(1<<i) != 0 {
			n++
		}
	}
	if n%2 == 0 {
		return b | 0x80
	}
	return b
}

// field1 builds a cc_data triplet for CEA-608 field 1.
func field1(b1, b2 byte) []byte { return []byte{0xfc, odd(b1), odd(b2)} }

// field2 builds a cc_data triplet for CEA-608 field 2.
func field2(b1, b2 byte) []byte { return []byte{0xfd, odd(b1), odd(b2)} }

// ctrl is a control code pair, sent twice like real encoders do.
func ctrl(f func(b1, b2 byte) []byte, b1, b2 byte) []byte {
	return append(f(b1, b2), f(b1, b2)...)
}

// text encodes ASCII text as CEA-608 character pairs.
func text(f func(b1, b2 byte) []byte, s string) []byte {
	var out []byte
	b := []byte(s)
	for i := 0; i < len(b); i += 2 {
		b2 := byte(0)
		if i+1 < len(b) {
			b2 = b[i+1]
		}
		out = append(out, f(b[i], b2)...)
	}
	return out
}

func cat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func texts(events []Event) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e.Text)
	}
	return out
}

func TestPopOnCaption(t *testing.T) {
	d := NewDecoder()
	// RCL, PAC row 14 col 0, text, PAC row 15, text: nothing shows yet.
	ev := d.Decode(cat(
		ctrl(field1, 0x14, 0x20),
		ctrl(field1, 0x14, 0x50), // row 14
		text(field1, "HELLO"),
		ctrl(field1, 0x14, 0x70), // row 15
		text(field1, "WORLD"),
	), 1*time.Second)
	require.Empty(t, ev, "pop-on text is loaded off screen")

	ev = d.Decode(ctrl(field1, 0x14, 0x2f), 2*time.Second) // EOC
	require.Equal(t, []Event{{Channel: CC1, Text: "HELLO\nWORLD", Time: 2 * time.Second}}, ev)

	// Load the next caption while the first stays on screen, then flip.
	ev = d.Decode(cat(ctrl(field1, 0x14, 0x20), ctrl(field1, 0x14, 0x70), text(field1, "NEXT")), 3*time.Second)
	require.Empty(t, ev)
	ev = d.Decode(ctrl(field1, 0x14, 0x2f), 4*time.Second)
	require.Equal(t, []string{"NEXT"}, texts(ev))

	// EDM clears the screen.
	ev = d.Decode(ctrl(field1, 0x14, 0x2c), 5*time.Second)
	require.Equal(t, []Event{{Channel: CC1, Text: "", Time: 5 * time.Second}}, ev)
	require.Empty(t, d.Decode(ctrl(field1, 0x14, 0x2c), 6*time.Second), "clearing an empty display is not a change")
}

func TestEndOfCaptionPreservesMemories(t *testing.T) {
	d := NewDecoder()
	load := func(s string, at time.Duration) {
		d.Decode(cat(ctrl(field1, 0x14, 0x2e), ctrl(field1, 0x14, 0x70), text(field1, s)), at)
	}
	load("FIRST", time.Second)
	require.Equal(t, []string{"FIRST"}, texts(d.Decode(ctrl(field1, 0x14, 0x2f), 2*time.Second)))
	load("SECOND", 3*time.Second)
	require.Equal(t, []string{"SECOND"}, texts(d.Decode(ctrl(field1, 0x14, 0x2f), 4*time.Second)))
	require.Equal(t, []string{"FIRST"}, texts(d.Decode(ctrl(field1, 0x14, 0x2f), 5*time.Second)), "EOC swaps without erasing")
	require.Empty(t, d.Decode(ctrl(field1, 0x14, 0x2e), 6*time.Second), "ENM does not change the display")
	require.Equal(t, []string{""}, texts(d.Decode(ctrl(field1, 0x14, 0x2f), 7*time.Second)), "ENM clears the memory later swapped on screen")
}

func TestRollUpScrollsAndCarriageReturns(t *testing.T) {
	d := NewDecoder()
	ev := d.Decode(cat(
		ctrl(field1, 0x14, 0x26), // RU3
		ctrl(field1, 0x14, 0x70), // PAC row 15
		text(field1, "ONE"),
	), time.Second)
	require.Equal(t, []string{"ONE"}, texts(ev), "roll-up text paints directly")

	ev = d.Decode(cat(ctrl(field1, 0x14, 0x2d), text(field1, "TWO")), 2*time.Second)
	require.Equal(t, []string{"ONE\nTWO"}, texts(ev))

	ev = d.Decode(cat(ctrl(field1, 0x14, 0x2d), text(field1, "THREE")), 3*time.Second)
	require.Equal(t, []string{"ONE\nTWO\nTHREE"}, texts(ev))

	ev = d.Decode(cat(ctrl(field1, 0x14, 0x2d), text(field1, "FOUR")), 4*time.Second)
	require.Equal(t, []string{"TWO\nTHREE\nFOUR"}, texts(ev), "a 3-row roll-up drops the top row")

	// Moving the base row keeps the rows; the PAC puts the cursor at column
	// 0 of the new base row, so the next character overwrites there.
	ev = d.Decode(cat(ctrl(field1, 0x14, 0x50), text(field1, "!")), 5*time.Second) // PAC row 14
	require.Equal(t, []string{"TWO\nTHREE\n!OUR"}, texts(ev))
}

func TestPaintOnBackspaceAndDeleteToEndOfRow(t *testing.T) {
	d := NewDecoder()
	ev := d.Decode(cat(
		ctrl(field1, 0x14, 0x29), // RDC
		ctrl(field1, 0x14, 0x70),
		text(field1, "ABCD"),
	), time.Second)
	require.Equal(t, []string{"ABCD"}, texts(ev))
	ev = d.Decode(cat(ctrl(field1, 0x14, 0x21), ctrl(field1, 0x14, 0x21), text(field1, "X")), 2*time.Second) // BS BS X
	require.Equal(t, []string{"ABX"}, texts(ev))
	ev = d.Decode(cat(ctrl(field1, 0x14, 0x70), text(field1, "Q"), ctrl(field1, 0x14, 0x24)), 3*time.Second) // PAC, Q, DER
	require.Equal(t, []string{"Q"}, texts(ev))
}

func TestSpecialAndExtendedCharacters(t *testing.T) {
	d := NewDecoder()
	ev := d.Decode(cat(
		ctrl(field1, 0x14, 0x29),
		ctrl(field1, 0x14, 0x70),
		text(field1, "e"),
		ctrl(field1, 0x12, 0x21), // extended É replaces the e just sent
		field1(0x11, 0x37),       // special: ♪
		text(field1, "n"),
		field1(0x11, 0x20),   // mid-row code renders as a space
		text(field1, "\x7e"), // basic ñ
	), time.Second)
	require.Equal(t, []string{"É♪n ñ"}, texts(ev))
}

func TestChannelsAndXDS(t *testing.T) {
	d := NewDecoder()
	ev := d.Decode(cat(
		// CC2 (field 1, data channel 2) roll-up.
		ctrl(field1, 0x1c, 0x25),
		ctrl(field1, 0x1c, 0x70),
		text(field1, "TWO"),
		// CC3 (field 2) with an XDS packet interleaved: XDS bytes must not
		// render, and the control code returns to captions.
		ctrl(field2, 0x14, 0x25),
		ctrl(field2, 0x14, 0x70),
		field2(0x01, 0x03), // XDS: current class, program name
		field2('X', 'D'),
		field2('S', ' '),
		field2(0x0f, 0x10), // XDS end
		text(field2, "THREE"),
		// CC4
		ctrl(field2, 0x1c, 0x25),
		ctrl(field2, 0x1c, 0x70),
		text(field2, "FOUR"),
		// XDS can also be cut off by a caption control code.
		field2(0x05, 0x01),
		field2('J', 'U'),
		ctrl(field2, 0x14, 0x2d), // CR
		text(field2, "!"),
	), time.Second)
	byChan := map[Channel]string{}
	for _, e := range ev {
		byChan[e.Channel] = e.Text
	}
	require.Equal(t, map[Channel]string{CC2: "TWO", CC3: "THREE\n!", CC4: "FOUR"}, byChan)
}

func TestNullPairsAndInvalidTripletsIgnored(t *testing.T) {
	d := NewDecoder()
	ev := d.Decode(cat(
		[]byte{0xf8, 0x80, 0x80}, // cc_valid clear
		field1(0x80, 0x80),       // null pad
		ctrl(field1, 0x14, 0x29),
		ctrl(field1, 0x14, 0x70),
		text(field1, "OK"),
		[]byte{0xfa, 0x00, 0x00}, // cc_type 2 continuation with nothing open
	), time.Second)
	require.Equal(t, []string{"OK"}, texts(ev))
}

// dtvccPacket wraps service-1 bytes in a DTVCC packet as cc_type 3 + 2
// triplets.
func dtvccPacket(seq byte, svc []byte) []byte {
	block := append([]byte{1<<5 | byte(len(svc))}, svc...)
	pkt := append([]byte{0}, block...) // header placeholder
	if len(pkt)%2 == 1 {
		pkt = append(pkt, 0)
	}
	pkt[0] = seq<<6 | byte(len(pkt)/2)
	var out []byte
	for i := 0; i < len(pkt); i += 2 {
		typ := byte(0xfe)
		if i == 0 {
			typ = 0xff
		}
		out = append(out, typ, pkt[i], pkt[i+1])
	}
	return out
}

func TestDTVCCService1Windows(t *testing.T) {
	d := NewDecoder()
	define := []byte{0x98, 0x20 | 0x3, 0x00, 0x00, 0x01, 0x1f, 0x00} // DF0: visible, 2 rows, 32 cols
	ev := d.Decode(cat(
		dtvccPacket(0, cat(define, []byte("Hello"))),
		dtvccPacket(1, []byte{0x0d}), // CR
		dtvccPacket(2, []byte("World")),
	), time.Second)
	require.Equal(t, []string{"Hello\nWorld"}, texts(ev), "complete packets decode on arrival; one Decode reports one display state")

	ev = d.Decode(dtvccPacket(3, []byte{0x8a, 0x01}), 2*time.Second) // HDW window 0
	require.Equal(t, []string{""}, texts(ev))
	ev = d.Decode(dtvccPacket(0, []byte{0x89, 0x01}), 2*time.Second) // DSW window 0
	require.Equal(t, []string{"Hello\nWorld"}, texts(ev))

	ev = d.Decode(dtvccPacket(1, []byte{0x0d, 0x10, 0x25, 0xe9}), 3*time.Second) // CR scrolls the full window, G2 ellipsis, G1 é
	require.Equal(t, []string{"World\n…é"}, texts(ev))
	require.Equal(t, Service1, ev[0].Channel)
	ev = d.Decode(dtvccPacket(2, []byte{0x8f}), 4*time.Second) // RST
	require.Equal(t, []string{""}, texts(ev))
}

func TestIncompleteDTVCCPacketsLeaveDisplayUnchanged(t *testing.T) {
	d := NewDecoder()
	require.Equal(t, []string{"shown"}, texts(d.Decode(dtvccPacket(0, []byte("shown")), time.Second)))
	pkt := dtvccPacket(1, []byte("tail"))
	pkt[1] += 2 // the header claims two more pairs than will ever arrive
	require.Empty(t, d.Decode(pkt, 2*time.Second))
	require.Empty(t, d.Decode(dtvccPacket(2, []byte{0}), 3*time.Second), "a new packet discards the incomplete one")
	require.Equal(t, []string{"shown!"}, texts(d.Decode(dtvccPacket(3, []byte("!")), 4*time.Second)))
}

func TestExtractCCDataSEIFraming(t *testing.T) {
	d := NewDecoder()
	cc := cat(ctrl(field1, 0x14, 0x29), ctrl(field1, 0x14, 0x70), text(field1, "SEI"))
	nal := captionSEINAL(cc)
	// A 300-byte NAL's length begins 00 00 01, but is not an Annex B
	// start code. The following SEI must still be extracted.
	slice := make([]byte, 300)
	slice[0] = 0x65
	sample := cat(lenPrefixed(slice), lenPrefixed(nal))
	ev := d.Decode(ExtractCCData(sample), time.Second)
	require.Equal(t, []string{"SEI"}, texts(ev))

	require.Nil(t, ExtractCCData(lenPrefixed(slice)), "no SEI, no captions")
}

func TestExtractCCDataUnescapesEmulationPrevention(t *testing.T) {
	require.Equal(t, []byte{0, 0, 1, 0, 0, 0, 0, 4}, unescapeRBSP([]byte{0, 0, 3, 1, 0, 0, 3, 0, 0, 3, 4}))

	// A 00 00 03 run inside the caption payload is an escape whatever
	// follows it; the extractor strips it before parsing the triplets.
	cc := cat([]byte{0xfc, 0x00, 0x00}, ctrl(field1, 0x14, 0x29), ctrl(field1, 0x14, 0x70), text(field1, "EP"))
	nal := captionSEINAL(cc)
	i := strings.Index(string(nal), "\x00\x00\xfc")
	require.Positive(t, i)
	escaped := cat(nal[:i+2], []byte{0x03}, nal[i+2:])
	d := NewDecoder()
	require.Equal(t, []string{"EP"}, texts(d.Decode(ExtractCCData(lenPrefixed(escaped)), time.Second)))
}

func lenPrefixed(nal []byte) []byte {
	return append([]byte{byte(len(nal) >> 24), byte(len(nal) >> 16), byte(len(nal) >> 8), byte(len(nal))}, nal...)
}

// captionSEINAL builds a closed-caption SEI NAL (payload type 4, ATSC
// GA94 cc_data) around cc triplets, with emulation prevention applied.
func captionSEINAL(cc []byte) []byte {
	count := len(cc) / 3
	t35 := []byte{0xb5, 0x00, 0x31, 'G', 'A', '9', '4', 0x03, 0x40 | byte(count), 0xff}
	t35 = append(t35, cc...)
	t35 = append(t35, 0xff)
	var payload []byte
	payload = append(payload, 0x04)
	for n := len(t35); n >= 255; n -= 255 {
		payload = append(payload, 0xff)
	}
	payload = append(payload, byte(len(t35)%255))
	payload = append(payload, t35...)
	payload = append(payload, 0x80)
	return append([]byte{0x06}, escapeRBSP(payload)...)
}

func escapeRBSP(b []byte) []byte {
	var out []byte
	zeros := 0
	for _, c := range b {
		if zeros >= 2 && c <= 3 {
			out = append(out, 3)
			zeros = 0
		}
		out = append(out, c)
		if c == 0 {
			zeros++
		} else {
			zeros = 0
		}
	}
	return out
}
