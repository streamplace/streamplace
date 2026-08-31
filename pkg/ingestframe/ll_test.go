package ingestframe

import (
	"bytes"
	"testing"
)

func TestLLFramesRoundTripMetadataAndBytes(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.LLPart(LLFrame{Presentation: "p", Track: "v", Generation: 2, Timescale: 90000, MSN: 9, Part: 3, Start: 90, Duration: 30, Independent: true, Data: []byte("part")}); err != nil {
		t.Fatal(err)
	}

	typ, payload, err := NewReader(&buf).ReadFrame()
	if err != nil {
		t.Fatal(err)
	}
	if typ != LLPart {
		t.Fatalf("type = %v", typ)
	}
	got, err := DecodeLLFrame(payload)
	if err != nil {
		t.Fatal(err)
	}
	if got.Presentation != "p" || got.Track != "v" || got.Generation != 2 || got.Timescale != 90000 || got.MSN != 9 || got.Part != 3 || !got.Independent || !bytes.Equal(got.Data, []byte("part")) {
		t.Fatalf("frame = %+v", got)
	}
}
