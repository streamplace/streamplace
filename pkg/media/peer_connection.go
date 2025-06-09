package media

import (
	"context"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/rtcrec"
)

func (mm *MediaManager) NewPeerConnection(ctx context.Context, user string) (rtcrec.PeerConnection, error) {
	shouldRecord := false
	settings, err := mm.model.GetServerSettings(ctx, mm.cli.PublicHost, user)
	if err != nil {
		return nil, err
	}
	if settings != nil {
		spsettings, err := settings.ToStreamplaceServerSettings()
		if err != nil {
			return nil, err
		}
		if spsettings.DebugRecording != nil {
			shouldRecord = *spsettings.DebugRecording
		}
	}
	if !shouldRecord {
		log.Warn(ctx, "no server settings found, will not record")
	}
	pionpc, err := mm.webrtcAPI.NewPeerConnection(mm.webrtcConfig)
	if err != nil {
		return nil, err
	}
	return rtcrec.NewRecordingPeerConnection(ctx, *mm.cli, user, pionpc, shouldRecord)
}
