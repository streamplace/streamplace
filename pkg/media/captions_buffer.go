package media

import (
	"bytes"
	"context"
	"io"
	"sync"
	"time"

	"stream.place/streamplace/pkg/log"
)

type arrivedBytes struct {
	data []byte
	at   time.Time
}

// ingestByteBuffer allows a normal caption hold without stalling appsink.
// Its bounded staging window backpressures a persistently slower consumer.
// Graceful Close drains queued media; CloseWithError discards it immediately.
type ingestByteBuffer struct {
	mu        sync.Mutex
	ready     *sync.Cond
	chunks    []arrivedBytes
	offset    int
	closed    bool
	err       error
	bytes     int
	pressured bool
	ctx       context.Context
	lastRead  time.Time
}

const ingestByteCapacity = 32 * 1024 * 1024
const ingestByteChunk = 64 * 1024

func newIngestByteBuffer(contexts ...context.Context) *ingestByteBuffer {
	ctx := context.Background()
	if len(contexts) > 0 {
		ctx = contexts[0]
	}
	b := &ingestByteBuffer{ctx: ctx}
	b.ready = sync.NewCond(&b.mu)
	return b
}
func (b *ingestByteBuffer) Write(p []byte) (int, error) {
	return b.writeAt(p, time.Now())
}
func (b *ingestByteBuffer) writeAt(p []byte, at time.Time) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	written := 0
	for len(p) > 0 {
		for b.bytes == ingestByteCapacity && !b.closed {
			if !b.pressured {
				b.pressured = true
				log.Warn(b.ctx, "caption media staging capacity reached; applying backpressure", "bytes", b.bytes)
			}
			b.ready.Wait()
		}
		if b.closed {
			return written, io.ErrClosedPipe
		}
		n := min(len(p), ingestByteChunk, ingestByteCapacity-b.bytes)
		b.chunks = append(b.chunks, arrivedBytes{data: bytes.Clone(p[:n]), at: at})
		b.bytes += n
		written += n
		p = p[n:]
		b.ready.Broadcast()
	}
	if b.closed && written == 0 {
		return 0, io.ErrClosedPipe
	}
	return written, nil
}
func (b *ingestByteBuffer) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for len(b.chunks) == 0 && !b.closed {
		b.ready.Wait()
	}
	if len(b.chunks) == 0 {
		if b.err != nil {
			return 0, b.err
		}
		return 0, io.EOF
	}
	b.lastRead = b.chunks[0].at
	n := copy(p, b.chunks[0].data[b.offset:])
	b.offset += n
	if b.offset == len(b.chunks[0].data) {
		b.bytes -= len(b.chunks[0].data)
		b.chunks[0] = arrivedBytes{}
		b.chunks = b.chunks[1:]
		b.offset = 0
		b.ready.Broadcast()
	}
	return n, nil
}
func (b *ingestByteBuffer) Close() error { return b.CloseWithError(nil) }
func (b *ingestByteBuffer) CloseWithError(err error) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	b.err = err
	if err != nil {
		b.chunks = nil
		b.offset = 0
		b.bytes = 0
	}
	b.ready.Broadcast()
	return nil
}

func (b *ingestByteBuffer) LastReadTime() time.Time {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastRead
}

func captionReadTime(input io.Reader) time.Time {
	if timed, ok := input.(interface{ LastReadTime() time.Time }); ok {
		return timed.LastReadTime()
	}
	return time.Now()
}
