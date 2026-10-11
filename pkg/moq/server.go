package moq

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
	"stream.place/streamplace/pkg/log"
)

// Server publishes tracks on one UDP port over both bindings: raw QUIC
// sessions negotiate the moq-lite ALPN directly, and browsers reach the
// same publisher through WebTransport (HTTP/3 CONNECT on any path) with
// moq-lite as the subprotocol.
type Server struct {
	// TLSConfig supplies the certificate; its NextProtos are replaced.
	TLSConfig *tls.Config
	Publisher Publisher
	// Publishers, when set, is handed every session whose client declared
	// the publisher Role: a peer pushing broadcasts here, which it
	// discovers with Session.Announced and subscribes to. It runs for the
	// session's life; the session is closed when it returns. Without it
	// such a session is refused as unauthorized.
	Publishers func(ctx context.Context, sess *Session)

	ctx context.Context
	wt  *webtransport.Server
}

// Serve accepts sessions on conn until ctx ends. Every live session is
// closed on the way out.
func (s *Server) Serve(ctx context.Context, conn net.PacketConn) error {
	tlsConf := s.TLSConfig.Clone()
	tlsConf.NextProtos = []string{ALPN, http3.NextProtoH3}
	ln, err := quic.Listen(conn, tlsConf, quicConfig())
	if err != nil {
		return fmt.Errorf("listening for quic: %w", err)
	}
	wt := &webtransport.Server{
		H3:                   &http3.Server{Handler: http.HandlerFunc(s.upgrade)},
		ApplicationProtocols: []string{ALPN},
		// Media is public and the session carries no credentials; the
		// subscriber's identity is irrelevant to what it can read.
		CheckOrigin: func(*http.Request) bool { return true },
	}
	s.wt = wt
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.ctx = ctx
	go func() {
		<-ctx.Done()
		_ = ln.Close()
		_ = wt.Close()
	}()
	for {
		c, err := ln.Accept(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("accepting quic: %w", err)
		}
		switch proto := c.ConnectionState().TLS.NegotiatedProtocol; proto {
		case ALPN:
			// Off the accept loop: setup waits on the peer for stream
			// credit, and one slow peer must not hold up the rest.
			go func() {
				sess, err := newSession(ctx, quicConn{c}, s.Publisher, true, setupMessage(nil))
				if err != nil {
					log.Warn(ctx, "moq: session setup failed", "remote", c.RemoteAddr(), "error", err)
					return
				}
				s.run(sess)
			}()
		case http3.NextProtoH3:
			go func() {
				if err := wt.ServeQUICConn(c); err != nil && !errors.Is(err, http.ErrServerClosed) && ctx.Err() == nil {
					log.Debug(ctx, "moq: webtransport connection ended", "remote", c.RemoteAddr(), "error", err)
				}
			}()
		default:
			_ = c.CloseWithError(sessionProtocolViolation, "unsupported alpn")
		}
	}
}

// run sees an accepted session out: closed when the server stops, and in
// between either served by Publisher (the peer subscribes) or, when the
// peer declared itself a publisher, handed to Publishers to subscribe to.
func (s *Server) run(sess *Session) {
	go func() {
		select {
		case <-s.ctx.Done():
			_ = sess.Close()
		case <-sess.Done():
		}
	}()
	select {
	case <-sess.Done():
		return
	case <-sess.setupDone:
	}
	if sess.peerRole != rolePublisher {
		<-sess.Done()
		return
	}
	if s.Publishers == nil {
		sess.fail(sessionUnauthorized, errors.New("publishing is not accepted here"))
		return
	}
	s.Publishers(sess.ctx, sess)
	_ = sess.Close()
}

// upgrade turns an HTTP/3 CONNECT into a moq-lite session. The subprotocol
// must have been negotiated: a client that offered none would expect the
// pre-ALPN version handshake, which this package does not speak.
func (s *Server) upgrade(w http.ResponseWriter, r *http.Request) {
	sess, err := s.wt.Upgrade(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if proto := sess.SessionState().ApplicationProtocol; proto != ALPN {
		_ = sess.CloseWithError(sessionProtocolViolation, "subprotocol "+ALPN+" required")
		return
	}
	ms, err := newSession(r.Context(), wtConn{sess}, s.Publisher, true, setupMessage(nil))
	if err != nil {
		log.Warn(r.Context(), "moq: webtransport session setup failed", "remote", r.RemoteAddr, "error", err)
		return
	}
	go s.run(ms)
}
