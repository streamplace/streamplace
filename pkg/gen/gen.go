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
		streamplace.Livestream_NotificationSettings{},
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
		streamplace.MultistreamTarget{},
		streamplace.BroadcastOrigin{},
		streamplace.BroadcastSyndication{},
		streamplace.MetadataConfiguration{},
		streamplace.MetadataDistributionPolicy{},
		streamplace.MetadataContentRights{},
		streamplace.MetadataContentWarnings{},
		streamplace.ModerationPermission{},
		streamplace.LiveTeleport{},
		streamplace.LiveRecommendations{},
		streamplace.LiveViewerCount{},
		streamplace.Video{},
		streamplace.MuxlDefs_Archive{},
		streamplace.MuxlDefs_Catalog{},
		streamplace.MuxlDefs_VideoTrack{},
		streamplace.MuxlDefs_AudioTrack{},
		streamplace.MuxlCatalog{},
		streamplace.MuxlCatalog_TrackInit{},
		streamplace.MuxlSegment{},
		streamplace.MuxlSegment_TrackSegment{},
	); err != nil {
		panic(err)
	}
}
