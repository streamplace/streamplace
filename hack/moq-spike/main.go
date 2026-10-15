// Command moq-spike is a two-peer evaluation of moq.dev/moq v0.7.14 for
// Streamplace transport: in one process, a publisher serves a "low" and a
// "high" track of authentic S2PA-signed MUXL segments over real UDP loopback,
// and a subscriber consumes both live (low-first priority, finite max age),
// then fetches every retained high group over a separate session,
// byte-comparing everything against what the publisher wrote. See README.md
// for invocation and pass/fail criteria.
//
// Events are JSON lines on stderr; the final report is one JSON document on
// stdout. Exit 0 means every check passed, 1 means a check failed, 2 means
// the run itself broke.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

const (
	exitPass  = 0
	exitFail  = 1
	exitError = 2
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	var pcfg publishConfig
	var scfg subscribeConfig
	fs := flag.NewFlagSet("moq-spike", flag.ExitOnError)
	publishFlags(fs, &pcfg)
	subscribeFlags(fs, &scfg)
	scheme := fs.String("scheme", "https", "URL scheme the subscriber dials the publisher with: https (WebTransport) or moqt (raw QUIC)")
	_ = fs.Parse(os.Args[1:])
	if fs.NArg() != 0 {
		log.Error("unexpected positional arguments", "args", fs.Args())
		os.Exit(exitError)
	}
	scfg.broadcast = pcfg.broadcast
	scfg.statsEvery = pcfg.statsEvery
	scfg.highRequiredBps = pcfg.highMaxBps
	scfg.url = *scheme + "://" + pcfg.listen

	code, err := run(ctx, log, pcfg, scfg)
	if err != nil {
		log.Error("fatal", "error", err.Error())
		os.Exit(exitError)
	}
	os.Exit(code)
}

func publishFlags(fs *flag.FlagSet, c *publishConfig) {
	fs.StringVar(&c.listen, "listen", "127.0.0.1:4443", "UDP address the publisher listens on")
	fs.StringVar(&c.broadcast, "broadcast", "spike", "broadcast path")
	fs.DurationVar(&c.duration, "duration", 20*time.Second, "how long to publish before ending both tracks")
	fs.DurationVar(&c.linger, "linger", 3*time.Minute, "upper bound on how long the publisher keeps serving after the tracks end; the run ends it as soon as the subscriber is done")
	fs.StringVar(&c.lowFixture, "low-fixture", "test/fixtures/h264-opus-frag.mp4", "MP4 fixture signed into the low track's segments")
	fs.StringVar(&c.highFixture, "high-fixture", "test/fixtures/sample-segment.mp4", "MP4 fixture signed into the high track's segments")
	fs.Uint64Var(&c.lowMaxBps, "low-max-bps", 1_000_000, "bandwidth reservation ceiling for the low track, bits/s")
	fs.Uint64Var(&c.highMaxBps, "high-max-bps", 12_000_000, "bandwidth reservation ceiling for the high track, bits/s")
	fs.DurationVar(&c.lowRetention, "low-max-age", 5*time.Second, "low publisher delivery age budget; not a cache TTL")
	fs.DurationVar(&c.highRetention, "high-max-age", 0, "high publisher delivery age budget; 0 derives duration+linger+30s; does not guarantee retention")
	fs.DurationVar(&c.statsEvery, "stats-every", time.Second, "interval between stats snapshots and grant polls")
}

func subscribeFlags(fs *flag.FlagSet, c *subscribeConfig) {
	fs.Var(newPriorityFlag(&c.lowPriority, 200), "low-priority", "live subscription priority for the low track (higher preempts)")
	fs.Var(newPriorityFlag(&c.highPriority, 100), "high-priority", "live subscription priority for the high track")
	fs.Var(newPriorityFlag(&c.backfillPriority, 50), "backfill-priority", "delivery priority for backfill fetches")
	fs.DurationVar(&c.liveMaxAge, "live-max-age", 2*time.Second, "live subscription max age: a group this far behind the live edge is skipped")
	fs.DurationVar(&c.connectTimeout, "connect-timeout", 30*time.Second, "bound on dialing and resolving the broadcast")
	fs.DurationVar(&c.liveTimeout, "live-timeout", 5*time.Minute, "bound on the live phase if the tracks never end")
	fs.DurationVar(&c.backfillTimeout, "backfill-timeout", 3*time.Minute, "bound on the whole backfill phase, every fetch included")
	fs.DurationVar(&c.backfillDelay, "backfill-delay", 0, "wait before FETCH to exercise archive recovery after cache expiry")
	fs.IntVar(&c.backfillParallel, "backfill-parallel", 0, "fetches in flight at once; 0 issues every fetch immediately so no group idles past the publisher's 30s cache expiry")
	fs.Float64Var(&c.lowMaxMissingPct, "low-max-missing-pct", 0, "pass threshold: max percentage of low groups missing live")
	fs.DurationVar(&c.lowMaxP95, "low-max-p95", 1500*time.Millisecond, "pass threshold: max p95 completion latency of live low groups")
	fs.DurationVar(&c.manifestTimeout, "manifest-timeout", 30*time.Second, "bound on waiting for the publisher's manifest after the live tracks end")
}

// priorityFlag is a uint8 flag (flag has no uint8 flavour); constructing it
// sets the default.
type priorityFlag struct {
	p *uint8
}

func (f priorityFlag) String() string {
	if f.p == nil {
		return "0"
	}
	return strconv.Itoa(int(*f.p))
}

func (f priorityFlag) Set(s string) error {
	v, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		return err
	}
	*f.p = uint8(v)
	return nil
}

func newPriorityFlag(p *uint8, def uint8) priorityFlag {
	*p = def
	return priorityFlag{p}
}

// run drives both peers: the publisher hands its generated certificate
// fingerprints over ready (the subscriber pins them) and the group manifest
// over finished (the subscriber scores against it); the subscriber's verdict
// is the exit code, and the publisher is stopped once the subscriber is done.
func run(ctx context.Context, log *slog.Logger, pcfg publishConfig, scfg subscribeConfig) (int, error) {
	pctx, pcancel := context.WithCancel(ctx)
	defer pcancel()
	ready := make(chan []string, 1)
	finished := make(chan *manifest, 1)
	type pubOut struct {
		summary *publishSummary
		err     error
	}
	pubDone := make(chan pubOut, 1)
	go func() {
		s, err := runPublish(pctx, pcfg, log.With("role", "publisher"), ready, finished)
		pubDone <- pubOut{s, err}
	}()

	// A publisher that fails before it is ready (bad fixture, bind failure)
	// must not leave the subscriber waiting on the channel.
	var fingerprints []string
	select {
	case fingerprints = <-ready:
	case out := <-pubDone:
		return exitError, fmt.Errorf("publisher: %w", out.err)
	case <-ctx.Done():
		return exitError, ctx.Err()
	}
	scfg.tlsFingerprints = fingerprints

	rep, serr := runSubscribe(ctx, scfg, log.With("role", "subscriber"), func(ctx context.Context) (*manifest, error) {
		select {
		case m := <-finished:
			return m, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		case out := <-pubDone:
			pubDone <- out
			return nil, fmt.Errorf("publisher ended before producing a manifest: %v", out.err)
		}
	})
	pcancel()
	pub := <-pubDone
	if pub.err != nil {
		return exitError, fmt.Errorf("publisher: %w", pub.err)
	}
	if serr != nil {
		return exitError, serr
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]any{"publisher": pub.summary, "subscriber": rep}); err != nil {
		return exitError, err
	}
	if rep.Pass {
		return exitPass, nil
	}
	return exitFail, nil
}
