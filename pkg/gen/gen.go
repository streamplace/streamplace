package main

import (
	"stream.place/streamplace/pkg/streamplace"

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
		streamplace.ChatGate{},
	); err != nil {
		panic(err)
	}
}
