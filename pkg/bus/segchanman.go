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

type SegChan struct {
	C       chan *Seg
	Context context.Context
}

func segChanKey(user string, rendition string) string {
	return fmt.Sprintf("%s::%s", user, rendition)
}

// get a channel to subscribe to new segments for a given user and rendition.
// Only segments published after the subscribe are delivered — there is
// deliberately no replay of recent segments; subscribers start at the live
// edge.
func (b *Bus) SubscribeSegment(ctx context.Context, user string, rendition string) *SegChan {
	key := segChanKey(user, rendition)
	b.segChansMutex.Lock()
	defer b.segChansMutex.Unlock()
	chs, ok := b.segChans[key]
	if !ok {
		chs = []*SegChan{}
		b.segChans[key] = chs
	}
	ch := make(chan *Seg)
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

func (b *Bus) PublishSegment(ctx context.Context, user string, rendition string, seg *Seg) {
	ctx, span := otel.Tracer("signer").Start(ctx, "PublishSegment")
	defer span.End()
	key := segChanKey(user, rendition)
	b.segChansMutex.Lock()
	defer b.segChansMutex.Unlock()
	chs, ok := b.segChans[key]
	if !ok {
		return
	}
	for _, ch := range chs {
		go func(segChan *SegChan) {
			select {
			case segChan.C <- seg:
			case <-segChan.Context.Done():
				return
			case <-time.After(1 * time.Minute):
				log.Warn(ctx, "failed to send segment to channel, timing out", "user", user, "rendition", rendition)
			}

		}(ch)
	}
}
