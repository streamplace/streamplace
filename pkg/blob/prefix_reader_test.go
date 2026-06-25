package blob

import (
	"bytes"
	"io"
	"testing"
)

// bytesBlob is a blob.Reader backed by an in-memory byte slice.
type bytesBlob struct{ b []byte }

func (r bytesBlob) ReadAt(p []byte, off int64) (int, error) {
	return bytes.NewReader(r.b).ReadAt(p, off)
}
func (r bytesBlob) Size() int64  { return int64(len(r.b)) }
func (r bytesBlob) Close() error { return nil }

func TestPrefixReader(t *testing.T) {
	header := []byte("HEADER!")              // 7 bytes
	body := bytesBlob{[]byte("0123456789")}  // 10 bytes
	r := NewPrefixReader(header, body, 2, 5) // window "23456"
	const want = "HEADER!23456"              // 12 bytes

	if got := r.Size(); got != int64(len(want)) {
		t.Fatalf("Size = %d, want %d", got, len(want))
	}

	// Full read via SectionReader (exercises ReadAt across the boundary).
	all, err := io.ReadAll(io.NewSectionReader(r, 0, r.Size()))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(all) != want {
		t.Fatalf("ReadAll = %q, want %q", all, want)
	}

	cases := []struct {
		name    string
		off, n  int64
		want    string
		wantEOF bool
	}{
		{"header only", 0, 4, "HEAD", false},
		{"header into body", 4, 6, "ER!234", false},
		{"body only", 7, 3, "234", false},
		{"exact tail", 9, 3, "456", false},
		{"past end trims + EOF", 10, 5, "56", true},
		{"start at last byte", 11, 1, "6", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, tc.n)
			got, err := r.ReadAt(buf, tc.off)
			if string(buf[:got]) != tc.want {
				t.Fatalf("ReadAt(%d,%d) = %q, want %q", tc.off, tc.n, buf[:got], tc.want)
			}
			if tc.wantEOF && err != io.EOF {
				t.Fatalf("ReadAt(%d,%d) err = %v, want io.EOF", tc.off, tc.n, err)
			}
			if !tc.wantEOF && err != nil {
				t.Fatalf("ReadAt(%d,%d) err = %v, want nil", tc.off, tc.n, err)
			}
		})
	}

	// Reading exactly at the end is io.EOF with no bytes.
	if n, err := r.ReadAt(make([]byte, 4), r.Size()); n != 0 || err != io.EOF {
		t.Fatalf("ReadAt at end = (%d,%v), want (0, EOF)", n, err)
	}
}
