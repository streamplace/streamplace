// Package main contains an example.
package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/bluenviron/gortmplib"
	"github.com/bluenviron/gortmplib/pkg/h264conf"
	"github.com/bluenviron/gortmplib/pkg/message"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/rtmps"
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

	listener, sni := rtmps.ListenerRTMP, ""
	if tc, ok := sc.RW.(*tls.Conn); ok {
		listener, sni = rtmps.ListenerRTMPS, tc.ConnectionState().ServerName
	}
	// sc.URL's scheme and host are the connect command's tcUrl.
	release := a.IngestHosts.Open(ctx, listener, sni, sc.URL.String(), streamer)
	defer release()

	defer func() {
		a.rtmpSessionsLock.Lock()
		delete(a.rtmpSessions, streamer)
		a.rtmpSessionsLock.Unlock()
		close(session.EventChan)
	}()

	r := &gortmplib.Reader{
		Conn: sc,
	}
	err = r.Initialize()
	if err != nil {
		return err
	}

	for _, track := range r.Tracks() {
		log.Log(ctx, "get track", "track", track)

		switch track := track.(type) {
		case *format.H264:
			session.VideoTrack = track
			r.OnDataH264(track, func(pts time.Duration, dts time.Duration, au [][]byte) {
				// log.Log(ctx, "got H264", "len", len(au), "pts", pts, "dts", dts)
				session.EventChan <- &media.RTMPH264Data{
					AU:  au,
					PTS: pts,
					DTS: dts,
				}
			})

		case *format.MPEG4Audio:
			session.AudioTrack = track
			r.OnDataMPEG4Audio(track, func(pts time.Duration, au []byte) {
				// log.Log(ctx, "got MPEG4Au", "len", len(au), "pts", pts)
				session.EventChan <- &media.RTMPAACData{
					AU:  au,
					PTS: pts,
				}
			})

		default:
			return fmt.Errorf("unsupported track type: %T", track)
		}
	}

	g, ctx := errgroup.WithContext(ctx)
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
		return a.MediaManager.RTMPIngest(ctx, fmt.Sprintf("rtmp://%s/live/%s", a.rtmpInternalPlaybackAddr, streamer), mediaSigner)
	})

	return g.Wait()
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
	tracks := []format.Format{}
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
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-session.EventChan:
			if event == nil {
				return fmt.Errorf("RTMP session closed")
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

	err = sc.Accept()
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

	err = sc.Accept()
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
	var conf h264conf.Conf
	for _, nalu := range au {
		if len(nalu) == 0 {
			return nil, false
		}
		switch nalu[0] & 0x1f {
		case 7: // SPS
			if conf.SPS != nil {
				return nil, false
			}
			conf.SPS = nalu
		case 8: // PPS
			if conf.PPS != nil {
				return nil, false
			}
			conf.PPS = nalu
		default:
			return nil, false
		}
	}
	if conf.SPS == nil || conf.PPS == nil {
		return nil, false
	}
	buf, err := conf.Marshal()
	if err != nil {
		return nil, false
	}
	return &message.Video{
		ChunkStreamID:   message.VideoChunkStreamID,
		MessageStreamID: 0x1000000,
		Codec:           message.CodecH264,
		IsKeyFrame:      true,
		Type:            message.VideoTypeConfig,
		Payload:         buf,
		DTS:             dts,
	}, true
}
