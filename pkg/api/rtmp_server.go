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
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
)

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
		VideoTracks: map[uint8]*format.H264{},
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

	// eRTMP multitrack (OBS "Multitrack Video") track count
	// Note that this currently only supports H.264 video and 1 track of AAC audio
	videoTrackCount := 0
	// relayDone is closed the moment RTMPIngest (the consumer of EventChan via
	// the internal playback relay) returns. The data callbacks below send into
	// EventChan synchronously inside r.Read(); if the relay dies nobody drains
	// that channel, an unbounded send blocks r.Read() forever, so the 10s read
	// deadline never fires and the session never tears down. Guarding every send
	// on relayDone turns a permanent hang into a quick drop-and-exit.
	relayDone := make(chan struct{})
	for _, track := range r.Tracks() {
		log.Log(ctx, "get track", "track", track)

		switch track := track.(type) {
		case *format.H264:
			if videoTrackCount >= constants.MaxVideoTracks {
				log.Log(ctx, "dropping extra H.264 track beyond limit", "track", videoTrackCount, "max", constants.MaxVideoTracks)
				continue
			}
			trackID := uint8(videoTrackCount)
			videoTrackCount++
			session.VideoTracks[trackID] = track
			r.OnDataH264(track, func(pts time.Duration, dts time.Duration, au [][]byte) {
				// Guarded send: if the relay is gone, drop the frame instead of
				// blocking inside r.Read() forever (see relayDone below).
				select {
				case session.EventChan <- &media.RTMPH264Data{
					TrackID: trackID,
					AU:      au,
					PTS:     pts,
					DTS:     dts,
				}:
				case <-relayDone:
				}
			})

		case *format.MPEG4Audio:
			if session.AudioTrack != nil {
				return fmt.Errorf("multitrack audio is not supported (send a single AAC audio track)")
			}
			session.AudioTrack = track
			r.OnDataMPEG4Audio(track, func(pts time.Duration, au []byte) {
				select {
				case session.EventChan <- &media.RTMPAACData{
					AU:  au,
					PTS: pts,
				}:
				case <-relayDone:
				}
			})

		default:
			// Unsupported track type: drop and log it, and keep the session
			// running on the remaining tracks, rather than rejecting the whole
			// stream over one unsupported track. If this drops the only video,
			// the videoTrackCount == 0 check after the loop fails the session.
			log.Log(ctx, "dropping unsupported track", "type", fmt.Sprintf("%T", track))
		}
	}
	if videoTrackCount == 0 {
		return fmt.Errorf("no video track found")
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		for {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			select {
			case <-relayDone:
				return fmt.Errorf("relay stopped, ending publish session")
			default:
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
		defer close(relayDone)
		return a.MediaManager.RTMPIngest(ctx, fmt.Sprintf("rtmp://%s/live/%s", a.rtmpInternalPlaybackAddr, streamer), mediaSigner, videoTrackCount)
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

	// Video tracks first, in session track-ID order. gortmplib assigns wire
	// track IDs by slice position, so this keeps relay IDs identical to the
	// session's
	tracks := make([]format.Format, 0, len(session.VideoTracks)+1)
	for i := 0; i < len(session.VideoTracks); i++ {
		track, ok := session.VideoTracks[uint8(i)]
		if !ok {
			return fmt.Errorf("missing video track %d", i)
		}
		tracks = append(tracks, track)
	}
	if session.AudioTrack != nil {
		tracks = append(tracks, session.AudioTrack)
	}

	w := &gortmplib.Writer{
		Conn:   sc,
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
				track, ok := session.VideoTracks[event.TrackID]
				if !ok {
					return fmt.Errorf("unknown video track ID: %d", event.TrackID)
				}
				err := w.WriteH264(track, event.PTS, event.DTS, event.AU)
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
