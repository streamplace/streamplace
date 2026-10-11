package moq

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// testAnnouncer is a publisher whose broadcasts come and go.
type testAnnouncer struct {
	testPublisher
	mu        sync.Mutex
	live      map[string]bool
	listeners []chan Announcement
}

func (p *testAnnouncer) Announce(ctx context.Context, prefix string) <-chan Announcement {
	ch := make(chan Announcement, 16)
	p.mu.Lock()
	for path := range p.live {
		ch <- Announcement{Path: path, Active: true}
	}
	p.listeners = append(p.listeners, ch)
	p.mu.Unlock()
	go func() {
		<-ctx.Done()
		p.mu.Lock()
		defer p.mu.Unlock()
		for i, l := range p.listeners {
			if l == ch {
				p.listeners = append(p.listeners[:i], p.listeners[i+1:]...)
				close(ch)
				return
			}
		}
	}()
	return ch
}

func (p *testAnnouncer) set(path string, active bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if active {
		p.live[path] = true
	} else {
		delete(p.live, path)
	}
	for _, l := range p.listeners {
		l <- Announcement{Path: path, Active: active}
	}
}

// A client that dials as a publisher is handed to the server's Publishers
// hook, which learns its broadcasts by ANNOUNCE and subscribes to them
// like any publisher's: the whole of a push, over both bindings.
func TestPushAnnouncesAndServes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, binding := range []string{"moqt", "https"} {
		t.Run(binding, func(t *testing.T) {
			tr := &testTrack{groups: groups(3), closed: make(chan struct{})}
			pub := &testAnnouncer{
				testPublisher: testPublisher{tracks: map[string]*testTrack{"did:plc:a/source": tr}},
				live:          map[string]bool{"did:plc:a": true},
			}
			type got struct {
				anns   []Announcement
				frames []string
				err    error
			}
			results := make(chan got, 1)
			port, clientTLS := serveWith(t, ctx, &Server{Publishers: func(ctx context.Context, sess *Session) {
				g := got{}
				g.err = func() error {
					ann, err := sess.Announced(ctx, "")
					if err != nil {
						return err
					}
					a, err := ann.Next(ctx)
					if err != nil {
						return err
					}
					g.anns = append(g.anns, a)
					sub, err := sess.Subscribe(ctx, a.Path, "source")
					if err != nil {
						return err
					}
					for {
						f, err := sub.Next(ctx)
						if err == io.EOF {
							break
						}
						if err != nil {
							return err
						}
						g.frames = append(g.frames, string(f.Payload))
					}
					// A broadcast ending is announced too.
					a, err = ann.Next(ctx)
					if err != nil {
						return err
					}
					g.anns = append(g.anns, a)
					return nil
				}()
				results <- g
				<-ctx.Done()
			}})

			sess, err := Dial(ctx, fmt.Sprintf("%s://127.0.0.1:%d/push", binding, port), DialOptions{TLS: clientTLS, Publisher: pub})
			require.NoError(t, err)
			defer sess.Close()
			// The receiver drains the track; then the broadcast goes away.
			select {
			case <-tr.closed:
			case <-time.After(5 * time.Second):
				t.Fatal("receiver never subscribed to the pushed track")
			}
			pub.set("did:plc:a", false)
			select {
			case g := <-results:
				require.NoError(t, g.err)
				require.Equal(t, []Announcement{{Path: "did:plc:a", Active: true}, {Path: "did:plc:a"}}, g.anns)
				require.Equal(t, []string{"segment-0", "segment-1", "segment-2"}, g.frames)
			case <-time.After(5 * time.Second):
				t.Fatal("receiver did not get the pushed broadcast")
			}
		})
	}
}

// A server with no Publishers hook refuses a publishing client outright,
// as unauthorized, rather than holding a session it will never read from.
func TestPushRefusedWithoutAcceptor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port, clientTLS := serve(t, ctx, &testPublisher{})
	sess, err := Dial(ctx, fmt.Sprintf("moqt://127.0.0.1:%d", port), DialOptions{TLS: clientTLS, Publisher: &testPublisher{}})
	require.NoError(t, err)
	defer sess.Close()
	select {
	case <-sess.Done():
		require.Error(t, sess.Err())
	case <-time.After(5 * time.Second):
		t.Fatal("publishing session was not refused")
	}
}

// A subscriber asking a plain Publisher for announcements gets the one
// covering route, and the announcements end with the session.
func TestAnnouncedCoveringRoute(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port, clientTLS := serve(t, ctx, &testPublisher{})
	sess, err := Dial(ctx, fmt.Sprintf("moqt://127.0.0.1:%d", port), DialOptions{TLS: clientTLS})
	require.NoError(t, err)
	defer sess.Close()
	ann, err := sess.Announced(ctx, "did:")
	require.NoError(t, err)
	a, err := ann.Next(ctx)
	require.NoError(t, err)
	require.Equal(t, Announcement{Path: "did:", Active: true}, a)
	ann.Close()
	_, err = ann.Next(ctx)
	require.Error(t, err)
}
