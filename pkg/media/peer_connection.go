package media

import "stream.place/streamplace/pkg/rtcrec"

func (mm *MediaManager) NewPeerConnection() (rtcrec.PeerConnection, error) {
	pionpc, err := mm.webrtcAPI.NewPeerConnection(mm.webrtcConfig)
	if err != nil {
		return nil, err
	}
	return rtcrec.NewRecorderPeerConnection(pionpc), nil
}
