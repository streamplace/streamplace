package media

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/ingestframe"
)

// unixPair returns a connected (client, server) unix-socket pair. Unlike
// net.Pipe these are kernel-buffered, so small writes don't block on a reader —
// the frameServer can flush its buffer without a concurrent drainer.
func unixPair(t *testing.T) (client net.Conn, server net.Conn) {
	t.Helper()
	tempDir, err := os.MkdirTemp("/tmp", "sp-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempDir) })
	sock := filepath.Join(tempDir, "p.sock")
	ln, err := net.Listen("unix", sock)
	require.NoError(t, err)
	defer ln.Close()
	type res struct {
		c net.Conn
		e error
	}
	ch := make(chan res, 1)
	go func() {
		c, e := ln.Accept()
		ch <- res{c, e}
	}()
	client, err = net.Dial("unix", sock)
	require.NoError(t, err)
	r := <-ch
	require.NoError(t, r.e)
	t.Cleanup(func() { client.Close(); r.c.Close() })
	return client, r.c
}

func seg(i int) []byte { return []byte(fmt.Sprintf("seg-%04d", i)) }

func readSegs(t *testing.T, r *ingestframe.Reader, n int) [][]byte {
	t.Helper()
	out := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		typ, payload, err := r.ReadFrame()
		require.NoError(t, err, "read frame %d", i)
		require.Equal(t, ingestframe.Segment, typ)
		out = append(out, payload)
	}
	return out
}

// TestFrameServerBufferFlushReconnect is the zero-downtime guarantee at the
// transport level: segments produced while main is disconnected are buffered and
// replayed, in order, when main reconnects — nothing is lost across the gap. The
// attach/detach are driven explicitly so the assertion is deterministic.
func TestFrameServerBufferFlushReconnect(t *testing.T) {
	srv := newFrameServer(1000) // ample buffer: no drops

	// Detached: the first 3 segments buffer.
	for i := 0; i < 3; i++ {
		require.NoError(t, srv.Segment(seg(i)))
	}

	// Main connects: buffered 0,1,2 replayed, then 3,4 live.
	clientA, serverA := unixPair(t)
	srv.attach(serverA)
	for i := 3; i < 5; i++ {
		require.NoError(t, srv.Segment(seg(i)))
	}
	got := readSegs(t, ingestframe.NewReader(clientA), 5)
	for i := 0; i < 5; i++ {
		require.Equal(t, seg(i), got[i], "frame %d in order before the restart", i)
	}

	// Main restarts: detach + drop the connection. Segments 5,6 produced while
	// it's gone must buffer, not vanish.
	srv.detachConn(serverA)
	clientA.Close()
	serverA.Close()
	for i := 5; i < 7; i++ {
		require.NoError(t, srv.Segment(seg(i)))
	}

	// Main reconnects on a fresh connection: buffered 5,6 replayed, then 7 live.
	clientB, serverB := unixPair(t)
	srv.attach(serverB)
	require.NoError(t, srv.Segment(seg(7)))
	got = readSegs(t, ingestframe.NewReader(clientB), 3)
	for i := 5; i < 8; i++ {
		require.Equal(t, seg(i), got[i-5], "frame %d replayed/live after reconnect", i)
	}

	require.Equal(t, 0, srv.droppedCount(), "ample buffer drops nothing across a brief restart")
}

func TestFrameServerReplaysLatestLLInitOnReconnect(t *testing.T) {
	srv := newFrameServer(1000)
	initFrame := ingestframe.LLFrame{
		Presentation: "whip-1-test",
		Track:        "video",
		Generation:   1,
		Data:         []byte("init"),
	}
	partFrame := initFrame
	partFrame.MSN = 1
	partFrame.Data = []byte("part")
	initPayload, err := ingestframe.EncodeLLFrame(initFrame)
	require.NoError(t, err)
	partPayload, err := ingestframe.EncodeLLFrame(partFrame)
	require.NoError(t, err)

	// The first init is delivered before main disconnects. The next part is
	// buffered, so a fresh main must receive the init again before that part.
	clientA, serverA := unixPair(t)
	srv.attach(serverA)
	srv.push(ingestframe.LLInit, initPayload)
	_, _, err = ingestframe.NewReader(clientA).ReadFrame()
	require.NoError(t, err)
	srv.detachConn(serverA)
	clientA.Close()
	serverA.Close()
	srv.push(ingestframe.LLPart, partPayload)

	clientB, serverB := unixPair(t)
	srv.attach(serverB)
	reader := ingestframe.NewReader(clientB)
	typ, payload, err := reader.ReadFrame()
	require.NoError(t, err)
	require.Equal(t, ingestframe.LLInit, typ)
	require.Equal(t, initPayload, payload)
	typ, payload, err = reader.ReadFrame()
	require.NoError(t, err)
	require.Equal(t, ingestframe.LLPart, typ)
	require.Equal(t, partPayload, payload)
}

func TestFrameServerBoundsBlockedClientWrite(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	srv := newFrameServer(3)
	srv.mu.Lock()
	srv.conn = server
	srv.w = ingestframe.NewWriter(server)
	srv.mu.Unlock()

	start := time.Now()
	require.NoError(t, srv.Segment(seg(0)))
	require.Less(t, time.Since(start), 500*time.Millisecond, "blocked client write exceeded its deadline")

	srv.mu.Lock()
	defer srv.mu.Unlock()
	require.Nil(t, srv.conn)
	require.Nil(t, srv.w)
	require.Len(t, srv.pending, 1)
}

// TestFrameServerDropsOldestBeyondBound: a main outage longer than the buffer
// window drops the OLDEST frames (bounded memory), loudly via droppedCount —
// never grows without limit.
func TestFrameServerDropsOldestBeyondBound(t *testing.T) {
	srv := newFrameServer(3)
	for i := 0; i < 6; i++ { // 0,1,2 should be dropped; 3,4,5 retained
		require.NoError(t, srv.Segment(seg(i)))
	}
	require.Equal(t, 3, srv.droppedCount(), "oldest 3 dropped")

	client, server := unixPair(t)
	srv.attach(server)
	got := readSegs(t, ingestframe.NewReader(client), 3)
	for i := 3; i < 6; i++ {
		require.Equal(t, seg(i), got[i-3], "newest 3 survive in order")
	}
}

// TestServeFrameSocketAttachAndFlush exercises the real accept loop: frames
// buffered before any client are flushed on connect, then live frames stream.
func TestServeFrameSocketAttachAndFlush(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "frames.sock")
	ln, err := net.Listen("unix", sock)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := newFrameServer(1000)
	for i := 0; i < 3; i++ { // buffered before anyone connects
		require.NoError(t, srv.Segment(seg(i)))
	}
	go serveFrameSocket(ctx, ln, srv, newManifestHolder(nil))

	client, err := net.Dial("unix", sock)
	require.NoError(t, err)
	defer client.Close()
	r := ingestframe.NewReader(client)

	// The 3 buffered frames are replayed once the accept loop attaches us.
	got := readSegs(t, r, 3)
	for i := 0; i < 3; i++ {
		require.Equal(t, seg(i), got[i])
	}
	// Reading them confirms we're attached; subsequent pushes stream live.
	for i := 3; i < 6; i++ {
		require.NoError(t, srv.Segment(seg(i)))
	}
	got = readSegs(t, r, 3)
	for i := 3; i < 6; i++ {
		require.Equal(t, seg(i), got[i-3])
	}

	// End frame then a clean EOF on cancel.
	require.NoError(t, srv.End())
	typ, _, err := r.ReadFrame()
	require.NoError(t, err)
	require.Equal(t, ingestframe.End, typ)

	cancel()
	ln.Close()
}
