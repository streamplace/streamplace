package bus

import (
	"sync"
)

type Message any
type Subscription chan Message

type ViewerCountUpdate struct {
	Streamer string
	Count    int
	Origin   string
}

// Bus is a simple pub/sub system for backing websocket connections
type Bus struct {
	mu                       sync.Mutex
	clients                  map[string][]Subscription
	segChans                 map[string][]*SegChan
	segChansMutex            sync.Mutex
	segBuf                   map[string][]*Seg
	segBufMutex              sync.RWMutex
	viewerCounts             map[string]map[string]int
	viewerCountsMutex        sync.RWMutex
	viewerCountSubscriptions []chan ViewerCountUpdate
}

func NewBus() *Bus {
	return &Bus{
		clients:                  make(map[string][]Subscription),
		segChans:                 make(map[string][]*SegChan),
		segBuf:                   make(map[string][]*Seg),
		viewerCounts:             make(map[string]map[string]int),
		viewerCountSubscriptions: []chan ViewerCountUpdate{},
	}
}

func (b *Bus) Subscribe(user string) <-chan Message {
	if b == nil {
		return make(<-chan Message)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan Message, 100)
	b.clients[user] = append(b.clients[user], ch)
	return ch
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
		if sub == ch {
			// Remove the subscription by replacing it with the last element
			// and then truncating the slice
			subs[i] = subs[len(subs)-1]
			b.clients[user] = subs[:len(subs)-1]
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
		go func(sub Subscription) {
			sub <- msg
		}(sub)
	}
}

func (b *Bus) GetViewerCount(user string) int {
	b.viewerCountsMutex.RLock()
	defer b.viewerCountsMutex.RUnlock()
	streamerCounts, ok := b.viewerCounts[user]
	if !ok {
		return 0
	}
	count := 0
	for _, viewers := range streamerCounts {
		count += viewers
	}
	return count
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
