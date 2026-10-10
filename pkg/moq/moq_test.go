package moq

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Varints and timestamp deltas are the only non-trivial encoding on the
// wire; every message is built from them, so a boundary bug here corrupts
// everything after it.
func TestVarintRoundTrip(t *testing.T) {
	for _, v := range []uint64{0, 1, 63, 64, 16383, 16384, 1<<30 - 1, 1 << 30, maxVarint} {
		b := appendVarint(nil, v)
		got, err := readVarint(bufio.NewReader(bytes.NewReader(b)))
		require.NoError(t, err, "%d", v)
		require.Equal(t, v, got)
		p := body{b: b}
		require.Equal(t, v, p.varint())
		require.NoError(t, p.done())
	}
	for _, d := range []int64{0, 1, -1, 1 << 40, -(1 << 40), 3_000_000_000_000} {
		require.Equal(t, d, unzigzag(zigzag(d)), "%d", d)
	}
	// A truncated varint is unexpected EOF, not a short value.
	_, err := readVarint(bufio.NewReader(bytes.NewReader([]byte{0x40})))
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

// testTrack serves a fixed run of single-frame groups, then ends. It
// records whether the server released it, which is how a subscriber going
// away must reach the application.
type testTrack struct {
	groups []*Group
	closed chan struct{}
	ctxErr chan error
}

func (tr *testTrack) Info() TrackInfo {
	return TrackInfo{Priority: 1, MaxAge: 2 * time.Second, Timescale: 1000}
}

func (tr *testTrack) Next(ctx context.Context) (*Group, error) {
	if len(tr.groups) == 0 {
		if tr.ctxErr != nil {
			// Block like a live track with nothing to say until the
			// subscriber leaves.
			<-ctx.Done()
			tr.ctxErr <- ctx.Err()
			return nil, ctx.Err()
		}
		return nil, io.EOF
	}
	g := tr.groups[0]
	tr.groups = tr.groups[1:]
	return g, nil
}

func (tr *testTrack) Close() { close(tr.closed) }

type testPublisher struct {
	tracks map[string]*testTrack
}

func (p *testPublisher) Track(_ context.Context, broadcast, name string) (Track, error) {
	tr, ok := p.tracks[broadcast+"/"+name]
	if !ok {
		return nil, ErrNotFound
	}
	return tr, nil
}

func selfSigned(t *testing.T) (*tls.Config, *tls.Config) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "moq-test"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}},
		&tls.Config{RootCAs: pool}
}

// serve starts a Server on a loopback UDP port for the test's lifetime and
// returns its port.
func serve(t *testing.T, ctx context.Context, pub Publisher) (int, *tls.Config) {
	t.Helper()
	serverTLS, clientTLS := selfSigned(t)
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &Server{TLSConfig: serverTLS, Publisher: pub}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx, pc) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})
	return pc.LocalAddr().(*net.UDPAddr).Port, clientTLS
}

func groups(n int) []*Group {
	out := make([]*Group, n)
	for i := range out {
		out[i] = &Group{Sequence: uint64(i), Frames: []Frame{{Timestamp: int64(1_700_000_000_000 + i*1000), Payload: []byte(fmt.Sprintf("segment-%d", i))}}}
	}
	return out
}

// A subscription delivers every group's frame, in order, with the
// timestamps the publisher sent, then reports the track's end; over both
// bindings, since they share nothing below the session.
func TestSubscribeDeliversTrack(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, binding := range []string{"moqt", "https"} {
		t.Run(binding, func(t *testing.T) {
			tr := &testTrack{groups: groups(5), closed: make(chan struct{})}
			port, clientTLS := serve(t, ctx, &testPublisher{tracks: map[string]*testTrack{"did:plc:a/source": tr}})
			sess, err := Dial(ctx, fmt.Sprintf("%s://127.0.0.1:%d/anything?x=1", binding, port), DialOptions{TLS: clientTLS})
			require.NoError(t, err)
			defer sess.Close()

			sub, err := sess.Subscribe(ctx, "did:plc:a", "source")
			require.NoError(t, err)
			for i := 0; i < 5; i++ {
				f, err := sub.Next(ctx)
				require.NoError(t, err)
				require.Equal(t, uint64(i), f.Group)
				require.Equal(t, int64(1_700_000_000_000+i*1000), f.Timestamp)
				require.Equal(t, fmt.Sprintf("segment-%d", i), string(f.Payload))
			}
			_, err = sub.Next(ctx)
			require.ErrorIs(t, err, io.EOF)
			select {
			case <-tr.closed:
			case <-time.After(5 * time.Second):
				t.Fatal("publisher did not release the track after it ended")
			}
		})
	}
}

// A track the publisher does not have is refused with a reset, which
// Subscribe reports rather than waiting on.
func TestSubscribeUnknownTrackRefused(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	port, clientTLS := serve(t, ctx, &testPublisher{})
	sess, err := Dial(ctx, fmt.Sprintf("moqt://127.0.0.1:%d", port), DialOptions{TLS: clientTLS})
	require.NoError(t, err)
	defer sess.Close()
	sub, err := sess.Subscribe(ctx, "did:plc:nobody", "source")
	require.NoError(t, err)
	_, err = sub.Next(ctx)
	require.ErrorContains(t, err, "subscription reset")
}

// A subscriber that closes its subscription, or its whole session, ends
// the served track: the publisher's Next sees its context cancelled and
// Close runs. Otherwise every viewer that left would pin a bus
// subscription for the life of the node.
func TestSubscriberLeavingReleasesTrack(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, how := range []string{"subscription", "session"} {
		t.Run(how, func(t *testing.T) {
			tr := &testTrack{groups: groups(1), closed: make(chan struct{}), ctxErr: make(chan error, 1)}
			port, clientTLS := serve(t, ctx, &testPublisher{tracks: map[string]*testTrack{"did:plc:a/source": tr}})
			sess, err := Dial(ctx, fmt.Sprintf("moqt://127.0.0.1:%d", port), DialOptions{TLS: clientTLS})
			require.NoError(t, err)
			defer sess.Close()
			sub, err := sess.Subscribe(ctx, "did:plc:a", "source")
			require.NoError(t, err)
			_, err = sub.Next(ctx)
			require.NoError(t, err)
			if how == "subscription" {
				sub.Close()
			} else {
				require.NoError(t, sess.Close())
				_, err = sub.Next(ctx)
				require.ErrorIs(t, err, ErrClosed)
			}
			select {
			case err := <-tr.ctxErr:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(5 * time.Second):
				t.Fatal("track was not cancelled")
			}
			select {
			case <-tr.closed:
			case <-time.After(5 * time.Second):
				t.Fatal("track was not closed")
			}
		})
	}
}

// When the server goes away mid-stream the subscriber's Next fails with
// the transport's error instead of blocking forever; the pull loop
// reconnects on that.
func TestServerShutdownFailsSubscription(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srvCtx, stop := context.WithCancel(ctx)
	tr := &testTrack{groups: groups(1), closed: make(chan struct{}), ctxErr: make(chan error, 1)}
	port, clientTLS := serve(t, srvCtx, &testPublisher{tracks: map[string]*testTrack{"did:plc:a/source": tr}})
	sess, err := Dial(ctx, fmt.Sprintf("moqt://127.0.0.1:%d", port), DialOptions{TLS: clientTLS})
	require.NoError(t, err)
	defer sess.Close()
	sub, err := sess.Subscribe(ctx, "did:plc:a", "source")
	require.NoError(t, err)
	_, err = sub.Next(ctx)
	require.NoError(t, err)
	stop()
	_, err = sub.Next(ctx)
	require.Error(t, err)
	require.False(t, errors.Is(err, io.EOF), "a dead server is not a clean end: %v", err)
	require.Error(t, sess.Err())
}

// fakeGroup is a group stream already in memory, for driving delivery
// without a network.
type fakeGroup struct{ *bytes.Reader }

func (fakeGroup) CancelRead(uint64) {}

func newFakeGroup(seq uint64, payload string) groupStream {
	var b []byte
	b = appendVarint(b, zigzag(int64(seq*1000)))
	b = appendVarint(b, uint64(len(payload)))
	b = append(b, payload...)
	r := bytes.NewReader(b)
	return groupStream{seq: seq, r: bufio.NewReader(r), st: fakeGroup{r}}
}

// Groups arrive on independent streams in whatever order the network and
// the transport's accept queue produce, and a subscription hands them out
// in sequence order regardless: a straggler is waited for, a group the
// publisher dropped is not, and one that never shows up is given up on
// after gapTimeout rather than stalling the track forever.
func TestSubscriptionOrdersGroups(t *testing.T) {
	gapTimeout = 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sctx, scancel := context.WithCancelCause(ctx)
	defer scancel(nil)
	s := &Session{ctx: sctx, cancel: scancel, subs: map[uint64]*Subscription{}}
	sub := &Subscription{s: s, done: make(chan struct{}), frames: make(chan *Frame, 16), notify: make(chan struct{}, 1), started: true}
	go sub.run()
	defer sub.Close()

	next := func() uint64 {
		f, err := sub.Next(ctx)
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("g%d", f.Group), string(f.Payload))
		return f.Group
	}

	// Out of order: 2 and 1 wait for 0.
	sub.enqueue(newFakeGroup(2, "g2"))
	sub.enqueue(newFakeGroup(1, "g1"))
	select {
	case f := <-sub.frames:
		t.Fatalf("group %d delivered before group 0", f.Group)
	case <-time.After(50 * time.Millisecond):
	}
	sub.enqueue(newFakeGroup(0, "g0"))
	require.Equal(t, uint64(0), next())
	require.Equal(t, uint64(1), next())
	require.Equal(t, uint64(2), next())

	// Dropped: the publisher said 3 and 4 will never come, so 5 is next.
	sub.mu.Lock()
	sub.drops = append(sub.drops, [2]uint64{3, 4})
	sub.mu.Unlock()
	sub.enqueue(newFakeGroup(5, "g5"))
	require.Equal(t, uint64(5), next())

	// Missing: 6 never arrives, 7 is delivered once the gap times out.
	sub.enqueue(newFakeGroup(7, "g7"))
	started := time.Now()
	require.Equal(t, uint64(7), next())
	require.GreaterOrEqual(t, time.Since(started), gapTimeout)

	// Ended at 9 with 8 missing: the track ends after the gap, not never.
	sub.mu.Lock()
	sub.ended, sub.end = true, 9
	sub.mu.Unlock()
	sub.signal()
	_, err := sub.Next(ctx)
	require.ErrorIs(t, err, io.EOF)
}

// stalledGroup is a group stream whose peer has gone quiet mid-frame: Read
// blocks until the stream is cancelled.
type stalledGroup struct {
	cancelled chan struct{}
	once      sync.Once
}

func (g *stalledGroup) Read([]byte) (int, error) {
	<-g.cancelled
	return 0, errors.New("cancelled")
}

func (g *stalledGroup) CancelRead(uint64) { g.once.Do(func() { close(g.cancelled) }) }

// Closing a subscription cuts short a read the peer has stalled, and
// releases the group streams still queued behind it: otherwise a viewer
// that left would hold a goroutine and the streams' flow-control credit
// for as long as the peer kept the session alive.
func TestSubscriptionCloseCancelsStreams(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sctx, scancel := context.WithCancelCause(ctx)
	defer scancel(nil)
	s := &Session{ctx: sctx, cancel: scancel, subs: map[uint64]*Subscription{}}
	sub := &Subscription{s: s, done: make(chan struct{}), frames: make(chan *Frame, 16), notify: make(chan struct{}, 1), started: true}
	go sub.run()

	stalled := &stalledGroup{cancelled: make(chan struct{})}
	queued := &stalledGroup{cancelled: make(chan struct{})}
	sub.enqueue(groupStream{seq: 0, r: bufio.NewReader(stalled), st: stalled})
	sub.enqueue(groupStream{seq: 1, r: bufio.NewReader(queued), st: queued})
	// Let the reader block on group 0's first byte.
	require.Eventually(t, func() bool {
		sub.mu.Lock()
		defer sub.mu.Unlock()
		return sub.active == stalled
	}, 5*time.Second, 10*time.Millisecond)

	sub.Close()
	_, err := sub.Next(ctx)
	require.ErrorIs(t, err, io.EOF, "a closed subscription ends, it does not hang")
	for _, g := range []*stalledGroup{stalled, queued} {
		select {
		case <-g.cancelled:
		case <-time.After(5 * time.Second):
			t.Fatal("group stream was not cancelled")
		}
	}
}

// Drop ranges are remembered merged, so a publisher repeating or splitting
// the same announcement costs nothing, and bounded, so one that keeps
// announcing ranges ahead of delivery is cut off instead of growing state.
func TestSubscriptionDropRanges(t *testing.T) {
	sub := &Subscription{}
	for i := 0; i < 1000; i++ {
		require.NoError(t, sub.addDrop(1_000_000, 1_000_000))
	}
	require.NoError(t, sub.addDrop(1_000_001, 1_000_005))
	require.NoError(t, sub.addDrop(999_990, 999_999))
	require.Equal(t, [][2]uint64{{999_990, 1_000_005}}, sub.drops)
	require.Error(t, sub.addDrop(5, 4), "inverted range")
	sub.next = 2_000_000
	require.NoError(t, sub.addDrop(10, 20), "a range already behind delivery")
	require.Len(t, sub.drops, 1)
	var err error
	for i := uint64(0); err == nil && i < 2*maxDrops; i++ {
		err = sub.addDrop(3_000_000+10*i, 3_000_000+10*i+1)
	}
	require.Error(t, err, "unbounded ranges ahead of delivery")
}

// A bounded subscription (a Group End) is served up to the bound and then
// ended, rather than fed the live track forever.
func TestSubscribeBounded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tr := &testTrack{groups: groups(5), closed: make(chan struct{})}
	port, clientTLS := serve(t, ctx, &testPublisher{tracks: map[string]*testTrack{"did:plc:a/source": tr}})
	sess, err := Dial(ctx, fmt.Sprintf("moqt://127.0.0.1:%d", port), DialOptions{TLS: clientTLS})
	require.NoError(t, err)
	defer sess.Close()
	sub, err := sess.subscribe(ctx, "did:plc:a", "source", 3)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		f, err := sub.Next(ctx)
		require.NoError(t, err)
		require.Equal(t, uint64(i), f.Group)
	}
	_, err = sub.Next(ctx)
	require.ErrorIs(t, err, io.EOF)
}
