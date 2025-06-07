package media

import "stream.place/streamplace/pkg/peerproxy"

func (mm *MediaManager) NewPeerConnection() (peerproxy.PeerConnection, error) {
	pionpc, err := mm.webrtcAPI.NewPeerConnection(mm.webrtcConfig)
	if err != nil {
		return nil, err
	}
	return peerproxy.NewWrappedPC(pionpc), nil
}
