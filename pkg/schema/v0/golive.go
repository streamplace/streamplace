package v0

import "stream.place/streamplace/pkg/schema"

var Name = "Streamplace"
var Version = "0.0.1"

type V0Schema struct {
	Identity  Identity
	StreamKey StreamKey
}
type Identity struct {
	Handle string `json:"handle"`
	DID    string `json:"did"`
}

type StreamKey struct {
	Authorized string `json:"authorized"`
}

func MakeV0Schema() (schema.Schema, error) {
	return schema.MakeSchema(Name, Version, V0Schema{})
}
