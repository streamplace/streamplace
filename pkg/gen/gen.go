package main

import (
	"reflect"

	"github.com/bluesky-social/indigo/mst"
	"stream.place/streamplace/pkg/streamplace"

	cbg "github.com/whyrusleeping/cbor-gen"
)

func main() {
	var typVals []any
	for _, typ := range mst.CBORTypes() {
		typVals = append(typVals, reflect.New(typ).Elem().Interface())
	}

	genCfg := cbg.Gen{
		MaxStringLength: 1_000_000,
	}

	if err := genCfg.WriteMapEncodersToFile("pkg/streamplace/cbor_gen.go", "streamplace",
		streamplace.Key{},
		streamplace.Livestream{},
		streamplace.Segment{},
		streamplace.Segment_Audio{},
		streamplace.Segment_Video{},
		streamplace.Segment_Framerate{},
		streamplace.ChatMessage{},
		streamplace.RichtextFacet{},
		streamplace.ChatProfile{},
		streamplace.ChatProfile_Color{},
	); err != nil {
		panic(err)
	}
}
