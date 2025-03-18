package bus

import (
	"sync"
)

type Message any
type Subscription chan Message

// Bus is a simple pub/sub system for backing websocket connections
type Bus struct {
	mu      sync.Mutex
	clients map[string][]Subscription
}

func NewBus() *Bus {
	return &Bus{
		clients: make(map[string][]Subscription),
	}
}

func (b *Bus) Subscribe(user string) <-chan Message {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan Message)
	b.clients[user] = append(b.clients[user], ch)
	return ch
}

func (b *Bus) Unsubscribe(user string, ch <-chan Message) {
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
	for _, sub := range b.clients[user] {
		go func(sub Subscription) {
			select {
			case sub <- msg:
			default:
				close(sub)
			}
		}(sub)
	}
}
