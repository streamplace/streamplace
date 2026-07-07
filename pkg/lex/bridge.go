package lex

import (
	"github.com/ipfs/go-cid"

	lexutil "github.com/bluesky-social/indigo/lex/util"
)

// This file bridges between glex's go-dasl-native Blob type and indigo's
// lexutil.LexBlob, for the boundaries where Streamplace records interoperate
// with indigo's bundled bsky/atproto types (e.g. com.atproto.repo.uploadBlob
// output, or app.bsky embed types) which still use lexutil.LexBlob.
//
// Since Blob is a type alias for glexrt.Blob (from another package), we cannot
// define methods on it here — so the conversions are free functions.

// BlobFromLexUtil converts an indigo *lexutil.LexBlob to a *Blob (nil-safe).
func BlobFromLexUtil(b *lexutil.LexBlob) *Blob {
	if b == nil {
		return nil
	}
	return &Blob{
		Ref:      Link(cid.Cid(b.Ref)),
		MimeType: b.MimeType,
		Size:     b.Size,
	}
}

// BlobToLexUtil converts a *Blob to an indigo *lexutil.LexBlob (nil-safe).
func BlobToLexUtil(b *Blob) *lexutil.LexBlob {
	if b == nil {
		return nil
	}
	return &lexutil.LexBlob{
		Ref:      lexutil.LexLink(cid.Cid(b.Ref)),
		MimeType: b.MimeType,
		Size:     b.Size,
	}
}
