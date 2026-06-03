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
		streamplace.RichtextVideoFacet{},
		streamplace.ChatProfile{},
		streamplace.ChatProfile_BadgeSelections{},
		streamplace.ChatProfile_StreamerBadgeSelection{},
		streamplace.ChatProfile_Color{},
		streamplace.ChatMessage_ReplyRef{},
		streamplace.ServerSettings{},
		streamplace.ChatGate{},
		streamplace.ChatPinnedRecord{},
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
		streamplace.BadgeDef{},
		streamplace.BadgeIssuance{},
		streamplace.Defs_ActivityGame{},
		streamplace.Defs_ActivityLabel{},
		streamplace.Video{},
		streamplace.MediaTrack{},
		streamplace.MediaOrigin{},
		streamplace.MediaDefs_MuxlTrack{},
		streamplace.MediaDefs_SourceTracks{},
		streamplace.MediaDefs_SourceClip{},
		streamplace.MediaTrack_CommonMetadata{},
		streamplace.MediaViewCount{},
		streamplace.MediaViewCount_TrackUsage{},
		streamplace.Video_Connection{},
		streamplace.BetaInvite{},
		streamplace.BetaRequest{},
		streamplace.VodComment{},
		streamplace.VodComment_ReplyRef{},
		streamplace.Like{},
		streamplace.VodGate{},
	); err != nil {
		panic(err)
	}
}
