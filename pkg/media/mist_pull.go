package media

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"stream.place/streamplace/pkg/log"
)

// mistPullConnectGrace bounds how long we retry the initial GET while Mist
// boots the freshly-pushed stream: PUSH_REWRITE fires BEFORE Mist accepts the
// push, so the stream may 404 (or refuse the connection) for a moment before
// media flows. Var (not const) so tests can shorten it.
var mistPullConnectGrace = 30 * time.Second

// mistPullRetryBackoff paces those initial connect retries.
var mistPullRetryBackoff = 500 * time.Millisecond

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
// It retries while Mist boots the stream (mistPullConnectGrace): a refused
// connection or a non-200 just means the push hasn't started flowing yet.
func mistPullConnect(ctx context.Context, hostport, path string) (*net.TCPConn, []byte, bool, error) {
	giveUp := time.Now().Add(mistPullConnectGrace)
	for {
		conn, prebuf, chunked, err := tryMistGET(ctx, hostport, path)
		if err == nil {
			return conn, prebuf, chunked, nil
		}
		if time.Now().After(giveUp) {
			return nil, nil, false, fmt.Errorf("stream never came up at %s%s: %w", hostport, path, err)
		}
		select {
		case <-ctx.Done():
			return nil, nil, false, ctx.Err()
		case <-time.After(mistPullRetryBackoff):
		}
	}
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
