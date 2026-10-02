package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

// recordingTeeTestManager returns a MediaManager whose debug recordings go to
// the given S3 endpoint — the production shape.
func recordingTeeTestManager(t *testing.T, endpoint string) *MediaManager {
	t.Helper()
	return &MediaManager{cli: &config.CLI{
		DataDir:           t.TempDir(),
		S3Endpoint:        endpoint,
		S3Bucket:          "debug-bucket",
		S3AccessKeyID:     "test-access",
		S3SecretAccessKey: "test-secret",
		S3Region:          "auto",
	}}
}

// recordingTeePayload builds deterministic bytes to push through the tee. The
// recording only ever copies the ingest stream, so the content is arbitrary.
func recordingTeePayload(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

// readTeeWithin reads r to EOF in the background and fails the test if it does
// not finish promptly: a stalled read IS the regression these tests guard
// against, so a hang has to fail rather than block the suite.
func readTeeWithin(t *testing.T, r io.Reader, limit time.Duration) []byte {
	t.Helper()
	type result struct {
		b   []byte
		err error
	}
	ch := make(chan result, 1)
	go func() {
		b, err := io.ReadAll(r)
		ch <- result{b, err}
	}()
	select {
	case res := <-ch:
		require.NoError(t, res.err)
		return res.b
	case <-time.After(limit):
		t.Fatalf("ingest read stalled >%s with a failing debug recorder: recordings must never block the stream", limit)
		return nil
	}
}

// TestRecordTeeSurvivesRecorderCreateFailure covers a bucket that refuses the
// upload outright — rejected credentials, unpaid billing. Without a fail-safe
// tee the recorder dies on the create error, leaving the tee's pipe writer with
// no reader, so the very next read of the ingest stream blocks forever and the
// whole broadcast goes dark: the exact production failure this guards.
func TestRecordTeeSurvivesRecorderCreateFailure(t *testing.T) {
	fake := newFakeS3Server()
	fake.failInitiate = true
	srv := httptest.NewServer(fake)
	defer srv.Close()

	mm := recordingTeeTestManager(t, srv.URL)
	payload := recordingTeePayload(1 << 20)

	input, finalize := mm.recordTee(context.Background(), bytes.NewReader(payload), "did:plc:test", ".rtmp.mp4")
	defer finalize()

	require.Equal(t, payload, readTeeWithin(t, input, 15*time.Second), "the ingest stream reaches its consumer verbatim")
	initiate, _ := fake.failureCounts()
	require.Positive(t, initiate, "the recorder must have attempted (and failed) the upload")
}

// gatedReader hands out the payload in small, paced chunks as a live ingest
// stream does, and once it has handed over gateAt bytes — comfortably past the
// 16 MB that dispatches the recording's first S3 part — it waits for the fake to
// actually reject that part before continuing. Without the barrier the outcome
// races the upload's HTTP round trip: the tee can drain the whole payload before
// the client sees the 403, and the failure then only surfaces at commit, after
// ingest, which is not the case under test.
type gatedReader struct {
	r      io.Reader
	sent   int64
	gateAt int64
	gate   func() bool
	delay  time.Duration
}

func (g *gatedReader) Read(b []byte) (int, error) {
	if len(b) > 32*1024 {
		b = b[:32*1024]
	}
	if g.sent >= g.gateAt {
		deadline := time.Now().Add(10 * time.Second)
		for !g.gate() {
			if time.Now().After(deadline) {
				return 0, fmt.Errorf("S3 fake never rejected a part upload")
			}
			time.Sleep(time.Millisecond)
		}
	}
	time.Sleep(g.delay)
	n, err := g.r.Read(b)
	g.sent += int64(n)
	return n, err
}

// TestRecordTeeSurvivesRecorderMidStreamFailure covers the mid-stream shape: the
// upload starts, a part is rejected (a long recording dies after its first
// 16 MB part), and ingest must keep flowing from wherever the recorder died —
// not just fail to start.
func TestRecordTeeSurvivesRecorderMidStreamFailure(t *testing.T) {
	fake := newFakeS3Server()
	fake.failPart = true
	srv := httptest.NewServer(fake)
	defer srv.Close()

	mm := recordingTeeTestManager(t, srv.URL)
	// 18 MB puts the first part on the wire with 6 MB still to come: exactly the
	// in-flight stream a dead recorder used to strand.
	payload := recordingTeePayload(24 << 20)
	input, finalize := mm.recordTee(context.Background(), &gatedReader{
		r:      bytes.NewReader(payload),
		gateAt: 18 << 20,
		gate: func() bool {
			_, parts := fake.failureCounts()
			return parts > 0
		},
		delay: 250 * time.Microsecond,
	}, "did:plc:test", ".rtmp.mp4")
	defer finalize()

	require.Equal(t, payload, readTeeWithin(t, input, 30*time.Second), "the ingest stream reaches its consumer verbatim")
	_, parts := fake.failureCounts()
	require.Positive(t, parts, "the recorder must have attempted (and failed) a part upload mid-stream")
}
