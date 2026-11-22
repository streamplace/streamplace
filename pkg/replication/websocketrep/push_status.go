package websocketrep

import (
	"slices"
	"time"
)

type PushPeerStatus struct {
	Peer        string    `json:"peer"`
	MoqURL      string    `json:"moqURL"`
	Connected   bool      `json:"connected"`
	ConnectedAt time.Time `json:"connectedAt"`
	LastError   string    `json:"lastError,omitempty"`
}

type PushStatus struct {
	Peers []PushPeerStatus `json:"peers"`
	Live  []string         `json:"live"`
}

// PushStatus snapshots outgoing sessions and the streams announced on them.
func (r *WebsocketReplicator) PushStatus() PushStatus {
	out := PushStatus{Peers: make([]PushPeerStatus, 0, len(r.cli.Peers)), Live: []string{}}
	r.pushMu.Lock()
	for _, peer := range r.cli.Peers {
		state := r.pushPeers[peer]
		state.Peer = peer
		out.Peers = append(out.Peers, state)
	}
	r.pushMu.Unlock()
	r.liveMu.Lock()
	for streamer := range r.live {
		out.Live = append(out.Live, streamer)
	}
	r.liveMu.Unlock()
	slices.Sort(out.Live)
	return out
}
