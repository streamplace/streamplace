package media

import (
	"context"
	"sort"
	"strconv"

	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/log"
)

// sourceAudioKeep is how many recent source segments' Opus tracks a node
// keeps per streamer, to pair with the rendition addenda that follow them
// a transcoder round trip later.
const sourceAudioKeep = 8

type sourceAudio struct {
	tfdt uint64            // the source video track's first decode time
	opus map[string][]byte // the main group's Opus track and each stem's, canonical bytes by track id
}

// sourceOpus returns a source segment's Opus tracks, the main group's and
// each stem's, so a rendition viewer gets the stems too, with its video
// track's first decode time, which the renditions transcoded from it share.
func sourceOpus(ctx context.Context, seg []byte) (uint64, map[string][]byte, bool) {
	events, err := unwrapMuxlEvents(ctx, seg)
	if err != nil {
		log.Debug(ctx, "rendition playback: could not read source segment", "error", err)
		return 0, nil, false
	}
	cat, tracks := catalogAndTracks(events)
	if cat == nil || cat.Video == nil || cat.Audio == nil || tracks == nil {
		return 0, nil, false
	}
	var videoID string
	for _, v := range cat.Video.Renditions {
		videoID = strconv.FormatUint(uint64(v.TrackID()), 10)
		break
	}
	all := audioTracks(cat)
	var opusIDs []uint32
	for _, a := range mainAudioTracks(all) {
		if isOpusCodec(a.Codec) {
			opusIDs = append(opusIDs, a.TrackID())
			break
		}
	}
	if videoID == "" || len(opusIDs) == 0 {
		return 0, nil, false
	}
	for _, a := range stemOpusTracks(all) {
		opusIDs = append(opusIDs, a.TrackID())
	}
	opus := map[string][]byte{}
	for _, id := range opusIDs {
		key := strconv.FormatUint(uint64(id), 10)
		if len(tracks[key]) > 0 {
			opus[key] = tracks[key]
		}
	}
	if len(opus) == 0 {
		return 0, nil, false
	}
	tfdt, ok := firstTfdt(tracks[videoID])
	return tfdt, opus, ok
}

// rememberSourceAudio keeps a replicated source segment's Opus tracks (see
// sourceOpus), keyed by its video's first decode time, for
// PublishRenditionsForPlayback.
func (mm *MediaManager) rememberSourceAudio(ctx context.Context, did string, seg []byte) {
	tfdt, opus, ok := sourceOpus(ctx, seg)
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
			list[i].opus = opus
			replaced = true
			break
		}
	}
	if !replaced {
		list = append(list, sourceAudio{tfdt: tfdt, opus: opus})
	}
	if len(list) > sourceAudioKeep {
		list = list[len(list)-sourceAudioKeep:]
	}
	mm.sourceAudios[did] = list
}

func (mm *MediaManager) sourceAudioAt(did string, tfdt uint64) map[string][]byte {
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
// is the pairing key), is packetized and published on the bus under the
// rendition's name ("720p"), as the transcoding node publishes its own. A
// node that only syndicates the stream received the renditions for HLS but
// had nothing on its bus, so a WebRTC viewer who picked one got no video.
func (mm *MediaManager) PublishRenditionsForPlayback(ctx context.Context, did string, addendum []byte, published bool) {
	if mm.bus == nil {
		return
	}
	packets := mm.renditionPackets(ctx, addendum, func(tfdt uint64) map[string][]byte {
		return mm.sourceAudioAt(did, tfdt)
	})
	for _, p := range packets {
		mm.bus.PublishSegment(ctx, did, p.name, &bus.Seg{PacketizedData: p.packet, Published: published})
	}
}

// PacketizeRenditions packetizes each rendition of an addendum this node
// minted from source, paired with source's own Opus tracks: the mix and
// the stems, which the transcoder's output lacks. The result is keyed by
// track id (see RenditionTrackID); a rendition that can't be paired is
// missing from it.
func (mm *MediaManager) PacketizeRenditions(ctx context.Context, source, addendum []byte) map[uint32]*bus.PacketizedSegment {
	_, opus, ok := sourceOpus(ctx, source)
	if !ok {
		return nil
	}
	out := map[uint32]*bus.PacketizedSegment{}
	for _, p := range mm.renditionPackets(ctx, addendum, func(uint64) map[string][]byte { return opus }) {
		out[p.trackID] = p.packet
	}
	return out
}

// renditionPacket is one rendition track of an addendum, packetized.
type renditionPacket struct {
	trackID uint32
	name    string // from its coded size, as live HLS names it
	packet  *bus.PacketizedSegment
}

// renditionPackets packetizes each rendition track in an addendum together
// with the source Opus tracks opusAt returns for its first decode time, in
// track-id order.
func (mm *MediaManager) renditionPackets(ctx context.Context, addendum []byte, opusAt func(tfdt uint64) map[string][]byte) []renditionPacket {
	if len(addendum) == 0 {
		return nil
	}
	events, err := unwrapMuxlEvents(ctx, addendum)
	if err != nil {
		log.Warn(ctx, "rendition playback: could not read addendum", "error", err)
		return nil
	}
	cat, tracks := catalogAndTracks(events)
	if cat == nil || cat.Video == nil || tracks == nil {
		return nil
	}
	var packets []renditionPacket
	for _, v := range cat.Video.Renditions {
		id := strconv.FormatUint(uint64(v.TrackID()), 10)
		video := tracks[id]
		if v.CodedHeight == 0 || len(video) == 0 {
			continue
		}
		tfdt, ok := firstTfdt(video)
		if !ok {
			continue
		}
		opus := opusAt(tfdt)
		if opus == nil {
			log.Debug(ctx, "rendition playback: no source audio for this rendition segment", "tfdt", tfdt)
			continue
		}
		name := RenditionName(v.CodedWidth, v.CodedHeight)
		// The rendition's video plus the source's Opus tracks, in track-id
		// order, is a bare canonical segment, which Packetize presents and
		// selects from (the mix and the stems) like the source's own.
		m4s := map[string][]byte{id: video}
		for key, track := range opus {
			m4s[key] = track
		}
		packet, err := Packetize(ctx, mm.cli, &bus.Seg{Muxl: concatTracksByID(m4s)})
		if err != nil {
			log.Warn(ctx, "rendition playback: could not packetize rendition", "rendition", name, "error", err)
			continue
		}
		packets = append(packets, renditionPacket{trackID: v.TrackID(), name: name, packet: packet})
	}
	sort.Slice(packets, func(i, j int) bool { return packets[i].trackID < packets[j].trackID })
	return packets
}
