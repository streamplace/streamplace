package atproto

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
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

// dedupWindow bounds how long a key is remembered. Relays run within seconds of
// each other, so a few minutes covers all realistic skew.
const dedupWindow = 5 * time.Minute

type ATProtoSynchronizer struct {
	CLI                *config.CLI
	Model              model.Model
	StatefulDB         *statedb.StatefulDB
	Noter              notificationpkg.FirebaseNotifier
	Bus                *bus.Bus
	PLCDirectory       identity.Directory
	CachedPLCDirectory identity.Directory
	OATProxy           *oatproxy.OATProxy

	// firehose liveness, written from every relay consumer concurrently
	// (unix nanos).
	lastSeen  atomic.Int64
	lastEvent atomic.Int64

	// cross-relay dedup, shared by every relay consumer. Initialized at the
	// top of StartFirehose.
	commitDedup   *firehoseDeduper
	identityDedup *firehoseDeduper
}

func (atsync *ATProtoSynchronizer) markSeen() {
	atsync.lastSeen.Store(time.Now().UnixNano())
}

func (atsync *ATProtoSynchronizer) markEvent(t time.Time) {
	atsync.lastEvent.Store(t.UnixNano())
}

func sinceNanos(ns int64) time.Duration {
	if ns == 0 {
		return 0
	}
	return time.Since(time.Unix(0, ns))
}

// relayHosts returns the ordered, de-duplicated list of relay websocket URLs to
// consume: the comma-separated --relay-host list, plus our own PDS firehose so
// records published to our built-in PDS are always indexed immediately (rather
// than waiting for an external relay to crawl them back to us). We only add
// ourselves when we actually have an HTTP listener to dial — i.e. never in
// tests that run without the server.
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
	if self := atsync.selfRelayURL(); self != "" {
		add(self)
	}
	return hosts
}

// selfRelayURL is the websocket URL of our own firehose listener, or ""
// when there's no HTTP listener to dial (e.g. tests run without the
// server). Used both to add ourselves to the relay set and to recognize
// that connection in connectRelay so we can present the right Host header.
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

// StartFirehose subscribes to every configured relay (plus our own PDS, if
// enabled) concurrently, de-duplicating the overlapping event streams so each
// commit is indexed exactly once. It returns only when ctx is cancelled; a
// single relay flapping is logged and retried forever rather than taking the
// node down, since surviving relay outages is the whole point of running more
// than one.
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

	log.Log(ctx, "starting firehose consumers", "relays", relays)

	g, ctx := errgroup.WithContext(ctx)
	for _, relay := range relays {
		relay := relay
		g.Go(func() error {
			atsync.consumeRelay(ctx, relay)
			return nil
		})
	}
	g.Go(func() error {
		atsync.monitorFirehose(ctx)
		return nil
	})
	return g.Wait()
}

// consumeRelay keeps a single relay's firehose connected, reconnecting with
// capped exponential backoff. It never returns an error: an unhealthy relay
// must not disturb the others (or crash the node), so it just loops until ctx
// is cancelled.
func (atsync *ATProtoSynchronizer) consumeRelay(ctx context.Context, relay string) {
	ctx = log.WithLogValues(ctx, "relay", relay)

	cursor := atsync.newRelayCursor(ctx, relay)
	// Persist progress on a timer and once more on the way out, so a restart
	// resumes near where we left off rather than re-tailing from live.
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
		err := atsync.connectRelay(ctx, relay, cursor)
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

// connectRelay dials one relay and pumps its firehose until the connection
// drops or ctx is cancelled. Event handlers are spawned on the parent ctx (not
// the per-connection one) so an in-flight commit keeps indexing across a
// reconnect — important because dedup has already claimed it, so no other relay
// will re-deliver it to us.
func (atsync *ATProtoSynchronizer) connectRelay(ctx context.Context, relay string, cursor *relayCursor) error {
	u, err := url.Parse(relay)
	if err != nil {
		return fmt.Errorf("invalid relay URI %q: %w", relay, err)
	}
	// MoQ relays (moqt:// and aliases) are consumed over QUIC instead of
	// WebSocket. Everything downstream of frame-decode — dedup, cursor,
	// handlers, backoff — is shared, so this is just a transport swap.
	switch u.Scheme {
	case "moqt", "moql", "moq", "moqs":
		return atsync.connectRelayMoq(ctx, relay, cursor)
	}
	u.Path = "xrpc/com.atproto.sync.subscribeRepos"
	if seq, ok := cursor.param(); ok {
		q := u.Query()
		q.Set("cursor", strconv.FormatInt(seq, 10))
		u.RawQuery = q.Encode()
	}

	header := http.Header{"User-Agent": []string{aqhttp.UserAgent}}
	// Our own loopback listener serves both the broadcaster and the
	// server-repo firehoses on one port and disambiguates them by the
	// request Host (see Server.isServerPDS). Dialing loopback sends
	// Host: 127.0.0.1, which lands us on the broadcaster firehose, so the
	// server repo's own records — most importantly place.stream.media.origin,
	// which getVideoList's "can this node serve it" filter depends on —
	// would never get indexed locally. Force Host to ServerHost so the
	// self-subscription gets the server-PDS firehose, matching what a
	// single-node (ServerHost == BroadcasterHost) deployment gets for free.
	// gorilla/websocket pulls the "Host" header out and uses it as the
	// HTTP Host while still dialing the loopback address in u.
	if relay == atsync.selfRelayURL() && atsync.CLI.ServerHost != "" {
		header.Set("Host", atsync.CLI.ServerHost)
	}
	con, _, err := websocket.DefaultDialer.Dial(u.String(), header)
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

// relayProtocol classifies a relay URL as "moq" (moqt:// and aliases) or
// "websocket" (everything else), for per-protocol metric labels.
func relayProtocol(relay string) string {
	if u, err := url.Parse(relay); err == nil {
		switch u.Scheme {
		case "moqt", "moql", "moq", "moqs":
			return "moq"
		}
	}
	return "websocket"
}

// repoStreamCallbacks builds the event callbacks shared by the WebSocket and
// MoQ transports: per-relay metrics, liveness marking, cursor advance,
// cross-relay dedup, and spawning the indexing handlers on the parent ctx.
// cancel ends the current connection when the relay sends an error frame. The
// handlers run on ctx (not the per-connection context) so an in-flight commit
// keeps indexing across a reconnect.
func (atsync *ATProtoSynchronizer) repoStreamCallbacks(ctx context.Context, relay string, cursor *relayCursor, cancel context.CancelFunc) *events.RepoStreamCallbacks {
	protocol := relayProtocol(relay)
	// observeSeq advances the cursor and republishes the per-relay high-water
	// seq gauge; both relays carry the upstream's seq, so the gauge's cross-relay
	// difference measures how far apart the relays are.
	observeSeq := func(seq int64) {
		cursor.observe(seq)
		spmetrics.FirehoseRelayHighSeq.WithLabelValues(relay, protocol).Set(float64(cursor.highSeq()))
	}
	return &events.RepoStreamCallbacks{
		RepoCommit: func(evt *indigoatproto.SyncSubscribeRepos_Commit) error {
			atsync.markSeen()
			observeSeq(evt.Seq)
			spmetrics.FirehoseEventsReceivedTotal.WithLabelValues(relay, protocol, "commit").Inc()
			if atsync.commitDedup.seen(evt.Commit.String()) {
				spmetrics.FirehoseEventsDedupedTotal.WithLabelValues(relay, protocol, "commit").Inc()
				return nil
			}
			go atsync.handleCommitEventOps(ctx, evt)
			return nil
		},
		RepoIdentity: func(evt *indigoatproto.SyncSubscribeRepos_Identity) error {
			atsync.markSeen()
			observeSeq(evt.Seq)
			spmetrics.FirehoseEventsReceivedTotal.WithLabelValues(relay, protocol, "identity").Inc()
			if atsync.identityDedup.seen(identityDedupKey(evt)) {
				spmetrics.FirehoseEventsDedupedTotal.WithLabelValues(relay, protocol, "identity").Inc()
				return nil
			}
			go atsync.handleIdentityEventOps(ctx, evt)
			return nil
		},
		Error: func(evt *events.ErrorFrame) error {
			cancel()
			return fmt.Errorf("firehose error: %s: %s", evt.Error, evt.Message)
		},
	}
}

// identityDedupKey distinguishes a single identity broadcast (so the same one
// relayed by several relays collapses) from genuinely separate updates to the
// same DID (a later handle change must NOT be dropped). The originating Time is
// preserved as the event is relayed, so (did, time, handle) is stable for one
// broadcast yet distinct across real changes. seq is deliberately excluded — it
// is assigned per-relay and so differs for the very duplicates we want to fold.
func identityDedupKey(evt *indigoatproto.SyncSubscribeRepos_Identity) string {
	handle := ""
	if evt.Handle != nil {
		handle = *evt.Handle
	}
	return evt.Did + "\x00" + evt.Time + "\x00" + handle
}

func (atsync *ATProtoSynchronizer) monitorFirehose(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			since := sinceNanos(atsync.lastEvent.Load())
			goroutines := runtime.NumGoroutine()
			if since > 10*time.Second {
				log.Warn(ctx, fmt.Sprintf("firehose is %s behind real time", since), "goroutines", goroutines)
			} else {
				log.Debug(ctx, fmt.Sprintf("firehose is %s behind real time", since), "goroutines", goroutines)
			}
			if dry := sinceNanos(atsync.lastSeen.Load()); dry > 10*time.Second {
				log.Warn(ctx, fmt.Sprintf("firehose dry; no new events for %s", dry))
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
}

func (atsync *ATProtoSynchronizer) handleCommitEventOps(ctx context.Context, evt *indigoatproto.SyncSubscribeRepos_Commit) {
	ctx = log.WithLogValues(ctx, "event", "commit", "did", evt.Repo, "rev", evt.Rev, "seq", fmt.Sprintf("%d", evt.Seq), "func", "handleCommitEventOps")

	if evt.TooBig {
		log.Warn(ctx, "skipping tooBig events for now")
		return
	}

	rr, err := repo.ReadRepoFromCar(ctx, bytes.NewReader(evt.Blocks))
	if err != nil {
		log.Error(ctx, "failed to read repo from car", "err", err)
		return
	}

	for _, op := range evt.Ops {
		collection, rkey, err := syntax.ParseRepoPath(op.Path)
		uri := fmt.Sprintf("at://%s/%s", evt.Repo, op.Path)
		if err != nil {
			log.Error(ctx, "invalid path in repo op", "eventKind", op.Action, "path", op.Path)
			return
		}
		ctx = log.WithLogValues(ctx, "eventKind", op.Action, "collection", collection.String(), "rkey", rkey.String())

		if len(CollectionFilter) > 0 {
			keep := slices.Contains(CollectionFilter, collection.String())
			if strings.HasPrefix(collection.String(), "place.stream.") {
				keep = true
			}
			if !keep {
				continue
			}
		}

		aqt, err := aqtime.FromString(evt.Time)
		if err != nil {
			log.Error(ctx, "failed to parse time", "err", err)
			continue
		}
		opTime := aqt.Time()
		atsync.markEvent(opTime)

		r, err := atsync.Model.GetRepo(evt.Repo)
		if err != nil {
			log.Error(ctx, "failed to get repo", "err", err)
			continue
		}
		// log.Warn(ctx, "got record we care about", "collection", collection, "rkey", rkey)

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

			err = atsync.handleCreateUpdate(ctx, evt.Repo, rkey, recCBOR, op.Cid.String(), collection, ek == repomgr.EvtKindUpdateRecord, false)
			if err != nil {
				log.Error(ctx, "failed to handle create update", "err", err)
				continue
			}

		case repomgr.EvtKindDeleteRecord:
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
				err := atsync.Model.DeleteTeleport(ctx, uri)
				if err != nil {
					log.Error(ctx, "failed to delete teleport", "err", err)
				}
			}

			if collection.String() == constants.PLACE_STREAM_MODERATION_PERMISSION {
				log.Debug(ctx, "deleting moderation delegation", "userDID", evt.Repo, "rkey", rkey.String())
				err := atsync.Model.DeleteModerationDelegation(ctx, rkey.String())
				if err != nil {
					log.Error(ctx, "failed to delete moderation delegation", "err", err)
				}
				// Publish deletion to WebSocket bus for real-time updates
				// Create a deleted record marker to notify frontend
				deletedRecord := map[string]any{
					"$type":    "place.stream.moderation.permission",
					"deleted":  true,
					"rkey":     rkey.String(),
					"streamer": evt.Repo,
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
}

func (atsync *ATProtoSynchronizer) handleIdentityEventOps(ctx context.Context, evt *indigoatproto.SyncSubscribeRepos_Identity) {
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
