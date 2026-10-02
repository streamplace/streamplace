package api

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bluenviron/gortmplib"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
)

type replayRTMP struct{ io.Reader }

func (replayRTMP) Write(p []byte) (int, error) { return len(p), nil }

var errShadowIdleTimeout = errors.New("shadow RTMP input idle timeout")

type shadowIdleReader struct {
	io.Reader
	timer *time.Timer
}

func (r shadowIdleReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err != nil {
		// Publisher EOF starts downstream drain, not another input deadline.
		r.timer.Stop()
	} else if n > 0 {
		r.timer.Reset(RTMPTimeout)
	}
	return n, err
}

// RunDuplicateMistWorker consumes the decrypted, client-only RTMP conversation.
// gortmplib's non-strict plain handshake consumes C2 without comparing it to
// the shadow's S1, so the publisher's response to Mist's handshake is valid.
// All shadow replies are discarded; only Mist negotiates with the publisher.
// This runs ONLY in a child process: no auth, database, storage, or live state.
func RunDuplicateMistWorker(parent context.Context, input io.Reader) (retErr error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	ctx = log.WithLogValues(ctx, "component", "duplicate-mist-test")
	stopInput := context.AfterFunc(ctx, func() {
		if closer, ok := input.(io.Closer); ok {
			_ = closer.Close()
		}
	})
	defer stopInput()
	var timedOut atomic.Bool
	idle := time.AfterFunc(RTMPTimeout, func() {
		timedOut.Store(true)
		cancel()
	})
	defer func() {
		idle.Stop()
		if timedOut.Load() {
			retErr = errShadowIdleTimeout
		}
	}()

	br := bufio.NewReader(shadowIdleReader{Reader: input, timer: idle})
	version, err := br.Peek(1)
	if err != nil {
		return fmt.Errorf("shadow RTMP handshake: %w", err)
	}
	if version[0] != 3 {
		return fmt.Errorf("shadow requires plain RTMP inside TLS (version 3); RTMPE cannot be passively replayed")
	}
	sc := &gortmplib.ServerConn{RW: replayRTMP{br}}
	if err := sc.Initialize(); err != nil {
		return shadowProtocolError("initialize", err)
	}
	if err := sc.Accept(); err != nil {
		return shadowProtocolError("accept", err)
	}
	if !sc.Publish {
		return fmt.Errorf("shadow connection is not an RTMP publisher")
	}
	if !strings.HasPrefix(sc.URL.Path, RTMPPrefix) {
		return fmt.Errorf("shadow publisher path must start with /live/")
	}
	// Never use the supplied stream key: the signer and session identity are
	// throwaway, and only the media-only verifier consumes their output.
	ms, err := media.NewEphemeralMediaSigner("did:example:rtmp-shadow")
	if err != nil {
		return fmt.Errorf("shadow signer: %w", err)
	}
	session := &media.RTMPSession{EventChan: make(chan any, 1024), MediaSigner: ms}
	r, err := initializeRTMPReader(ctx, sc, session)
	if err != nil {
		return shadowProtocolError("read tracks", err)
	}
	gstinit.InitGST()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("shadow playback listener: %w", err)
	}
	defer ln.Close()
	stopListener := context.AfterFunc(ctx, func() { _ = ln.Close() })
	defer stopListener()
	a := &StreamplaceAPI{rtmpSessions: map[string]*media.RTMPSession{ms.Streamer(): session}}
	g, groupCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		defer close(session.EventChan)
		for groupCtx.Err() == nil {
			if err := r.Read(); err != nil {
				if errors.Is(err, io.EOF) || groupCtx.Err() != nil {
					return nil
				}
				cancel()
				return shadowProtocolError("read media", err)
			}
		}
		return nil
	})
	g.Go(func() error {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			cancel()
			return fmt.Errorf("shadow playback accept: %w", err)
		}
		defer conn.Close()
		stop := context.AfterFunc(groupCtx, func() { _ = conn.Close() })
		defer stop()
		err = a.HandleRTMPPlaybackConn(groupCtx, conn)
		if errors.Is(err, io.EOF) {
			// A full close with unread RTMP acknowledgements can reset TCP,
			// discarding buffered media. FIN follows every written byte; keep
			// draining peer replies until the native pipeline reaches EOS.
			err = conn.(*net.TCPConn).CloseWrite()
			if err == nil {
				_, err = io.Copy(io.Discard, conn)
			}
		}
		if err == nil || groupCtx.Err() != nil {
			return nil
		}
		cancel()
		return fmt.Errorf("shadow native relay: %w", err)
	})
	var verified int
	var lastProgress time.Time
	g.Go(func() error {
		err := media.RTMPIngestUnpublished(groupCtx, &config.CLI{}, "rtmp://"+ln.Addr().String()+"/live/"+ms.Streamer(), ms, func(c context.Context, segment []byte) error {
			if _, err := media.ValidateMP4Media(c, segment); err != nil {
				return fmt.Errorf("shadow signed segment validation: %w", err)
			}
			verified++
			if time.Since(lastProgress) >= 30*time.Second {
				log.Log(c, "duplicate-mist-test segment verified", "segments_verified", verified)
				lastProgress = time.Now()
			}
			return nil
		})
		if groupCtx.Err() != nil && errors.Is(err, context.Canceled) {
			return nil
		}
		cancel()
		return err
	})
	err = g.Wait()
	log.Log(ctx, "duplicate-mist-test shadow finished", "segments_verified", verified)
	if err != nil {
		return err
	}
	if parent.Err() != nil {
		return parent.Err()
	}
	if verified == 0 {
		return fmt.Errorf("shadow ended without a verified segment")
	}
	return nil
}

// Protocol errors may embed entire connect/publish commands, including keys.
// Preserve EOF classification, but never print untrusted command arguments.
func shadowProtocolError(phase string, err error) error {
	if errors.Is(err, io.EOF) {
		return fmt.Errorf("shadow RTMP %s: %w", phase, io.EOF)
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf("shadow RTMP %s: %w", phase, io.ErrUnexpectedEOF)
	}
	return fmt.Errorf("shadow RTMP %s failed (%T; command details redacted to protect stream keys)", phase, err)
}
