package media

import (
	"context"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/rtcrec"
)

func (mm *MediaManager) shouldRecord(ctx context.Context, user string) (bool, error) {
	shouldRecord := false
	settings, err := mm.model.GetServerSettings(ctx, mm.cli.BroadcasterHost, user)
	if err != nil {
		return false, err
	}
	if settings != nil {
		spsettings, err := settings.ToStreamplaceServerSettings()
		if err != nil {
			return false, err
		}
		if spsettings.DebugRecording != nil {
			shouldRecord = *spsettings.DebugRecording
		}
	}
	return shouldRecord, nil
}

func (mm *MediaManager) NewPeerConnection(ctx context.Context, user string) (rtcrec.PeerConnection, error) {
	ctx = log.WithLogValues(ctx, "func", "NewPeerConnection", "streamer", user)
	shouldRecord, err := mm.shouldRecord(ctx, user)
	if err != nil {
		return nil, err
	}
	pionpc, err := mm.webrtcAPI.NewPeerConnection(mm.webrtcConfig)
	if err != nil {
		return nil, err
	}
	return rtcrec.NewRecordingPeerConnection(ctx, *mm.cli, user, pionpc, shouldRecord)
}
