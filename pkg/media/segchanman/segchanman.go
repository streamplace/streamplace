package segchanman

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
)

// it's a segment channel manager, you see

type Seg struct {
	Filepath string
	Data     []byte
}

type SegChanMan struct {
	segChans      map[string][]chan *Seg
	segChansMutex sync.Mutex
}

func MakeSegChanMan() *SegChanMan {
	return &SegChanMan{
		segChans: make(map[string][]chan *Seg),
	}
}

func segChanKey(user string, rendition string) string {
	return fmt.Sprintf("%s::%s", user, rendition)
}

func (s *SegChanMan) SubscribeSegment(ctx context.Context, user string, rendition string) <-chan *Seg {
	key := segChanKey(user, rendition)
	s.segChansMutex.Lock()
	defer s.segChansMutex.Unlock()
	chs, ok := s.segChans[key]
	if !ok {
		chs = []chan *Seg{}
		s.segChans[key] = chs
	}
	ch := make(chan *Seg, 1024)
	chs = append(chs, ch)
	s.segChans[key] = chs
	return ch
}

func (s *SegChanMan) UnsubscribeSegment(ctx context.Context, user string, rendition string, ch <-chan *Seg) {
	key := segChanKey(user, rendition)
	s.segChansMutex.Lock()
	defer s.segChansMutex.Unlock()
	chs, ok := s.segChans[key]
	if !ok {
		return
	}
	for i, c := range chs {
		if c == ch {
			chs = append(chs[:i], chs[i+1:]...)
			break
		}
	}
	s.segChans[key] = chs
}

func (s *SegChanMan) PublishSegment(ctx context.Context, user string, rendition string, seg *Seg) {
	ctx, span := otel.Tracer("signer").Start(ctx, "PublishSegment")
	defer span.End()
	key := segChanKey(user, rendition)
	s.segChansMutex.Lock()
	defer s.segChansMutex.Unlock()
	chs, ok := s.segChans[key]
	if !ok {
		return
	}
	for _, ch := range chs {
		go func(ch chan *Seg) {
			ch <- seg
		}(ch)
	}
}
