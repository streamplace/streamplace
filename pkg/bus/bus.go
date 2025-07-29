package bus

import (
	"sync"
)

type Message any
type Subscription chan Message

// Bus is a simple pub/sub system for backing websocket connections
type Bus struct {
	mu            sync.Mutex
	clients       map[string][]Subscription
	segChans      map[string][]*SegChan
	segChansMutex sync.Mutex
	segBuf        map[string][]*Seg
	segBufMutex   sync.RWMutex
}

func NewBus() *Bus {
	return &Bus{
		clients:  make(map[string][]Subscription),
		segChans: make(map[string][]*SegChan),
		segBuf:   make(map[string][]*Seg),
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
