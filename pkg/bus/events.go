package bus

import "context"

// SubscribeEvents returns a typed view of a bus subscription: it
// subscribes to user's channel and forwards only messages assignable to
// T, dropping everything else. The subscription is released and the
// returned channel closed when ctx ends.
func SubscribeEvents[T any](ctx context.Context, b *Bus, user string) <-chan T {
	out := make(chan T, 100)
	sub := b.Subscribe(user)
	go func() {
		defer close(out)
		defer b.Unsubscribe(user, sub)
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-sub:
				if !ok {
					return
				}
				if v, ok := msg.(T); ok {
					select {
					case out <- v:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
	return out
}
