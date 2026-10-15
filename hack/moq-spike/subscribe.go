package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"moq.dev/moq"
)

type subscribeConfig struct {
	url             string
	broadcast       string
	highRequiredBps uint64
	// tlsFingerprints pins the publisher's generated certificate; TLS
	// verification stays on.
	tlsFingerprints  []string
	lowPriority      uint8
	highPriority     uint8
	backfillPriority uint8
	// liveMaxAge is the per-subscription max age: a group this far behind
	// the live edge is skipped rather than waited for.
	liveMaxAge      time.Duration
	connectTimeout  time.Duration
	liveTimeout     time.Duration
	manifestTimeout time.Duration
	// backfillTimeout bounds the whole backfill phase, every fetch included.
	backfillTimeout time.Duration
	backfillDelay   time.Duration
	// backfillParallel caps fetches in flight; 0 issues every fetch at once,
	// which pins each group in the publisher's cache before its 30s idle
	// expiry can reclaim it while earlier groups transfer.
	backfillParallel int
	statsEvery       time.Duration
	// Pass/fail thresholds for the low track's live delivery.
	lowMaxMissingPct float64
	lowMaxP95        time.Duration
}

type liveGroup struct {
	Seq           uint64 `json:"seq"`
	Size          int    `json:"size"`
	SHA256        string `json:"sha256"`
	Frames        int    `json:"frames"`
	TimestampUs   uint64 `json:"timestamp_us"`
	ArrivedUnixUs int64  `json:"arrived_unix_us"`
	LatencyUs     int64  `json:"latency_us"`
	Error         string `json:"error,omitempty"`
}

type liveTrack struct {
	Info       moq.TrackInfo `json:"track_info"`
	Expected   int           `json:"expected"`
	Received   int           `json:"received"`
	Missing    []uint64      `json:"missing"`
	MissingPct float64       `json:"missing_pct"`
	Mismatched []uint64      `json:"mismatched"`
	Aborted    int           `json:"aborted"`
	LatencyP50 int64         `json:"latency_p50_us"`
	LatencyP95 int64         `json:"latency_p95_us"`
	LatencyMax int64         `json:"latency_max_us"`
	TrackError string        `json:"track_error,omitempty"`
	Groups     []liveGroup   `json:"groups"`
	complete   bool
}

type fetchResult struct {
	Seq             uint64 `json:"seq"`
	Size            int    `json:"size,omitempty"`
	SHA256          string `json:"sha256,omitempty"`
	TimestampUs     uint64 `json:"timestamp_us,omitempty"`
	MatchesManifest bool   `json:"matches_manifest"`
	// MatchesLive is nil when the live phase never received this group.
	MatchesLive *bool  `json:"matches_live,omitempty"`
	DurationUs  int64  `json:"duration_us"`
	Error       string `json:"error,omitempty"`
	NotFound    bool   `json:"not_found,omitempty"`
}

type backfill struct {
	Expected       int           `json:"expected"`
	Fetched        int           `json:"fetched"`
	Matched        int           `json:"matched"`
	CoveragePct    float64       `json:"coverage_pct"`
	Errors         []uint64      `json:"errors"`
	Mismatched     []uint64      `json:"mismatched"`
	PastFinalProbe fetchResult   `json:"past_final_probe"`
	Groups         []fetchResult `json:"groups"`
}

type statusEvent struct {
	UnixUs int64                `json:"unix_us"`
	Status moq.ConnectionStatus `json:"status"`
	Epoch  uint64               `json:"epoch"`
}

type check struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

type report struct {
	Pass          bool                `json:"pass"`
	Checks        []check             `json:"checks"`
	Low           *liveTrack          `json:"low"`
	HighLive      *liveTrack          `json:"high_live"`
	HighBackfill  *backfill           `json:"high_backfill"`
	Status        []statusEvent       `json:"status_events"`
	LiveStats     moq.ConnectionStats `json:"live_session_stats"`
	BackfillStats moq.ConnectionStats `json:"backfill_session_stats"`
}

type subscriber struct {
	cfg subscribeConfig
	log *slog.Logger
	mu  sync.Mutex
	rep *report
}

// runSubscribe consumes both tracks live on one session until the publisher
// ends them, then opens a second session and fetches every high group the
// manifest names. Live and fetched results are kept apart: nothing fetched
// is ever counted as received live.
func runSubscribe(ctx context.Context, cfg subscribeConfig, log *slog.Logger, getManifest func(context.Context) (*manifest, error)) (*report, error) {
	s := &subscriber{cfg: cfg, log: log, rep: &report{}}

	client, stop, err := s.dial(ctx)
	if err != nil {
		return nil, err
	}

	bc, err := s.resolveBroadcast(ctx, client)
	if err == nil {
		err = s.live(ctx, client, bc)
	}
	s.rep.LiveStats = client.Session().Stats()
	logStats(log, "live_stats", 1, s.rep.LiveStats)
	stop()
	if err != nil {
		return nil, fmt.Errorf("live phase: %w", err)
	}

	mctx, mcancel := context.WithTimeout(ctx, cfg.manifestTimeout)
	man, err := getManifest(mctx)
	mcancel()
	if err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	s.reconcile(s.rep.Low, man.Tracks[trackLow])
	s.reconcile(s.rep.HighLive, man.Tracks[trackHigh])
	select {
	case <-time.After(cfg.backfillDelay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if err := s.backfill(ctx, man.Tracks[trackHigh]); err != nil {
		return nil, fmt.Errorf("backfill phase: %w", err)
	}
	s.evaluate()
	return s.rep, nil
}

// dial connects with the publisher's certificate pinned and starts the
// status watcher for the session. The returned stop closes the client and
// joins the watcher, so nothing appends to the report after it returns.
func (s *subscriber) dial(ctx context.Context) (*moq.Client, func(), error) {
	dctx, dcancel := context.WithTimeout(ctx, s.cfg.connectTimeout)
	defer dcancel()
	client, err := moq.Dial(dctx, s.cfg.url,
		moq.WithTLSFingerprints(s.cfg.tlsFingerprints...),
		moq.WithWebSocketEnabled(false),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("dial %s: %w", s.cfg.url, err)
	}
	s.log.Info("connected", "url", s.cfg.url, "epoch", client.Session().Epoch(), "pinned", s.cfg.tlsFingerprints)

	wctx, wcancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.watchStatus(wctx, client.Session())
	}()
	stop := func() {
		client.Close()
		wcancel()
		wg.Wait()
	}
	return client, stop, nil
}

// resolveBroadcast waits for the publisher's route announcement to reach
// this session, then resolves the broadcast. RequestBroadcast would race the
// announcement on a fresh session and fail as unroutable.
func (s *subscriber) resolveBroadcast(ctx context.Context, client *moq.Client) (*moq.BroadcastConsumer, error) {
	cctx, cancel := context.WithTimeout(ctx, s.cfg.connectTimeout)
	defer cancel()
	announced, err := client.AnnouncedBroadcast(s.cfg.broadcast)
	if err != nil {
		return nil, fmt.Errorf("announced broadcast %s: %w", s.cfg.broadcast, err)
	}
	bc, err := announced.Available(cctx)
	if err != nil {
		return nil, fmt.Errorf("broadcast %s available: %w", s.cfg.broadcast, err)
	}
	return bc, nil
}

func (s *subscriber) watchStatus(ctx context.Context, session *moq.Session) {
	for {
		st, err := session.Status(ctx)
		if err != nil {
			if !moq.IsShutdown(err) && ctx.Err() == nil {
				s.log.Error("status", "error", err.Error())
			}
			return
		}
		ev := statusEvent{UnixUs: time.Now().UnixMicro(), Status: st, Epoch: session.Epoch()}
		s.log.Info("status", "status", st, "epoch", ev.Epoch)
		s.mu.Lock()
		s.rep.Status = append(s.rep.Status, ev)
		s.mu.Unlock()
	}
}

// live subscribes to both tracks, low at the higher priority, and reads
// groups in arrival order until each track ends or liveTimeout passes.
func (s *subscriber) live(ctx context.Context, client *moq.Client, bc *moq.BroadcastConsumer) error {
	lctx, lcancel := context.WithTimeout(ctx, s.cfg.liveTimeout)
	defer lcancel()
	maxAge := uint64(s.cfg.liveMaxAge.Microseconds())

	tracks := map[string]*liveTrack{}
	consumers := map[string]*moq.TrackConsumer{}
	for _, t := range []struct {
		name     string
		priority uint8
	}{{trackLow, s.cfg.lowPriority}, {trackHigh, s.cfg.highPriority}} {
		tc, err := bc.SubscribeTrack(lctx, t.name, &moq.Subscription{Priority: t.priority, MaxAgeUs: maxAge})
		if err != nil {
			return fmt.Errorf("subscribe %s: %w", t.name, err)
		}
		info, err := tc.Info()
		if err != nil {
			return fmt.Errorf("track info %s: %w", t.name, err)
		}
		s.log.Info("subscribed", "track", t.name, "priority", t.priority, "max_age_us", maxAge,
			"publisher_priority", info.Priority, "publisher_max_age_us", info.MaxAgeUs, "timescale", info.Timescale)
		tracks[t.name] = &liveTrack{Info: info}
		consumers[t.name] = tc
	}
	s.rep.Low = tracks[trackLow]
	s.rep.HighLive = tracks[trackHigh]

	var wg sync.WaitGroup
	for name, tc := range consumers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.readLive(lctx, name, tc, tracks[name])
		}()
	}
	done := make(chan struct{})
	statsDone := make(chan struct{})
	fellBack := false
	go func() {
		defer close(statsDone)
		ticker := time.NewTicker(s.cfg.statsEvery)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				stats := client.Session().Stats()
				logStats(s.log, "stats", 1, stats)
				if !fellBack && stats.EstimatedRecvRateBps != nil && *stats.EstimatedRecvRateBps < s.cfg.highRequiredBps {
					fellBack = true
					s.log.Info("fallback_low", "estimated_bps", *stats.EstimatedRecvRateBps, "high_required_bps", s.cfg.highRequiredBps)
					consumers[trackHigh].Cancel()
				}
			}
		}
	}()
	wg.Wait()
	close(done)
	<-statsDone
	for name, t := range tracks {
		if !t.complete && !(name == trackHigh && fellBack) {
			return fmt.Errorf("track %s did not end within %s", name, s.cfg.liveTimeout)
		}
	}
	return nil
}

func (s *subscriber) readLive(ctx context.Context, name string, tc *moq.TrackConsumer, t *liveTrack) {
	for g, err := range tc.GroupsAsArrived(ctx) {
		if err != nil {
			t.TrackError = err.Error()
			s.log.Error("live_track_error", "track", name, "error", err.Error())
			return
		}
		lg := readGroup(ctx, g)
		lg.ArrivedUnixUs = time.Now().UnixMicro()
		lg.LatencyUs = lg.ArrivedUnixUs - int64(lg.TimestampUs)
		if name == trackHigh && lg.Error == "" && time.Duration(lg.LatencyUs)*time.Microsecond > s.cfg.liveMaxAge {
			lg.Error = "late high group excluded from live"
		}
		if lg.Error != "" {
			t.Aborted++
			s.log.Info("live_group_aborted", "track", name, "seq", lg.Seq, "frames", lg.Frames, "bytes", lg.Size, "error", lg.Error)
		} else {
			s.log.Info("live_group", "track", name, "seq", lg.Seq, "bytes", lg.Size, "latency_ms", lg.LatencyUs/1000, "sha256", lg.SHA256)
		}
		t.Groups = append(t.Groups, lg)
	}
	t.complete = ctx.Err() == nil
	s.log.Info("live_track_end", "track", name, "groups", len(t.Groups), "complete", t.complete)
}

// readGroup drains one group, hashing its payload. A group that aborts
// mid-read keeps what arrived and carries the error.
func readGroup(ctx context.Context, g *moq.GroupConsumer) liveGroup {
	lg := liveGroup{Seq: g.Sequence()}
	h := sha256.New()
	for f, err := range g.Frames(ctx) {
		if err != nil {
			lg.Error = err.Error()
			break
		}
		if lg.Frames == 0 {
			lg.TimestampUs = f.TimestampUs
		}
		lg.Frames++
		lg.Size += len(f.Payload)
		h.Write(f.Payload)
	}
	lg.SHA256 = hex.EncodeToString(h.Sum(nil))
	return lg
}

// reconcile scores one track's live delivery against what the publisher
// wrote: which groups never arrived whole, which arrived with other bytes,
// and the completion-latency distribution of the ones that did.
func (s *subscriber) reconcile(t *liveTrack, published []groupRecord) {
	t.Expected = len(published)
	t.Missing = []uint64{}
	t.Mismatched = []uint64{}
	got := map[uint64]liveGroup{}
	for _, g := range t.Groups {
		if g.Error == "" {
			got[g.Seq] = g
		}
	}
	var latencies []int64
	for _, rec := range published {
		g, ok := got[rec.Seq]
		if !ok {
			t.Missing = append(t.Missing, rec.Seq)
			continue
		}
		if g.SHA256 != rec.SHA256 || g.Size != rec.Size || g.TimestampUs != rec.TimestampUs {
			t.Mismatched = append(t.Mismatched, rec.Seq)
			continue
		}
		t.Received++
		latencies = append(latencies, g.LatencyUs)
	}
	if t.Expected > 0 {
		t.MissingPct = 100 * float64(len(t.Missing)) / float64(t.Expected)
	}
	t.LatencyP50, t.LatencyP95, t.LatencyMax = percentiles(latencies)
}

func percentiles(v []int64) (p50, p95, mx int64) {
	if len(v) == 0 {
		return 0, 0, 0
	}
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	at := func(q float64) int64 {
		i := int(q*float64(len(v))+0.999999) - 1
		if i < 0 {
			i = 0
		}
		return v[i]
	}
	return at(0.5), at(0.95), v[len(v)-1]
}

// backfill opens a fresh session and fetches every high group the publisher
// wrote, one FetchGroup each, comparing bytes against the manifest and, where
// the live phase also received the group, against those bytes. Fetches are
// issued concurrently (see backfillParallel) and scored in sequence order.
func (s *subscriber) backfill(ctx context.Context, published []groupRecord) error {
	bctx, bcancel := context.WithTimeout(ctx, s.cfg.backfillTimeout)
	defer bcancel()
	client, stop, err := s.dial(bctx)
	if err != nil {
		return err
	}
	defer stop()
	bc, err := s.resolveBroadcast(bctx, client)
	if err != nil {
		return err
	}

	parallel := s.cfg.backfillParallel
	if parallel <= 0 || parallel > len(published) {
		parallel = max(len(published), 1)
	}
	sem := make(chan struct{}, parallel)
	results := make([]fetchResult, len(published))
	var wg sync.WaitGroup
	for i, rec := range published {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = s.fetchOne(bctx, bc, rec.Seq)
		}()
	}
	wg.Wait()

	b := &backfill{Expected: len(published), Errors: []uint64{}, Mismatched: []uint64{}}
	liveHigh := map[uint64]liveGroup{}
	for _, g := range s.rep.HighLive.Groups {
		if g.Error == "" {
			liveHigh[g.Seq] = g
		}
	}
	var lastSeq uint64
	for i, rec := range published {
		lastSeq = rec.Seq
		fr := results[i]
		if fr.Error == "" {
			b.Fetched++
			fr.MatchesManifest = fr.SHA256 == rec.SHA256 && fr.Size == rec.Size && fr.TimestampUs == rec.TimestampUs
			if lg, ok := liveHigh[rec.Seq]; ok {
				same := lg.SHA256 == fr.SHA256 && lg.TimestampUs == fr.TimestampUs
				fr.MatchesLive = &same
			}
			switch {
			case fr.MatchesManifest && (fr.MatchesLive == nil || *fr.MatchesLive):
				b.Matched++
			default:
				b.Mismatched = append(b.Mismatched, rec.Seq)
			}
		} else {
			b.Errors = append(b.Errors, rec.Seq)
		}
		s.log.Info("fetch", "track", trackHigh, "seq", fr.Seq, "bytes", fr.Size, "duration_ms", fr.DurationUs/1000,
			"matches_manifest", fr.MatchesManifest, "matches_live", fr.MatchesLive, "error", fr.Error)
		b.Groups = append(b.Groups, fr)
	}
	if b.Expected > 0 {
		b.CoveragePct = 100 * float64(b.Matched) / float64(b.Expected)
	}
	// One group past the final sequence can never exist; upstream documents a
	// NotFound here, so the probe records what this build actually does.
	if len(published) > 0 {
		b.PastFinalProbe = s.fetchOne(bctx, bc, lastSeq+1)
		s.log.Info("fetch_past_final", "seq", b.PastFinalProbe.Seq, "error", b.PastFinalProbe.Error, "not_found", b.PastFinalProbe.NotFound)
	}
	s.rep.HighBackfill = b
	s.rep.BackfillStats = client.Session().Stats()
	logStats(s.log, "backfill_stats", 2, s.rep.BackfillStats)
	return nil
}

func (s *subscriber) fetchOne(ctx context.Context, bc *moq.BroadcastConsumer, seq uint64) fetchResult {
	start := time.Now()
	fr := fetchResult{Seq: seq}
	g, err := bc.FetchGroup(ctx, trackHigh, seq, &moq.FetchGroupOptions{Priority: s.cfg.backfillPriority})
	if err != nil {
		fr.Error = err.Error()
		fr.NotFound = errors.Is(err, moq.ErrNotFound)
		fr.DurationUs = time.Since(start).Microseconds()
		return fr
	}
	lg := readGroup(ctx, g)
	fr.Size, fr.SHA256, fr.TimestampUs, fr.Error = lg.Size, lg.SHA256, lg.TimestampUs, lg.Error
	fr.DurationUs = time.Since(start).Microseconds()
	return fr
}

func (s *subscriber) evaluate() {
	r := s.rep
	low, hb := r.Low, r.HighBackfill
	r.Checks = []check{
		{
			Name:   "low_live_missing",
			Pass:   low.MissingPct <= s.cfg.lowMaxMissingPct,
			Detail: fmt.Sprintf("missing %d of %d (%.1f%%), threshold %.1f%%", len(low.Missing), low.Expected, low.MissingPct, s.cfg.lowMaxMissingPct),
		},
		{
			Name:   "low_live_bytes",
			Pass:   len(low.Mismatched) == 0 && low.TrackError == "",
			Detail: fmt.Sprintf("%d groups with bytes differing from the publisher, track error %q", len(low.Mismatched), low.TrackError),
		},
		{
			Name:   "low_live_p95_latency",
			Pass:   low.Received > 0 && time.Duration(low.LatencyP95)*time.Microsecond <= s.cfg.lowMaxP95,
			Detail: fmt.Sprintf("p50 %dms p95 %dms max %dms over %d groups, threshold %s", low.LatencyP50/1000, low.LatencyP95/1000, low.LatencyMax/1000, low.Received, s.cfg.lowMaxP95),
		},
		{
			Name: "high_backfill_coverage",
			Pass: hb.Expected > 0 && hb.Matched == hb.Expected,
			Detail: fmt.Sprintf("%d of %d fetched and byte-identical to the publisher (%.1f%%), %d fetch errors, %d mismatches; live delivered %d, missed %d",
				hb.Matched, hb.Expected, hb.CoveragePct, len(hb.Errors), len(hb.Mismatched), r.HighLive.Received, len(r.HighLive.Missing)),
		},
	}
	r.Pass = true
	for _, c := range r.Checks {
		r.Pass = r.Pass && c.Pass
		s.log.Info("check", "name", c.Name, "pass", c.Pass, "detail", c.Detail)
	}
}
