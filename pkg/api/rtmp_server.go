// Package main contains an example.
package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/abema/go-mp4"
	"github.com/bluenviron/gortmplib"
	"github.com/bluenviron/gortmplib/pkg/codecs"
	"github.com/bluenviron/gortmplib/pkg/message"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/av1"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
)

// This example shows how to:
// 1. create a RTMP server
// 2. accept a stream from a reader.
// 3. broadcast the stream to readers.

var RTMPTimeout = 10 * time.Second

const RTMPPrefix = "/live/"

func (a *StreamplaceAPI) HandleRTMPPublisher(ctx context.Context, sc *gortmplib.ServerConn) error {
	err := sc.RW.(net.Conn).SetReadDeadline(time.Now().Add(RTMPTimeout))
	if err != nil {
		return err
	}

	if !strings.HasPrefix(sc.URL.Path, RTMPPrefix) {
		return fmt.Errorf("RTMP publisher is not allowed to publish to %s (must start with %s)", sc.URL.String(), RTMPPrefix)
	}
	streamKey := strings.TrimPrefix(sc.URL.Path, RTMPPrefix)
	mediaSigner, err := a.MakeMediaSigner(ctx, streamKey)
	if err != nil {
		return fmt.Errorf("failed to make media signer: %w", err)
	}

	streamer := mediaSigner.Streamer()
	ctx = log.WithLogValues(ctx, "streamer", streamer)
	session := &media.RTMPSession{
		EventChan:   make(chan any, 1024),
		MediaSigner: mediaSigner,
	}
	a.rtmpSessionsLock.Lock()
	a.rtmpSessions[streamer] = session
	a.rtmpSessionsLock.Unlock()

	defer func() {
		a.rtmpSessionsLock.Lock()
		delete(a.rtmpSessions, streamer)
		a.rtmpSessionsLock.Unlock()
		close(session.EventChan)
	}()

	g, ctx := errgroup.WithContext(ctx)
	r, err := initializeRTMPReader(ctx, sc, session)
	if err != nil {
		return err
	}

	stopRead := context.AfterFunc(ctx, func() { _ = sc.RW.(net.Conn).Close() })
	defer stopRead()
	g.Go(func() error {
		for {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			err = sc.RW.(net.Conn).SetReadDeadline(time.Now().Add(RTMPTimeout))
			if err != nil {
				return err
			}
			err = r.Read()
			if err != nil {
				return err
			}
		}
	})

	g.Go(func() error {
		return a.MediaManager.RTMPIngest(ctx, fmt.Sprintf("rtmp://%s/live/%s", a.rtmpInternalPlaybackAddr, streamer), mediaSigner, session.VideoCodec())
	})

	return g.Wait()
}

// initializeRTMPReader is shared by live publishers and the unpublished shadow.
func initializeRTMPReader(ctx context.Context, sc *gortmplib.ServerConn, session *media.RTMPSession) (*gortmplib.Reader, error) {
	// The Reader accepts the publish only on a bare *ServerConn; wrapped,
	// it is ours to accept.
	if err := sc.AcceptAction(); err != nil {
		return nil, err
	}
	conn := &av1SequenceStartConn{Conn: sc}
	r := &gortmplib.Reader{Conn: conn}
	if err := r.Initialize(); err != nil {
		return nil, err
	}
	session.AV1SequenceHeader = conn.seq
	send := func(event any) {
		select {
		case session.EventChan <- event:
		case <-ctx.Done():
		}
	}
	for _, track := range r.Tracks() {
		switch track.Codec.(type) {
		case *codecs.H264:
			session.VideoTrack = track
			r.OnDataH264(track, func(pts, dts time.Duration, au [][]byte) {
				send(&media.RTMPH264Data{AU: au, PTS: pts, DTS: dts})
			})
		case *codecs.AV1:
			session.VideoTrack = track
			r.OnDataAV1(track, func(pts time.Duration, tu [][]byte) {
				send(&media.RTMPAV1Data{TU: tu, PTS: pts})
			})
		case *codecs.MPEG4Audio:
			session.AudioTrack = track
			r.OnDataMPEG4Audio(track, func(pts time.Duration, au []byte) {
				send(&media.RTMPAACData{AU: au, PTS: pts})
			})
		default:
			return nil, fmt.Errorf("unsupported track codec: %T", track.Codec)
		}
	}
	return r, nil
}

// av1SequenceStartConn keeps the sequence header OBU of the publisher's AV1
// sequence start, which gortmplib's Reader parses for the track and discards.
// Only the one read during Initialize is used: the Reader ignores later
// sequence starts too, and the relay follows in-band sequence headers.
type av1SequenceStartConn struct {
	gortmplib.Conn
	seq []byte
}

func (c *av1SequenceStartConn) Read() (message.Message, error) {
	msg, err := c.Conn.Read()
	if start, ok := msg.(*message.VideoExSequenceStart); ok && start.FourCC == message.FourCCAV1 {
		var obus av1.Bitstream
		if obus.Unmarshal(start.AV1Config.ConfigOBUs) == nil {
			for _, obu := range obus {
				var h av1.OBUHeader
				if h.Unmarshal(obu) == nil && h.Type == av1.OBUTypeSequenceHeader {
					c.seq = obu
				}
			}
		}
	}
	return msg, err
}

func (a *StreamplaceAPI) HandleRTMPPlayback(ctx context.Context, sc *gortmplib.ServerConn) error {
	if !strings.HasPrefix(sc.URL.Path, RTMPPrefix) {
		return fmt.Errorf("RTMP publisher is not allowed to publish to %s (must start with %s)", sc.URL.String(), RTMPPrefix)
	}
	streamer := strings.TrimPrefix(sc.URL.Path, RTMPPrefix)
	a.rtmpSessionsLock.Lock()
	session, ok := a.rtmpSessions[streamer]
	a.rtmpSessionsLock.Unlock()
	if !ok {
		return fmt.Errorf("RTMP session not found for streamer %s", streamer)
	}

	return relayRTMPSession(ctx, sc, session)
}

// relayRTMPSession writes a publisher's session out to conn, the internal
// playback connection the ingest pipeline reads with rtmp2src, until the
// session closes or ctx ends.
func relayRTMPSession(ctx context.Context, conn gortmplib.Conn, session *media.RTMPSession) error {
	tracks := []*gortmplib.Track{}
	if session.VideoTrack != nil {
		tracks = append(tracks, session.VideoTrack)
	}
	if session.AudioTrack != nil {
		tracks = append(tracks, session.AudioTrack)
	}
	w := &gortmplib.Writer{
		Conn:   conn,
		Tracks: tracks,
	}
	err := w.Initialize()
	if err != nil {
		return err
	}
	var av1Seq []byte // the sequence header OBU the relay's av1C was built from
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-session.EventChan:
			if event == nil {
				return fmt.Errorf("RTMP session closed: %w", io.EOF)
			}
			switch event := event.(type) {
			case *media.RTMPH264Data:
				// gortmplib's reader hands on an AVC sequence header as an AU
				// of just SPS and PPS, including the stream's first one, which
				// it replays after reading ahead for the tracks. Written back as
				// a frame, it takes the timestamp of the keyframe after it, and
				// h264parse drops that keyframe's PTS, which fails the
				// segmenter's muxer. Write it as the sequence header it was.
				if msg, ok := h264ConfigMessage(event.AU, event.DTS); ok {
					if err := conn.Write(msg); err != nil {
						return fmt.Errorf("error writing H264 config: %w", err)
					}
					continue
				}
				err := w.WriteH264(session.VideoTrack, event.PTS, event.DTS, event.AU)
				if err != nil {
					return fmt.Errorf("error writing H264: %w", err)
				}
			case *media.RTMPAV1Data:
				// gortmplib's reader drops the publisher's av1C and its writer
				// announces a placeholder, so the relay rebuilds the record from
				// the sequence header OBU the stream carries, or else the one
				// the publisher's sequence start carried, and holds frames until
				// it has one: nothing can decode them before it.
				var seq []byte
				key := false
				tu := slices.DeleteFunc(event.TU, func(obu []byte) bool {
					var h av1.OBUHeader
					if h.Unmarshal(obu) != nil {
						return false
					}
					if h.Type == av1.OBUTypeSequenceHeader {
						seq = obu
					}
					key = key || media.VideoAV1.IsKeyframe(obu)
					// WriteAV1 writes the temporal unit's own delimiter.
					return h.Type == av1.OBUTypeTemporalDelimiter
				})
				inBand := seq != nil
				if !inBand && av1Seq == nil {
					seq = session.AV1SequenceHeader
				}
				if seq != nil && !bytes.Equal(seq, av1Seq) {
					msg, err := av1ConfigMessage(seq)
					if err != nil {
						return err
					}
					if err := conn.Write(msg); err != nil {
						return fmt.Errorf("error writing AV1 config: %w", err)
					}
					av1Seq = seq
				}
				if av1Seq == nil {
					continue
				}
				// av1parse ignores the av1C, and a segment starting at a key
				// frame must carry the sequence header, so an encoder that
				// sends it only in the sequence start gets it in-band here.
				if key && !inBand {
					tu = slices.Insert(tu, 0, av1Seq)
				}
				err := w.WriteAV1(session.VideoTrack, event.PTS, tu)
				if err != nil {
					return fmt.Errorf("error writing AV1: %w", err)
				}
			case *media.RTMPAACData:
				err := w.WriteMPEG4Audio(session.AudioTrack, event.PTS, event.AU)
				if err != nil {
					return fmt.Errorf("error writing MPEG4Audio: %w", err)
				}
			default:
				return fmt.Errorf("unsupported event type: %T", event)
			}
		}
	}
}

func (a *StreamplaceAPI) HandleRTMPPublishConn(ctx context.Context, conn net.Conn) error {
	err := conn.SetReadDeadline(time.Now().Add(RTMPTimeout))
	if err != nil {
		return err
	}

	sc := &gortmplib.ServerConn{
		RW: conn,
	}
	err = sc.Initialize()
	if err != nil {
		return err
	}

	err = sc.AcceptConn()
	if err != nil {
		return err
	}

	if sc.Publish {
		return a.HandleRTMPPublisher(ctx, sc)
	}
	return fmt.Errorf("RTMP playback is not allowed")
}

func (a *StreamplaceAPI) HandleRTMPPlaybackConn(ctx context.Context, conn net.Conn) error {
	err := conn.SetReadDeadline(time.Now().Add(RTMPTimeout))
	if err != nil {
		return err
	}

	sc := &gortmplib.ServerConn{
		RW: conn,
	}
	err = sc.Initialize()
	if err != nil {
		return err
	}

	err = sc.AcceptConn()
	if err != nil {
		return err
	}

	if !sc.Publish {
		return a.HandleRTMPPlayback(ctx, sc)
	}
	return fmt.Errorf("RTMP playback is not allowed")
}

func (a *StreamplaceAPI) ServeRTMP(ctx context.Context) error {
	ln, err := net.Listen("tcp", a.CLI.RTMPAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	defer ln.Close()

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	log.Log(ctx, "rtmp server starting", "addr", a.CLI.RTMPAddr)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return a.ServeRTMPInternalPlayback(ctx)
	})
	g.Go(func() error {
		for {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			conn, err := ln.Accept()
			if err != nil {
				return fmt.Errorf("error accepting RTMP connection: %w", err)
			}
			go func() {
				err := a.HandleRTMPPublishConn(ctx, conn)
				if err != nil {
					log.Error(ctx, "error handling RTMP publish connection", "error", err)
				}
			}()
		}
	})

	return g.Wait()
}

// Serve RTMP internal playback server for gstreamer to pull from
func (a *StreamplaceAPI) ServeRTMPInternalPlayback(ctx context.Context) error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	addr := ln.Addr().String()
	defer ln.Close()

	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("failed to split host and port: %w", err)
	}

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	a.rtmpInternalPlaybackAddr = fmt.Sprintf("127.0.0.1:%s", port)

	log.Log(ctx, "rtmp internal playback server starting", "addr", a.rtmpInternalPlaybackAddr)

	// Accept loop in a goroutine so we can select on context.Done
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		conn, err := ln.Accept()
		if err != nil {
			return fmt.Errorf("error accepting RTMP connection: %w", err)
		}

		go func() {
			err := a.HandleRTMPPlaybackConn(ctx, conn)
			if err != nil {
				log.Error(ctx, "error handling RTMP internal playback connection", "error", err)
			}
		}()
	}
}

func (a *StreamplaceAPI) ServeRTMPS(ctx context.Context, cli *config.CLI) error {
	var tlsConfig *tls.Config
	if a.ACME != nil {
		tlsConfig = a.ACME.TLSConfig()
	} else {
		cert, err := tls.LoadX509KeyPair(cli.TLSCertPath, cli.TLSKeyPath)
		if err != nil {
			return fmt.Errorf("failed to load TLS certificate: %w", err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
	}

	ln, err := tls.Listen("tcp", cli.RTMPSAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to create RTMPS listener: %w", err)
	}

	log.Log(ctx, "rtmps server starting", "addr", cli.RTMPAddr)

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return a.ServeRTMPInternalPlayback(ctx)
	})
	g.Go(func() error {
		for {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			conn, err := ln.Accept()
			if err != nil {
				return fmt.Errorf("error accepting RTMP connection: %w", err)
			}
			go func() {
				err := a.HandleRTMPPublishConn(ctx, conn)
				if err != nil {
					log.Error(ctx, "error handling RTMP publish connection", "error", err)
				}
			}()
		}
	})

	return g.Wait()
}

// h264ConfigMessage returns the FLV sequence header for an H.264 access unit
// that holds nothing but parameter sets (one SPS and one PPS), or false for
// any other AU.
func h264ConfigMessage(au [][]byte, dts time.Duration) (*message.Video, bool) {
	var sps, pps []byte
	for _, nalu := range au {
		if len(nalu) == 0 {
			return nil, false
		}
		switch nalu[0] & 0x1f {
		case 7: // SPS
			if sps != nil {
				return nil, false
			}
			sps = nalu
		case 8: // PPS
			if pps != nil {
				return nil, false
			}
			pps = nalu
		default:
			return nil, false
		}
	}
	// profile_idc, constraint flags and level_idc follow the NAL header
	if len(sps) < 4 || pps == nil {
		return nil, false
	}
	avcc := &mp4.AVCDecoderConfiguration{
		ConfigurationVersion:       1,
		Profile:                    sps[1],
		ProfileCompatibility:       sps[2],
		Level:                      sps[3],
		Reserved:                   0b111111,
		LengthSizeMinusOne:         3,
		Reserved2:                  0b111,
		NumOfSequenceParameterSets: 1,
		SequenceParameterSets:      []mp4.AVCParameterSet{{Length: uint16(len(sps)), NALUnit: sps}},
		NumOfPictureParameterSets:  1,
		PictureParameterSets:       []mp4.AVCParameterSet{{Length: uint16(len(pps)), NALUnit: pps}},
	}
	avcc.SetType(mp4.BoxTypeAvcC())
	return &message.Video{
		ChunkStreamID:   message.VideoChunkStreamID,
		MessageStreamID: 0x1000000,
		Codec:           message.CodecH264,
		FrameType:       message.VideoFrameTypeKeyFrame,
		PacketType:      message.VideoPacketTypeConfig,
		AVCConfig:       avcc,
		DTS:             dts,
	}, true
}

// av1ConfigMessage returns the Enhanced RTMP sequence start for an AV1 stream
// with sequence header OBU seq (as gortmplib splits OBUs: no size field): the
// AV1CodecConfigurationRecord flvdemux hands on as the stream's codec_data.
func av1ConfigMessage(seq []byte) (*message.VideoExSequenceStart, error) {
	av1C, err := media.AV1CodecConfig(seq)
	if err != nil {
		return nil, err
	}
	return &message.VideoExSequenceStart{
		ChunkStreamID:   message.VideoChunkStreamID,
		MessageStreamID: 0x1000000,
		FourCC:          message.FourCCAV1,
		AV1Config:       av1C,
	}, nil
}
