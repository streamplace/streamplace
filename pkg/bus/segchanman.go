package bus

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spmetrics"
)

// it's a segment channel manager, you see

type Seg struct {
	Filepath       string
	Data           []byte // presentation flat MP4
	Muxl           []byte // bare canonical .m4s (blindly concatenatable)
	PacketizedData *PacketizedSegment
	Published      bool
}

type PacketizedSegment struct {
	Video    [][]byte
	Audio    [][]byte
	Duration time.Duration
}

var chanSize = 1024

type SegChan struct {
	C       chan *Seg
	Context context.Context
}

var bufSize = 10

func segChanKey(user string, rendition string) string {
	return fmt.Sprintf("%s::%s", user, rendition)
}

// get a channel to subscribe to new segments for a given user and rendition
func (b *Bus) SubscribeSegment(ctx context.Context, user string, rendition string) *SegChan {
	return b.SubscribeSegmentBuf(ctx, user, rendition, 0)
}

// get a channel to subscribe to new segments for a given user and rendition,
// starting with bufSize cached segments that we already have. The channel is
// buffered (chanSize) and preloaded with the cached segments, oldest first, so
// a new player joins with a warmup buffer instead of at the bare live edge.
func (b *Bus) SubscribeSegmentBuf(ctx context.Context, user string, rendition string, bufSize int) *SegChan {
	key := segChanKey(user, rendition)
	b.segChansMutex.Lock()
	defer b.segChansMutex.Unlock()
	chs, ok := b.segChans[key]
	if !ok {
		chs = []*SegChan{}
		b.segChans[key] = chs
	}
	ch := make(chan *Seg, chanSize)
	b.segBufMutex.RLock()
	defer b.segBufMutex.RUnlock()
	curBuf, ok := b.segBuf[key]
	if ok {
		if bufSize > len(curBuf) {
			bufSize = len(curBuf)
		}
		for i := 0; i < bufSize; i += 1 {
			ch <- curBuf[len(curBuf)-bufSize+i]
		}
	}
	segChan := &SegChan{C: ch, Context: ctx}
	chs = append(chs, segChan)
	b.segChans[key] = chs
	spmetrics.SegmentSubscriptionsOpen.WithLabelValues(user, rendition).Set(float64(len(chs)))
	return segChan
}

// unsubscribe from a channel for a given user and rendition
func (b *Bus) UnsubscribeSegment(ctx context.Context, user string, rendition string, ch *SegChan) {
	key := segChanKey(user, rendition)
	b.segChansMutex.Lock()
	defer b.segChansMutex.Unlock()
	chs, ok := b.segChans[key]
	if !ok {
		return
	}
	for i, c := range chs {
		if c == ch {
			chs = append(chs[:i], chs[i+1:]...)
			break
		}
	}
	spmetrics.SegmentSubscriptionsOpen.WithLabelValues(user, rendition).Set(float64(len(chs)))
	b.segChans[key] = chs
}

// PublishSegment fans a segment out to every subscriber of the user+rendition
// and folds it into the replay buffer new subscribers warm up from. Delivery
// order per subscriber is always publish call order: sends are non-blocking
// into buffered channels under the publish mutex, with no per-publish
// goroutines that could complete out of order.
func (b *Bus) PublishSegment(ctx context.Context, user string, rendition string, seg *Seg) {
	ctx, span := otel.Tracer("signer").Start(ctx, "PublishSegment")
	defer span.End()
	key := segChanKey(user, rendition)
	b.segChansMutex.Lock()
	defer b.segChansMutex.Unlock()
	b.segBufMutex.Lock()
	defer b.segBufMutex.Unlock()
	curBuf, ok := b.segBuf[key]
	if !ok {
		curBuf = []*Seg{}
		b.segBuf[key] = curBuf
	}
	curBuf = append(curBuf, seg)
	if len(curBuf) > bufSize {
		curBuf = curBuf[1:]
	}
	b.segBuf[key] = curBuf
	chs, ok := b.segChans[key]
	if !ok {
		return
	}
	for _, ch := range chs {
		// Non-blocking under the publish mutex: subscriber channels are
		// buffered, so delivery order is always publish order. A subscriber a
		// full buffer behind loses its oldest queued segment instead of the
		// live edge — with ordered delivery a skip recovers at the next
		// keyframe, while falling ever further behind does not.
		select {
		case ch.C <- seg:
		default:
			select {
			case <-ch.C:
			default:
			}
			select {
			case ch.C <- seg:
			default:
			}
			spmetrics.SegmentPublishDropped.WithLabelValues(user, rendition).Inc()
			log.Warn(ctx, "subscriber a full buffer behind, dropped its oldest segment", "user", user, "rendition", rendition)
		}
	}
}

func (b *Bus) EndSession(ctx context.Context, user string, rendition string) {
	b.segChansMutex.Lock()
	defer b.segChansMutex.Unlock()
	b.segBufMutex.Lock()
	defer b.segBufMutex.Unlock()

	key := segChanKey(user, rendition)
	delete(b.segBuf, key)
}
