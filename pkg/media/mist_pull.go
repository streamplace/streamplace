package media

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"stream.place/streamplace/pkg/log"
)

// mistPullConnectGrace bounds how long we retry the initial GET while Mist
// boots the freshly-pushed stream: PUSH_REWRITE fires BEFORE Mist accepts the
// push, so the stream may 404 (or refuse the connection) for a moment, and
// once it answers it may serve a trackless header until the encoder's media
// registers. Var (not const) so tests can shorten it.
var mistPullConnectGrace = 30 * time.Second

// mistPullRetryBackoff paces those initial connect retries.
var mistPullRetryBackoff = 500 * time.Millisecond

// errNoTracks means Mist answered 200 and served a valid MP4 header, but the
// moov had no tracks yet — the push connected before any media registered. It
// is retryable exactly like a 404: the push is still coming up.
var errNoTracks = errors.New("stream has no tracks yet")

// mistPullHeaderWait bounds how long we wait for the moov on any one GET. A
// live header is a few KB and arrives as soon as Mist has tracks; a stalled
// push that connects but never sends media must not wedge the retry loop, so
// we give up on the attempt and retry (or, past the grace, fail). Var (not
// const) so tests can shorten it.
var mistPullHeaderWait = 10 * time.Second

// mistPullHeaderScanCap bounds how many body bytes we scan looking for the
// moov on one GET, so a pathological header can't burn unbounded time/memory.
// A real live moov lands within the first few KB. Var so tests can shorten it.
var mistPullHeaderScanCap = int64(8 << 20)

// MistPullIngest ingests a live stream by PULLING MistServer's fragmented-MP4
// HTTP output for mistStreamName — the replacement for the old MKVExec push
// bridge (Mist exec'ing `streamplace live` and POSTing MKV to /live). Pulling
// .mp4 instead of receiving .mkv matters: MP4 fragments carry real decode
// timestamps, so ingest no longer has to reconstruct DTS from the H264
// bitstream (see buildMP4IngestPipeline).
//
// It's kicked off from the PUSH_REWRITE trigger — the moment we've authed an
// incoming Mist push and minted its signer — and runs for the life of the
// stream: the GET body ends when the push ends. The isolated path hands the
// raw response connection to a detached worker (fd-passing, exactly like the
// old hijacked-POST path), so a main restart doesn't interrupt the ingest.
//
// The connect waits for a PLAYABLE header (a moov with tracks), not just a
// 200: an encoder can connect before its media registers, and handing a
// trackless header to the pipeline kills the ingest with no retry.
func (mm *MediaManager) MistPullIngest(ctx context.Context, mistStreamName string, ms MediaSigner) error {
	hostport := fmt.Sprintf("127.0.0.1:%d", mm.cli.MistHTTPPort)
	// PathEscape leaves '+' (a legal path character) alone, but HTTP servers
	// commonly decode it as a space — Mist wildcard names are full of them
	// (stream+<did>_<ms>), so escape it explicitly.
	path := "/" + strings.ReplaceAll(url.PathEscape(mistStreamName), "+", "%2B") + ".mp4"
	ctx = log.WithLogValues(ctx, "streamer", ms.Streamer(), "mist-stream", mistStreamName)

	conn, prebuf, chunked, err := mistPullConnect(ctx, hostport, path)
	if err != nil {
		return fmt.Errorf("mist pull: %w", err)
	}
	log.Log(ctx, "mist pull connected", "url", hostport+path)

	if mm.cli.IsolatedIngest {
		// Zero-downtime path: the detached worker owns the pull connection (so
		// it survives a main restart) and serves signed segments back over its
		// socket — the same machinery as the old hijacked inbound push, with
		// the connection pointing the other way.
		return mm.MP4IngestDetached(ctx, conn, prebuf, chunked, ms)
	}
	defer conn.Close()
	body := WorkerInput(IngestWorkerConfig{Prebuf: prebuf, Chunked: chunked}, conn)
	return mm.MP4Ingest(ctx, body, ms)
}

// mistPullConnect dials Mist's HTTP output and issues the GET by hand — not
// through http.Client — because the isolated path needs the raw *net.TCPConn
// to fd-pass to the worker. It consumes the response headers and returns the
// connection positioned at the body, plus any body bytes the header read
// buffered past the headers (prebuf) and whether the body is chunked — the
// same (conn, prebuf, chunked) shape the old hijacked-POST ingest produced, so
// the downstream machinery is shared unchanged.
//
// It retries while Mist boots the stream (mistPullConnectGrace). Two retryable
// shapes: a refused connection or non-200 (the push hasn't started flowing),
// and a 200 whose header has no tracks yet — the push connected but no media
// registered, and handing that to qtdemux kills the ingest with no retry (see
// awaitPlayableHeader). Success means a playable header, not just a 200.
func mistPullConnect(ctx context.Context, hostport, path string) (*net.TCPConn, []byte, bool, error) {
	giveUp := time.Now().Add(mistPullConnectGrace)
	sawUp := false
	var lastErr error
	for {
		conn, prebuf, chunked, err := tryMistGET(ctx, hostport, path)
		if err == nil {
			sawUp = true
			prebuf, err = awaitPlayableHeader(conn, prebuf, chunked)
			if err == nil {
				return conn, prebuf, chunked, nil
			}
			conn.Close() // retry with a fresh GET; Mist re-serves the header
		}
		lastErr = err
		log.Debug(ctx, "mist pull header not playable yet, retrying", "url", hostport+path, "error", err)
		if time.Now().After(giveUp) {
			break
		}
		select {
		case <-ctx.Done():
			return nil, nil, false, ctx.Err()
		case <-time.After(mistPullRetryBackoff):
		}
	}
	if sawUp {
		return nil, nil, false, fmt.Errorf("stream came up at %s%s but never served a playable header: %w", hostport, path, lastErr)
	}
	return nil, nil, false, fmt.Errorf("stream never came up at %s%s: %w", hostport, path, lastErr)
}

// tryMistGET is one attempt: dial, send the GET, read the response headers.
// On a 200 it hands back the connection + buffered body bytes; anything else
// is an error and the connection is closed.
func tryMistGET(ctx context.Context, hostport, path string) (*net.TCPConn, []byte, bool, error) {
	d := net.Dialer{Timeout: 5 * time.Second}
	raw, err := d.DialContext(ctx, "tcp", hostport)
	if err != nil {
		return nil, nil, false, err
	}
	conn, ok := raw.(*net.TCPConn)
	if !ok {
		raw.Close()
		return nil, nil, false, fmt.Errorf("expected TCP connection, got %T", raw)
	}
	// A real *http.Request serialized by the stdlib — we only own the conn by
	// hand (it gets fd-passed to the worker), not the HTTP framing. req.Close
	// sends Connection: close: one stream per connection, body runs to EOF (or
	// chunked-EOS) when the Mist stream ends. No keepalive reuse to reason about.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+hostport+path, nil)
	if err != nil {
		conn.Close()
		return nil, nil, false, err
	}
	req.Close = true
	req.Header.Set("User-Agent", "streamplace-ingest")
	req.Header.Set("Accept", "video/mp4")
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		conn.Close()
		return nil, nil, false, err
	}
	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, nil, false, err
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		conn.Close()
		return nil, nil, false, err
	}
	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, nil, false, fmt.Errorf("mist returned %s", resp.Status)
	}
	if err := conn.SetDeadline(time.Time{}); err != nil { // clear; streaming has no deadline
		conn.Close()
		return nil, nil, false, err
	}
	chunked := false
	for _, te := range resp.TransferEncoding {
		if te == "chunked" {
			chunked = true
		}
	}
	// The header read buffered some raw body bytes; peel them off the bufio so
	// the caller can prepend them to the (otherwise unbuffered) connection.
	// resp.Body is deliberately never read — it would de-chunk, and the worker
	// wants the raw stream + the chunked flag.
	var prebuf []byte
	if n := br.Buffered(); n > 0 {
		prebuf = make([]byte, n)
		if _, err := io.ReadFull(br, prebuf); err != nil {
			conn.Close()
			return nil, nil, false, err
		}
	}
	return conn, prebuf, chunked, nil
}

// awaitPlayableHeader consumes the pull body until it has parsed a complete
// moov, gating the handoff to ingest on the stream actually having tracks. A
// 200 from Mist only means the stream exists; if the push connected but no
// media has registered yet, Mist serves a moov with zero traks and qtdemux
// would die instantly ("This file contains no playable streams."), killing the
// worker with no retry. Waiting here turns that into a retryable connect
// failure.
//
// Every byte consumed comes back in the returned prebuf verbatim (chunked
// framing included, if chunked) — WorkerInput re-prepends and de-chunks it, so
// the pipeline sees the exact stream. On success the read deadline is cleared
// for streaming; on error the caller closes the conn, so the prebuf is moot.
func awaitPlayableHeader(conn net.Conn, prebuf []byte, chunked bool) ([]byte, error) {
	if err := conn.SetReadDeadline(time.Now().Add(mistPullHeaderWait)); err != nil {
		return nil, err
	}

	var consumed bytes.Buffer
	raw := io.TeeReader(io.MultiReader(bytes.NewReader(prebuf), conn), &consumed)
	src := raw
	if chunked {
		src = httputil.NewChunkedReader(raw)
	}

	tracks, err := scanPlayableHeader(src, mistPullHeaderScanCap)
	if err != nil {
		return nil, err // caller closes the conn; the deadline goes with it
	}
	if tracks == 0 {
		return nil, errNoTracks
	}
	// Clear the deadline for streaming reads.
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return nil, err
	}
	return consumed.Bytes(), nil
}

// scanPlayableHeader walks top-level MP4 boxes from r until it finds a
// complete moov, returning the number of trak children it holds. Non-moov
// boxes are skipped. It reads at most maxRead bytes; exceeding that, hitting
// EOF before a moov, or meeting a malformed box is an error (all retryable at
// the connect layer).
func scanPlayableHeader(r io.Reader, maxRead int64) (int, error) {
	cr := &cappedReader{r: r, left: maxRead, cap: maxRead}
	for {
		var hdr [8]byte
		if _, err := io.ReadFull(cr, hdr[:]); err != nil {
			return 0, err
		}
		size := int64(binary.BigEndian.Uint32(hdr[0:4]))
		typ := string(hdr[4:8])
		headerLen := int64(8)
		if size == 1 { // 64-bit largesize follows
			var large [8]byte
			if _, err := io.ReadFull(cr, large[:]); err != nil {
				return 0, err
			}
			size = int64(binary.BigEndian.Uint64(large[:]))
			headerLen = 16
		}
		if size < headerLen {
			return 0, fmt.Errorf("invalid MP4 box %q: size %d < header %d", typ, size, headerLen)
		}
		bodyLen := size - headerLen
		if typ == "moov" {
			body := make([]byte, bodyLen)
			if _, err := io.ReadFull(cr, body); err != nil {
				return 0, err
			}
			return countTraks(body), nil
		}
		if _, err := io.CopyN(io.Discard, cr, bodyLen); err != nil {
			return 0, err
		}
	}
}

// countTraks counts the trak boxes among a moov body's immediate children.
// Defensive against malformed/trailing bytes: it returns what it has counted
// rather than erroring, since the moov was already accepted as complete.
func countTraks(moovBody []byte) int {
	n := 0
	r := bytes.NewReader(moovBody)
	for {
		var hdr [8]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return n
		}
		size := int64(binary.BigEndian.Uint32(hdr[0:4]))
		typ := string(hdr[4:8])
		headerLen := int64(8)
		if size == 1 {
			var large [8]byte
			if _, err := io.ReadFull(r, large[:]); err != nil {
				return n
			}
			size = int64(binary.BigEndian.Uint64(large[:]))
			headerLen = 16
		} else if size == 0 { // extends to end of the moov body
			size = int64(r.Len()) + headerLen
		}
		if typ == "trak" {
			n++
		}
		if size < headerLen {
			return n
		}
		if _, err := r.Seek(size-headerLen, io.SeekCurrent); err != nil {
			return n
		}
	}
}

// cappedReader bounds the total bytes read from r to cap, returning a "no
// moov" error once exhausted instead of reading forever.
type cappedReader struct {
	r    io.Reader
	left int64
	cap  int64
}

func (c *cappedReader) Read(p []byte) (int, error) {
	if c.left <= 0 {
		return 0, fmt.Errorf("no moov within %d bytes", c.cap)
	}
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	return n, err
}
