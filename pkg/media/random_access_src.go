package media

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/log"
)

// newReadTracer returns a function that records each ReadAt the source
// issues. When the SP_READ_TRACE env var names a file, every call is
// written there as "readAt pos=N size=M"; that capture can be replayed
// against the pkg/s3 CachingReaderAt simulation (TestCachingReaderAtSim)
// to model cache behavior on real demuxer access patterns. It's a
// development aid only — with the env var unset it returns a no-op and
// costs nothing. Writes go through a buffered channel so file I/O never
// blocks the gstreamer streaming thread; the writer drains and flushes
// when ctx is cancelled.
func newReadTracer(ctx context.Context) func(pos, size int64) {
	path := os.Getenv("SP_READ_TRACE")
	if path == "" {
		return func(int64, int64) {}
	}
	fd, err := os.Create(path)
	if err != nil {
		log.Error(ctx, "SP_READ_TRACE disabled: could not create trace file", "path", path, "error", err)
		return func(int64, int64) {}
	}
	log.Log(ctx, "SP_READ_TRACE enabled", "path", path)

	lines := make(chan string, 4096)
	go func() {
		w := bufio.NewWriter(fd)
		defer func() {
			// Drain whatever is still queued, then flush + close.
			for {
				select {
				case s := <-lines:
					fmt.Fprintln(w, s)
				default:
					_ = w.Flush()
					_ = fd.Close()
					return
				}
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case s := <-lines:
				fmt.Fprintln(w, s)
			}
		}
	}()

	return func(pos, size int64) {
		select {
		case lines <- fmt.Sprintf("readAt pos=%d size=%d", pos, size):
		case <-ctx.Done():
		}
	}
}

// RandomAccessSrcBin wraps an appsrc element in random-access (BYTES) mode
// inside a gst.Bin with a single ghost src pad named "src". The element is
// driven by the supplied io.ReaderAt, whose total length must be known up
// front and passed as size.
//
// The intended use is feeding arbitrary uploaded media (which lives on local
// disk or S3) into parsebin/qtdemux/etc. parsebin needs random access so it
// can find the moov atom near the end of MP4 files.
//
// The context is captured for the bin's lifetime. Callers MUST cancel ctx
// when they're done with the bin so any in-flight ReadAt — particularly the
// S3 case, where ReadAt is an HTTP request — can abort cleanly.
func RandomAccessSrcBin(ctx context.Context, name string, src io.ReaderAt, size int64) (*gst.Bin, error) {
	ctx = log.WithLogValues(ctx, "func", "RandomAccessSrcBin")
	if size < 0 {
		return nil, fmt.Errorf("size must be non-negative, got %d", size)
	}
	bin := gst.NewBin(name + "-bin")

	appSrc, err := gst.NewElementWithProperties("appsrc", map[string]interface{}{
		"name": name,
	})
	if err != nil {
		return nil, fmt.Errorf("create appsrc: %w", err)
	}
	if err := bin.Add(appSrc); err != nil {
		return nil, fmt.Errorf("add appsrc to bin: %w", err)
	}

	source := app.SrcFromElement(appSrc)
	// appsrc defaults to GST_FORMAT_BYTES, which is what we want for byte-
	// offset seeking. SetSize lets downstream elements (parsebin, qtdemux)
	// query a duration in bytes and seek to the end of the stream.
	source.SetSize(size)
	source.SetStreamType(app.AppStreamTypeRandomAccess)

	// pos tracks where the next NeedDataFunc read should start. eos is a
	// local guard against pushing past end-of-stream after we've already
	// emitted it; SeekDataFunc clears it so post-EOS seeks (which qtdemux
	// performs after parsing moov) can resume reads.
	//
	// appsrc serializes its own callbacks, so no mutex is needed.
	var pos int64
	var eos bool

	trace := newReadTracer(ctx)

	source.SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: func(self *app.Source, length uint) {
			if ctx.Err() != nil {
				self.EndStream()
				return
			}
			if eos {
				return
			}
			remaining := size - pos
			if remaining <= 0 {
				self.EndStream()
				eos = true
				return
			}
			n := int64(length)
			if n <= 0 {
				n = 64 * 1024
			}
			if n > remaining {
				n = remaining
			}
			buf := make([]byte, n)
			trace(pos, n)
			read, err := src.ReadAt(buf, pos)
			if read > 0 {
				gbuf := gst.NewBufferWithSize(int64(read))
				gbuf.Map(gst.MapWrite).WriteData(buf[:read])
				gbuf.Unmap()
				if ret := self.PushBuffer(gbuf); ret != gst.FlowOK {
					log.Debug(ctx, "RandomAccessSrcBin: push buffer non-OK", "ret", ret.String())
				}
				pos += int64(read)
			}
			if err != nil && !errors.Is(err, io.EOF) {
				log.Error(ctx, "RandomAccessSrcBin: read failed", "offset", pos, "error", err)
				self.Error("read failed", err)
				eos = true
				return
			}
			if errors.Is(err, io.EOF) || pos >= size {
				self.EndStream()
				eos = true
			}
		},
		SeekDataFunc: func(self *app.Source, offset uint64) bool {
			if ctx.Err() != nil {
				return false
			}
			if int64(offset) > size {
				return false
			}
			log.Debug(ctx, "RandomAccessSrcBin: seek", "from", pos, "to", int64(offset))
			pos = int64(offset)
			eos = false
			return true
		},
	})

	srcPad := appSrc.GetStaticPad("src")
	if srcPad == nil {
		return nil, fmt.Errorf("appsrc missing src pad")
	}
	ghost := gst.NewGhostPad("src", srcPad)
	if ghost == nil {
		return nil, fmt.Errorf("create ghost pad")
	}
	if !bin.AddPad(ghost.Pad) {
		return nil, fmt.Errorf("add ghost pad to bin")
	}

	return bin, nil
}
