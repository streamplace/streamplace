package media

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/muxl"
)

func TestSigningCompletionDrainsCallbacksBeforeTermination(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gate := make(chan struct{})
	entered := make(chan struct{})
	finished := make(chan error, 1)
	sign := func(_ context.Context, _ io.Reader, ch chan *muxl.MuxlEvent) error {
		ch <- &muxl.MuxlEvent{Type: "signed-segment", Tracks: map[string][]byte{"1": {1}}}
		return nil
	}
	_, done, err := muxlSignSegmentElem(ctx, &config.CLI{}, sign, func(context.Context, []byte) error { close(entered); <-gate; return nil }, func(err error) { finished <- err })
	require.NoError(t, err)
	<-entered
	select {
	case <-finished:
		t.Fatal("termination preceded final callback")
	default:
	}
	close(gate)
	select {
	case err := <-finished:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("termination omitted")
	}
	<-done
}

func TestSigningEpochFailureAbortsSelectedPeer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mm := loopbackPlaybackManager()
	s := earlyState(t, 1)
	s.publish(earlyItem(true, 0, 1))
	s.publish(earlyItem(false, 0, 1))
	peer := s.join()
	require.NotNil(t, peer)
	mm.earlySessions = map[string]*earlyAACSession{t.Name(): s}
	_, done, err := mm.ingestSigningElem(ctx, func(_ context.Context, _ io.Reader, ch chan *muxl.MuxlEvent) error {
		ch <- &muxl.MuxlEvent{Type: "signed-segment", Tracks: map[string][]byte{"1": {1}}}
		return nil
	}, func(callbackCtx context.Context, _ []byte) error {
		state := ingestState(callbackCtx)
		state.mu.Lock()
		state.did = t.Name()
		state.mu.Unlock()
		return errors.New("validation rejected source")
	})
	require.NoError(t, err)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("failed signer owner did not finish")
	}
	require.Error(t, peer.ctx.Err())
	require.False(t, s.ended, "failure must not masquerade as normal EOS")
	require.Empty(t, mm.earlySessions)
}

func TestSigningCompletionFailureNeverReportsOrderlyEOF(t *testing.T) {
	for _, kind := range []string{"signer", "callback", "cancellation"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			failure := errors.New("source failure")
			finished := make(chan error, 1)
			sign := func(_ context.Context, _ io.Reader, ch chan *muxl.MuxlEvent) error {
				if kind == "signer" {
					return failure
				}
				if kind == "cancellation" {
					cancel()
					return nil
				}
				ch <- &muxl.MuxlEvent{Type: "signed-segment", Tracks: map[string][]byte{"1": {1}}}
				return nil
			}
			_, done, err := muxlSignSegmentElem(ctx, &config.CLI{}, sign, func(context.Context, []byte) error { return failure }, func(err error) { finished <- err })
			require.NoError(t, err)
			<-done
			require.Error(t, <-finished)
		})
	}
}
