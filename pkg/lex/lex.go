// Package lex is a compatibility shim that re-exports the glex runtime
// (github.com/streamplace/glex/runtime, package glexrt) under the historical
// streamplace import path. This allows existing generated code and hand-written
// callers to keep importing "stream.place/streamplace/pkg/lex" while the
// underlying implementation lives in the standalone glex module.
//
// Once all generated code is regenerated to import glexrt directly, this shim
// can be removed.
package lex

import (
	"github.com/streamplace/glex/runtime"
)

// Re-export the value types.
type Link = glexrt.Link
type Blob = glexrt.Blob
type Bytes = glexrt.Bytes

// Re-export the adapter helpers.
var (
	MarshalCBOR   = glexrt.MarshalCBOR
	UnmarshalCBOR = glexrt.UnmarshalCBOR
)

// Re-export the registry and decode functions.
var (
	RegisterType       = glexrt.RegisterType
	CborDecodeValue    = glexrt.CborDecodeValue
	JsonDecodeValue    = glexrt.JsonDecodeValue
	NewFromType        = glexrt.NewFromType
	ErrUnrecognizedType = glexrt.ErrUnrecognizedType
)

// Re-export the type decoder and extract helpers.
type LexiconTypeDecoder = glexrt.LexiconTypeDecoder

var (
	TypeExtract            = glexrt.TypeExtract
	CborTypeExtract        = glexrt.CborTypeExtract
	CborTypeExtractReader  = glexrt.CborTypeExtractReader
)

// Re-export the XRPC client interface and constants.
type LexClient = glexrt.LexClient

const (
	Query     = glexrt.Query
	Procedure = glexrt.Procedure
)
