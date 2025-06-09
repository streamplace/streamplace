package media

import (
	"context"

	"stream.place/streamplace/pkg/rtcrec"
)

func (mm *MediaManager) NewPeerConnection(ctx context.Context, user string) (rtcrec.PeerConnection, error) {
	pionpc, err := mm.webrtcAPI.NewPeerConnection(mm.webrtcConfig)
	if err != nil {
		return nil, err
	}
	return rtcrec.NewRecordingPeerConnection(ctx, *mm.cli, user, pionpc)
}
