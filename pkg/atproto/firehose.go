package atproto

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	indigoatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/events"
	"github.com/bluesky-social/indigo/events/schedulers/parallel"
	"github.com/bluesky-social/indigo/repo"
	"github.com/bluesky-social/indigo/repomgr"
	"github.com/ipfs/go-cid"
	glex "github.com/streamplace/glex/runtime"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	notificationpkg "stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/spmetrics"
	"stream.place/streamplace/pkg/statedb"

	"slices"

	"github.com/gorilla/websocket"
)

// dedupWindow bounds how long a key is remembered; relays skew by seconds.
const dedupWindow = 5 * time.Minute

type ATProtoSynchronizer struct {
	CLI        *config.CLI
	Model      model.Model
	StatefulDB *statedb.StatefulDB
	Noter      notificationpkg.Notifier
	Bus        *bus.Bus
	// Identity directories, built on first use behind dirMu; read them through
	// [ATProtoSynchronizer.directory].
	PLCDirectory       identity.Directory
	CachedPLCDirectory identity.Directory
	dirMu              sync.Mutex
	OATProxy           *oatproxy.OATProxy

	// Liveness, written concurrently by every relay consumer (unix nanos).
	// lastSeen is the last reception; lastEvent is the newest event stamp,
	// which is what "behind real time" measures.
	lastSeen  atomic.Int64
	lastEvent highWater
	// Events seen across all relays, pre-dedup (for the in-sync ping).
	seenEvents atomic.Int64

	// cross-relay dedup, shared by every relay consumer. Initialized at the
	// top of StartFirehose.
	commitDedup   *firehoseDeduper
	identityDedup *firehoseDeduper

	// sweeping is held for the length of a sweep, so the periodic ticker
	// cannot start a second one on top of the first.
	sweeping sync.Mutex
}

func (atsync *ATProtoSynchronizer) markSeen() {
	atsync.lastSeen.Store(time.Now().UnixNano())
	atsync.seenEvents.Add(1)
}

// markEvent records the newest event stamp. Max-wins, so a relay replaying
// history cannot drag the position back past a fresher event.
func (atsync *ATProtoSynchronizer) markEvent(t time.Time) {
	atsync.lastEvent.observe(t.UnixNano())
}

func sinceNanos(ns int64) time.Duration {
	if ns == 0 {
		return 0
	}
	return time.Since(time.Unix(0, ns))
}

// relayHosts returns the de-duplicated relay list: --relay-host, plus our own
// firehose when we have a listener, so local records are indexed immediately
// (never with an upstream station, whose users publish to their own PDSes).
func (atsync *ATProtoSynchronizer) relayHosts() []string {
	seen := map[string]struct{}{}
	var hosts []string
	add := func(h string) {
		h = strings.TrimSpace(h)
		if h == "" {
			return
		}
		if _, ok := seen[h]; ok {
			return
		}
		seen[h] = struct{}{}
		hosts = append(hosts, h)
	}
	for _, h := range strings.Split(atsync.CLI.RelayHost, ",") {
		add(h)
	}
	if self := atsync.selfRelayURL(); self != "" && !atsync.upstreamMode() {
		add(self)
	}
	return hosts
}

// selfRelayURL is our own firehose URL, or "" when there is no listener.
func (atsync *ATProtoSynchronizer) selfRelayURL() string {
	if atsync.CLI.HTTPAddr == "" {
		return ""
	}
	return httpToWSURL(atsync.CLI.OwnPublicURL())
}

func httpToWSURL(u string) string {
	if strings.HasPrefix(u, "https://") {
		return "wss://" + strings.TrimPrefix(u, "https://")
	}
	if strings.HasPrefix(u, "http://") {
		return "ws://" + strings.TrimPrefix(u, "http://")
	}
	return u
}

// StartFirehose consumes every configured relay concurrently, deduplicating
// overlapping streams. It returns only when ctx is cancelled.
func (atsync *ATProtoSynchronizer) StartFirehose(ctx context.Context) error {
	ctx = log.WithLogValues(ctx, "func", "StartFirehose")

	relays := atsync.relayHosts()
	if len(relays) == 0 {
		return fmt.Errorf("no relay hosts configured")
	}

	atsync.commitDedup = newFirehoseDeduper(dedupWindow)
	atsync.identityDedup = newFirehoseDeduper(dedupWindow)
	atsync.markSeen()
	atsync.markEvent(time.Now())
	if atsync.upstreamMode() {
		return atsync.startUpstreamFirehose(ctx, relays)
	}

	log.Log(ctx, "starting firehose consumers", "relays", relays)

	g, ctx := errgroup.WithContext(ctx)
	for _, relay := range relays {
		relay := relay
		spmetrics.TouchRelay(relay, relayProtocol(relay))
		g.Go(func() error {
			atsync.consumeRelay(ctx, relay, "")
			return nil
		})
	}
	g.Go(func() error {
		atsync.monitorFirehose(ctx)
		return nil
	})
	return g.Wait()
}

// consumeRelay keeps one relay connected, backing off between attempts. It
// never returns an error: a bad relay must not disturb the others.
func (atsync *ATProtoSynchronizer) consumeRelay(ctx context.Context, relay, track string) {
	ctx = log.WithLogValues(ctx, "relay", relay, "track", track)

	// On-demand DID tracks reset their group sequences when no subscribers
	// remain. Only aggregate tracks have durable replay cursors.
	cursor := &relayCursor{host: relay}
	var repairs chan struct{}
	if track == "" {
		cursor = atsync.newRelayCursor(ctx, relay)
		flushDone := make(chan struct{})
		go func() {
			defer close(flushDone)
			ticker := time.NewTicker(cursorFlushInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					cursor.flush(ctx)
					return
				case <-ticker.C:
					cursor.flush(ctx)
				}
			}
		}()
		defer func() { <-flushDone }()
	} else {
		// One pending repair per DID coalesces reconnects. Once a repair starts,
		// another gap queues one follow-up rather than starting another worker.
		repairs = make(chan struct{}, 1)
		repairDone := make(chan struct{})
		go func() {
			defer close(repairDone)
			atsync.upstreamRepairLoop(ctx, track, repairs)
		}()
		defer func() { <-repairDone }()
	}

	const (
		minBackoff = time.Second
		maxBackoff = 30 * time.Second
	)
	backoff := minBackoff
	for {
		if ctx.Err() != nil {
			return
		}
		start := time.Now()
		var err error
		if track == "" {
			err = atsync.connectRelay(ctx, relay, cursor)
		} else {
			err = atsync.connectRelayMoq(ctx, relay, track, cursor, repairs)
		}
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Error(ctx, "relay firehose disconnected; reconnecting", "err", err, "backoff", backoff)
		} else {
			log.Warn(ctx, "relay firehose closed; reconnecting", "backoff", backoff)
		}
		// A connection that stayed healthy for a while earns a fresh backoff.
		if time.Since(start) > time.Minute {
			backoff = minBackoff
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// firehoseReplayWindow is how stale a stored cursor may be and still be
// replayed; zero or negative disables the cap.
func (atsync *ATProtoSynchronizer) firehoseReplayWindow() time.Duration {
	if atsync.CLI == nil {
		return config.DefaultFirehoseReplayWindow
	}
	return atsync.CLI.FirehoseReplayWindow
}

// dropStaleCursor abandons a cursor too old to replay and kicks a sweep to
// heal the gap. sweepOnce ignores a kick while one is already running.
func (atsync *ATProtoSynchronizer) dropStaleCursor(ctx context.Context, cursor *relayCursor) {
	if !cursor.dropIfStale(ctx, atsync.firehoseReplayWindow()) {
		return
	}
	// Drop cached identities: a pre-gap resolution would agree with the stale
	// repo row and hide the change from the sweep.
	atsync.resetIdentCache()
	if atsync.StatefulDB == nil {
		// No index to sweep (tests that exercise the firehose on its own).
		return
	}
	log.Log(ctx, "kicking off a sweep to heal the skipped firehose replay")
	go atsync.sweepOnce(ctx, atsync.Sweep)
}

// connectRelay pumps one relay until it drops. Handlers use the parent ctx so
// an in-flight commit survives a reconnect; dedup has already claimed it.
func (atsync *ATProtoSynchronizer) connectRelay(ctx context.Context, relay string, cursor *relayCursor) error {
	u, err := url.Parse(relay)
	if err != nil {
		return fmt.Errorf("invalid relay URI %q: %w", relay, err)
	}
	// An old cursor would ask the relay to replay hours of traffic; check on
	// every attempt because backoff ages a cursor that was fresh at load.
	atsync.dropStaleCursor(ctx, cursor)
	// MoQ relays use QUIC; everything after frame decode is shared.
	switch u.Scheme {
	case "moqt", "moql", "moq", "moqs":
		return atsync.connectRelayMoq(ctx, relay, "atproto", cursor, nil)
	}
	u.Path = "xrpc/com.atproto.sync.subscribeRepos"
	if seq, ok := cursor.param(); ok {
		q := u.Query()
		q.Set("cursor", strconv.FormatInt(seq, 10))
		u.RawQuery = q.Encode()
	}

	header := http.Header{"User-Agent": []string{aqhttp.UserAgent}}
	// Our listener picks the broadcaster or server-repo firehose by Host, and
	// dialing loopback sends 127.0.0.1. Present ServerHost to get the server
	// repo, whose media.origin records getVideoList depends on.
	isSelf := relay == atsync.selfRelayURL()
	if isSelf && atsync.CLI.ServerHost != "" {
		header.Set("Host", atsync.CLI.ServerHost)
	}

	dialer := websocket.DefaultDialer
	if isSelf && u.Scheme == "wss" {
		// The cert is issued for ServerHost, not the loopback IP we dial. The
		// peer is this same process, so skipping verification trusts nothing.
		d := *websocket.DefaultDialer
		d.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		dialer = &d
	}
	con, _, err := dialer.Dial(u.String(), header)
	if err != nil {
		return fmt.Errorf("subscribing to firehose failed (dialing): %w", err)
	}
	defer con.Close()

	protocol := relayProtocol(relay)
	spmetrics.FirehoseRelaysConnected.WithLabelValues(relay, protocol).Set(1)
	defer spmetrics.FirehoseRelaysConnected.WithLabelValues(relay, protocol).Set(0)

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	rsc := atsync.repoStreamCallbacks(ctx, relay, cursor, cancel)
	scheduler := parallel.NewScheduler(10, 100, relay, rsc.EventHandler)

	log.Log(ctx, "connected to relay firehose")
	return events.HandleRepoStream(streamCtx, con, scheduler, nil)
}

// relayProtocol is the metric label for a relay's transport.
func relayProtocol(relay string) string {
	if u, err := url.Parse(relay); err == nil {
		switch u.Scheme {
		case "moqt", "moql", "moq", "moqs":
			return "moq"
		}
	}
	return "websocket"
}

// repoStreamCallbacks builds the callbacks shared by the WebSocket and MoQ
// transports. cancel ends the connection on an error frame.
//
// Handlers run inline on the scheduler's workers, not in per-event goroutines:
// that bounds concurrency, preserves per-repo ordering, and applies
// backpressure. They use the captured parent ctx (not the scheduler's
// context.TODO()) so an in-flight commit survives a reconnect.
func (atsync *ATProtoSynchronizer) repoStreamCallbacks(ctx context.Context, relay string, cursor *relayCursor, cancel context.CancelFunc) *events.RepoStreamCallbacks {
	protocol := relayProtocol(relay)
	// observeSeq advances the cursor and the per-relay seq gauge. On a moq relay
	// the cursor holds group sequences, so the upstream at-seq feeds the gauge
	// only; mixing them would bury the cursor and stall the next resume.
	moq := protocol == "moq"
	var gaugeHigh highWater // upstream-seq high-water for the moq gauge only
	observeSeq := func(seq int64) {
		if moq {
			gaugeHigh.observe(seq)
			spmetrics.FirehoseRelayHighSeq.WithLabelValues(relay, protocol).Set(float64(gaugeHigh.get()))
			return
		}
		cursor.observe(seq)
		spmetrics.FirehoseRelayHighSeq.WithLabelValues(relay, protocol).Set(float64(cursor.highSeq()))
	}
	// observeTime updates the per-relay position and the node-wide mark, before
	// dedup: a duplicate still proves how current the relay is. It follows what
	// the relays send, not what we index.
	observeTime := func(t string) {
		aqt, err := aqtime.FromString(t)
		if err != nil {
			return
		}
		at := aqt.Time()
		// Fold a future stamp to the receive time: max-wins would otherwise
		// pin both marks ahead of the clock and hide real lag.
		if now := time.Now(); at.After(now) {
			at = now
		}
		atsync.markEvent(at)
		cursor.observeTime(at)
	}
	return &events.RepoStreamCallbacks{
		RepoCommit: func(evt *indigoatproto.SyncSubscribeRepos_Commit) error {
			atsync.markSeen()
			observeSeq(evt.Seq)
			observeTime(evt.Time)
			spmetrics.FirehoseEventsReceivedTotal.WithLabelValues(relay, protocol, "commit").Inc()
			if atsync.commitDedup.seen(evt.Commit.String()) {
				spmetrics.FirehoseEventsDedupedTotal.WithLabelValues(relay, protocol, "commit").Inc()
				return nil
			}
			atsync.handleCommitEventOps(ctx, evt)
			return nil
		},
		RepoIdentity: func(evt *indigoatproto.SyncSubscribeRepos_Identity) error {
			atsync.markSeen()
			observeSeq(evt.Seq)
			observeTime(evt.Time)
			spmetrics.FirehoseEventsReceivedTotal.WithLabelValues(relay, protocol, "identity").Inc()
			if atsync.identityDedup.seen(identityDedupKey(evt)) {
				spmetrics.FirehoseEventsDedupedTotal.WithLabelValues(relay, protocol, "identity").Inc()
				return nil
			}
			atsync.handleIdentityEventOps(ctx, evt)
			return nil
		},
		Error: func(evt *events.ErrorFrame) error {
			cancel()
			return fmt.Errorf("firehose error: %s: %s", evt.Error, evt.Message)
		},
	}
}

// identityDedupKey folds one relayed broadcast across relays without dropping
// real updates: (did, time, handle) is stable per broadcast. seq is per-relay.
func identityDedupKey(evt *indigoatproto.SyncSubscribeRepos_Identity) string {
	handle := ""
	if evt.Handle != nil {
		handle = *evt.Handle
	}
	return evt.Did + "\x00" + evt.Time + "\x00" + handle
}

// firehoseSyncPingInterval spaces out the Info-level "in sync" heartbeat.
const firehoseSyncPingInterval = 60 * time.Second

func (atsync *ATProtoSynchronizer) monitorFirehose(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	lastPing := time.Now()
	lastPingSeen := atsync.seenEvents.Load()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			since := sinceNanos(atsync.lastEvent.get())
			goroutines := runtime.NumGoroutine()
			dry := sinceNanos(atsync.lastSeen.Load())
			// "Behind" is only meaningful while events arrive; a silent stream is
			// dry, not behind.
			switch {
			case dry > 10*time.Second:
				log.Warn(ctx, fmt.Sprintf("firehose dry; no new events for %s", dry), "goroutines", goroutines)
			case since > 10*time.Second:
				log.Warn(ctx, fmt.Sprintf("firehose is %s behind real time", since), "goroutines", goroutines)
			default:
				log.Debug(ctx, fmt.Sprintf("firehose is %s behind real time", since), "goroutines", goroutines)
			}
			// Healthy operation is otherwise Debug-only, so heartbeat at Info.
			if since <= 10*time.Second && dry <= 10*time.Second &&
				time.Since(lastPing) >= firehoseSyncPingInterval {
				seen := atsync.seenEvents.Load()
				log.Log(ctx, "firehose in sync",
					"behind", since,
					"events", seen-lastPingSeen,
					"goroutines", goroutines)
				lastPing = time.Now()
				lastPingSeen = seen
			}
		}
	}
}

var CollectionFilter = []string{
	constants.APP_BSKY_GRAPH_FOLLOW,
	constants.APP_BSKY_FEED_POST,
	constants.APP_BSKY_GRAPH_BLOCK,
	constants.APP_BSKY_ACTOR_PROFILE,
	constants.PLACE_STREAM_LIVE_RECOMMENDATIONS,
	constants.APP_BSKY_GRAPH_VERIFICATION,
}

// indexedCollection is [CollectionFilter] plus our own place.stream.* space.
// An empty filter means "index everything".
func indexedCollection(collection string) bool {
	if len(CollectionFilter) == 0 {
		return true
	}
	if slices.Contains(CollectionFilter, collection) {
		return true
	}
	return strings.HasPrefix(collection, "place.stream.")
}

// commitHasIndexedOps reports from op paths alone whether a commit touches
// anything we index, so most of the stream skips the CAR parse. An unparsable
// path counts as indexed, so the main loop can log it.
func commitHasIndexedOps(evt *indigoatproto.SyncSubscribeRepos_Commit) bool {
	for _, op := range evt.Ops {
		collection, _, err := syntax.ParseRepoPath(op.Path)
		if err != nil {
			return true
		}
		if indexedCollection(collection.String()) {
			return true
		}
	}
	return false
}

func (atsync *ATProtoSynchronizer) handleCommitEventOps(ctx context.Context, evt *indigoatproto.SyncSubscribeRepos_Commit) {
	if !atsync.indexEventAllowed(ctx, evt.Repo) {
		return
	}
	ctx = log.WithLogValues(ctx, "event", "commit", "did", evt.Repo, "rev", evt.Rev, "seq", fmt.Sprintf("%d", evt.Seq), "func", "handleCommitEventOps")

	if evt.TooBig {
		log.Warn(ctx, "skipping tooBig events for now")
		return
	}

	// Filtered commits still advance the rev chain (which needs no CAR);
	// skipping them would make the next real commit look like a hole.
	if commitHasIndexedOps(evt) && !atsync.handleIndexedOps(ctx, evt) {
		return
	}

	// Only a clean pass may claim the commit; a bail-out leaves the rev for
	// the next event to notice and repair.
	atsync.trackCommitRev(ctx, evt)
}

// handleIndexedOps reads the CAR and applies the ops we index, reporting
// whether it got all the way through. False leaves the rev unclaimed so the
// next commit repairs the gap.
func (atsync *ATProtoSynchronizer) handleIndexedOps(ctx context.Context, evt *indigoatproto.SyncSubscribeRepos_Commit) bool {
	rr, err := repo.ReadRepoFromCar(ctx, bytes.NewReader(evt.Blocks))
	if err != nil {
		log.Error(ctx, "failed to read repo from car", "err", err)
		return false
	}

	complete := true
	for _, op := range evt.Ops {
		collection, rkey, err := syntax.ParseRepoPath(op.Path)
		uri := fmt.Sprintf("at://%s/%s", evt.Repo, op.Path)
		if err != nil {
			log.Error(ctx, "invalid path in repo op", "eventKind", op.Action, "path", op.Path)
			return false
		}
		ctx := log.WithLogValues(ctx, "eventKind", op.Action, "collection", collection.String(), "rkey", rkey.String())

		if !indexedCollection(collection.String()) {
			continue
		}

		// opTime stamps the deletes below.
		aqt, err := aqtime.FromString(evt.Time)
		if err != nil {
			log.Error(ctx, "failed to parse time", "err", err)
			continue
		}
		opTime := aqt.Time()

		r, err := atsync.Model.GetRepo(evt.Repo)
		if err != nil {
			log.Error(ctx, "failed to get repo", "err", err)
			continue
		}
		if r != nil {
			// Enrich once so every log line below carries the handle.
			ctx = log.WithLogValues(ctx, "handle", r.Handle)
		}
		atsync.reviveRepo(ctx, r)

		ek := repomgr.EventKind(op.Action)
		switch ek {
		case repomgr.EvtKindCreateRecord, repomgr.EvtKindUpdateRecord:
			// read the record bytes from blocks, and verify CID
			rc, recCBOR, err := rr.GetRecordBytes(ctx, op.Path)
			if err != nil {
				log.Error(ctx, "reading record from event blocks (CAR)", "err", err)
				break
			}
			if !op.Cid.Defined() || glex.Link(rc) != glex.Link(cid.Cid(*op.Cid)) {
				log.Error(ctx, "mismatch between commit op CID and record block", "recordCID", rc, "opCID", op.Cid)
				break
			}

			// One malformed record must not panic the whole firehose consumer.
			err = log.Recover(ctx, func() error {
				return atsync.handleCreateUpdate(ctx, evt.Repo, rkey, recCBOR, op.Cid.String(), collection, ek == repomgr.EvtKindUpdateRecord, false, evt.Rev)
			})
			if err != nil {
				log.Error(ctx, "failed to handle create update", "err", err)
				continue
			}

		case repomgr.EvtKindDeleteRecord:
			if collection.String() == constants.PLACE_STREAM_SERVER_SETTINGS {
				if err := atsync.Model.DeleteServerSettings(ctx, rkey.String(), evt.Repo); err != nil {
					log.Error(ctx, "failed to delete server settings", "err", err)
					complete = false
				}
			}

			if collection.String() == constants.APP_BSKY_GRAPH_FOLLOW {
				if r == nil {
					log.Debug(ctx, "no repo found for follow", "userDID", evt.Repo, "subjectDID", rkey.String())
					continue
				}
				log.Debug(ctx, "deleting follow", "userDID", evt.Repo, "subjectDID", rkey.String())
				err := atsync.Model.DeleteFollow(ctx, evt.Repo, rkey.String())
				if err != nil {
					log.Debug(ctx, "failed to delete follow", "err", err)
				}
			}

			if collection.String() == constants.APP_BSKY_GRAPH_BLOCK {
				if r == nil {
					log.Debug(ctx, "no repo found for block", "userDID", evt.Repo, "subjectDID", rkey.String())
					continue
				}
				log.Warn(ctx, "deleting block", "userDID", evt.Repo, "subjectDID", rkey.String())
				err := atsync.Model.DeleteBlock(ctx, rkey.String())
				if err != nil {
					log.Error(ctx, "failed to delete block", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_KEY {
				log.Warn(ctx, "revoking stream key", "userDID", evt.Repo, "rkey", rkey.String())
				key, err := atsync.Model.GetSigningKeyByRKey(ctx, rkey.String())
				if err != nil {
					log.Error(ctx, "failed to get signing key", "err", err)
					continue
				}
				if key == nil {
					log.Warn(ctx, "no signing key found for stream key", "userDID", evt.Repo, "rkey", rkey.String())
					continue
				}
				now := time.Now()
				key.RevokedAt = &now
				err = atsync.Model.UpdateSigningKey(key)
				if err != nil {
					log.Error(ctx, "failed to revoke signing key", "err", err)
				}
				atsync.Bus.Publish(evt.Repo, key)
			}

			if collection.String() == constants.APP_BSKY_GRAPH_VERIFICATION {
				if err := atsync.Model.DeleteVerification(ctx, uri); err != nil {
					log.Error(ctx, "failed to delete verification", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_CHAT_ACCESS {
				if err := atsync.Model.DeleteChatAccessRule(ctx, uri); err != nil {
					log.Error(ctx, "failed to delete chat access rule", "err", err)
				}
				atsync.NoteChatAccessRule(ctx, &model.ChatAccessRule{RepoDID: evt.Repo})
			}

			if collection.String() == constants.PLACE_STREAM_CHAT_MESSAGE {
				msg, err := atsync.Model.GetChatMessage(uri)
				if err != nil {
					log.Error(ctx, "failed to get chat message", "err", err)
					continue
				}
				if msg == nil {
					log.Warn(ctx, "no chat message found for uri", "uri", uri)
					continue
				}
				log.Warn(ctx, "deleting chat message", "userDID", evt.Repo, "uri", uri)
				err = atsync.Model.DeleteChatMessage(ctx, uri, &opTime)
				if err != nil {
					log.Error(ctx, "failed to delete chat message", "err", err)
					continue
				}
				mv, err := msg.ToStreamplaceMessageView()
				if err != nil {
					log.Error(ctx, "failed to convert chat message to streamplace message view", "err", err)
					continue
				}
				isTrue := true
				mv.Deleted = &isTrue
				atsync.Bus.Publish(msg.StreamerRepoDID, mv)
			}

			if collection.String() == constants.PLACE_STREAM_LIVE_TELEPORT {
				atsync.handleTeleportDelete(ctx, uri)
			}

			if collection.String() == constants.PLACE_STREAM_MODERATION_PERMISSION {
				log.Debug(ctx, "deleting moderation delegation", "userDID", evt.Repo, "rkey", rkey.String())
				err := atsync.Model.DeleteModerationDelegation(ctx, evt.Repo, rkey.String())
				if err != nil {
					log.Error(ctx, "failed to delete moderation delegation", "err", err)
					continue
				}
				// Publish deletion to WebSocket bus for real-time updates
				deletedRecord := map[string]any{
					"$type":    constants.PLACE_STREAM_MODERATION_PERMISSION,
					"deleted":  true,
					"uri":      uri,
					"rkey":     rkey.String(),
					"streamer": evt.Repo,
					"repoRev":  evt.Rev,
				}
				go atsync.Bus.Publish(evt.Repo, deletedRecord)
			}

			if collection.String() == constants.PLACE_STREAM_CHAT_GATE {
				log.Debug(ctx, "deleting gate", "userDID", evt.Repo, "rkey", rkey.String())
				err := atsync.Model.DeleteGate(ctx, rkey.String())
				if err != nil {
					log.Error(ctx, "failed to delete gate", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_CHAT_PINNED_RECORD {
				log.Debug(ctx, "deleting pinned record", "userDID", evt.Repo, "rkey", rkey.String())
				err := atsync.Model.DeletePinnedRecord(ctx, rkey.String())
				if err != nil {
					log.Error(ctx, "failed to delete pinned record", "err", err)
				}
				deletedPin := map[string]any{
					"$type":   constants.PLACE_STREAM_CHAT_PINNED_RECORD,
					"rkey":    rkey.String(),
					"deleted": true,
				}
				go atsync.Bus.Publish(evt.Repo, deletedPin)
			}

			if collection.String() == constants.PLACE_STREAM_BADGE_DEF {
				log.Debug(ctx, "deleting badge def", "uri", uri)
				err := atsync.Model.DeleteBadgeDef(ctx, uri)
				if err != nil {
					log.Error(ctx, "failed to delete badge def", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_BADGE_ISSUANCE {
				log.Debug(ctx, "deleting badge issuance", "uri", uri)
				err := atsync.Model.DeleteBadgeIssuance(ctx, uri)
				if err != nil {
					log.Error(ctx, "failed to delete badge issuance", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_VIDEO {
				log.Debug(ctx, "deleting video", "uri", uri)
				if err := atsync.Model.DeleteVideo(ctx, uri); err != nil {
					log.Error(ctx, "failed to delete video", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_MEDIA_TRACK {
				log.Debug(ctx, "deleting media track", "uri", uri)
				if err := atsync.Model.DeleteMediaTrack(ctx, uri); err != nil {
					log.Error(ctx, "failed to delete media track", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_MEDIA_ORIGIN {
				log.Debug(ctx, "deleting media origin", "uri", uri)
				if err := atsync.Model.DeleteMediaOrigin(ctx, uri); err != nil {
					log.Error(ctx, "failed to delete media origin", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_BETA_INVITE {
				log.Debug(ctx, "deleting beta invite", "uri", uri)
				if err := atsync.Model.DeleteBetaInvite(ctx, uri); err != nil {
					log.Error(ctx, "failed to delete beta invite", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_BETA_REQUEST {
				log.Debug(ctx, "deleting beta request", "uri", uri)
				if err := atsync.Model.DeleteBetaRequest(ctx, uri); err != nil {
					log.Error(ctx, "failed to delete beta request", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_MEDIA_VIEW_COUNT {
				log.Debug(ctx, "deleting media view count", "uri", uri)
				if err := atsync.Model.DeleteMediaViewCount(ctx, uri); err != nil {
					log.Error(ctx, "failed to delete media view count", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_VOD_COMMENT {
				log.Debug(ctx, "deleting VOD comment", "uri", uri)
				if err := atsync.Model.DeleteVodComment(ctx, uri, &opTime); err != nil {
					log.Error(ctx, "failed to delete VOD comment", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_LIKE {
				log.Debug(ctx, "deleting like", "uri", uri)
				if err := atsync.Model.DeleteLike(ctx, uri); err != nil {
					log.Error(ctx, "failed to delete like", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_VOD_GATE {
				log.Debug(ctx, "deleting VOD gate", "userDID", evt.Repo, "rkey", rkey.String())
				if err := atsync.Model.DeleteVodGate(ctx, rkey.String()); err != nil {
					log.Error(ctx, "failed to delete VOD gate", "err", err)
				}
			}

		default:
			log.Error(ctx, "unexpected record op kind")
		}
	}

	return complete
}

func (atsync *ATProtoSynchronizer) handleTeleportDelete(ctx context.Context, uri string) {
	// The delete only carries the URI; the record identifies the recipients and
	// the CID, so a newer version is not cleared.
	teleport, err := atsync.Model.GetTeleportByURI(uri)
	if err != nil {
		log.Error(ctx, "failed to get teleport before deleting", "err", err, "uri", uri)
		teleport = nil
	}

	if err := atsync.Model.DeleteTeleport(ctx, uri); err != nil {
		log.Error(ctx, "failed to delete teleport", "err", err, "uri", uri)
		return
	}
	if teleport == nil {
		return
	}

	cancelMsg := map[string]any{
		"$type":       "place.stream.livestream#teleportCanceled",
		"teleportUri": uri,
		"cid":         teleport.CID,
		"reason":      "deleted",
	}
	atsync.Bus.Publish(teleport.RepoDID, cancelMsg)
	atsync.Bus.Publish(teleport.TargetDID, cancelMsg)
}

// reviveRepo clears a terminal status: a commit proves the account is back.
// The caller passes the row it already loaded, so the common case is free.
func (atsync *ATProtoSynchronizer) reviveRepo(ctx context.Context, r *model.Repo) {
	if r == nil || !r.TerminalStatus() {
		return
	}
	log.Log(ctx, "repo committed while parked, clearing terminal status", "did", r.DID, "handle", r.Handle, "status", r.Status)
	if err := atsync.Model.SetRepoStatus(ctx, r.DID, model.RepoStatusOK); err != nil {
		log.Error(ctx, "failed to clear repo status", "did", r.DID, "handle", r.Handle, "err", err)
		return
	}
	r.Status = model.RepoStatusOK
}

func (atsync *ATProtoSynchronizer) handleIdentityEventOps(ctx context.Context, evt *indigoatproto.SyncSubscribeRepos_Identity) {
	if !atsync.indexEventAllowed(ctx, evt.Did) {
		return
	}
	handle := ""
	if evt.Handle != nil {
		handle = *evt.Handle
	}
	ctx = log.WithLogValues(ctx, "event", "identity", "did", evt.Did, "handle", handle, "func", "handleIdentityEventOps")
	r, err := atsync.Model.GetRepo(evt.Did)
	if err != nil {
		log.Error(ctx, "failed to get repo", "err", err)
		return
	}
	if r == nil {
		log.Debug(ctx, "no repo found for identity", "did", evt.Did)
		return
	}
	_, err = atsync.RefreshIdentity(ctx, evt.Did)
	if err != nil {
		log.Error(ctx, "failed to refresh ident", "err", err)
		return
	}
}
