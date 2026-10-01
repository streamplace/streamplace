// Package cea608 decodes the closed captions embedded in H264 video: CEA-608
// (CC1-CC4, pop-on, roll-up and paint-on) and the text of CEA-708 service 1,
// carried as cc_data in ATSC A/53 SEI messages.
//
// The decoders keep the caption display model and report the displayed text
// each time it changes; a caller turns those display states into timed cues.
package cea608

import (
	"strings"
	"time"
)

// Channel names a caption channel in the cc_data stream.
type Channel int

const (
	CC1      Channel = iota + 1 // CEA-608 field 1, data channel 1
	CC2                         // field 1, data channel 2
	CC3                         // field 2, data channel 1
	CC4                         // field 2, data channel 2
	Service1                    // CEA-708 service 1
)

// Event is a change of a channel's displayed text. Text is the full display
// after the change, rows joined by newlines; an empty Text means the display
// was cleared.
type Event struct {
	Channel Channel
	Text    string
	Time    time.Duration
}

// Decoder decodes the cc_data of one video stream across every channel.
type Decoder struct {
	fields [2]*field608
	dtv    *dtvcc
	last   map[Channel]string
}

func NewDecoder() *Decoder {
	return &Decoder{
		fields: [2]*field608{newField608(), newField608()},
		dtv:    newDTVCC(),
		last:   map[Channel]string{},
	}
}

// Decode feeds one access unit's cc_data triplets presented at the given
// time, and returns the display changes they caused.
func (d *Decoder) Decode(cc []byte, at time.Duration) []Event {
	for i := 0; i+2 < len(cc); i += 3 {
		valid := cc[i]&0x04 != 0
		ccType := cc[i] & 0x03
		b1, b2 := cc[i+1], cc[i+2]
		switch ccType {
		case 0, 1:
			if valid {
				d.fields[ccType].pair(b1&0x7f, b2&0x7f)
			}
		case 2, 3:
			d.dtv.triplet(valid, ccType == 3, b1, b2)
		}
	}
	return d.collect(at)
}

func (d *Decoder) collect(at time.Duration) []Event {
	var events []Event
	emit := func(ch Channel, text string) {
		if prev, ok := d.last[ch]; ok && prev == text {
			return
		}
		if text == "" && d.last[ch] == "" {
			return
		}
		d.last[ch] = text
		events = append(events, Event{Channel: ch, Text: text, Time: at})
	}
	for f, field := range d.fields {
		for c := 0; c < 2; c++ {
			emit(Channel(f*2+c+1), field.chans[c].displayedText())
		}
	}
	emit(Service1, d.dtv.displayedText())
	return events
}

// --- CEA-608 ---

const (
	rows608 = 15
	cols608 = 32
)

type mode608 int

const (
	modePopOn mode608 = iota
	modeRollUp
	modePaintOn
	modeText
)

type grid608 [rows608][cols608]rune

func (g *grid608) clear() {
	*g = grid608{}
}

func (g *grid608) text() string {
	var lines []string
	for r := 0; r < rows608; r++ {
		var sb strings.Builder
		for c := 0; c < cols608; c++ {
			ch := g[r][c]
			if ch == 0 {
				ch = ' '
			}
			sb.WriteRune(ch)
		}
		line := strings.TrimSpace(sb.String())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// channel608 is the display model of one data channel: the displayed and
// non-displayed memories, the caption mode, and the cursor.
type channel608 struct {
	mode      mode608
	displayed grid608
	hidden    grid608 // pop-on's non-displayed memory
	row, col  int
	rollDepth int // roll-up rows (2-4)
	baseRow   int // roll-up base row (1-based)
}

func newChannel608() *channel608 {
	return &channel608{mode: modePopOn, row: rows608, col: 0, rollDepth: 2, baseRow: rows608}
}

func (c *channel608) displayedText() string {
	return c.displayed.text()
}

// target is the memory characters are written into for the current mode.
func (c *channel608) target() *grid608 {
	if c.mode == modePopOn {
		return &c.hidden
	}
	return &c.displayed
}

func (c *channel608) writeRune(r rune) {
	if c.mode == modeText {
		return
	}
	if c.col >= cols608 {
		// The last column holds; CEA-608 overwrites it rather than wrapping.
		c.col = cols608 - 1
	}
	g := c.target()
	g[c.row-1][c.col] = r
	c.col++
}

func (c *channel608) backspace() {
	if c.mode == modeText {
		return
	}
	if c.col > 0 {
		c.col--
	}
	c.target()[c.row-1][c.col] = 0
}

func (c *channel608) deleteToEndOfRow() {
	g := c.target()
	for i := c.col; i < cols608; i++ {
		g[c.row-1][i] = 0
	}
}

func (c *channel608) carriageReturn() {
	if c.mode != modeRollUp {
		return
	}
	top := c.baseRow - c.rollDepth + 1
	if top < 1 {
		top = 1
	}
	for r := top; r < c.baseRow; r++ {
		c.displayed[r-1] = c.displayed[r]
	}
	c.displayed[c.baseRow-1] = [cols608]rune{}
	c.row = c.baseRow
	c.col = 0
}

// setRollUp enters roll-up mode with the given depth; the displayed rows
// outside the window are cleared, as the spec requires on a mode change.
func (c *channel608) setRollUp(depth int) {
	if c.mode != modeRollUp {
		c.displayed.clear()
		c.hidden.clear()
	}
	c.mode = modeRollUp
	c.rollDepth = depth
	if c.baseRow < depth {
		c.baseRow = depth
	}
	c.row = c.baseRow
	c.col = 0
}

// preamble moves the cursor; in roll-up mode it also moves the roll-up
// window to end at the new row.
func (c *channel608) preamble(row, indent int) {
	if c.mode == modeRollUp {
		if row != c.baseRow {
			old := c.baseRow
			top := old - c.rollDepth + 1
			if top < 1 {
				top = 1
			}
			// Move the window's rows to the new base.
			var moved grid608
			for i := 0; i < c.rollDepth; i++ {
				src := top + i
				dst := row - c.rollDepth + 1 + i
				if src >= 1 && src <= rows608 && dst >= 1 && dst <= rows608 {
					moved[dst-1] = c.displayed[src-1]
				}
			}
			c.displayed = moved
			c.baseRow = row
		}
		c.row = c.baseRow
	} else {
		c.row = row
	}
	c.col = indent
}

func (c *channel608) endOfCaption() {
	// Flip memories: the loaded caption becomes visible and the previously
	// displayed one becomes the next buffer to load into.
	c.mode = modePopOn
	c.displayed, c.hidden = c.hidden, c.displayed
}

// field608 decodes one field's byte pairs, routing them to the data channel
// selected by the latest control code.
type field608 struct {
	chans    [2]*channel608
	cur      int
	lastPair [2]byte
	inXDS    bool
}

func newField608() *field608 {
	return &field608{chans: [2]*channel608{newChannel608(), newChannel608()}, lastPair: [2]byte{0xff, 0xff}}
}

func (f *field608) pair(b1, b2 byte) {
	if b1 == 0 && b2 == 0 {
		return // null pad
	}
	if b1 < 0x10 {
		// XDS (field 2) packet start/continuation: 0x01-0x0E class, 0x0F end.
		f.inXDS = b1 != 0x0f
		f.lastPair = [2]byte{0xff, 0xff}
		return
	}
	if b1 < 0x20 {
		// Control codes are sent twice for robustness; act once.
		if f.lastPair == [2]byte{b1, b2} {
			f.lastPair = [2]byte{0xff, 0xff}
			return
		}
		f.lastPair = [2]byte{b1, b2}
		f.inXDS = false
		f.control(b1, b2)
		return
	}
	f.lastPair = [2]byte{0xff, 0xff}
	if f.inXDS {
		return
	}
	ch := f.chans[f.cur]
	if r := basicChar(b1); r != 0 {
		ch.writeRune(r)
	}
	if r := basicChar(b2); r != 0 {
		ch.writeRune(r)
	}
}

func (f *field608) control(b1, b2 byte) {
	// Bit 3 of the first byte selects data channel 2.
	f.cur = int(b1>>3) & 1
	ch := f.chans[f.cur]
	code := b1 &^ 0x08

	switch {
	case b2 >= 0x40 && code >= 0x10 && code <= 0x17:
		// Preamble address code.
		row, ok := pacRow(code, b2)
		if !ok {
			return
		}
		indent := 0
		if b2&0x10 != 0 {
			indent = int((b2&0x0e)>>1) * 4
		}
		ch.preamble(row, indent)
	case code == 0x11 && b2 >= 0x20 && b2 <= 0x2f:
		// Mid-row code (color/italics/underline): renders as a space.
		ch.writeRune(' ')
	case code == 0x11 && b2 >= 0x30 && b2 <= 0x3f:
		ch.writeRune(specialChars[b2-0x30])
	case (code == 0x12 || code == 0x13) && b2 >= 0x20 && b2 <= 0x3f:
		// Extended characters replace the basic character sent just before.
		ch.backspace()
		if code == 0x12 {
			ch.writeRune(extendedChars12[b2-0x20])
		} else {
			ch.writeRune(extendedChars13[b2-0x20])
		}
	case code == 0x14 || code == 0x15:
		if b2 < 0x20 || b2 > 0x2f {
			return
		}
		f.misc(ch, b2)
	case code == 0x17 && b2 >= 0x21 && b2 <= 0x23:
		// Tab offset.
		ch.col += int(b2 - 0x20)
		if ch.col > cols608 {
			ch.col = cols608
		}
	}
}

func (f *field608) misc(ch *channel608, b2 byte) {
	switch b2 {
	case 0x20: // RCL: resume caption loading (pop-on)
		if ch.mode == modeRollUp {
			ch.hidden.clear()
		}
		ch.mode = modePopOn
	case 0x21: // BS
		ch.backspace()
	case 0x24: // DER
		ch.deleteToEndOfRow()
	case 0x25, 0x26, 0x27: // RU2, RU3, RU4
		ch.setRollUp(int(b2 - 0x23))
	case 0x29: // RDC: resume direct captioning (paint-on)
		if ch.mode == modeText || ch.mode == modeRollUp {
			ch.row, ch.col = rows608, 0
		}
		ch.mode = modePaintOn
	case 0x2a, 0x2b: // TR, RTD: text mode; its characters are not captions
		ch.mode = modeText
	case 0x2c: // EDM
		ch.displayed.clear()
	case 0x2d: // CR
		ch.carriageReturn()
	case 0x2e: // ENM
		ch.hidden.clear()
	case 0x2f: // EOC
		ch.endOfCaption()
	}
}

// pacRow maps a preamble address code to its 1-based row.
func pacRow(b1, b2 byte) (int, bool) {
	hi := b2&0x20 != 0
	switch b1 {
	case 0x11:
		return pick(hi, 1, 2), true
	case 0x12:
		return pick(hi, 3, 4), true
	case 0x15:
		return pick(hi, 5, 6), true
	case 0x16:
		return pick(hi, 7, 8), true
	case 0x17:
		return pick(hi, 9, 10), true
	case 0x10:
		if hi {
			return 0, false
		}
		return 11, true
	case 0x13:
		return pick(hi, 12, 13), true
	case 0x14:
		return pick(hi, 14, 15), true
	}
	return 0, false
}

func pick(hi bool, lo, high int) int {
	if hi {
		return high
	}
	return lo
}

// basicChar maps a CEA-608 basic character code to its rune, or 0 for
// codes below 0x20.
func basicChar(b byte) rune {
	if b < 0x20 {
		return 0
	}
	switch b {
	case 0x2a:
		return 'á'
	case 0x5c:
		return 'é'
	case 0x5e:
		return 'í'
	case 0x5f:
		return 'ó'
	case 0x60:
		return 'ú'
	case 0x7b:
		return 'ç'
	case 0x7c:
		return '÷'
	case 0x7d:
		return 'Ñ'
	case 0x7e:
		return 'ñ'
	case 0x7f:
		return '█'
	}
	return rune(b)
}

var specialChars = [16]rune{'®', '°', '½', '¿', '™', '¢', '£', '♪', 'à', ' ', 'è', 'â', 'ê', 'î', 'ô', 'û'}

var extendedChars12 = [32]rune{
	'Á', 'É', 'Ó', 'Ú', 'Ü', 'ü', '‘', '¡', '*', '\'', '—', '©', '℠', '•', '“', '”',
	'À', 'Â', 'Ç', 'È', 'Ê', 'Ë', 'ë', 'Î', 'Ï', 'ï', 'Ô', 'Ù', 'ù', 'Û', '«', '»',
}

var extendedChars13 = [32]rune{
	'Ã', 'ã', 'Í', 'Ì', 'ì', 'Ò', 'ò', 'Õ', 'õ', '{', '}', '\\', '^', '_', '|', '~',
	'Ä', 'ä', 'Ö', 'ö', 'ß', '¥', '¤', '¦', 'Å', 'å', 'Ø', 'ø', '┌', '┐', '└', '┘',
}
