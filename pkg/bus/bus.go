package bus

import (
	"strings"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

type Message any
type Subscription chan Message

const maxQueuedMessages = 1000

type subscriber struct {
	ch         Subscription
	mu         sync.Mutex
	cond       *sync.Cond
	queue      []Message
	limit      int
	onOverflow func()
	closed     bool
	done       chan struct{}
}

func newSubscriber(limit int, onOverflow func()) *subscriber {
	s := &subscriber{
		ch:         make(Subscription, 100),
		done:       make(chan struct{}),
		limit:      limit,
		onOverflow: onOverflow,
	}
	s.cond = sync.NewCond(&s.mu)
	go s.deliver()
	return s
}

func (s *subscriber) enqueue(msg Message) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	if s.limit > 0 && len(s.queue) >= s.limit {
		s.closed = true
		s.queue = nil
		close(s.done)
		s.cond.Signal()
		onOverflow := s.onOverflow
		s.mu.Unlock()
		if onOverflow != nil {
			onOverflow()
		}
		return
	}
	s.queue = append(s.queue, msg)
	s.cond.Signal()
	s.mu.Unlock()
}

func (s *subscriber) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.queue = nil
	close(s.done)
	s.cond.Signal()
}

func (s *subscriber) deliver() {
	for {
		s.mu.Lock()
		for len(s.queue) == 0 && !s.closed {
			s.cond.Wait()
		}
		if s.closed {
			s.mu.Unlock()
			return
		}
		msg := s.queue[0]
		s.queue[0] = nil
		s.queue = s.queue[1:]
		s.mu.Unlock()

		select {
		case s.ch <- msg:
		case <-s.done:
			return
		}
	}
}

type ViewerCountUpdate struct {
	Streamer string
	Count    int
	Origin   string
}

// Bus is a simple pub/sub system for backing websocket connections
type Bus struct {
	mu                       sync.Mutex
	clients                  map[string][]*subscriber
	segChans                 map[string][]*SegChan
	segChansMutex            sync.Mutex
	segBuf                   map[string][]*Seg
	segBufMutex              sync.RWMutex
	viewerCounts             map[string]map[string]int
	viewerCountsMutex        sync.RWMutex
	viewerCountSubscriptions []chan ViewerCountUpdate

	// federatedViewCounts stores reported view counts from remote servers.
	// Key: "{streamerDID}:{serverDID}", Value: int (count).
	// 2min TTL so stale servers drop off (records update every ~30s).
	federatedViewCounts *cache.Cache
}

func NewBus() *Bus {
	return &Bus{
		clients:                  make(map[string][]*subscriber),
		segChans:                 make(map[string][]*SegChan),
		segBuf:                   make(map[string][]*Seg),
		viewerCounts:             make(map[string]map[string]int),
		viewerCountSubscriptions: []chan ViewerCountUpdate{},
		federatedViewCounts:      cache.New(2*time.Minute, 4*time.Minute),
	}
}

func (b *Bus) Subscribe(user string) <-chan Message {
	return b.subscribeWithOverflow(user, 0, nil)
}

// SubscribeWithBacklogLimit bounds queued messages and calls onOverflow when a
// consumer falls behind. Use this for clients that can reconnect and receive a
// fresh snapshot after overflow.
func (b *Bus) SubscribeWithBacklogLimit(user string, onOverflow func()) <-chan Message {
	return b.subscribeWithOverflow(user, maxQueuedMessages, onOverflow)
}

func (b *Bus) subscribeWithOverflow(user string, limit int, onOverflow func()) <-chan Message {
	if b == nil {
		return make(<-chan Message)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	sub := newSubscriber(limit, onOverflow)
	b.clients[user] = append(b.clients[user], sub)
	return sub.ch
}

func (b *Bus) Unsubscribe(user string, ch <-chan Message) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	subs, ok := b.clients[user]
	if !ok {
		return
	}

	for i, sub := range subs {
		if sub.ch == ch {
			// Remove the subscription by replacing it with the last element
			// and then truncating the slice
			subs[i] = subs[len(subs)-1]
			b.clients[user] = subs[:len(subs)-1]
			sub.close()
			break
		}
	}
}

func (b *Bus) SubscribeToViewerCount() <-chan ViewerCountUpdate {
	b.viewerCountsMutex.Lock()
	defer b.viewerCountsMutex.Unlock()
	ch := make(chan ViewerCountUpdate, 100)
	b.viewerCountSubscriptions = append(b.viewerCountSubscriptions, ch)
	return ch
}

func (b *Bus) Publish(user string, msg Message) {
	b.mu.Lock()
	defer b.mu.Unlock()
	subs, ok := b.clients[user]
	if !ok {
		return
	}
	for _, sub := range subs {
		sub.enqueue(msg)
	}
}

func (b *Bus) GetViewerCount(user string) int {
	b.viewerCountsMutex.RLock()
	defer b.viewerCountsMutex.RUnlock()
	// Local viewer counts (from HLS connections on this node)
	count := 0
	if streamerCounts, ok := b.viewerCounts[user]; ok {
		for _, viewers := range streamerCounts {
			count += viewers
		}
	}
	// Federated view counts (reported by remote servers via atproto)
	prefix := user + ":"
	for k, v := range b.federatedViewCounts.Items() {
		if strings.HasPrefix(k, prefix) && !v.Expired() {
			if c, ok := v.Object.(int); ok {
				count += c
			}
		}
	}
	return count
}

// SetFederatedViewCount stores a view count reported by a remote server.
// The count will expire after 2 minutes if not refreshed.
func (b *Bus) SetFederatedViewCount(streamer string, server string, count int) {
	key := streamer + ":" + server
	b.federatedViewCounts.SetDefault(key, count)
}

func (b *Bus) SetViewerCount(user string, origin string, count int) {
	b.viewerCountsMutex.Lock()
	defer b.viewerCountsMutex.Unlock()
	_, ok := b.viewerCounts[user]
	if !ok {
		b.viewerCounts[user] = make(map[string]int)
	}
	b.viewerCounts[user][origin] = count
	b.notifyViewerCountSubscribers(user, count, origin)
}

func (b *Bus) IncrementViewerCount(user string, origin string) {
	b.viewerCountsMutex.Lock()
	defer b.viewerCountsMutex.Unlock()
	_, ok := b.viewerCounts[user]
	if !ok {
		b.viewerCounts[user] = make(map[string]int)
	}
	b.viewerCounts[user][origin] += 1
	b.notifyViewerCountSubscribers(user, b.viewerCounts[user][origin], origin)
}

func (b *Bus) DecrementViewerCount(user string, origin string) {
	b.viewerCountsMutex.Lock()
	defer b.viewerCountsMutex.Unlock()
	_, ok := b.viewerCounts[user]
	if !ok {
		b.viewerCounts[user] = make(map[string]int)
	}
	b.viewerCounts[user][origin] -= 1
	b.notifyViewerCountSubscribers(user, b.viewerCounts[user][origin], origin)
}

// only call if you're holding viewerCountsMutex
func (b *Bus) notifyViewerCountSubscribers(user string, count int, origin string) {
	for _, sub := range b.viewerCountSubscriptions {
		go func() {
			sub <- ViewerCountUpdate{Streamer: user, Count: count, Origin: origin}
		}()
	}
}
