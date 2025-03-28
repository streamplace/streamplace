package segchanman

import (
	"context"
	"fmt"
	"sync"
)

// it's a segment channel manager, you see

type SegChanMan struct {
	segChans      map[string][]chan string
	segChansMutex sync.Mutex
}

func MakeSegChanMan() *SegChanMan {
	return &SegChanMan{
		segChans: make(map[string][]chan string),
	}
}

func segChanKey(user string, rendition string) string {
	return fmt.Sprintf("%s::%s", user, rendition)
}

func (s *SegChanMan) SubscribeSegment(ctx context.Context, user string, rendition string) <-chan string {
	key := segChanKey(user, rendition)
	s.segChansMutex.Lock()
	defer s.segChansMutex.Unlock()
	chs, ok := s.segChans[key]
	if !ok {
		chs = []chan string{}
		s.segChans[key] = chs
	}
	ch := make(chan string)
	chs = append(chs, ch)
	s.segChans[key] = chs
	return ch
}

func (s *SegChanMan) UnsubscribeSegment(ctx context.Context, user string, rendition string, ch <-chan string) {
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

func (s *SegChanMan) PublishSegment(ctx context.Context, user string, rendition string, seg string) {
	key := segChanKey(user, rendition)
	s.segChansMutex.Lock()
	defer s.segChansMutex.Unlock()
	chs, ok := s.segChans[key]
	if !ok {
		return
	}
	for _, ch := range chs {
		go func(ch chan string) {
			ch <- seg
		}(ch)
	}
}
