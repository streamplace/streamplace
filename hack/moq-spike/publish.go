package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	muxl "github.com/streamplace/muxl/go"
	"moq.dev/moq"
)

const (
	trackLow  = "low"
	trackHigh = "high"
)

type publishConfig struct {
	listen      string
	broadcast   string
	duration    time.Duration
	linger      time.Duration
	lowFixture  string
	highFixture string
	lowMaxBps   uint64
	highMaxBps  uint64
	// Publisher max ages bound delivery freshness, not cache retention.
	lowRetention  time.Duration
	highRetention time.Duration
	statsEvery    time.Duration
}

// groupRecord is what the publisher wrote for one group: the subscriber's
// ground truth for byte comparison. TimestampUs is the frame timestamp, which
// the publisher sets to its wall clock in unix microseconds, so a receiver on
// the same host measures freshness as now-TimestampUs and a fetched group
// proves it is the right one by carrying the same timestamp.
type groupRecord struct {
	Seq         uint64 `json:"seq"`
	Size        int    `json:"size"`
	SHA256      string `json:"sha256"`
	TimestampUs uint64 `json:"timestamp_us"`
}

type manifest struct {
	Broadcast      string                   `json:"broadcast"`
	Tracks         map[string][]groupRecord `json:"tracks"`
	FinishedUnixUs int64                    `json:"finished_unix_us"`
}

type grantEvent struct {
	UnixUs   int64   `json:"unix_us"`
	Session  uint64  `json:"session"`
	Track    string  `json:"track"`
	GrantBps *uint64 `json:"grant_bps"`
}

type segmentSummary struct {
	Fixture    string `json:"fixture"`
	Segments   int    `json:"segments"`
	Bytes      int    `json:"bytes"`
	DurationUs uint64 `json:"duration_us"`
}

type publishSummary struct {
	Listen       string                         `json:"listen"`
	Fingerprints []string                       `json:"cert_fingerprints"`
	Broadcast    string                         `json:"broadcast"`
	Payload      map[string]segmentSummary      `json:"payload"`
	RetentionUs  map[string]uint64              `json:"retention_us"`
	Published    map[string]int                 `json:"published_groups"`
	Sessions     uint64                         `json:"sessions"`
	GrantEvents  []grantEvent                   `json:"grant_events"`
	FinalStats   map[uint64]moq.ConnectionStats `json:"final_stats"`
}

type publisher struct {
	cfg  publishConfig
	log  *slog.Logger
	low  *moq.TrackProducer
	high *moq.TrackProducer

	sessions atomic.Uint64
	// wg joins the accept loop and every held session before the summary is
	// assembled, so grants and stats are never read while still being written.
	wg      sync.WaitGroup
	mu      sync.Mutex
	held    []*moq.Session
	grants  []grantEvent
	stats   map[uint64]moq.ConnectionStats
	archive map[uint64]moq.Frame
}

// runPublish serves the broadcast for cfg.duration, sending its certificate
// fingerprints on ready once the broadcast is announced and the manifest on
// finished once both tracks ended, then keeps the server (and so the cached
// groups) alive for cfg.linger or until ctx ends, whichever is first.
func runPublish(ctx context.Context, cfg publishConfig, log *slog.Logger, ready chan<- []string, finished chan<- *manifest) (*publishSummary, error) {
	eng, err := muxl.NewWASM(ctx)
	if err != nil {
		return nil, fmt.Errorf("muxl engine: %w", err)
	}
	defer eng.Close(ctx)
	id, err := newEphemeralSigner(ctx, eng)
	if err != nil {
		return nil, err
	}
	payload := map[string][]segment{}
	summary := &publishSummary{
		Broadcast:   cfg.broadcast,
		Payload:     map[string]segmentSummary{},
		RetentionUs: map[string]uint64{},
		Published:   map[string]int{},
		FinalStats:  map[uint64]moq.ConnectionStats{},
	}
	for name, fixture := range map[string]string{trackLow: cfg.lowFixture, trackHigh: cfg.highFixture} {
		segs, err := signFixture(ctx, eng, id, fixture)
		if err != nil {
			return nil, err
		}
		payload[name] = segs
		ss := segmentSummary{Fixture: fixture, Segments: len(segs)}
		for _, s := range segs {
			ss.Bytes += len(s.payload)
			ss.DurationUs += s.durationUs
		}
		summary.Payload[name] = ss
		log.Info("payload", "track", name, "fixture", fixture, "segments", ss.Segments, "bytes", ss.Bytes, "duration_us", ss.DurationUs)
	}

	cacheBytes := uint64(2 * 1024 * 1024)
	origin := moq.NewOriginProducerWithConfig(moq.OriginConfig{CacheCapacityBytes: &cacheBytes})
	server, err := moq.Listen(ctx, cfg.listen, moq.WithTLSGenerate("localhost"), moq.WithServerPublishOrigin(origin))
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", cfg.listen, err)
	}
	defer server.Close()
	summary.Listen = server.LocalAddr()
	summary.Fingerprints, err = server.CertFingerprints()
	if err != nil {
		return nil, fmt.Errorf("certificate fingerprints: %w", err)
	}
	log.Info("listening", "addr", summary.Listen, "cert_fingerprints", summary.Fingerprints)

	bc, err := origin.CreateBroadcast(cfg.broadcast)
	if err != nil {
		return nil, fmt.Errorf("create broadcast: %w", err)
	}
	defer bc.Close()

	highRetention := cfg.highRetention
	if highRetention == 0 {
		highRetention = cfg.duration + cfg.linger + 30*time.Second
	}
	p := &publisher{cfg: cfg, log: log, stats: map[uint64]moq.ConnectionStats{}, archive: map[uint64]moq.Frame{}}
	for _, t := range []struct {
		name      string
		retention time.Duration
		dst       **moq.TrackProducer
	}{
		{trackLow, cfg.lowRetention, &p.low},
		{trackHigh, highRetention, &p.high},
	} {
		maxAge := uint64(t.retention.Microseconds())
		priority := uint8(100)
		if t.name == trackLow {
			priority = 200
		}
		tp, err := bc.PublishTrack(t.name, &moq.TrackInfo{Priority: priority, MaxAgeUs: &maxAge})
		if err != nil {
			return nil, fmt.Errorf("publish track %s: %w", t.name, err)
		}
		*t.dst = tp
		summary.RetentionUs[t.name] = maxAge
		log.Info("track", "track", t.name, "max_age_us", maxAge)
	}
	dynamic, err := p.high.Dynamic()
	if err != nil {
		return nil, fmt.Errorf("high fetch handler: %w", err)
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for request, err := range dynamic.Requests(ctx) {
			if err != nil {
				return
			}
			p.mu.Lock()
			frame, ok := p.archive[request.Sequence()]
			p.mu.Unlock()
			if !ok {
				_ = request.Abort(404)
				continue
			}
			group, err := request.Accept()
			if err == nil {
				err = group.WriteFrame(frame)
				if err == nil {
					err = group.Finish()
				}
				if err != nil {
					_ = group.Abort(500)
				}
			}
			p.log.Info("archive_fetch", "seq", request.Sequence(), "error", errString(err))
		}
	}()
	if err := bc.Announce(moq.Route{}); err != nil {
		return nil, fmt.Errorf("announce: %w", err)
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.acceptLoop(ctx, server)
	}()
	ready <- summary.Fingerprints

	man := &manifest{Broadcast: cfg.broadcast, Tracks: map[string][]groupRecord{}}
	var wg sync.WaitGroup
	var manMu sync.Mutex
	writeErrors := make(chan error, 2)
	for name, track := range map[string]*moq.TrackProducer{trackLow: p.low, trackHigh: p.high} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			recs, err := p.publishTrack(ctx, name, track, payload[name])
			writeErrors <- err
			if err != nil {
				_ = track.Abort(500)
			}
			manMu.Lock()
			man.Tracks[name] = recs
			manMu.Unlock()
		}()
	}
	wg.Wait()
	var publishErr error
	for range 2 {
		if err := <-writeErrors; err != nil {
			publishErr = err
		}
	}
	man.FinishedUnixUs = time.Now().UnixMicro()
	for name, recs := range man.Tracks {
		summary.Published[name] = len(recs)
	}
	log.Info("finished", "low_groups", summary.Published[trackLow], "high_groups", summary.Published[trackHigh], "linger", cfg.linger.String())
	finished <- man

	select {
	case <-ctx.Done():
	case <-time.After(cfg.linger):
	}
	// Stop accepting, end every held session, and join their goroutines
	// before touching what they wrote.
	server.Close()
	dynamic.Cancel()
	p.mu.Lock()
	for _, s := range p.held {
		s.Shutdown()
	}
	p.mu.Unlock()
	p.wg.Wait()
	summary.Sessions = p.sessions.Load()
	summary.GrantEvents = p.grants
	for id, st := range p.stats {
		summary.FinalStats[id] = st
	}
	return summary, publishErr
}

// publishTrack waits for a subscriber to want the track, then writes one
// group per signed segment at the segment's own cadence, looping over the
// fixture for cfg.duration, and ends the track. Writing before anyone has
// subscribed would only inflate the first groups' measured latency.
func (p *publisher) publishTrack(ctx context.Context, name string, track *moq.TrackProducer, segs []segment) ([]groupRecord, error) {
	demand, err := track.Demand()
	if err != nil {
		return nil, fmt.Errorf("%s demand: %w", name, err)
	}
	if err := demand.Used(ctx); err != nil {
		return nil, fmt.Errorf("%s await demand: %w", name, err)
	}
	p.log.Info("demand", "track", name)
	var recs []groupRecord
	deadline := time.Now().Add(p.cfg.duration)
	next := time.Now()
	for i := 0; ctx.Err() == nil && time.Now().Before(deadline); i++ {
		seg := segs[i%len(segs)]
		g, err := track.AppendGroup()
		if err != nil {
			return recs, fmt.Errorf("%s append group: %w", name, err)
		}
		ts := uint64(time.Now().UnixMicro())
		if name == trackHigh {
			p.mu.Lock()
			p.archive[g.Sequence()] = moq.Frame{Payload: seg.payload, TimestampUs: ts}
			p.mu.Unlock()
		}
		err = g.WriteFrame(moq.Frame{Payload: seg.payload, TimestampUs: ts})
		if err == nil {
			err = g.Finish()
		}
		if err != nil {
			return recs, fmt.Errorf("%s write group: %w", name, err)
		}
		recs = append(recs, groupRecord{Seq: g.Sequence(), Size: len(seg.payload), SHA256: seg.sha256, TimestampUs: ts})
		p.log.Info("group", "track", name, "seq", g.Sequence(), "bytes", len(seg.payload), "sha256", seg.sha256, "timestamp_us", ts)

		dur := time.Duration(seg.durationUs) * time.Microsecond
		if dur == 0 {
			return recs, fmt.Errorf("%s fixture has zero-duration segment", name)
		}
		next = next.Add(dur)
		select {
		case <-ctx.Done():
		case <-time.After(time.Until(next)):
		}
	}
	if err := track.Finish(); err != nil {
		return recs, fmt.Errorf("%s finish track: %w", name, err)
	}
	return recs, ctx.Err()
}

func (p *publisher) acceptLoop(ctx context.Context, server *moq.Server) {
	for req, err := range server.Requests(ctx) {
		if err != nil {
			if !moq.IsShutdown(err) && ctx.Err() == nil {
				p.log.Error("accept", "error", err.Error())
			}
			return
		}
		session, err := req.Accept(ctx)
		if err != nil {
			p.log.Error("accept_session", "error", err.Error())
			continue
		}
		id := p.sessions.Add(1)
		p.log.Info("session", "session", id, "path", req.Path(), "transport", req.Transport())
		p.mu.Lock()
		p.held = append(p.held, session)
		p.mu.Unlock()
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.holdSession(ctx, id, session)
		}()
	}
}

// holdSession keeps one subscriber session alive, reserves both tracks on
// its bandwidth allocator, and logs every grant change plus a stats snapshot
// each statsEvery until the session closes.
func (p *publisher) holdSession(ctx context.Context, id uint64, session *moq.Session) {
	bw := session.Bandwidth()
	reservations := map[string]*moq.Reservation{}
	for name, track := range map[string]*moq.TrackProducer{trackLow: p.low, trackHigh: p.high} {
		maxBps := p.cfg.lowMaxBps
		if name == trackHigh {
			maxBps = p.cfg.highMaxBps
		}
		res, err := bw.Reserve(track, maxBps)
		if err != nil {
			// The session is still held below: a failed reservation is
			// evidence to report, not a reason to drop the subscriber.
			p.log.Error("reserve", "session", id, "track", name, "max_bps", maxBps, "error", err.Error())
			continue
		}
		reservations[name] = res
		p.log.Info("reserve", "session", id, "track", name, "max_bps", maxBps)
	}

	closed := make(chan error, 1)
	go func() { closed <- session.Closed(ctx) }()

	last := map[string]*uint64{}
	poll := func() {
		for name, res := range reservations {
			p.observeGrant(id, name, res.Grant(), last)
		}
		st := session.Stats()
		p.mu.Lock()
		p.stats[id] = st
		p.mu.Unlock()
		logStats(p.log, "stats", id, st)
	}
	ticker := time.NewTicker(p.cfg.statsEvery)
	defer ticker.Stop()
	for {
		select {
		case err := <-closed:
			poll()
			p.log.Info("session_closed", "session", id, "error", errString(err))
			return
		case <-ticker.C:
			poll()
		}
	}
}

func (p *publisher) observeGrant(session uint64, track string, grant *uint64, last map[string]*uint64) {
	prev := last[track]
	same := (grant == nil) == (prev == nil) && (grant == nil || *grant == *prev)
	if same {
		return
	}
	last[track] = grant
	ev := grantEvent{UnixUs: time.Now().UnixMicro(), Session: session, Track: track, GrantBps: grant}
	p.mu.Lock()
	p.grants = append(p.grants, ev)
	p.mu.Unlock()
	p.log.Info("grant", "session", session, "track", track, "grant_bps", grant)
}

func logStats(log *slog.Logger, event string, session uint64, st moq.ConnectionStats) {
	log.Info(event, "session", session,
		"rtt_us", st.RttUs,
		"send_bps", st.EstimatedSendRateBps,
		"recv_bps", st.EstimatedRecvRateBps,
		"bytes_sent", st.BytesSent,
		"bytes_received", st.BytesReceived,
		"bytes_lost", st.BytesLost,
		"packets_sent", st.PacketsSent,
		"packets_received", st.PacketsReceived,
		"packets_lost", st.PacketsLost,
	)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
