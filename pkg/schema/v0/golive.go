package v0

import "aquareum.tv/aquareum/pkg/schema"

var Name = "Aquareum"
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
