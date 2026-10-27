package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/bluenviron/gortmplib"
	"github.com/bluenviron/gortmplib/pkg/bytecounter"
	"github.com/bluenviron/gortmplib/pkg/codecs"
	"github.com/bluenviron/gortmplib/pkg/message"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/av1"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4"
	mp4codecs "github.com/bluenviron/mediacommon/v2/pkg/formats/mp4/codecs"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/muxl"
)

// testSPS is a real 352x288 High profile SPS; gortmplib's writer parses it.
var testSPS = []byte{
	0x67, 0x64, 0x00, 0x0c, 0xac, 0x3b, 0x50, 0xb0,
	0x4b, 0x42, 0x00, 0x00, 0x03, 0x00, 0x02, 0x00,
	0x00, 0x03, 0x00, 0x3d, 0x08,
}

func TestH264ConfigMessage(t *testing.T) {
	sps := testSPS
	pps := []byte{0x68, 0xce, 0x3c, 0x80}
	idr := []byte{0x65, 0x88, 0x84}
	slice := []byte{0x41, 0x9a}

	msg, ok := h264ConfigMessage([][]byte{sps, pps}, 40*time.Millisecond)
	require.True(t, ok)
	require.Equal(t, message.VideoPacketTypeConfig, msg.PacketType)
	require.EqualValues(t, message.CodecH264, msg.Codec)
	require.Equal(t, 40*time.Millisecond, msg.DTS)
	requireAVCConfig(t, msg, sps, pps)

	for name, au := range map[string][][]byte{
		"keyframe with parameter sets": {sps, pps, idr},
		"slice":                        {slice},
		"SPS alone":                    {sps},
		"two SPS":                      {sps, sps, pps},
		"empty NALU":                   {sps, {}, pps},
		"empty AU":                     nil,
	} {
		_, ok := h264ConfigMessage(au, 0)
		require.False(t, ok, name)
	}
}

// recordingConn is a gortmplib.Conn that keeps what's written to it.
type recordingConn struct {
	msgs []message.Message
}

func (c *recordingConn) BytesReceived() uint64          { return 0 }
func (c *recordingConn) BytesSent() uint64              { return 0 }
func (c *recordingConn) Read() (message.Message, error) { return nil, errors.New("write-only") }
func (c *recordingConn) Write(msg message.Message) error {
	c.msgs = append(c.msgs, msg)
	return nil
}

// What the relay writes for a stream's start as gortmplib's reader hands it
// on: the replayed sequence header as an SPS/PPS access unit, then the first
// keyframe at the same timestamp. The header has to go out as a sequence
// header, not a frame of its own, so the keyframe is the only frame at that
// timestamp.
func TestRelayRTMPSessionSequenceHeader(t *testing.T) {
	sps := testSPS
	pps := []byte{0x68, 0xce, 0x3c, 0x80}
	idr := []byte{0x65, 0x88, 0x84, 0x00}
	slice := []byte{0x41, 0x9a, 0x02}

	session := &media.RTMPSession{
		EventChan:  make(chan any, 8),
		VideoTrack: &gortmplib.Track{Codec: &codecs.H264{SPS: sps, PPS: pps}},
	}
	session.EventChan <- &media.RTMPH264Data{AU: [][]byte{sps, pps}}
	session.EventChan <- &media.RTMPH264Data{AU: [][]byte{idr}}
	session.EventChan <- &media.RTMPH264Data{AU: [][]byte{slice}, PTS: 33 * time.Millisecond, DTS: 33 * time.Millisecond}
	close(session.EventChan)

	conn := &recordingConn{}
	err := relayRTMPSession(context.Background(), conn, session)
	require.ErrorContains(t, err, "session closed")

	var video []*message.Video
	for _, msg := range conn.msgs {
		if v, ok := msg.(*message.Video); ok {
			video = append(video, v)
		}
	}
	// the writer's own header for the track, the relayed one, then frames
	require.Len(t, video, 4)
	for i, want := range []struct {
		typ   message.VideoPacketType
		dts   time.Duration
		frame message.VideoFrameType
	}{
		{message.VideoPacketTypeConfig, 0, message.VideoFrameTypeKeyFrame},
		{message.VideoPacketTypeConfig, 0, message.VideoFrameTypeKeyFrame},
		{message.VideoPacketTypeAU, 0, message.VideoFrameTypeKeyFrame},
		{message.VideoPacketTypeAU, 33 * time.Millisecond, message.VideoFrameTypeInterFrame},
	} {
		require.Equal(t, want.typ, video[i].PacketType, "message %d", i)
		require.Equal(t, want.dts, video[i].DTS, "message %d", i)
		require.Equal(t, want.frame, video[i].FrameType, "message %d", i)
	}
	requireAVCConfig(t, video[1], sps, pps)
}

// requireAVCConfig checks that msg's sequence header survives the wire: it
// is marshalled and read back through gortmplib's own parser.
func requireAVCConfig(t *testing.T, msg *message.Video, sps, pps []byte) {
	t.Helper()
	var buf bytes.Buffer
	rw := message.NewReadWriter(&buf, bytecounter.NewReadWriter(&buf), false)
	require.NoError(t, rw.Write(msg))
	read, err := rw.Read()
	require.NoError(t, err)
	got, ok := read.(*message.Video)
	require.True(t, ok)
	require.NotNil(t, got.AVCConfig)
	require.Equal(t, sps, got.AVCConfig.SequenceParameterSets[0].NALUnit)
	require.Equal(t, pps, got.AVCConfig.PictureParameterSets[0].NALUnit)
}

// An Enhanced RTMP AV1 publish (OBS 29.1+'s av01) goes through the real relay
// and ingest pipeline (flvdemux, av1parse, the muxl signer) to signed segments
// whose catalog names the publisher's AV1 stream. The codec string comes from
// the av1C the relay rebuilds out of the stream's sequence header; gortmplib's
// placeholder record would read av01.0.08M.08. An encoder needn't repeat the
// sequence header in-band, so a publish that carries it only in its sequence
// start must work too.
func TestRTMPIngestAV1(t *testing.T) {
	t.Run("in-band sequence header", func(t *testing.T) { testRTMPIngestAV1(t, false) })
	t.Run("sequence start only", func(t *testing.T) { testRTMPIngestAV1(t, true) })
}

// replaceSequenceStart publishes start in place of gortmplib's placeholder
// AV1 sequence start.
type replaceSequenceStart struct {
	gortmplib.Conn
	start *message.VideoExSequenceStart
}

func (c *replaceSequenceStart) Write(msg message.Message) error {
	if _, ok := msg.(*message.VideoExSequenceStart); ok {
		msg = c.start
	}
	return c.Conn.Write(msg)
}

func testRTMPIngestAV1(t *testing.T, startOnly bool) {
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	gstinit.InitGST()

	// An encoder's frames, in timestamp order, from a fragmented MP4 of
	// AV1 320x240 and AAC.
	buf, err := os.ReadFile("../../test/fixtures/av1-aac-frag.mp4")
	require.NoError(t, err)
	var init fmp4.Init
	require.NoError(t, init.Unmarshal(bytes.NewReader(buf)))
	var parts fmp4.Parts
	require.NoError(t, parts.Unmarshal(buf))
	video := &gortmplib.Track{Codec: &codecs.AV1{}}
	var audio *gortmplib.Track
	var seq []byte
	tracks := map[int]*fmp4.InitTrack{}
	for _, track := range init.Tracks {
		tracks[track.ID] = track
		switch c := track.Codec.(type) {
		case *mp4codecs.MPEG4Audio:
			audio = &gortmplib.Track{Codec: &codecs.MPEG4Audio{Config: &c.Config}}
		case *mp4codecs.AV1:
			seq = c.SequenceHeader
		}
	}
	require.NotNil(t, audio)
	type frame struct {
		pts time.Duration
		tu  [][]byte // video, or else
		au  []byte   // audio
	}
	var frames []frame
	for _, part := range parts {
		for _, pt := range part.Tracks {
			track := tracks[pt.ID]
			ts := pt.BaseTime
			for _, s := range pt.Samples {
				f := frame{pts: time.Duration(int64(ts)+int64(s.PTSOffset)) * time.Second / time.Duration(track.TimeScale)}
				ts += uint64(s.Duration)
				if _, ok := track.Codec.(*mp4codecs.AV1); ok {
					var tu av1.Bitstream
					require.NoError(t, tu.Unmarshal(s.Payload))
					f.tu = tu
				} else {
					f.au = s.Payload
				}
				frames = append(frames, f)
			}
		}
	}
	sort.SliceStable(frames, func(i, j int) bool { return frames[i].pts < frames[j].pts })

	publishLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer publishLn.Close()
	playbackLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer playbackLn.Close()
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		u, err := url.Parse("rtmp://" + publishLn.Addr().String() + "/live/av1")
		if err != nil {
			return err
		}
		c := &gortmplib.Client{URL: u, Publish: true}
		if err := c.Initialize(gctx); err != nil {
			return err
		}
		defer c.Close()
		var conn gortmplib.Conn = c
		if startOnly {
			start, err := av1ConfigMessage(seq)
			if err != nil {
				return err
			}
			conn = &replaceSequenceStart{Conn: c, start: start}
		}
		w := &gortmplib.Writer{Conn: conn, Tracks: []*gortmplib.Track{video, audio}}
		if err := w.Initialize(); err != nil {
			return err
		}
		for _, f := range frames {
			if f.tu != nil {
				tu := f.tu
				if startOnly {
					tu = slices.DeleteFunc(slices.Clone(tu), func(obu []byte) bool {
						var h av1.OBUHeader
						return h.Unmarshal(obu) == nil && h.Type == av1.OBUTypeSequenceHeader
					})
				}
				err = w.WriteAV1(video, f.pts, tu)
			} else {
				err = w.WriteMPEG4Audio(audio, f.pts, f.au)
			}
			if err != nil {
				return err
			}
		}
		// Half-close and drain, so the server's unread acknowledgements
		// can't reset the connection under media it hasn't read yet.
		if err := c.NetConn().(*net.TCPConn).CloseWrite(); err != nil {
			return err
		}
		_, err = io.Copy(io.Discard, c.NetConn())
		return err
	})

	// The publisher's side, as HandleRTMPPublisher runs it.
	conn, err := publishLn.Accept()
	require.NoError(t, err)
	defer conn.Close()
	sc := &gortmplib.ServerConn{RW: conn}
	require.NoError(t, sc.Initialize())
	require.NoError(t, sc.AcceptConn())
	ms, err := media.NewEphemeralMediaSigner("did:example:rtmp-av1")
	require.NoError(t, err)
	session := &media.RTMPSession{EventChan: make(chan any, 1024), MediaSigner: ms}
	r, err := initializeRTMPReader(gctx, sc, session)
	require.NoError(t, err)
	require.Equal(t, media.VideoAV1.Name, session.VideoCodec().Name)
	g.Go(func() error {
		defer conn.Close()
		defer close(session.EventChan)
		for {
			if err := r.Read(); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
		}
	})
	a := &StreamplaceAPI{rtmpSessions: map[string]*media.RTMPSession{ms.Streamer(): session}}
	g.Go(func() error {
		conn, err := playbackLn.Accept()
		if err != nil {
			return err
		}
		defer conn.Close()
		err = a.HandleRTMPPlaybackConn(gctx, conn)
		if !errors.Is(err, io.EOF) {
			return err
		}
		// Everything is relayed: let rtmp2src read it all to EOS.
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			return err
		}
		if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
			return err
		}
		_, err = io.Copy(io.Discard, conn)
		return err
	})
	var segments [][]byte
	g.Go(func() error {
		return media.RTMPIngestUnpublished(gctx, &config.CLI{}, "rtmp://"+playbackLn.Addr().String()+"/live/"+ms.Streamer(), ms, session.VideoCodec(), func(_ context.Context, segment []byte) error {
			segments = append(segments, segment)
			return nil
		})
	})
	require.NoError(t, g.Wait())

	require.NotEmpty(t, segments)
	for _, segment := range segments {
		events := make(chan *muxl.MuxlEvent, 16)
		done := make(chan error, 1)
		go func() {
			done <- muxl.RunMuxlUnwrapEvents(ctx, bytes.NewReader(segment), events)
			close(events)
		}()
		var codecs []string
		for ev := range events {
			if ev.Type == "init" && ev.Catalog != nil && ev.Catalog.Video != nil {
				for _, v := range ev.Catalog.Video.Renditions {
					codecs = append(codecs, v.Codec)
				}
			}
		}
		require.NoError(t, <-done)
		require.Equal(t, []string{"av01.0.00M.08"}, codecs)
	}
}
