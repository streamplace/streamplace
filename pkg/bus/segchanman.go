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
	Streamer       string
	Rendition      string
	Timing         *SegmentTiming
}

// PacketizedSample is one WebRTC-writable sample — a video access unit or an
// Opus packet — with its real duration on the source timeline. Carrying the
// per-sample duration (rather than dividing the segment evenly) keeps
// non-uniform frame spacing intact: an encoder shedding frames under
// bandwidth pressure sends bursts and gaps, and respacing those uniformly
// rubber-bands the video against the audio.
type PacketizedSample struct {
	Data     []byte
	Duration time.Duration
}

type PacketizedSegment struct {
	Video     []PacketizedSample
	Audio     []PacketizedSample
	Duration  time.Duration
	Streamer  string
	Rendition string
	Timing    *SegmentTiming
}

var chanSize = 1024

type SegChan struct {
	C       chan *Seg
	Context context.Context
	// Keeps publication handoff independent from the consumer-facing channel.
	publish chan *Seg
	cancel  context.CancelFunc
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
// starting with bufSize cached segments that we already have
func (b *Bus) SubscribeSegmentBuf(ctx context.Context, user string, rendition string, bufSize int) *SegChan {
	key := segChanKey(user, rendition)
	b.segChansMutex.Lock()
	defer b.segChansMutex.Unlock()
	chs, ok := b.segChans[key]
	if !ok {
		chs = []*SegChan{}
		b.segChans[key] = chs
	}
	b.segBufMutex.RLock()
	defer b.segBufMutex.RUnlock()
	curBuf, ok := b.segBuf[key]
	myCh := make(chan *Seg, chanSize)
	if ok {
		if bufSize > len(curBuf) {
			bufSize = len(curBuf)
		}
		for i := 0; i < bufSize; i += 1 {
			myCh <- curBuf[len(curBuf)-bufSize+i]
		}
	}
	dispatchCtx, cancel := context.WithCancel(ctx)
	segChan := &SegChan{
		C:       myCh,
		Context: dispatchCtx,
		publish: make(chan *Seg, chanSize),
		cancel:  cancel,
	}
	chs = append(chs, segChan)
	b.segChans[key] = chs
	spmetrics.SegmentSubscriptionsOpen.WithLabelValues(user, rendition).Set(float64(len(chs)))
	go dispatchSegments(segChan, user, rendition)
	return segChan
}

func dispatchSegments(ch *SegChan, user string, rendition string) {
	// Serialize delivery per subscriber without making the publisher wait for a
	// slow consumer.
	for {
		select {
		case <-ch.Context.Done():
			return
		case seg := <-ch.publish:
			select {
			case ch.C <- seg:
			case <-ch.Context.Done():
				return
			case <-time.After(time.Minute):
				log.Warn(ch.Context, "failed to send segment to channel, timing out", "user", user, "rendition", rendition)
			}
		}
	}
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
			ch.cancel()
			break
		}
	}
	spmetrics.SegmentSubscriptionsOpen.WithLabelValues(user, rendition).Set(float64(len(chs)))
	b.segChans[key] = chs
}

func (b *Bus) PublishSegment(ctx context.Context, user string, rendition string, seg *Seg) {
	ctx, span := otel.Tracer("signer").Start(ctx, "PublishSegment")
	defer span.End()
	key := segChanKey(user, rendition)
	b.segBufMutex.Lock()
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
	b.segBufMutex.Unlock()

	b.segChansMutex.Lock()
	chs := append([]*SegChan(nil), b.segChans[key]...)
	b.segChansMutex.Unlock()
	if len(chs) == 0 {
		return
	}
	// A live subscriber that cannot keep up must not stall publication to other
	// subscribers.
	for _, ch := range chs {
		select {
		case ch.publish <- seg:
		case <-ch.Context.Done():
		default:
			log.Warn(ctx, "dropping segment for slow subscriber", "user", user, "rendition", rendition)
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
