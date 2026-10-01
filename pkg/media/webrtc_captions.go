package media

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/livecue"
	"stream.place/streamplace/pkg/log"
)

// captionChannels delivers a streamer's live caption cues to one WHEP viewer
// over a WebRTC data channel labeled livecue.Label. Each message is the JSON
// of a place.stream.caption.defs#liveCue, the same as on the livestream
// websocket; this is the low-latency path for third-party players that speak
// WHEP but not our websocket.
//
// The channel exists only when the viewer's offer negotiates a data channel
// (an m=application section). The server opens the channel itself, and a
// channel with the same label that the viewer opens is served too, whichever
// side's the player listens on.
type captionChannels struct {
	user   string
	viewer string
	mm     *MediaManager
	hub    *captions.Hub

	mu   sync.Mutex
	open map[*webrtc.DataChannel]struct{}
}

// offersDataChannel reports whether an SDP offer negotiates a data channel.
func offersDataChannel(offer *webrtc.SessionDescription) bool {
	parsed, err := offer.Unmarshal()
	if err != nil {
		return false
	}
	for _, md := range parsed.MediaDescriptions {
		if md.MediaName.Media != "application" || md.MediaName.Port.Value == 0 {
			continue
		}
		for _, f := range md.MediaName.Formats {
			if strings.EqualFold(f, "webrtc-datachannel") {
				return true
			}
		}
	}
	return false
}

// newCaptionChannels prepares the caption data channel of a WHEP session. It
// must be called before the offer is applied to pc, and returns nil, leaving
// the session unchanged, when the offer has no data channel or the node has
// no caption hub. Run starts delivery.
func newCaptionChannels(pc *webrtc.PeerConnection, mm *MediaManager, user, viewer string, offer *webrtc.SessionDescription) (*captionChannels, error) {
	if mm == nil || mm.bus == nil || mm.bus.Captions == nil || !offersDataChannel(offer) {
		return nil, nil
	}
	c := &captionChannels{
		user: user, viewer: viewer, mm: mm, hub: mm.bus.Captions,
		open: map[*webrtc.DataChannel]struct{}{},
	}
	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		if dc.Label() == livecue.Label {
			c.attach(dc)
		}
	})
	dc, err := pc.CreateDataChannel(livecue.Label, nil)
	if err != nil {
		return nil, err
	}
	c.attach(dc)
	return c, nil
}

// allowed applies the live playback privacy rule at the instant a caption is
// sent. Keeping an unpublished channel attached lets an owner preview it and
// lets a public viewer begin receiving cues when the stream is published,
// without replaying anything from the private interval.
func (c *captionChannels) allowed() bool {
	return c.mm != nil && (c.mm.LiveWindowPublished(c.user) || c.viewer == c.user || (c.mm.cli != nil && c.mm.cli.WideOpen))
}

// attach serves dc once it opens, starting with the current line.
func (c *captionChannels) attach(dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		c.mu.Lock()
		c.open[dc] = struct{}{}
		c.mu.Unlock()
		if !c.allowed() {
			return
		}
		for _, ev := range livecue.Recent(c.hub, c.user, livecue.JoinWindow, time.Now()) {
			if msg, err := livecue.Message(ev); err == nil && c.allowed() {
				_ = dc.SendText(string(msg))
			}
		}
	})
	dc.OnClose(func() {
		c.mu.Lock()
		delete(c.open, dc)
		c.mu.Unlock()
	})
}

// Run forwards the streamer's caption events to the open channels until ctx
// is done.
func (c *captionChannels) Run(ctx context.Context) {
	for ev := range c.hub.Subscribe(ctx, c.user) {
		if !c.allowed() {
			continue
		}
		msg, err := livecue.Message(ev)
		if err != nil {
			log.Error(ctx, "could not marshal caption cue", "error", err)
			continue
		}
		c.mu.Lock()
		channels := make([]*webrtc.DataChannel, 0, len(c.open))
		for dc := range c.open {
			channels = append(channels, dc)
		}
		c.mu.Unlock()
		for _, dc := range channels {
			if !c.allowed() {
				continue
			}
			if err := dc.SendText(string(msg)); err != nil {
				log.Debug(ctx, "could not send caption cue", "error", err)
			}
		}
	}
}
