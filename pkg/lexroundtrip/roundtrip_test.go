package lexroundtrip_test

import (
	"bytes"
	"testing"

	"github.com/ipfs/go-cid"
	glex "github.com/streamplace/glex/runtime"
	"stream.place/streamplace/pkg/placestream"
)

// TestAdapterRegistryRoundtrip exercises the full go-dasl path: marshal a
// generated record through its cbg-shaped adapter, then decode it back via
// indigo's lexutil registry (as the firehose does) and confirm the concrete
// type + fields survive.
func TestAdapterRegistryRoundtrip(t *testing.T) {
	c := cid.MustParse("bafkreib2rxk3rybk3aobmv5cjuql3bm2twh4jo5uxyjfxzvjcamdmc76jm")
	orig := &placestream.Livestream{
		CreatedAt: "2026-07-07T00:00:00Z",
		Title:     "hello world",
		Tags:      []string{"lang:en"},
		Thumb:     &glex.Blob{Ref: glex.Link(c), MimeType: "image/jpeg", Size: 4096},
		Activity: &placestream.Livestream_Activity{
			Defs_ActivityGame: &placestream.Defs_ActivityGame{Uri: "at://did:plc:x/games.gamesgamesgamesgames.game/abc"},
		},
	}

	var buf bytes.Buffer
	if err := orig.MarshalCBOR(&buf); err != nil {
		t.Fatalf("MarshalCBOR: %v", err)
	}
	enc := buf.Bytes()

	// Marshal stamps $type on a copy: the original record must NOT be
	// mutated (a marshal that writes to the record is a data race for
	// records marshaled concurrently).
	if orig.LexiconTypeID != "" {
		t.Errorf("marshal mutated the record's LexiconTypeID to %q", orig.LexiconTypeID)
	}

	// Decode via the glex runtime registry, exactly like the firehose does.
	// Registry dispatch succeeding is itself proof $type was stamped into
	// the encoded bytes.
	decoded, err := glex.CborDecodeValue(enc)
	if err != nil {
		t.Fatalf("CborDecodeValue: %v", err)
	}
	ls, ok := decoded.(*placestream.Livestream)
	if !ok {
		t.Fatalf("decoded to %T, want *placestream.Livestream", decoded)
	}
	if ls.LexiconTypeID != "place.stream.livestream" {
		t.Errorf("decoded $type: got %q, want place.stream.livestream", ls.LexiconTypeID)
	}
	if ls.Title != orig.Title || ls.CreatedAt != orig.CreatedAt {
		t.Errorf("scalar mismatch: got title=%q created=%q", ls.Title, ls.CreatedAt)
	}
	if ls.Thumb == nil || ls.Thumb.MimeType != "image/jpeg" || ls.Thumb.Size != 4096 || ls.Thumb.Ref.String() != c.String() {
		t.Errorf("blob mismatch: %+v", ls.Thumb)
	}
	if ls.Activity == nil || ls.Activity.Defs_ActivityGame == nil {
		t.Fatalf("activity union lost in roundtrip: %+v", ls.Activity)
	}

	// Re-marshal must be byte-stable (=> stable record CID).
	var buf2 bytes.Buffer
	if err := ls.MarshalCBOR(&buf2); err != nil {
		t.Fatalf("re-MarshalCBOR: %v", err)
	}
	if !bytes.Equal(enc, buf2.Bytes()) {
		t.Errorf("re-marshal not byte-stable")
	}
}
