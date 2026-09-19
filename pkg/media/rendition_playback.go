package media

import (
	"bytes"
	"context"
	"sort"
	"strconv"

	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// sourceAudioKeep is how many recent source segments' Opus tracks a node
// keeps per streamer, to pair with the rendition addenda that follow them
// a transcoder round trip later.
const sourceAudioKeep = 8

type sourceAudio struct {
	tfdt uint64 // the source video track's first decode time
	opus []byte // the Opus track's canonical bytes
}

// rememberSourceAudio keeps a replicated source segment's Opus track, keyed
// by its video track's first decode time, for PublishRenditionsForPlayback.
func (mm *MediaManager) rememberSourceAudio(ctx context.Context, did string, seg []byte) {
	events, err := unwrapMuxlEvents(ctx, seg)
	if err != nil {
		log.Debug(ctx, "rendition playback: could not read source segment", "streamer", did, "error", err)
		return
	}
	cat, tracks := catalogAndTracks(events)
	if cat == nil || cat.Video == nil || cat.Audio == nil || tracks == nil {
		return
	}
	var videoID, opusID string
	for _, v := range cat.Video.Renditions {
		videoID = strconv.FormatUint(uint64(v.TrackID()), 10)
		break
	}
	for _, a := range cat.Audio.Renditions {
		if isOpusCodec(a.Codec) {
			opusID = strconv.FormatUint(uint64(a.TrackID()), 10)
			break
		}
	}
	if videoID == "" || opusID == "" || len(tracks[opusID]) == 0 {
		return
	}
	tfdt, ok := firstTfdt(tracks[videoID])
	if !ok {
		return
	}
	mm.sourceAudioMu.Lock()
	defer mm.sourceAudioMu.Unlock()
	if mm.sourceAudios == nil {
		mm.sourceAudios = map[string][]sourceAudio{}
	}
	list := mm.sourceAudios[did]
	// A publisher reconnect starts a fresh media timeline, so early tfdt
	// values repeat while the previous session's entries are still cached:
	// the newest audio at a tfdt is the one that goes with the renditions
	// arriving now, so it replaces an older entry rather than sitting
	// behind it.
	replaced := false
	for i := range list {
		if list[i].tfdt == tfdt {
			list[i].opus = tracks[opusID]
			replaced = true
			break
		}
	}
	if !replaced {
		list = append(list, sourceAudio{tfdt: tfdt, opus: tracks[opusID]})
	}
	if len(list) > sourceAudioKeep {
		list = list[len(list)-sourceAudioKeep:]
	}
	mm.sourceAudios[did] = list
}

func (mm *MediaManager) sourceAudioAt(did string, tfdt uint64) []byte {
	mm.sourceAudioMu.Lock()
	defer mm.sourceAudioMu.Unlock()
	list := mm.sourceAudios[did]
	for i := len(list) - 1; i >= 0; i-- {
		if list[i].tfdt == tfdt {
			return list[i].opus
		}
	}
	return nil
}

// PublishRenditionsForPlayback makes an addendum of rendition tracks (see
// MintVideoRenditions) playable over WebRTC on this node: each rendition,
// paired with the Opus audio of the source segment it was transcoded from
// (the renditions are re-timed onto the source video's decode time, which
// is the pairing key), is presented, packetized and published on the bus
// under the rendition's name ("720p"), as the transcoding node publishes
// its own. A node that only syndicates the stream received the renditions
// for HLS but had nothing on its bus, so a WebRTC viewer who picked one
// got no video.
func (mm *MediaManager) PublishRenditionsForPlayback(ctx context.Context, did string, addendum []byte, published bool) {
	if mm.bus == nil || len(addendum) == 0 {
		return
	}
	events, err := unwrapMuxlEvents(ctx, addendum)
	if err != nil {
		log.Warn(ctx, "rendition playback: could not read addendum", "streamer", did, "error", err)
		return
	}
	cat, tracks := catalogAndTracks(events)
	if cat == nil || cat.Video == nil || tracks == nil {
		return
	}
	names := map[string]string{}
	for _, v := range cat.Video.Renditions {
		if v.CodedHeight > 0 {
			names[strconv.FormatUint(uint64(v.TrackID()), 10)] = RenditionName(v.CodedWidth, v.CodedHeight)
		}
	}
	ids := make([]string, 0, len(tracks))
	for id := range tracks {
		if names[id] != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		video := tracks[id]
		tfdt, ok := firstTfdt(video)
		if !ok {
			continue
		}
		opus := mm.sourceAudioAt(did, tfdt)
		if opus == nil {
			log.Debug(ctx, "rendition playback: no source audio for this rendition segment", "streamer", did, "tfdt", tfdt)
			continue
		}
		name := names[id]
		var flat bytes.Buffer
		if err := muxl.RunMuxlWrap(ctx, bytes.NewReader(append(append([]byte{}, video...), opus...)), "flat", &flat); err != nil {
			log.Warn(ctx, "rendition playback: could not present rendition", "streamer", did, "rendition", name, "error", err)
			continue
		}
		seg := &bus.Seg{Data: flat.Bytes(), Published: published}
		packet, err := Packetize(ctx, mm.cli, seg)
		if err != nil {
			log.Warn(ctx, "rendition playback: could not packetize rendition", "streamer", did, "rendition", name, "error", err)
			continue
		}
		seg.PacketizedData = packet
		mm.bus.PublishSegment(ctx, did, name, seg)
	}
}
