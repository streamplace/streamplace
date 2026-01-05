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
	"github.com/bluenviron/gortsplib/v5/pkg/format"
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

	w := &gortmplib.Writer{
		Conn:   sc,
		Tracks: []format.Format{session.VideoTrack, session.AudioTrack},
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
	cert, err := tls.LoadX509KeyPair(cli.TLSCertPath, cli.TLSKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load TLS certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
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
