package main

import (
	"stream.place/streamplace/pkg/streamplace"

	"github.com/bluesky-social/indigo/atproto/lexicon"
	cbg "github.com/whyrusleeping/cbor-gen"
)

func main() {
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
		streamplace.ChatMessage_ReplyRef{},
		streamplace.ServerSettings{},
		lexicon.SchemaFile{},
	); err != nil {
		panic(err)
	}
}
