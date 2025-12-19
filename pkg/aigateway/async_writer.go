package aigateway

import (
	"context"
	"io"
	"sync/atomic"

	"stream.place/streamplace/pkg/log"
)

const (
	// AsyncWriterBufferSize is the channel buffer size for async writes.
	// If the buffer fills, writes will be dropped to prevent blocking the main pipeline.
	AsyncWriterBufferSize = 64
)

// AsyncWriter wraps a WriteCloser with non-blocking write semantics.
// Writes are buffered and processed asynchronously. If the buffer fills,
// new writes are dropped rather than blocking the caller.
type AsyncWriter struct {
	ctx       context.Context
	cancel    context.CancelFunc
	ch        chan []byte
	dst       io.WriteCloser
	closed    atomic.Bool
	dropped   atomic.Int64
	written   atomic.Int64
	closeDone chan struct{}
}

// NewAsyncWriter creates a new AsyncWriter that writes to dst.
// The writer starts a background goroutine to drain writes to dst.
// Call Close() to stop the writer and wait for pending writes to complete.
func NewAsyncWriter(ctx context.Context, dst io.WriteCloser) *AsyncWriter {
	ctx, cancel := context.WithCancel(ctx)
	aw := &AsyncWriter{
		ctx:       ctx,
		cancel:    cancel,
		ch:        make(chan []byte, AsyncWriterBufferSize),
		dst:       dst,
		closeDone: make(chan struct{}),
	}
	go aw.drain()
	return aw
}

// Write queues data for asynchronous writing to the underlying writer.
// It always returns len(p), nil to satisfy io.Writer - actual write errors
// are logged but not returned to avoid blocking the caller.
// If the buffer is full, data is dropped and tracked in the dropped counter.
func (aw *AsyncWriter) Write(p []byte) (int, error) {
	if aw.closed.Load() {
		return len(p), nil
	}

	cpy := make([]byte, len(p))
	copy(cpy, p)

	select {
	case aw.ch <- cpy:
		aw.written.Add(int64(len(p)))
	default:
		// Buffer full - drop the data rather than blocking
		aw.dropped.Add(int64(len(p)))
	}

	return len(p), nil
}

func (aw *AsyncWriter) drain() {
	defer close(aw.closeDone)
	defer func() {
		if aw.dst != nil {
			_ = aw.dst.Close()
		}
	}()

	for {
		select {
		case <-aw.ctx.Done():
			for {
				select {
				case data := <-aw.ch:
					_, _ = aw.dst.Write(data)
				default:
					return
				}
			}
		case data, ok := <-aw.ch:
			if !ok {
				return
			}
			if _, err := aw.dst.Write(data); err != nil {
				log.Debug(aw.ctx, "async writer dst write error, discarding future writes", "error", err)
				aw.closed.Store(true)
				return
			}
		}
	}
}

// Close stops accepting new writes and waits for pending writes to complete.
// It then closes the underlying writer.
func (aw *AsyncWriter) Close() error {
	if aw.closed.Swap(true) {
		return nil
	}
	aw.cancel()
	<-aw.closeDone
	return nil
}

// Stats returns the total bytes successfully written and dropped.
// Dropped bytes indicate buffer overflow - the AI gateway couldn't keep up.
func (aw *AsyncWriter) Stats() (written, dropped int64) {
	return aw.written.Load(), aw.dropped.Load()
}
