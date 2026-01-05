package rtmprec

import (
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/fxamacker/cbor/v2"
)

// type Conn interface {
// 	Read(b []byte) (n int, err error)
// 	Write(b []byte) (n int, err error)
// 	Close() error
// 	LocalAddr() Addr
// 	RemoteAddr() Addr
// 	SetDeadline(t time.Time) error
// 	SetReadDeadline(t time.Time) error
// 	SetWriteDeadline(t time.Time) error
// }

type RecordingConn struct {
	conn    net.Conn
	encoder *cbor.Encoder
	enabled bool
}

type TCPData struct {
	Time time.Time `json:"time,omitempty"`
	Data []byte    `json:"data,omitempty"`
}

func NewRecordingConn(conn net.Conn) *RecordingConn {
	f, err := os.Create("/home/iameli/Desktop/rtmprec.cbor")
	if err != nil {
		panic(err)
	}
	_, err = MakeTCPEncoder(f)
	if err != nil {
		panic(err)
	}
	return &RecordingConn{
		conn: conn,
	}
}

func (rc *RecordingConn) Read(b []byte) (int, error) {
	if !rc.enabled {
		return rc.conn.Read(b)
	}
	n, err := rc.conn.Read(b)
	if err != nil {
		return n, err
	}
	now := time.Now().UTC()
	dataCopy := b[:n]
	go func() {
		rc.encoder.Encode(TCPData{
			Time: time.Now().UTC(),
			Data: b[:n],
		})
	}()
	return n, err
}

func (rc *RecordingConn) Write(b []byte) (int, error) {
	return rc.conn.Write(b)
}

func (rc *RecordingConn) Close() error {
	return rc.conn.Close()
}

func (rc *RecordingConn) LocalAddr() net.Addr {
	return rc.conn.LocalAddr()
}

func (rc *RecordingConn) RemoteAddr() net.Addr {
	return rc.conn.RemoteAddr()
}

func (rc *RecordingConn) SetDeadline(t time.Time) error {
	return rc.conn.SetDeadline(t)
}

func (rc *RecordingConn) SetReadDeadline(t time.Time) error {
	return rc.conn.SetReadDeadline(t)
}

func (rc *RecordingConn) SetWriteDeadline(t time.Time) error {
	return rc.conn.SetWriteDeadline(t)
}

func MakeTCPEncoder(w io.Writer) (*cbor.Encoder, error) {
	opts := cbor.CoreDetEncOptions()
	opts.Time = cbor.TimeRFC3339Nano
	em, err := opts.EncMode()
	if err != nil {
		return nil, fmt.Errorf("failed to create encoder mode: %w", err)
	}
	encoder := em.NewEncoder(w)

	return encoder, nil
}
