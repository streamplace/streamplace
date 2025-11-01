package c2patypes

import (
	"errors"
	"fmt"
	"io"
)

// type Stream interface {
// 	// Read a stream of bytes from the stream
// 	ReadStream(length uint64) ([]byte, error)
// 	// Seek to a position in the stream
// 	SeekStream(pos int64, mode uint64) (uint64, error)
// 	// Write a stream of bytes to the stream
// 	WriteStream(data []byte) (uint64, error)
// }

//	pub enum SeekMode {
//	    Start = 0,
//	    End = 1,
//	    Current = 2,
//	}

const (
	SeekModeStart   uint64 = 0
	SeekModeEnd     uint64 = 1
	SeekModeCurrent uint64 = 2
)

func NewReader(rs io.ReadSeeker) *C2PAStreamReader {
	return &C2PAStreamReader{ReadSeeker: rs}
}

func NewWriter(rws io.ReadWriteSeeker) *C2PAStreamWriter {
	return &C2PAStreamWriter{ReadWriteSeeker: rws}
}

// Wrapped io.ReadSeeker for passing to Rust. Doesn't write.
type C2PAStreamReader struct {
	io.ReadSeeker
}

func (s *C2PAStreamReader) ReadStream(length uint64) ([]byte, error) {
	return readStream(s.ReadSeeker, length)
}

func (s *C2PAStreamReader) SeekStream(pos int64, mode uint64) (uint64, error) {
	return seekStream(s.ReadSeeker, pos, mode)
}

func (s *C2PAStreamReader) WriteStream(data []byte) (uint64, error) {
	return 0, fmt.Errorf("Writing is not implemented for C2PAStreamReader")
}

// Wrapped io.Writer for passing to Rust.
type C2PAStreamWriter struct {
	io.ReadWriteSeeker
}

func (s *C2PAStreamWriter) ReadStream(length uint64) ([]byte, error) {
	return readStream(s.ReadWriteSeeker, length)
}

func (s *C2PAStreamWriter) SeekStream(pos int64, mode uint64) (uint64, error) {
	return seekStream(s.ReadWriteSeeker, pos, mode)
}

func (s *C2PAStreamWriter) WriteStream(data []byte) (uint64, error) {
	return writeStream(s.ReadWriteSeeker, data)
}

func readStream(r io.ReadSeeker, length uint64) ([]byte, error) {
	// fmt.Printf("read length=%d\n", length)
	bs := make([]byte, length)
	read, err := r.Read(bs)
	if err != nil {
		if errors.Is(err, io.EOF) {
			if read == 0 {
				// fmt.Printf("read EOF read=%d returning empty?", read)
				return []byte{}, nil
			}
			// partial := bs[read:]
			// return partial, nil
		}
		// fmt.Printf("io error=%s\n", err)
		return []byte{}, err
	}
	if uint64(read) < length {
		partial := bs[:read]
		// fmt.Printf("read returning partial read=%d len=%d\n", read, len(partial))
		return partial, nil
	}
	// fmt.Printf("read returning full read=%d len=%d\n", read, len(bs))
	return bs, nil
}

func seekStream(r io.ReadSeeker, pos int64, mode uint64) (uint64, error) {
	// fmt.Printf("seek pos=%d\n", pos)
	var seekMode int
	if mode == SeekModeCurrent {
		seekMode = io.SeekCurrent
	} else if mode == SeekModeStart {
		seekMode = io.SeekStart
	} else if mode == SeekModeEnd {
		seekMode = io.SeekEnd
	} else {
		// fmt.Printf("seek mode unsupported mode=%d\n", mode)
		return 0, fmt.Errorf("unknown seek mode: %d", mode)
	}
	newPos, err := r.Seek(pos, seekMode)
	if err != nil {
		return 0, err
	}
	return uint64(newPos), nil
}

func writeStream(w io.ReadWriteSeeker, data []byte) (uint64, error) {
	wrote, err := w.Write(data)
	if err != nil {
		return uint64(wrote), err
	}
	return uint64(wrote), nil
}
