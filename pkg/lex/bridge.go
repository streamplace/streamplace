package lex

import (
	"github.com/ipfs/go-cid"

	lexutil "github.com/bluesky-social/indigo/lex/util"
)

// This file bridges between pkg/lex's go-dasl-native value types and indigo's
// lexutil equivalents, for the boundaries where Streamplace records interoperate
// with indigo's bundled bsky/atproto types (e.g. com.atproto.repo.uploadBlob
// output, or app.bsky embed types) which still use lexutil.LexBlob.

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

// LexUtil converts a *Blob to an indigo *lexutil.LexBlob (nil-safe).
func (b *Blob) LexUtil() *lexutil.LexBlob {
	if b == nil {
		return nil
	}
	return &lexutil.LexBlob{
		Ref:      lexutil.LexLink(cid.Cid(b.Ref)),
		MimeType: b.MimeType,
		Size:     b.Size,
	}
}
