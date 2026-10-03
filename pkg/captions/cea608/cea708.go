package cea608

import "strings"

// dtvcc decodes the CEA-708 side of cc_data: DTVCC packets are reassembled
// from cc_type 3 (start) and 2 (continuation) triplets, split into service
// blocks, and service 1's commands and text are applied to its windows.
// Best effort: text, pen location, window define/show/hide/clear/delete,
// and carriage control; styling commands are parsed and ignored.
type dtvcc struct {
	packet  []byte
	inPkt   bool
	windows [8]*window708
	cur     int
}

func newDTVCC() *dtvcc {
	return &dtvcc{}
}

const (
	maxRows708 = 15
	maxCols708 = 42
)

type window708 struct {
	visible  bool
	priority int
	rows     int
	cols     int
	row, col int
	text     [maxRows708][maxCols708]rune
}

func (w *window708) clear() {
	w.text = [maxRows708][maxCols708]rune{}
	w.row, w.col = 0, 0
}

func (w *window708) lines() []string {
	var out []string
	for r := 0; r < w.rows; r++ {
		var sb strings.Builder
		for c := 0; c < w.cols; c++ {
			ch := w.text[r][c]
			if ch == 0 {
				ch = ' '
			}
			sb.WriteRune(ch)
		}
		if line := strings.TrimSpace(sb.String()); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func (w *window708) write(r rune) {
	if w.row >= w.rows {
		w.row = w.rows - 1
	}
	if w.col >= w.cols {
		return
	}
	w.text[w.row][w.col] = r
	w.col++
}

func (w *window708) carriageReturn() {
	w.col = 0
	if w.row+1 < w.rows {
		w.row++
		return
	}
	// Scroll bottom-to-top, the default scroll direction.
	for r := 1; r < w.rows; r++ {
		w.text[r-1] = w.text[r]
	}
	w.text[w.rows-1] = [maxCols708]rune{}
}

func (d *dtvcc) triplet(valid, start bool, b1, b2 byte) {
	if start {
		d.packet = append(d.packet[:0], b1, b2)
		d.inPkt = valid
	} else {
		if !valid || !d.inPkt {
			return
		}
		d.packet = append(d.packet, b1, b2)
	}
	// The header says how long the packet is; decode it as soon as the last
	// byte lands rather than waiting for the next packet to start.
	if len(d.packet) >= packetSize(d.packet[0]) {
		d.flush()
	}
}

// packetSize is a DTVCC packet's length in bytes, header included.
func packetSize(hdr byte) int {
	if code := int(hdr & 0x3f); code != 0 {
		return code * 2
	}
	return 128
}

// flush decodes only a complete packet.
func (d *dtvcc) flush() {
	if !d.inPkt || len(d.packet) < packetSize(d.packet[0]) {
		d.inPkt = false
		return
	}
	d.inPkt = false
	size := packetSize(d.packet[0])
	data := d.packet[1:]
	if size-1 < len(data) {
		data = data[:size-1]
	}
	for len(data) > 0 {
		svc := int(data[0] >> 5)
		blockSize := int(data[0] & 0x1f)
		data = data[1:]
		if svc == 7 {
			if len(data) == 0 {
				return
			}
			svc = int(data[0] & 0x3f)
			data = data[1:]
		}
		if blockSize == 0 {
			continue
		}
		if blockSize > len(data) {
			return
		}
		if svc == 1 {
			d.service(data[:blockSize])
		}
		data = data[blockSize:]
	}
}

// service interprets one service block for service 1.
func (d *dtvcc) service(b []byte) {
	for i := 0; i < len(b); {
		c := b[i]
		switch {
		case c == 0x10: // EXT1: C2/G2/C3/G3
			if i+1 >= len(b) {
				return
			}
			n := d.ext(b[i+1:])
			i += 1 + n
		case c < 0x20:
			i += d.c0(b[i:])
		case c < 0x80:
			d.curWindow().write(g0(c))
			i++
		case c < 0xa0:
			i += d.c1(b[i:])
		default:
			d.curWindow().write(rune(c)) // G1: Latin-1
			i++
		}
	}
}

func (d *dtvcc) curWindow() *window708 {
	w := d.windows[d.cur]
	if w == nil {
		// Text before any DefineWindow: give it a default window so it is
		// not lost (some encoders rely on the decoder's implicit window).
		w = &window708{visible: true, rows: 2, cols: 32}
		d.windows[d.cur] = w
	}
	return w
}

// c0 handles a C0 code and returns its length.
func (d *dtvcc) c0(b []byte) int {
	c := b[0]
	switch c {
	case 0x08: // BS
		w := d.curWindow()
		if w.col > 0 {
			w.col--
			w.text[w.row][w.col] = 0
		}
	case 0x0c: // FF
		d.curWindow().clear()
	case 0x0d: // CR
		d.curWindow().carriageReturn()
	case 0x0e: // HCR
		w := d.curWindow()
		w.text[w.row] = [maxCols708]rune{}
		w.col = 0
	case 0x18: // P16: two-byte character, best effort as UTF-16 code unit
		if len(b) >= 3 {
			d.curWindow().write(rune(int(b[1])<<8 | int(b[2])))
		}
	}
	switch {
	case c <= 0x0f:
		return 1
	case c <= 0x17:
		return 2
	default:
		return 3
	}
}

// c1 handles a C1 window/pen command and returns its length.
func (d *dtvcc) c1(b []byte) int {
	c := b[0]
	switch {
	case c <= 0x87: // CWx
		d.cur = int(c - 0x80)
		return 1
	case c == 0x88: // CLW
		d.eachWindow(b, func(w *window708) { w.clear() })
		return 2
	case c == 0x89: // DSW
		d.eachWindow(b, func(w *window708) { w.visible = true })
		return 2
	case c == 0x8a: // HDW
		d.eachWindow(b, func(w *window708) { w.visible = false })
		return 2
	case c == 0x8b: // TGW
		d.eachWindow(b, func(w *window708) { w.visible = !w.visible })
		return 2
	case c == 0x8c: // DLW
		if len(b) >= 2 {
			for i := range 8 {
				if b[1]&(1<<i) != 0 {
					d.windows[i] = nil
				}
			}
		}
		return 2
	case c == 0x8d: // DLY
		return 2
	case c == 0x8e: // DLC
		return 1
	case c == 0x8f: // RST
		d.windows = [8]*window708{}
		d.cur = 0
		return 1
	case c == 0x90: // SPA
		return 3
	case c == 0x91: // SPC
		return 4
	case c == 0x92: // SPL
		if len(b) >= 3 {
			w := d.curWindow()
			w.row = int(b[1] & 0x0f)
			w.col = int(b[2] & 0x3f)
			if w.row >= w.rows {
				w.row = w.rows - 1
			}
			if w.col > w.cols {
				w.col = w.cols
			}
		}
		return 3
	case c == 0x97: // SWA
		return 5
	case c >= 0x98: // DFx
		if len(b) >= 7 {
			d.defineWindow(int(c-0x98), b[1:7])
		}
		return 7
	}
	return 1
}

func (d *dtvcc) eachWindow(b []byte, fn func(*window708)) {
	if len(b) < 2 {
		return
	}
	for i := range 8 {
		if b[1]&(1<<i) != 0 && d.windows[i] != nil {
			fn(d.windows[i])
		}
	}
}

func (d *dtvcc) defineWindow(id int, p []byte) {
	rows := int(p[3]&0x0f) + 1
	cols := int(p[4]&0x3f) + 1
	if rows > maxRows708 {
		rows = maxRows708
	}
	if cols > maxCols708 {
		cols = maxCols708
	}
	w := d.windows[id]
	if w == nil {
		w = &window708{}
		d.windows[id] = w
	}
	w.visible = p[0]&0x20 != 0
	w.priority = int(p[0] & 0x07)
	if w.rows != rows || w.cols != cols {
		w.rows, w.cols = rows, cols
		if w.row >= rows {
			w.row = rows - 1
		}
		if w.col > cols {
			w.col = cols
		}
	}
	d.cur = id
}

// ext handles the byte after EXT1 and returns how many bytes it consumed.
func (d *dtvcc) ext(b []byte) int {
	c := b[0]
	switch {
	case c < 0x08: // C2, no operands
		return 1
	case c < 0x10:
		return 2
	case c < 0x18:
		return 3
	case c < 0x20:
		return 4
	case c < 0x80: // G2
		if r := g2(c); r != 0 {
			d.curWindow().write(r)
		}
		return 1
	case c < 0x88: // C3
		return 5
	case c < 0x90:
		return 6
	case c < 0xa0:
		// Variable length: the second byte carries the length.
		if len(b) >= 2 {
			return 2 + int(b[1]&0x3f)
		}
		return len(b)
	case c == 0xa0: // G3: [CC] symbol
		d.curWindow().write('□')
		return 1
	default:
		d.curWindow().write('_')
		return 1
	}
}

func g0(c byte) rune {
	if c == 0x7f {
		return '♪'
	}
	return rune(c)
}

func g2(c byte) rune {
	switch c {
	case 0x20, 0x21:
		return ' '
	case 0x25:
		return '…'
	case 0x2a:
		return 'Š'
	case 0x2c:
		return 'Œ'
	case 0x30:
		return '█'
	case 0x31:
		return '‘'
	case 0x32:
		return '’'
	case 0x33:
		return '“'
	case 0x34:
		return '”'
	case 0x35:
		return '•'
	case 0x39:
		return '™'
	case 0x3a:
		return 'š'
	case 0x3c:
		return 'œ'
	case 0x3d:
		return '℠'
	case 0x3f:
		return 'Ÿ'
	case 0x76:
		return '⅛'
	case 0x77:
		return '⅜'
	case 0x78:
		return '⅝'
	case 0x79:
		return '⅞'
	case 0x7a:
		return '│'
	case 0x7b:
		return '┐'
	case 0x7c:
		return '└'
	case 0x7d:
		return '─'
	case 0x7e:
		return '┘'
	case 0x7f:
		return '┌'
	}
	return 0
}

// displayedText is the text of every visible window, highest priority
// (lowest number) first, rows joined by newlines.
func (d *dtvcc) displayedText() string {
	type vis struct {
		id int
		w  *window708
	}
	var shown []vis
	for i, w := range d.windows {
		if w != nil && w.visible {
			shown = append(shown, vis{i, w})
		}
	}
	for i := 1; i < len(shown); i++ {
		for j := i; j > 0 && (shown[j].w.priority < shown[j-1].w.priority || (shown[j].w.priority == shown[j-1].w.priority && shown[j].id < shown[j-1].id)); j-- {
			shown[j], shown[j-1] = shown[j-1], shown[j]
		}
	}
	var lines []string
	for _, v := range shown {
		lines = append(lines, v.w.lines()...)
	}
	return strings.Join(lines, "\n")
}
