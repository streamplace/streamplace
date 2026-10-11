package moq

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

// DialOptions configures Dial.
type DialOptions struct {
	// TLS verifies the server. nil uses the system roots.
	TLS *tls.Config
	// Publisher, when set, makes this a publishing session: the server
	// subscribes to its tracks (a push), and the Role in SETUP says so.
	// Without it the session subscribes.
	Publisher Publisher
}

// Dial opens a session. A moqt:// (or moq://, moql://, moqs://) URL dials
// raw QUIC with the moq-lite ALPN, carrying the URL's path and query in
// SETUP; an https:// URL dials WebTransport with moq-lite as the
// subprotocol. The default port is 443.
func Dial(ctx context.Context, rawURL string, opts DialOptions) (*Session, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parsing moq url: %w", err)
	}
	tlsConf := opts.TLS
	if tlsConf == nil {
		tlsConf = &tls.Config{}
	}
	tlsConf = tlsConf.Clone()
	if tlsConf.ServerName == "" {
		tlsConf.ServerName = u.Hostname()
	}
	switch u.Scheme {
	case "moqt", "moq", "moql", "moqs":
		return dialQUIC(ctx, u, tlsConf, opts.Publisher)
	case "https":
		return dialWebTransport(ctx, u, tlsConf, opts.Publisher)
	default:
		return nil, fmt.Errorf("unsupported moq url scheme %q", u.Scheme)
	}
}

// setupParams is the client's SETUP: Role narrows the session to what we
// do with it (draft §7.3.3).
func setupParams(pub Publisher) map[uint64][]byte {
	role := uint64(roleSubscriber)
	if pub != nil {
		role = rolePublisher
	}
	return map[uint64][]byte{setupParamRole: appendVarint(nil, role)}
}

func quicConfig() *quic.Config {
	return &quic.Config{
		MaxIdleTimeout:                   30 * time.Second,
		KeepAlivePeriod:                  10 * time.Second,
		EnableDatagrams:                  true,
		EnableStreamResetPartialDelivery: true,
	}
}

func dialQUIC(ctx context.Context, u *url.URL, tlsConf *tls.Config, pub Publisher) (*Session, error) {
	port := u.Port()
	if port == "" {
		port = "443"
	}
	tlsConf.NextProtos = []string{ALPN}
	c, err := quic.DialAddr(ctx, net.JoinHostPort(u.Hostname(), port), tlsConf, quicConfig())
	if err != nil {
		return nil, fmt.Errorf("dialing %s: %w", u.Host, err)
	}
	if proto := c.ConnectionState().TLS.NegotiatedProtocol; proto != ALPN {
		_ = c.CloseWithError(sessionProtocolViolation, "alpn")
		return nil, fmt.Errorf("server negotiated %q, want %s", proto, ALPN)
	}
	// The request target rides SETUP on a binding with no request URI
	// (draft §7.3.2).
	params := setupParams(pub)
	if path := u.RequestURI(); path != "" && path != "/" {
		params[setupParamPath] = []byte(path)
	}
	return newSession(ctx, quicConn{c}, pub, false, setupMessage(params))
}

func dialWebTransport(ctx context.Context, u *url.URL, tlsConf *tls.Config, pub Publisher) (*Session, error) {
	tlsConf.NextProtos = []string{http3.NextProtoH3}
	t := &webtransport.Transport{
		TLSClientConfig:      tlsConf,
		QUICConfig:           quicConfig(),
		ApplicationProtocols: []string{ALPN},
	}
	_, sess, err := t.Dial(ctx, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("dialing %s: %w", u.Host, err)
	}
	if proto := sess.SessionState().ApplicationProtocol; proto != ALPN {
		_ = sess.CloseWithError(sessionProtocolViolation, "subprotocol")
		return nil, fmt.Errorf("server negotiated %q, want %s", proto, ALPN)
	}
	return newSession(ctx, wtConn{sess}, pub, false, setupMessage(setupParams(pub)))
}
