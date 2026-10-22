package api

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
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

// Only a pending input Read is idle. Parsing buffered bytes, initializing native
// media, and draining after EOF must not consume the publisher's idle allowance.
type shadowIdleReader struct {
	io.Reader
	mu       sync.Mutex
	timer    *time.Timer
	deadline time.Time
	timedOut bool
	cancel   context.CancelFunc
}

func newShadowIdleReader(input io.Reader, cancel context.CancelFunc) *shadowIdleReader {
	r := &shadowIdleReader{Reader: input, cancel: cancel}
	r.timer = time.AfterFunc(RTMPTimeout, r.expire)
	r.timer.Stop()
	return r
}

func (r *shadowIdleReader) expire() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deadline.IsZero() {
		return
	}
	if remaining := time.Until(r.deadline); remaining > 0 {
		// Stop cannot retract a callback already queued by an earlier Read.
		// That callback must honor the current Read's full idle allowance.
		r.timer.Reset(remaining)
		return
	}
	r.timedOut = true
	r.cancel()
}

func (r *shadowIdleReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	if r.timedOut {
		r.mu.Unlock()
		return 0, errShadowIdleTimeout
	}
	r.deadline = time.Now().Add(RTMPTimeout)
	r.timer.Reset(RTMPTimeout)
	r.mu.Unlock()
	n, err := r.Reader.Read(p)
	r.mu.Lock()
	r.deadline = time.Time{}
	r.timer.Stop()
	if r.timedOut {
		err = errShadowIdleTimeout
	}
	r.mu.Unlock()
	return n, err
}

func (r *shadowIdleReader) stop() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deadline = time.Time{}
	r.timer.Stop()
	return r.timedOut
}

// RunDuplicateMistWorker consumes the decrypted, client-only RTMP conversation.
// gortmplib's non-strict plain handshake consumes C2 without comparing it to
// the shadow's S1, so the publisher's response to Mist's handshake is valid.
// All shadow replies are discarded; only Mist negotiates with the publisher.
// This runs ONLY in a child process: no auth, database, storage, or live state.
// progress is the private machine channel back to the parent (stdout in the
// child): exactly one byte of value 1 per validated segment, nothing else.
func RunDuplicateMistWorker(parent context.Context, input io.Reader, progress io.Writer) (retErr error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	ctx = log.WithLogValues(ctx, "component", "duplicate-mist-test")
	stopInput := context.AfterFunc(ctx, func() {
		if closer, ok := input.(io.Closer); ok {
			_ = closer.Close()
		}
	})
	defer stopInput()
	inputReader := newShadowIdleReader(input, cancel)
	defer func() {
		if inputReader.stop() {
			retErr = errShadowIdleTimeout
		}
	}()

	br := bufio.NewReader(inputReader)
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
	if err := sc.AcceptConn(); err != nil {
		return shadowProtocolError("accept", err)
	}
	if !sc.Publish {
		return fmt.Errorf("shadow connection is not an RTMP publisher")
	}
	// Mist owns publisher routing and authentication; the shadow accepts any
	// application path. Never use the supplied stream key: the signer and session
	// identity are throwaway, and only the media-only verifier consumes their output.
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
			// The playback handshake deadline must not truncate slow native
			// signing. The parent worker-exit grace bounds the entire drain.
			err = conn.SetReadDeadline(time.Time{})
			if err == nil {
				err = conn.(*net.TCPConn).CloseWrite()
			}
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
	report := [1]byte{1}
	g.Go(func() error {
		err := media.RTMPIngestUnpublished(groupCtx, &config.CLI{}, "rtmp://"+ln.Addr().String()+"/live/"+ms.Streamer(), ms, func(c context.Context, segment []byte) error {
			if _, err := media.ValidateMP4Media(c, segment); err != nil {
				return fmt.Errorf("shadow signed segment validation: %w", err)
			}
			// The parent counts only reported segments; a report it never
			// received must not end as a clean worker exit.
			n, err := progress.Write(report[:])
			if err == nil && n != len(report) {
				err = io.ErrShortWrite
			}
			if err != nil {
				return fmt.Errorf("shadow progress report: %w", err)
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
