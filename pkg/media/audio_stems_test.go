package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/ingestframe"
	"stream.place/streamplace/pkg/livehls"
	"stream.place/streamplace/pkg/muxl"
)

// audioCfg is a catalog audio rendition at track id with codec and group.
func audioCfg(id uint32, codec string, group uint32, label string) muxl.MuxlAudioConfig {
	return muxl.MuxlAudioConfig{
		Codec:            codec,
		Container:        muxl.MuxlContainer{Kind: "cmaf", Timescale: 48000, TrackID: id},
		SampleRate:       48000,
		NumberOfChannels: 2,
		AlternateGroup:   group,
		Label:            label,
	}
}

func catalogOf(audio ...muxl.MuxlAudioConfig) *muxl.MuxlCatalog {
	cat := &muxl.MuxlCatalog{
		Video: &muxl.MuxlCatalogVideo{Renditions: map[string]muxl.MuxlVideoConfig{
			"v": {Codec: "avc1.640028", Container: muxl.MuxlContainer{Kind: "cmaf", Timescale: 90000, TrackID: 1}},
		}},
		Audio: &muxl.MuxlCatalogAudio{Renditions: map[string]muxl.MuxlAudioConfig{}},
	}
	for i, a := range audio {
		cat.Audio.Renditions[fmt.Sprint("a", i)] = a
	}
	return cat
}

// The main audio is the lowest alternateGroup; an ungrouped segment is one
// group. Codec completion looks at the main group only: a stem missing a
// codec is distributed as sent, and a main group with both codecs needs
// nothing whatever the stems carry.
func TestMainAudioGroupSelectionAndCompletion(t *testing.T) {
	ungroupedAAC := catalogOf(audioCfg(2, "mp4a.40.2", 0, ""))
	ungroupedBoth := catalogOf(audioCfg(3, "opus", 0, ""), audioCfg(2, "mp4a.40.2", 0, ""))
	grouped := catalogOf(
		audioCfg(4, "mp4a.40.2", 2, "Music"), audioCfg(5, "opus", 2, "Music"),
		audioCfg(2, "mp4a.40.2", 1, "Mix"), audioCfg(3, "opus", 1, "Mix"),
		audioCfg(7, "opus", 3, "Mic"), audioCfg(8, "mp4a.40.2", 4, "Alerts"), audioCfg(9, "opus", 5, "Game"),
	)
	mainAACOnly := catalogOf(audioCfg(2, "mp4a.40.2", 1, "Mix"), audioCfg(4, "mp4a.40.2", 2, "Music"), audioCfg(5, "opus", 2, "Music"))
	stemAACOnly := catalogOf(audioCfg(2, "mp4a.40.2", 1, "Mix"), audioCfg(3, "opus", 1, "Mix"), audioCfg(4, "mp4a.40.2", 2, "Music"))

	ids := func(tracks []muxl.MuxlAudioConfig) []uint32 {
		var out []uint32
		for _, a := range tracks {
			out = append(out, a.TrackID())
		}
		return out
	}
	require.Equal(t, []uint32{2, 3}, ids(mainAudioTracks(audioTracks(ungroupedBoth))), "ungrouped: every track is main, in track order")
	require.Equal(t, []uint32{2, 3}, ids(mainAudioTracks(audioTracks(grouped))), "grouped: the lowest group is main")
	require.Equal(t, []uint32{5, 7}, ids(stemOpusTracks(audioTracks(grouped))), "stems: one Opus track per higher group, ascending, at most two")
	require.Empty(t, stemOpusTracks(audioTracks(ungroupedBoth)), "ungrouped: no stems")
	require.Empty(t, stemOpusTracks(audioTracks(stemAACOnly)), "a stem without Opus can't reach WebRTC")

	for _, tc := range []struct {
		name   string
		cat    *muxl.MuxlCatalog
		target string
		need   bool
	}{
		{"ungrouped AAC only", ungroupedAAC, "opus", true},
		{"ungrouped both", ungroupedBoth, "", false},
		{"grouped, every group complete", grouped, "", false},
		{"grouped, main lacks Opus", mainAACOnly, "opus", true},
		{"grouped, only a stem lacks Opus", stemAACOnly, "", false},
		{"no video", &muxl.MuxlCatalog{Audio: ungroupedAAC.Audio}, "", false},
		{"no catalog", nil, "", false},
	} {
		target, need := completionTarget(tc.cat)
		require.Equal(t, tc.need, need, tc.name)
		require.Equal(t, tc.target, target, tc.name)
	}
}

// groupedTestMeta labels the tracks groupedTestFMP4's muxer creates (video 1,
// then the audio pads in order) the way the OBS plugin labels a full mix on
// mixer Track 1 and a "Music" stem on Track 2, each as AAC and Opus.
var groupedTestMeta = map[uint32]muxl.TrackMeta{
	2: {AlternateGroup: 1, Label: "Mix"},
	3: {AlternateGroup: 1, Label: "Mix"},
	4: {AlternateGroup: 2, Label: "Music"},
	5: {AlternateGroup: 2, Label: "Music"},
}

// groupedTestFMP4 synthesizes two seconds of the plugin's stem shape: H264
// video with a 1s GoP plus two AAC and two Opus tracks, as a fragmented MP4.
// The tracks carry no groups or names yet (mp4mux writes none).
func groupedTestFMP4(t *testing.T, ctx context.Context) []byte {
	t.Helper()
	return runSynthPipeline(t, ctx, strings.Join([]string{
		"videotestsrc num-buffers=60 ! video/x-raw,width=320,height=240,framerate=30/1 ! x264enc key-int-max=30 tune=zerolatency speed-preset=ultrafast ! h264parse ! mp4mux name=mux fragment-duration=500 ! appsink name=sink",
		"audiotestsrc num-buffers=94 samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! fdkaacenc ! aacparse ! mux.",
		"audiotestsrc num-buffers=100 samplesperbuffer=960 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! opusenc ! opusparse ! mux.",
		"audiotestsrc wave=triangle num-buffers=94 samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! fdkaacenc ! aacparse ! mux.",
		"audiotestsrc wave=triangle num-buffers=100 samplesperbuffer=960 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! opusenc ! opusparse ! mux.",
	}, "\n"))
}

// groupedTestSegment returns groupedTestFMP4's first GoP as an unsigned bare
// canonical .m4s whose catalogs carry groupedTestMeta.
func groupedTestSegment(t *testing.T, ctx context.Context) []byte {
	t.Helper()
	events, err := collectMuxlEvents(func(ch chan *muxl.MuxlEvent) error {
		return muxl.RunMuxlSegmenterEvents(ctx, bytes.NewReader(groupedTestFMP4(t, ctx)), ch, muxl.WithSegmentTrackMeta(groupedTestMeta))
	})
	require.NoError(t, err)
	for _, ev := range events {
		if ev.Type == "segment" {
			return concatTracksByID(ev.Tracks)
		}
	}
	t.Fatal("segmenter emitted no segment")
	return nil
}

// requireGroupedCatalog checks that a segment's catalog carries
// groupedTestMeta: four audio tracks with the plugin's groups and names.
func requireGroupedCatalog(t *testing.T, ctx context.Context, seg []byte) {
	t.Helper()
	events, err := unwrapMuxlEvents(ctx, seg)
	require.NoError(t, err)
	cat, _ := catalogAndTracks(events)
	tracks := audioTracks(cat)
	require.Len(t, tracks, 4)
	for _, a := range tracks {
		want := groupedTestMeta[a.TrackID()]
		require.Equal(t, want.AlternateGroup, a.AlternateGroup, "track %d group", a.TrackID())
		require.Equal(t, want.Label, a.Label, "track %d label", a.TrackID())
	}
	require.True(t, isAACCodec(tracks[0].Codec) && isOpusCodec(tracks[1].Codec), "mix is AAC then Opus: %v", tracks)
	require.True(t, isAACCodec(tracks[2].Codec) && isOpusCodec(tracks[3].Codec), "stem is AAC then Opus: %v", tracks)
}

// The OBS plugin's push (fmp4-direct) is signed with its track metadata: the
// groups and names it wrote into the fMP4 reach every signed segment's
// catalog, on the detached-worker path. The segment record then lists every
// audio track with its codec, group and label.
func TestFMP4DirectSignsTrackMetaAndRecordsStems(t *testing.T) {
	ctx := context.Background()
	// The plugin's fMP4 carries tkhd.alternate_group and trak/udta/name; muxl
	// writes exactly those from the meta.
	labeled, err := muxl.RunMuxlCanonicalize(ctx, groupedTestFMP4(t, ctx), nil, muxl.WithTrackMeta(groupedTestMeta))
	require.NoError(t, err)

	ms := newBareSegmentSigner(t)
	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)
	manifest, err := ms.buildManifest(ctx, time.Now().UnixMilli())
	require.NoError(t, err)
	cfg := IngestWorkerConfig{
		StreamerDID: ms.Streamer(),
		KeyPEM:      keyPEM,
		CertPEM:     ms.Cert,
		Manifest:    manifest,
		Transport:   IngestTransportFMP4Direct,
	}
	var buf bytes.Buffer
	require.NoError(t, RunMP4IngestWorker(ctx, cfg, bytes.NewReader(labeled), ingestframe.NewWriter(&buf), func() []byte { return cfg.Manifest }))

	r := ingestframe.NewReader(&buf)
	var segs [][]byte
	for {
		_, payload, err := r.ReadFrame()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		segs = append(segs, payload)
	}
	require.GreaterOrEqual(t, len(segs), 2, "a 2s stream with 1s GoPs signs into several segments")
	for _, seg := range segs {
		requireGroupedCatalog(t, ctx, seg)
	}

	// Validation derives the record's audio list from the catalog, not from
	// the one track the gst parse reads.
	valid, err := ValidateMP4Media(ctx, segs[0])
	require.NoError(t, err)
	require.NotNil(t, valid.Catalog)
	audio := valid.MediaData.Audio
	require.Len(t, audio, 4)
	for i, want := range []struct {
		codec string
		group uint32
		label string
	}{{"aac", 1, "Mix"}, {"opus", 1, "Mix"}, {"aac", 2, "Music"}, {"opus", 2, "Music"}} {
		require.Equal(t, want.codec, audio[i].Codec, "track %d codec", i)
		require.Equal(t, want.group, audio[i].AlternateGroup, "track %d group", i)
		require.Equal(t, want.label, audio[i].Label, "track %d label", i)
		require.Equal(t, 48000, audio[i].Rate, "track %d rate", i)
		require.Equal(t, 2, audio[i].Channels, "track %d channels", i)
	}
	_, need := completionTarget(valid.Catalog)
	require.False(t, need, "the mix has both codecs, so nothing is completed")
}

// A segment with stems packetizes into the mix plus one Opus track per stem
// group, on the mix's clock; the codec filters keep the mix's track of the
// requested codec. An ungrouped segment packetizes exactly as before: no
// stems.
func TestPacketizeStems(t *testing.T) {
	withNoGSTLeaks(t, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		grouped := groupedTestSegment(t, ctx)

		aac, err := filterSegmentToCodec(ctx, grouped, false)
		require.NoError(t, err)
		require.Equal(t, []string{"mp4a.40.2"}, audioCodecsOf(t, ctx, aac), "RTMP/HLS get the mix's AAC only")
		_, kept, err := filterSegmentAudio(ctx, grouped, true, true)
		require.NoError(t, err)
		require.Equal(t, []playbackAudio{{TrackID: 3, Group: 1, Main: true}, {TrackID: 5, Group: 2}}, kept)

		packet, err := Packetize(ctx, &config.CLI{}, &bus.Seg{Muxl: grouped})
		require.NoError(t, err)
		require.Len(t, packet.Video, 30)
		require.NotEmpty(t, packet.Audio, "the mix's Opus")
		require.Len(t, packet.Stems, 1)
		require.Equal(t, uint32(2), packet.Stems[0].Group)
		require.InDelta(t, len(packet.Audio), len(packet.Stems[0].Audio), 1, "the stem spans the segment like the mix")
		var stemDur time.Duration
		for _, s := range packet.Stems[0].Audio {
			stemDur += s.Duration
		}
		require.Equal(t, packet.Duration, stemDur, "the stem is stretched to the segment's duration")

		ungrouped := sourceSegmentForTest(t)
		packet, err = Packetize(ctx, &config.CLI{}, &bus.Seg{Muxl: ungrouped})
		require.NoError(t, err)
		require.NotEmpty(t, packet.Audio)
		require.Empty(t, packet.Stems)
	})
}

// A WebRTC viewer's answer carries the mix and one "audio-g<group>" track per
// stem of the stream's latest segment, in that order, as many as the offer
// has audio transceivers for; without stems (or without a segment yet) the
// answer is the one audio track it always was.
func TestWebRTCPlayback2StemTracks(t *testing.T) {
	mm, _ := getStaticTestMediaManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	offer := func(audioTransceivers int) *webrtc.SessionDescription {
		pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
		require.NoError(t, err)
		defer pc.Close()
		_, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly})
		require.NoError(t, err)
		for range audioTransceivers {
			_, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly})
			require.NoError(t, err)
		}
		offer, err := pc.CreateOffer(nil)
		require.NoError(t, err)
		return &offer
	}
	audioTrackIDs := func(answer *webrtc.SessionDescription) []string {
		parsed, err := answer.Unmarshal()
		require.NoError(t, err)
		var ids []string
		// The answer has an m-line per offered audio transceiver; those
		// without a track of ours carry no msid.
		for _, md := range parsed.MediaDescriptions {
			if msid, ok := md.Attribute("msid"); ok && md.MediaName.Media == "audio" {
				ids = append(ids, strings.Fields(msid)[1])
			}
		}
		return ids
	}

	answer, err := mm.WebRTCPlayback2(ctx, "streamer", "source", offer(3), "")
	require.NoError(t, err)
	require.Equal(t, []string{"audio"}, audioTrackIDs(answer), "no segment yet: the mix only")

	mm.bus.PublishSegment(ctx, "streamer", "source", &bus.Seg{Published: true, PacketizedData: &bus.PacketizedSegment{
		Duration: time.Second,
		Stems:    []bus.PacketizedStem{{Group: 2}, {Group: 3}},
	}})
	answer, err = mm.WebRTCPlayback2(ctx, "streamer", "source", offer(3), "")
	require.NoError(t, err)
	require.Equal(t, []string{"audio", "audio-g2", "audio-g3"}, audioTrackIDs(answer))

	answer, err = mm.WebRTCPlayback2(ctx, "streamer", "source", offer(2), "")
	require.NoError(t, err)
	require.Equal(t, []string{"audio", "audio-g2"}, audioTrackIDs(answer), "an offer with fewer transceivers gets what fits")

	answer, err = mm.WebRTCPlayback2(ctx, "streamer", "source", offer(1), "")
	require.NoError(t, err)
	require.Equal(t, []string{"audio"}, audioTrackIDs(answer), "a client that can't take stems gets the mix")
}

// groupedSignedSegment is groupedTestFMP4's first GoP signed the way the
// in-process fmp4-direct ingest signs the plugin's push: the groups and
// names in the fMP4 reach the signed catalogs.
func groupedSignedSegment(t *testing.T, ctx context.Context) []byte {
	t.Helper()
	first := signLabeledSegments(t, ctx, newBareSegmentSigner(t), groupedTestFMP4(t, ctx), groupedTestMeta)[0]
	requireGroupedCatalog(t, ctx, first)
	return first
}

// signLabeledSegments writes meta into frag's tracks and signs every GoP the
// way fmp4-direct ingest does, returning the bare signed segments.
func signLabeledSegments(t *testing.T, ctx context.Context, ms *MediaSignerLocal, frag []byte, meta map[uint32]muxl.TrackMeta) [][]byte {
	t.Helper()
	labeled, err := muxl.RunMuxlCanonicalize(ctx, frag, nil, muxl.WithTrackMeta(meta))
	require.NoError(t, err)
	eventCh := make(chan *muxl.MuxlEvent, 16)
	errCh := make(chan error, 1)
	go func() {
		err := ms.SignSegmentStream(ctx, bytes.NewReader(labeled), eventCh, true)
		close(eventCh)
		errCh <- err
	}()
	var segs [][]byte
	for ev := range eventCh {
		if ev.Type == "signed-segment" {
			segs = append(segs, concatTracksByID(ev.Tracks))
		}
	}
	require.NoError(t, <-errCh)
	require.NotEmpty(t, segs)
	return segs
}

// Codec completion encodes the main group's track even when a stem comes
// first in track-id order: a mono stem on track 2 ahead of the stereo,
// AAC-only full mix on track 3. The completed Opus must be the mix's.
func TestStreamTranscoderCompletesTheMainGroup(t *testing.T) {
	ctx := context.Background()
	frag := runSynthPipeline(t, ctx, strings.Join([]string{
		"videotestsrc num-buffers=60 ! video/x-raw,width=320,height=240,framerate=30/1 ! x264enc key-int-max=30 tune=zerolatency speed-preset=ultrafast ! h264parse ! mp4mux name=mux fragment-duration=500 ! appsink name=sink",
		"audiotestsrc num-buffers=94 samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=1 ! audioconvert ! fdkaacenc ! aacparse ! mux.",
		"audiotestsrc num-buffers=94 samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! fdkaacenc ! aacparse ! mux.",
	}, "\n"))
	ms := newBareSegmentSigner(t)
	segs := signLabeledSegments(t, ctx, ms, frag, map[uint32]muxl.TrackMeta{
		2: {AlternateGroup: 2, Label: "Music"},
		3: {AlternateGroup: 1, Label: "Mix"},
	})

	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)
	mm := &MediaManager{cli: &config.CLI{BroadcasterHost: "test.example.com"}}
	// Close waits for the pipeline, which delivers every completion first.
	var completed [][]byte
	tr := mm.newStreamTranscoder(ctx, "opus", ms.Cert, keyPEM, func(_ any, c []byte) {
		completed = append(completed, c)
	})
	for i, s := range segs {
		require.NoError(t, tr.Feed(s, i))
	}
	require.NoError(t, tr.Close())
	require.NotEmpty(t, completed)

	for i, c := range completed {
		events, err := unwrapMuxlEvents(ctx, c)
		require.NoError(t, err)
		cat, _ := catalogAndTracks(events)
		var opus []muxl.MuxlAudioConfig
		for _, a := range audioTracks(cat) {
			if isOpusCodec(a.Codec) {
				opus = append(opus, a)
			}
		}
		require.Len(t, opus, 1, "segment %d", i)
		require.Equal(t, uint32(2), opus[0].NumberOfChannels, "segment %d: Opus encoded from the stereo mix, not the mono stem", i)
		require.Equal(t, uint32(1), opus[0].AlternateGroup, "segment %d", i)
		require.Equal(t, "Mix", opus[0].Label, "segment %d", i)
	}
}

// A syndicating node pairs a rendition with the source segment's Opus
// tracks: the mix and the stems, so a viewer who picked a rendition keeps
// the stems.
func TestPublishRenditionsForPlaybackCarriesStems(t *testing.T) {
	ctx := context.Background()
	src := groupedSignedSegment(t, ctx)
	rendition := runSynthPipeline(t, ctx,
		"videotestsrc num-buffers=30 ! video/x-raw,width=160,height=120,framerate=30/1 ! x264enc key-int-max=30 tune=zerolatency speed-preset=ultrafast ! h264parse ! mp4mux fragment-duration=1000 ! appsink name=sink")
	cert, keyPEM := nodeSignerForTest(t)
	b := bus.NewBus()
	mm := &MediaManager{cli: &config.CLI{BroadcasterHost: "node.test"}, liveWindows: map[string]*livehls.Writer{}, bus: b}
	addendum, err := mm.mintVideoRenditions(ctx, src, []RenditionInput{{Name: "120p", MP4: rendition}}, cert, keyPEM)
	require.NoError(t, err)
	require.NotEmpty(t, addendum)

	const did = "did:test:streamer"
	sub := b.SubscribeSegment(ctx, did, "120p")
	defer b.UnsubscribeSegment(ctx, did, "120p", sub)
	mm.rememberSourceAudio(ctx, did, src)
	mm.PublishRenditionsForPlayback(ctx, did, addendum, true)
	select {
	case seg := <-sub.C:
		packet := seg.PacketizedData
		require.NotNil(t, packet)
		require.NotEmpty(t, packet.Video, "video samples")
		require.NotEmpty(t, packet.Audio, "the mix's Opus")
		require.Len(t, packet.Stems, 1)
		require.Equal(t, uint32(2), packet.Stems[0].Group)
		require.InDelta(t, len(packet.Audio), len(packet.Stems[0].Audio), 1)
	case <-time.After(20 * time.Second):
		t.Fatal("no rendition segment published")
	}
}
