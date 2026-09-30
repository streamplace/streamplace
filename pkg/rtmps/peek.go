package rtmps

import (
	"fmt"
	"io"
	"strings"

	"github.com/bluenviron/gortmplib/pkg/amf0"
	"github.com/bluenviron/gortmplib/pkg/bytecounter"
	"github.com/bluenviron/gortmplib/pkg/message"
)

// rtmpHandshakeClientBytes is C0+C1+C2 of a plain RTMP handshake: a version
// byte, then two 1536-byte blocks.
const rtmpHandshakeClientBytes = 1 + 1536 + 1536

// peekMaxMessages bounds how far into a connection we look for the publish
// command before giving up; real encoders publish within a handful.
const peekMaxMessages = 32

// PublishInfo is what an RTMP client said about where it's publishing.
type PublishInfo struct {
	// TCURL is the tcUrl from the connect command, e.g.
	// rtmps://stream.place:1935/live.
	TCURL string
	// StreamKey is the publish command's stream name, without any query.
	StreamKey string
}

// PeekPublish reads the client-to-server side of an RTMP connection up to and
// including its publish command and returns what the client connected and
// published to. It only reads, so a proxy calls it on an io.TeeReader that
// forwards everything consumed to the real server, which does the talking;
// when it returns, the caller copies the rest of the stream. It reads past the
// publish command only as far as the chunk reader's buffering, all of which
// the tee has already forwarded.
//
// It returns an error for anything that isn't a plain RTMP publish (RTMPE's
// encrypted handshake, playback, or a client that doesn't publish within
// peekMaxMessages messages).
func PeekPublish(r io.Reader) (*PublishInfo, error) {
	var hs [rtmpHandshakeClientBytes]byte
	if _, err := io.ReadFull(r, hs[:]); err != nil {
		return nil, fmt.Errorf("reading RTMP handshake: %w", err)
	}
	if hs[0] != 3 {
		return nil, fmt.Errorf("unsupported RTMP handshake version %d", hs[0])
	}

	bcr := bytecounter.NewReader(r)
	mr := message.NewReader(bcr, bcr, func(uint32) error { return nil })
	var tcURL string
	for range peekMaxMessages {
		msg, err := mr.Read()
		if err != nil {
			return nil, fmt.Errorf("reading RTMP message: %w", err)
		}
		cmd, ok := msg.(*message.CommandAMF0)
		if !ok {
			continue
		}
		switch cmd.Name {
		case "connect":
			if len(cmd.Arguments) < 1 {
				return nil, fmt.Errorf("RTMP connect without a command object")
			}
			obj, ok := cmd.Arguments[0].(amf0.Object)
			if !ok {
				arr, isArr := cmd.Arguments[0].(amf0.ECMAArray)
				if !isArr {
					return nil, fmt.Errorf("RTMP connect with invalid command object")
				}
				obj = amf0.Object(arr)
			}
			tcURL, ok = obj.GetString("tcUrl")
			if !ok {
				tcURL, _ = obj.GetString("tcurl")
			}
			tcURL = strings.Trim(tcURL, "'")
		case "publish":
			if len(cmd.Arguments) < 2 {
				return nil, fmt.Errorf("RTMP publish without a stream name")
			}
			name, ok := cmd.Arguments[1].(string)
			if !ok {
				return nil, fmt.Errorf("RTMP publish with invalid stream name")
			}
			// Encoders may append a query (?foo=bar) to the stream key.
			name, _, _ = strings.Cut(name, "?")
			return &PublishInfo{TCURL: tcURL, StreamKey: name}, nil
		case "play":
			return nil, fmt.Errorf("RTMP client is playing, not publishing")
		}
	}
	return nil, fmt.Errorf("no RTMP publish within %d messages", peekMaxMessages)
}
