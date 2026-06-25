package blob

import (
	"errors"
	"io"
)

// PrefixReader presents header bytes followed by a byte-range window of an
// underlying blob.Reader as a single blob.Reader (io.ReaderAt + Size + Close).
//
// It's the serving primitive for flat-MP4 VOD: a synthesized faststart MP4
// header concatenated with the canonical MUXL blob's body bytes. muxl's
// flat-header synthesis owns all absolute offsets — the moov's co64 entries
// already point into the [header][body] address space — so we serve exactly
// header ++ body[bodyOffset : bodyOffset+bodyLen] and hand it to the same Range
// machinery the raw blob uses. For a whole VOD the window is the entire blob;
// for a clip it's the contiguous sub-range muxl sized the header for.
type PrefixReader struct {
	header     []byte
	body       Reader
	bodyOffset int64 // start of the window within body
	bodyLen    int64 // length of the window
}

// NewPrefixReader builds a Reader over header ++ body[bodyOffset:bodyOffset+bodyLen].
// It takes ownership of body: Close closes it.
func NewPrefixReader(header []byte, body Reader, bodyOffset, bodyLen int64) *PrefixReader {
	return &PrefixReader{header: header, body: body, bodyOffset: bodyOffset, bodyLen: bodyLen}
}

func (r *PrefixReader) Size() int64 { return int64(len(r.header)) + r.bodyLen }

func (r *PrefixReader) Close() error { return r.body.Close() }

// ReadAt implements io.ReaderAt over the virtual [header][body-window] file.
func (r *PrefixReader) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("blob.PrefixReader: negative offset")
	}
	total := r.Size()
	if off >= total {
		return 0, io.EOF
	}

	// Trim the request to what's actually available; a request running past the
	// end reports io.EOF once the available bytes are delivered.
	eof := false
	if avail := total - off; int64(len(p)) > avail {
		p = p[:avail]
		eof = true
	}

	n := 0
	h := int64(len(r.header))

	// Header portion.
	if off < h {
		hn := h - off
		if hn > int64(len(p)) {
			hn = int64(len(p))
		}
		copy(p[:hn], r.header[off:off+hn])
		n += int(hn)
		off += hn
		p = p[hn:]
	}

	// Body window portion.
	if len(p) > 0 {
		m, err := r.body.ReadAt(p, r.bodyOffset+(off-h))
		n += m
		if err != nil && !errors.Is(err, io.EOF) {
			return n, err
		}
		if errors.Is(err, io.EOF) {
			return n, io.EOF
		}
	}

	if eof {
		return n, io.EOF
	}
	return n, nil
}
