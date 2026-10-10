package moq

import (
	"context"
	"io"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/webtransport-go"
)

// conn is the slice of a QUIC or WebTransport connection moq-lite uses. Both
// bindings carry the same streams; they differ only in the Go types quic-go
// and webtransport-go hand back (and in the width of stream error codes),
// which these adapters paper over.
type conn interface {
	AcceptStream(ctx context.Context) (bidiStream, error)
	AcceptUniStream(ctx context.Context) (recvStream, error)
	OpenStreamSync(ctx context.Context) (bidiStream, error)
	OpenUniStreamSync(ctx context.Context) (sendStream, error)
	CloseWithError(code uint64, msg string) error
	// Context ends when the connection does.
	Context() context.Context
}

type recvStream interface {
	io.Reader
	CancelRead(code uint64)
}

type sendStream interface {
	io.Writer
	Close() error
	CancelWrite(code uint64)
}

type bidiStream interface {
	recvStream
	sendStream
}

// reset abandons both directions of a control stream with one error code.
func reset(s bidiStream, code uint64) {
	s.CancelWrite(code)
	s.CancelRead(code)
}

// --- raw QUIC ----------------------------------------------------------------

type quicConn struct{ *quic.Conn }

func (c quicConn) AcceptStream(ctx context.Context) (bidiStream, error) {
	s, err := c.Conn.AcceptStream(ctx)
	if err != nil {
		return nil, err
	}
	return quicBidi{s}, nil
}

func (c quicConn) AcceptUniStream(ctx context.Context) (recvStream, error) {
	s, err := c.Conn.AcceptUniStream(ctx)
	if err != nil {
		return nil, err
	}
	return quicRecv{s}, nil
}

func (c quicConn) OpenStreamSync(ctx context.Context) (bidiStream, error) {
	s, err := c.Conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return quicBidi{s}, nil
}

func (c quicConn) OpenUniStreamSync(ctx context.Context) (sendStream, error) {
	s, err := c.Conn.OpenUniStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return quicSend{s}, nil
}

func (c quicConn) CloseWithError(code uint64, msg string) error {
	return c.Conn.CloseWithError(quic.ApplicationErrorCode(code), msg)
}

type quicBidi struct{ *quic.Stream }

func (s quicBidi) CancelRead(code uint64)  { s.Stream.CancelRead(quic.StreamErrorCode(code)) }
func (s quicBidi) CancelWrite(code uint64) { s.Stream.CancelWrite(quic.StreamErrorCode(code)) }

type quicRecv struct{ *quic.ReceiveStream }

func (s quicRecv) CancelRead(code uint64) { s.ReceiveStream.CancelRead(quic.StreamErrorCode(code)) }

type quicSend struct{ *quic.SendStream }

func (s quicSend) CancelWrite(code uint64) { s.SendStream.CancelWrite(quic.StreamErrorCode(code)) }

// --- WebTransport ---------------------------------------------------------

type wtConn struct{ *webtransport.Session }

func (c wtConn) AcceptStream(ctx context.Context) (bidiStream, error) {
	s, err := c.Session.AcceptStream(ctx)
	if err != nil {
		return nil, err
	}
	return wtBidi{s}, nil
}

func (c wtConn) AcceptUniStream(ctx context.Context) (recvStream, error) {
	s, err := c.Session.AcceptUniStream(ctx)
	if err != nil {
		return nil, err
	}
	return wtRecv{s}, nil
}

func (c wtConn) OpenStreamSync(ctx context.Context) (bidiStream, error) {
	s, err := c.Session.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return wtBidi{s}, nil
}

func (c wtConn) OpenUniStreamSync(ctx context.Context) (sendStream, error) {
	s, err := c.Session.OpenUniStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return wtSend{s}, nil
}

func (c wtConn) CloseWithError(code uint64, msg string) error {
	return c.Session.CloseWithError(webtransport.SessionErrorCode(code), msg)
}

// WebTransport stream error codes are 32-bit; moq-lite's all fit.
type wtBidi struct{ *webtransport.Stream }

func (s wtBidi) CancelRead(code uint64)  { s.Stream.CancelRead(webtransport.StreamErrorCode(code)) }
func (s wtBidi) CancelWrite(code uint64) { s.Stream.CancelWrite(webtransport.StreamErrorCode(code)) }

type wtRecv struct{ *webtransport.ReceiveStream }

func (s wtRecv) CancelRead(code uint64) {
	s.ReceiveStream.CancelRead(webtransport.StreamErrorCode(code))
}

type wtSend struct{ *webtransport.SendStream }

func (s wtSend) CancelWrite(code uint64) {
	s.SendStream.CancelWrite(webtransport.StreamErrorCode(code))
}
