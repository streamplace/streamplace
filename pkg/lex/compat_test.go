package lex

import (
	"bytes"
	"encoding/hex"
	"testing"

	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/hyphacoop/go-dasl/drisl"
	"github.com/ipfs/go-cid"
)

func eq(t *testing.T, name string, cbgBytes, drislBytes []byte) {
	t.Helper()
	if !bytes.Equal(cbgBytes, drislBytes) {
		t.Errorf("%s MISMATCH\n  cbor-gen (%d): %s\n  drisl    (%d): %s", name,
			len(cbgBytes), hex.EncodeToString(cbgBytes), len(drislBytes), hex.EncodeToString(drislBytes))
	} else {
		t.Logf("%s MATCH (%d bytes)", name, len(cbgBytes))
	}
}

func TestLinkCompat(t *testing.T) {
	c := cid.MustParse("bafkreib2rxk3rybk3aobmv5cjuql3bm2twh4jo5uxyjfxzvjcamdmc76jm")
	// indigo cbor-gen
	il := lexutil.LexLink(c)
	var b bytes.Buffer
	if err := il.MarshalCBOR(&b); err != nil {
		t.Fatal(err)
	}
	// pkg/lex via drisl
	dl := Link(c)
	db, err := drisl.Marshal(dl)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "Link", b.Bytes(), db)
}

func TestBytesCompat(t *testing.T) {
	raw := []byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x11}
	ib := lexutil.LexBytes(raw)
	var b bytes.Buffer
	if err := ib.MarshalCBOR(&b); err != nil {
		t.Fatal(err)
	}
	db, err := drisl.Marshal(Bytes(raw))
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "Bytes", b.Bytes(), db)
}

func TestBlobCompat(t *testing.T) {
	c := cid.MustParse("bafkreib2rxk3rybk3aobmv5cjuql3bm2twh4jo5uxyjfxzvjcamdmc76jm")
	// modern blob
	ib := &lexutil.LexBlob{Ref: lexutil.LexLink(c), MimeType: "image/jpeg", Size: 12345}
	var b bytes.Buffer
	if err := ib.MarshalCBOR(&b); err != nil {
		t.Fatal(err)
	}
	db, err := drisl.Marshal(Blob{Ref: Link(c), MimeType: "image/jpeg", Size: 12345})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "Blob(modern)", b.Bytes(), db)

	// legacy blob
	ib2 := &lexutil.LexBlob{Ref: lexutil.LexLink(c), MimeType: "image/png", Size: -1}
	var b2 bytes.Buffer
	if err := ib2.MarshalCBOR(&b2); err != nil {
		t.Fatal(err)
	}
	db2, err := drisl.Marshal(Blob{Ref: Link(c), MimeType: "image/png", Size: -1})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "Blob(legacy)", b2.Bytes(), db2)
}

func TestBlobRoundtrip(t *testing.T) {
	c := cid.MustParse("bafkreib2rxk3rybk3aobmv5cjuql3bm2twh4jo5uxyjfxzvjcamdmc76jm")
	orig := Blob{Ref: Link(c), MimeType: "image/jpeg", Size: 999}
	enc, err := drisl.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}
	var got Blob
	if err := drisl.Unmarshal(enc, &got); err != nil {
		t.Fatal(err)
	}
	if got.MimeType != orig.MimeType || got.Size != orig.Size || got.Ref.String() != orig.Ref.String() {
		t.Errorf("roundtrip mismatch: got %+v want %+v", got, orig)
	} else {
		t.Logf("Blob roundtrip OK")
	}
}
