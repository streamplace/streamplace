package atproto

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/util"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	glex "github.com/streamplace/glex/runtime"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/devenv"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/reposync"
	"stream.place/streamplace/pkg/statedb"
)

// TestBackfillWalk drives SyncBlueskyRepo's walker path against the Bluesky
// reference PDS: real com.atproto.sync.getLatestCommit/getBlocks, real MST,
// real did:plc key resolution. The firehose is deliberately not started, so
// every record indexed here got there through the backfill.
func TestBackfillWalk(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	ctx := context.Background()
	atsync, mod := backfillTestSynchronizer(t, dev)

	user := dev.CreateAccount(t)

	// In range, via the place.stream. prefix.
	profile := createBackfillRecord(t, user, "place.stream.chat.profile", "self", &placestream.ChatProfile{})
	msg1 := createBackfillRecord(t, user, "place.stream.chat.message", "", chatMessageRecord(user.DID, "hello one"))
	msg2 := createBackfillRecord(t, user, "place.stream.chat.message", "", chatMessageRecord(user.DID, "hello two"))
	// In range, via a CollectionFilter collection (sorts well below place.stream.).
	displayName := "Backfill Tester"
	bskyProfile := createBackfillRecord(t, user, "app.bsky.actor.profile", "self", &appbsky.ActorProfile{DisplayName: &displayName})
	// Out of range, and adjacent to the app.bsky.actor.profile/ range on the
	// side where an off-by-one prefix bound would leak.
	createBackfillRecord(t, user, "app.bsky.actor.status", "", rawBackfillRecord(t, "app.bsky.actor.status", map[string]any{
		"status":    "app.bsky.actor.status#live",
		"createdAt": time.Now().UTC().Format(util.ISO8601),
	}))
	// Out of range, above everything we index.
	createBackfillRecord(t, user, "zzz.example.thing", "", rawBackfillRecord(t, "zzz.example.thing", map[string]any{
		"createdAt": time.Now().UTC().Format(util.ISO8601),
	}))

	wantPaths := []string{bskyProfile, profile, msg1, msg2}
	sort.Strings(wantPaths)

	// Walk the ranges the backfill uses, directly. This both waits for the PDS
	// to have committed everything and proves the out-of-range records are
	// never even fetched, which SyncBlueskyRepo alone cannot show (nothing
	// indexes an app.bsky.actor.status either way).
	var head *reposync.Head
	err := untilNoErrors(t, func() error {
		xrpcc := &xrpc.Client{Host: dev.PDSURL, Client: &aqhttp.Client}
		fetcher := &reposync.CachedFetcher{
			Cache: reposync.NewMemoryBlockCache(),
			Inner: &reposync.XRPCBlockFetcher{Client: xrpcc, DID: user.DID},
		}
		h, err := reposync.FetchVerifiedHead(ctx, xrpcc, fetcher, dev.TestDirectory(), user.DID)
		if err != nil {
			return err
		}
		var got []string
		walker := &reposync.Walker{Fetcher: fetcher}
		err = walker.WalkRanges(ctx, h.Root, backfillRanges(""), func(path string, rcid cid.Cid, rec []byte) error {
			got = append(got, path)
			return nil
		})
		if err != nil {
			return err
		}
		sort.Strings(got)
		if strings.Join(got, ",") != strings.Join(wantPaths, ",") {
			return fmt.Errorf("walked %v, want %v", got, wantPaths)
		}
		head = h
		return nil
	})
	require.NoError(t, err, "direct walk of backfill ranges")

	repo, err := atsync.SyncBlueskyRepoCached(ctx, user.DID)
	require.NoError(t, err)
	require.Equal(t, user.DID, repo.DID)
	require.Equal(t, head.Rev, repo.Version, "repo row should record the rev it was synced to")
	require.Equal(t, head.Root.String(), repo.RootCID, "repo row should record the verified MST root")

	// And it is durable, not just what SyncBlueskyRepo happened to return.
	stored, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.Equal(t, head.Rev, stored.Version)
	require.Equal(t, head.Root.String(), stored.RootCID)

	messages, err := mod.MostRecentChatMessages(user.DID)
	require.NoError(t, err)
	require.Len(t, messages, 2)
	texts := []string{
		messages[0].Record.Val.(*placestream.ChatMessage).Text,
		messages[1].Record.Val.(*placestream.ChatMessage).Text,
	}
	sort.Strings(texts)
	require.Equal(t, []string{"hello one", "hello two"}, texts)

	chatProfile, err := mod.GetChatProfile(ctx, user.DID)
	require.NoError(t, err)
	require.NotNil(t, chatProfile, "place.stream.chat.profile should have been indexed")

	indexedBsky, err := mod.GetBskyProfile(ctx, user.DID, false)
	require.NoError(t, err)
	require.NotNil(t, indexedBsky, "app.bsky.actor.profile should have been indexed")
	require.Equal(t, displayName, *indexedBsky.DisplayName)
}

// TestBackfillWedgeHeals covers the placeholder-row semantics: a repo row with
// an empty Version is a backfill that never finished and must be retried, while
// a row with a Version is authoritative and must not cause any network traffic.
func TestBackfillWedgeHeals(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	ctx := context.Background()
	atsync, mod := backfillTestSynchronizer(t, dev)

	user := dev.CreateAccount(t)
	createBackfillRecord(t, user, "place.stream.chat.message", "", chatMessageRecord(user.DID, "wedged"))

	// Exactly what a crashed backfill leaves behind.
	require.NoError(t, mod.UpdateRepo(&model.Repo{
		DID:     user.DID,
		PDS:     dev.PDSURL,
		Handle:  user.Handle,
		Version: "",
	}))

	err := untilNoErrors(t, func() error {
		repo, err := atsync.SyncBlueskyRepoCached(ctx, user.DID)
		if err != nil {
			return err
		}
		if repo.Version == "" {
			return fmt.Errorf("repo still has no version")
		}
		messages, err := mod.MostRecentChatMessages(user.DID)
		if err != nil {
			return err
		}
		if len(messages) != 1 {
			return fmt.Errorf("expected 1 message, got %d", len(messages))
		}
		return nil
	})
	require.NoError(t, err, "an incomplete backfill should be re-run")

	stored, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.NotEmpty(t, stored.Version)
	require.NotEmpty(t, stored.RootCID)

	// The inverse: a row with a Version short-circuits before anything is
	// resolved or fetched. This DID exists nowhere, so any attempt to sync it
	// would fail at identity resolution.
	missingDID := "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	require.NoError(t, mod.UpdateRepo(&model.Repo{
		DID:     missingDID,
		PDS:     "http://127.0.0.1:1",
		Version: "3lbogus000000",
	}))
	got, err := atsync.SyncBlueskyRepoCached(ctx, missingDID)
	require.NoError(t, err, "a complete repo row must be returned without touching the network")
	require.Equal(t, "3lbogus000000", got.Version)

	// Same unresolvable DID, but wedged: now it must actually try to sync, and
	// fail, rather than hand back the placeholder forever.
	require.NoError(t, mod.UpdateRepo(&model.Repo{
		DID:     missingDID,
		PDS:     "http://127.0.0.1:1",
		Version: "",
	}))
	_, err = atsync.SyncBlueskyRepoCached(ctx, missingDID)
	require.Error(t, err, "a placeholder row must not short-circuit the sync")
}

// TestBackfillFallsBackToGetRepo puts a host in front of the dev PDS that
// serves everything except the two sync methods the walk needs -- which is
// exactly what streamplace's own PDS looks like to another node today -- and
// checks that the backfill quietly finishes over the legacy full-CAR path.
func TestBackfillFallsBackToGetRepo(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	ctx := context.Background()
	atsync, mod := backfillTestSynchronizer(t, dev)

	user := dev.CreateAccount(t)
	createBackfillRecord(t, user, "place.stream.chat.message", "", chatMessageRecord(user.DID, "fallback"))

	target, err := url.Parse(dev.PDSURL)
	require.NoError(t, err)
	reverse := httputil.NewSingleHostReverseProxy(target)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "com.atproto.sync.getLatestCommit"),
			strings.HasSuffix(r.URL.Path, "com.atproto.sync.getBlocks"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"MethodNotImplemented","message":"not registered"}`))
		default:
			reverse.ServeHTTP(w, r)
		}
	}))
	defer proxy.Close()

	// A finished repo row, so that indexing a chat message does not kick off a
	// second sync of the same DID against the real (unproxied) PDS and index
	// the record for us.
	require.NoError(t, mod.UpdateRepo(&model.Repo{
		DID:     user.DID,
		PDS:     proxy.URL,
		Handle:  user.Handle,
		Version: "3lpretend0000",
	}))

	ident, err := atsync.resolveIdent(ctx, user.DID, false)
	require.NoError(t, err)

	var result backfillResult
	err = untilNoErrors(t, func() error {
		var err error
		result, err = atsync.backfillRepo(ctx, ident, &xrpc.Client{Host: proxy.URL, Client: &aqhttp.Client}, reposync.TIDForTime(time.Now().Add(-InitialWindow)))
		if err != nil {
			return err
		}
		messages, err := mod.MostRecentChatMessages(user.DID)
		if err != nil {
			return err
		}
		if len(messages) != 1 {
			return fmt.Errorf("expected 1 message, got %d", len(messages))
		}
		return nil
	})
	require.NoError(t, err, "backfill should have fallen back to getRepo")
	require.NotEmpty(t, result.Rev, "the legacy path still reports the commit rev")
	require.Empty(t, result.RootCID, "the legacy path has no verified MST root to record")
	require.Empty(t, result.Floor, "a full CAR download ignores the window")
	require.True(t, result.Done, "a full CAR download leaves no history to deepen")
}

func TestIsMethodNotSupported(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"wrapped 404", fmt.Errorf("getBlocks: %w", &xrpc.Error{StatusCode: http.StatusNotFound}), true},
		{"wrapped 405", fmt.Errorf("getBlocks: %w", &xrpc.Error{StatusCode: http.StatusMethodNotAllowed}), true},
		{
			// What streamplace's own PDS answers for an unregistered
			// /xrpc/* method: the wildcard proxy wants an OAuth session.
			"wrapped 401 from the spxrpc wildcard proxy",
			fmt.Errorf("getLatestCommit: %w", &xrpc.Error{
				StatusCode: http.StatusUnauthorized,
				Wrapped:    &xrpc.XRPCError{Message: "oauth session not found"},
			}),
			true,
		},
		{"wrapped 501", fmt.Errorf("getBlocks: %w", &xrpc.Error{StatusCode: http.StatusNotImplemented}), true},
		{
			"doubly wrapped 404 with body",
			fmt.Errorf("walking: %w", fmt.Errorf("getBlocks: %w", &xrpc.Error{
				StatusCode: http.StatusNotFound,
				Wrapped:    &xrpc.XRPCError{ErrStr: "MethodNotImplemented", Message: "no such route"},
			})),
			true,
		},
		{
			"MethodNotImplemented error name",
			fmt.Errorf("getLatestCommit: %w", &xrpc.XRPCError{ErrStr: "MethodNotImplemented", Message: "nope"}),
			true,
		},
		{
			// The whole point of looking at the error name: streamplace's own
			// getBlocks answers 404 BlockNotFound for a block it no longer
			// has. That host implements the method; falling back to a full
			// getRepo download because of it would be a disaster.
			"404 BlockNotFound",
			fmt.Errorf("getBlocks: %w", &xrpc.Error{
				StatusCode: http.StatusNotFound,
				Wrapped:    &xrpc.XRPCError{ErrStr: "BlockNotFound", Message: "bafyreib2"},
			}),
			false,
		},
		{
			// Same thing as it actually arrives from spxrpc, where echo's
			// default error handler puts the name in "message" and leaves
			// "error" empty.
			"404 BlockNotFound with the name only in the message",
			fmt.Errorf("getBlocks: %w", &xrpc.Error{
				StatusCode: http.StatusNotFound,
				Wrapped:    &xrpc.XRPCError{Message: "BlockNotFound"},
			}),
			false,
		},
		{
			"404 RepoNotFound",
			fmt.Errorf("getBlocks: %w", &xrpc.Error{
				StatusCode: http.StatusNotFound,
				Wrapped:    &xrpc.XRPCError{Message: "RepoNotFound"},
			}),
			false,
		},
		{
			"400 InvalidRequest",
			fmt.Errorf("getBlocks: %w", &xrpc.Error{
				StatusCode: http.StatusBadRequest,
				Wrapped:    &xrpc.XRPCError{ErrStr: "InvalidRequest", Message: "cids/0 must be a cid string"},
			}),
			false,
		},
		{
			"400 RepoNotFound",
			fmt.Errorf("getLatestCommit: %w", &xrpc.Error{
				StatusCode: http.StatusBadRequest,
				Wrapped:    &xrpc.XRPCError{ErrStr: "RepoNotFound", Message: "could not find repo"},
			}),
			false,
		},
		{
			"429",
			fmt.Errorf("getBlocks: %w", &xrpc.Error{StatusCode: http.StatusTooManyRequests}),
			false,
		},
		{"block mismatch", fmt.Errorf("fetching: %w", reposync.ErrBlockMismatch), false},
		{"missing block", fmt.Errorf("fetching: %w", reposync.ErrMissingBlock), false},
		{"canceled", fmt.Errorf("walking: %w", context.Canceled), false},
		{"plain error", errors.New("connection refused"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isMethodNotSupported(tc.err))
		})
	}
}

func TestIsStaleWalkError(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{
			// Our own client-side check, which is what fires when a host
			// answers with a short bag of blocks instead of an error.
			"missing block, as the walker wraps it",
			fmt.Errorf("failed to walk repo: %w", fmt.Errorf("fetching 3 MST nodes: %w", reposync.ErrMissingBlock)),
			true,
		},
		{
			// The TypeScript PDS shape, seen from both a bsky.network
			// mothership and a self-hosted instance.
			"400 Could not find cids",
			fmt.Errorf("getBlocks: %w", &xrpc.Error{
				StatusCode: http.StatusBadRequest,
				Wrapped:    &xrpc.XRPCError{ErrStr: "InvalidRequest", Message: "Could not find cids: bafyreib2"},
			}),
			true,
		},
		{
			"404 BlockNotFound",
			fmt.Errorf("getBlocks: %w", &xrpc.Error{
				StatusCode: http.StatusNotFound,
				Wrapped:    &xrpc.XRPCError{ErrStr: "BlockNotFound", Message: "bafyreib2"},
			}),
			true,
		},
		{
			"404 BlockNotFound from spxrpc, name in the message",
			fmt.Errorf("getBlocks: %w", &xrpc.Error{
				StatusCode: http.StatusNotFound,
				Wrapped:    &xrpc.XRPCError{Message: "BlockNotFound"},
			}),
			true,
		},
		{
			"a different InvalidRequest",
			fmt.Errorf("getBlocks: %w", &xrpc.Error{
				StatusCode: http.StatusBadRequest,
				Wrapped:    &xrpc.XRPCError{ErrStr: "InvalidRequest", Message: "cids/0 must be a cid string"},
			}),
			false,
		},
		{
			"RepoNotFound",
			fmt.Errorf("getLatestCommit: %w", &xrpc.Error{
				StatusCode: http.StatusBadRequest,
				Wrapped:    &xrpc.XRPCError{ErrStr: "RepoNotFound", Message: "could not find repo"},
			}),
			false,
		},
		{"throttled", fmt.Errorf("getBlocks: %w", &xrpc.Error{StatusCode: http.StatusTooManyRequests}), false},
		{"tampered block", fmt.Errorf("walking: %w", reposync.ErrBlockMismatch), false},
		{"malformed tree", fmt.Errorf("walking: %w", reposync.ErrInvalidNode), false},
		{"plain error", errors.New("connection refused"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isStaleWalkError(tc.err))
		})
	}
}

// TestWalkWithHeadRetry covers the live-repo race: the PDS garbage-collects the
// blocks of a commit we pinned, and the fix is to re-read the head rather than
// to fail. Staging that against a real PDS would mean making it GC mid-walk, so
// the head fetch and the walk are scripted here instead.
func TestWalkWithHeadRetry(t *testing.T) {
	ctx := context.Background()
	// A missing block, wrapped the way walkBackfill wraps it.
	gone := fmt.Errorf("failed to walk repo for did:plc:x from PDS https://pds: %w",
		fmt.Errorf("fetching 12 MST nodes: %w", reposync.ErrMissingBlock))

	t.Run("restarts from the new head", func(t *testing.T) {
		heads := []*reposync.Head{testHead(t, "3laaa"), testHead(t, "3lbbb")}
		fetches := 0
		fetchHead := func(context.Context) (*reposync.Head, error) {
			h := heads[min(fetches, len(heads)-1)]
			fetches++
			return h, nil
		}
		var walked []cid.Cid
		walk := func(_ context.Context, root cid.Cid) error {
			walked = append(walked, root)
			if len(walked) == 1 {
				return gone
			}
			return nil
		}

		head, err := walkWithHeadRetry(ctx, 3, time.Millisecond, fetchHead, walk)
		require.NoError(t, err)
		require.Equal(t, heads[1], head, "the completed walk was against the new head")
		require.Equal(t, []cid.Cid{heads[0].Root, heads[1].Root}, walked)
		require.Equal(t, 2, fetches)
	})

	t.Run("a head that did not move means the repo really is incomplete", func(t *testing.T) {
		// Never paper over a repo we could not read: the alternative is
		// recording a Version whose records we know we are missing.
		head := testHead(t, "3laaa")
		fetches := 0
		fetchHead := func(context.Context) (*reposync.Head, error) {
			fetches++
			return head, nil
		}
		walks := 0
		walk := func(context.Context, cid.Cid) error {
			walks++
			return gone
		}

		_, err := walkWithHeadRetry(ctx, 3, time.Millisecond, fetchHead, walk)
		require.Error(t, err)
		require.ErrorIs(t, err, reposync.ErrMissingBlock)
		require.Contains(t, err.Error(), "still the head")
		require.Equal(t, 1, walks, "no point walking the same tree again")
		require.Equal(t, 2, fetches)
	})

	t.Run("anything else fails immediately", func(t *testing.T) {
		fetches := 0
		fetchHead := func(context.Context) (*reposync.Head, error) {
			fetches++
			return testHead(t, "3laaa"), nil
		}
		walks := 0
		bad := fmt.Errorf("walking: %w", reposync.ErrBlockMismatch)
		walk := func(context.Context, cid.Cid) error {
			walks++
			return bad
		}

		_, err := walkWithHeadRetry(ctx, 3, time.Millisecond, fetchHead, walk)
		require.ErrorIs(t, err, reposync.ErrBlockMismatch)
		require.Equal(t, 1, walks)
		require.Equal(t, 1, fetches, "a verification failure is not worth a new head")
	})

	t.Run("a repo that keeps moving exhausts the budget", func(t *testing.T) {
		fetches := 0
		fetchHead := func(context.Context) (*reposync.Head, error) {
			fetches++
			return testHead(t, fmt.Sprintf("3l%03d", fetches)), nil
		}
		walks := 0
		walk := func(context.Context, cid.Cid) error {
			walks++
			return gone
		}

		_, err := walkWithHeadRetry(ctx, 3, time.Millisecond, fetchHead, walk)
		require.Error(t, err)
		require.ErrorIs(t, err, reposync.ErrMissingBlock)
		require.Contains(t, err.Error(), "gave up after 3 walk attempts")
		require.Equal(t, 3, walks)
	})

	t.Run("the first head fetch failing is just an error", func(t *testing.T) {
		boom := errors.New("no such host")
		walks := 0
		_, err := walkWithHeadRetry(ctx,
			3, time.Millisecond,
			func(context.Context) (*reposync.Head, error) { return nil, boom },
			func(context.Context, cid.Cid) error { walks++; return nil },
		)
		require.ErrorIs(t, err, boom)
		require.Zero(t, walks)
	})

	t.Run("cancellation during the backoff returns promptly", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		start := time.Now()
		_, err := walkWithHeadRetry(ctx,
			3, 30*time.Second,
			func(context.Context) (*reposync.Head, error) { return testHead(t, "3laaa"), nil },
			func(context.Context, cid.Cid) error { return gone },
		)
		require.Error(t, err)
		require.Less(t, time.Since(start), 5*time.Second)
		require.ErrorIs(t, err, context.Canceled)
		require.ErrorIs(t, err, reposync.ErrMissingBlock, "the walk failure is kept too")
	})
}

// testHead fabricates a head at a given rev; only Rev and Root are consulted.
func testHead(t *testing.T, rev string) *reposync.Head {
	t.Helper()
	root, err := cid.NewPrefixV1(cid.DagCBOR, multihash.SHA2_256).Sum([]byte("root:" + rev))
	require.NoError(t, err)
	return &reposync.Head{Rev: rev, Root: root}
}

// inRanges is the walker's own containment test, spelled out here so these
// tests check the ranges rather than trusting the code that builds them.
func inRanges(ranges []reposync.KeyRange, key string) bool {
	for _, r := range ranges {
		if r.Lo != nil && key < string(r.Lo) {
			continue
		}
		if r.Hi != nil && key >= string(r.Hi) {
			continue
		}
		return true
	}
	return false
}

func TestBackfillRanges(t *testing.T) {
	ranges := backfillRanges("")
	// One for place.stream., plus one per non-streamplace collection the
	// firehose accepts.
	want := 1
	for _, nsid := range CollectionFilter {
		if !strings.HasPrefix(nsid, placeStreamPrefix) {
			want++
		}
	}
	require.Len(t, ranges, want)

	inRange := func(key string) bool { return inRanges(ranges, key) }
	require.True(t, inRange("place.stream.chat.message/3l"))
	require.True(t, inRange("place.stream.live.recommendations/self"))
	require.True(t, inRange("app.bsky.actor.profile/self"))
	require.True(t, inRange("app.bsky.feed.post/3l"))
	require.False(t, inRange("app.bsky.actor.status/3l"))
	require.False(t, inRange("app.bsky.feed.postgate/3l"))
	require.False(t, inRange("app.bsky.feed.like/3l"))
	require.False(t, inRange("place.strea.thing/3l"))
	require.False(t, inRange("place.streamx.thing/3l"))
	require.False(t, inRange("zzz.example.thing/3l"))

	// Every windowed collection has to be one this node walks in the first
	// place, or windowing it would widen the sweep instead of narrowing it.
	for _, nsid := range windowedCollections {
		require.True(t, inRange(nsid+"/3l"), "windowed collection %s is not in the full ranges", nsid)
	}
}

// TestBackfillSyncClientBackoffHints is a wiring test, and worth having as one:
// the hint registry only does anything if the client the sync path builds its
// xrpc.Client on is the one watching responses. That is easy to undo by writing
// aqhttp.Client at a new call site, and the symptom -- retries guessing at waits
// a host was announcing -- is invisible from anywhere but a busy production
// sweep.
func TestBackfillSyncClientBackoffHints(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "42")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, err := SyncHTTPClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	hint, ok := pdsBackoffHints.Get(srv.URL)
	require.True(t, ok, "the sync client must record what a throttling host asked for")
	require.Equal(t, "retry-after", hint.Source)
	require.WithinDuration(t, time.Now().Add(42*time.Second), hint.Until, 5*time.Second)

	// And it is still the shared client underneath: same transport, so the same
	// connection pool and the same SSRF checks.
	require.Equal(t, aqhttp.Client.Timeout, SyncHTTPClient.Timeout)
	require.NotNil(t, SyncHTTPClient.Transport)
}

// TestBackfillRangesWindowed: with a floor, the high-volume collections are cut
// down to their recent history and everything else stays whole -- including the
// records that sort on either side of the hole cut out of place.stream.
func TestBackfillRangesWindowed(t *testing.T) {
	floor := reposync.TIDForTime(time.Now().Add(-24 * time.Hour))
	older := reposync.TIDForTime(time.Now().Add(-48 * time.Hour))
	newer := reposync.TIDForTime(time.Now().Add(-time.Hour))
	ranges := backfillRanges(floor)
	inRange := func(key string) bool { return inRanges(ranges, key) }

	// The windowed collections keep only what is at or after the floor.
	require.True(t, inRange("place.stream.chat.message/"+floor))
	require.True(t, inRange("place.stream.chat.message/"+newer))
	require.False(t, inRange("place.stream.chat.message/"+older))
	require.True(t, inRange("app.bsky.feed.post/"+newer))
	require.False(t, inRange("app.bsky.feed.post/"+older))
	require.True(t, inRange("app.bsky.graph.follow/"+newer))
	require.False(t, inRange("app.bsky.graph.follow/"+older))
	// A windowed collection is still walked exhaustively, just not all at once:
	// a non-TID rkey lands in whichever window its bytes fall into ("self"
	// sorts above every TID minted this century, so it comes in the first one)
	// and the windows together cover the whole collection either way.
	require.True(t, inRange("place.stream.chat.message/self"))
	require.False(t, inRange("place.stream.chat.message/!oldest"))

	// Everything else is untouched, on both sides of the hole in place.stream.
	require.True(t, inRange("place.stream.chat.gate/3l"))       // sorts before chat.message
	require.True(t, inRange("place.stream.chat.profile/self"))  // sorts after
	require.True(t, inRange("place.stream.live.livestream/3l")) // sorts after
	require.True(t, inRange("app.bsky.actor.profile/self"))
	// Current-state registries stay whole however big they get: a moderation
	// list that arrives hours late is worse than a slow sync.
	require.True(t, inRange("app.bsky.graph.block/"+older))
	require.True(t, inRange("place.stream.key/"+older))
	// And the ranges are still bounded where they were.
	require.False(t, inRange("app.bsky.feed.postgate/3l"))
	require.False(t, inRange("app.bsky.actor.status/3l"))
	require.False(t, inRange("zzz.example.thing/3l"))

	// The walker rejects an inverted or empty range outright, and windowing is
	// the only thing in here that builds a range out of two different strings.
	for _, r := range ranges {
		require.NotNil(t, r.Hi, "no unbounded range should come out of here: %s", r)
		require.Less(t, string(r.Lo), string(r.Hi), "inverted range %s", r)
	}
	// Windowing a collection that has a range of its own (app.bsky.*) replaces
	// that range with a bounded one and changes nothing about the count.
	// Windowing one under place.stream. punches a hole in the middle of that
	// prefix range instead, leaving a piece on either side of it plus the window
	// itself: two more ranges each.
	extra := 0
	for _, nsid := range windowedCollections {
		if strings.HasPrefix(nsid, placeStreamPrefix) {
			extra += 2
		}
	}
	require.Len(t, ranges, len(backfillRanges(""))+extra)
}

// TestWindowRanges: a deepening step reads the windowed collections and nothing
// else.
func TestWindowRanges(t *testing.T) {
	lo := reposync.TIDForTime(time.Now().Add(-7 * 24 * time.Hour))
	hi := reposync.TIDForTime(time.Now().Add(-24 * time.Hour))
	ranges := windowRanges(lo, hi)
	require.Len(t, ranges, len(windowedCollections))
	inRange := func(key string) bool { return inRanges(ranges, key) }

	mid := reposync.TIDForTime(time.Now().Add(-3 * 24 * time.Hour))
	require.True(t, inRange("place.stream.chat.message/"+mid))
	require.True(t, inRange("app.bsky.feed.post/"+mid))
	require.False(t, inRange("place.stream.chat.message/"+hi), "the floor is exclusive above")
	require.False(t, inRange("place.stream.chat.message/"+reposync.TIDForTime(time.Now().Add(-30*24*time.Hour))))
	// Nothing outside the windowed collections is read again.
	require.False(t, inRange("place.stream.chat.profile/self"))
	require.False(t, inRange("app.bsky.actor.profile/self"))

	// The genesis window: open below, so it sweeps up everything left,
	// including rkeys that are not TIDs at all.
	last := windowRanges("", hi)
	require.True(t, inRanges(last, "place.stream.chat.message/!oldest"))
	require.True(t, inRanges(last, "place.stream.chat.message/"+reposync.TIDForTime(time.Unix(0, 0))))
	require.False(t, inRanges(last, "place.stream.chat.message/"+hi))
	require.False(t, inRanges(last, "place.stream.chat.gate/3l"))
}

// TestNextBackfillWindow walks the ladder a repo climbs down, checking that
// each window abuts the last one (no gap, so no record can be skipped) and that
// it terminates.
func TestNextBackfillWindow(t *testing.T) {
	now := time.Now()

	// Nothing recorded: start at the top, open-ended above.
	first := nextBackfillWindow("", now)
	require.Equal(t, reposync.TIDForTime(now.Add(-InitialWindow)), first.Lo)
	require.Empty(t, first.Hi, "a first window has no upper bound")
	require.False(t, first.Genesis)

	// Then each rung, with the floor aging as if the previous window had just
	// finished. Every window starts where the last one ended.
	floor := first.Lo
	var spans []time.Duration
	for range len(backfillSpans) + 4 {
		win := nextBackfillWindow(floor, now)
		require.Equal(t, floor, win.Hi, "a window must pick up exactly where the last one stopped")
		if win.Genesis {
			require.Empty(t, win.Lo, "the last window runs to the start of the collection")
			break
		}
		require.Less(t, win.Lo, win.Hi, "windows must not be inverted")
		spans = append(spans, now.Sub(win.Horizon).Round(time.Hour))
		floor = win.Lo
	}
	require.Equal(t, []time.Duration{
		7 * 24 * time.Hour,
		30 * 24 * time.Hour,
		180 * 24 * time.Hour,
	}, spans, "the ladder after the initial window")
	require.True(t, nextBackfillWindow(floor, now).Genesis, "the ladder terminates")

	// A floor much older than the whole ladder goes straight to the end.
	ancient := reposync.TIDForTime(now.Add(-5 * 365 * 24 * time.Hour))
	require.True(t, nextBackfillWindow(ancient, now).Genesis)

	// A floor from the future (clock skew, a hand-edited row) still produces a
	// usable window rather than an inverted one.
	future := reposync.TIDForTime(now.Add(time.Hour))
	skewed := nextBackfillWindow(future, now)
	require.Less(t, skewed.Lo, skewed.Hi)

	// A watermark that is not a TID at all: one final window, and done.
	require.True(t, nextBackfillWindow("not-a-tid", now).Genesis)
}

func TestSubtractRange(t *testing.T) {
	r := func(lo, hi string) reposync.KeyRange {
		out := reposync.KeyRange{Lo: []byte(lo)}
		if hi != "" {
			out.Hi = []byte(hi)
		}
		return out
	}
	str := func(ranges []reposync.KeyRange) []string {
		out := []string{}
		for _, x := range ranges {
			out = append(out, string(x.Lo)+".."+string(x.Hi))
		}
		return out
	}

	require.Equal(t, []string{"a..b"}, str(subtractRange(r("a", "b"), r("c", "d"))), "disjoint")
	require.Equal(t, []string{"a..b"}, str(subtractRange(r("a", "b"), r("b", "d"))), "abutting")
	require.Equal(t, []string{"a..c", "d..z"}, str(subtractRange(r("a", "z"), r("c", "d"))), "a hole")
	require.Equal(t, []string{"d..z"}, str(subtractRange(r("a", "z"), r("a", "d"))), "cut off the front")
	require.Equal(t, []string{"a..d"}, str(subtractRange(r("a", "z"), r("d", "z"))), "cut off the back")
	require.Empty(t, subtractRange(r("a", "z"), r("a", "z")), "cut swallows the range")
	require.Empty(t, subtractRange(r("b", "c"), r("a", "z")), "cut swallows the range")
}

func backfillTestSynchronizer(t *testing.T, dev *devenv.DevEnv) (*ATProtoSynchronizer, model.Model) {
	t.Helper()
	cli := config.CLI{
		BroadcasterHost: "example.com",
		DBURL:           ":memory:",
		RelayHost:       strings.ReplaceAll(dev.PDSURL, "http://", "ws://"),
		PLCURL:          dev.PLCURL,
		DataDir:         t.TempDir(),
	}
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(context.Background(), &cli, nil, mod)
	require.NoError(t, err)
	return &ATProtoSynchronizer{
		CLI:        &cli,
		StatefulDB: state,
		Model:      mod,
		Bus:        bus.NewBus(),
	}, mod
}

func chatMessageRecord(streamerDID, text string) *placestream.ChatMessage {
	return &placestream.ChatMessage{
		LexiconTypeID: "place.stream.chat.message",
		Text:          text,
		CreatedAt:     time.Now().UTC().Format(util.ISO8601),
		Streamer:      streamerDID,
	}
}

// rawBackfillRecord builds a record for a lexicon nothing in this repo
// generates code for. The reference PDS stores unknown collections verbatim.
func rawBackfillRecord(t *testing.T, typ string, fields map[string]any) glex.Record {
	t.Helper()
	m := map[string]any{"$type": typ}
	for k, v := range fields {
		m[k] = v
	}
	rec, err := glex.RawJSON(m)
	require.NoError(t, err)
	return rec
}

// createBackfillRecord writes one record and returns its MST key
// ("collection/rkey"). rkey may be empty to let the PDS mint a TID.
func createBackfillRecord(t *testing.T, acct *devenv.DevEnvAccount, collection, rkey string, rec glex.Record) string {
	t.Helper()
	in := &comatproto.RepoCreateRecord_Input{
		Collection: collection,
		Repo:       acct.DID,
		Record:     &glex.LexiconTypeDecoder{Val: rec},
	}
	if rkey != "" {
		in.Rkey = &rkey
	}
	out, err := comatproto.RepoCreateRecord(context.Background(), acct.XRPC, in)
	require.NoError(t, err, "creating %s record", collection)
	return collection + "/" + out.Uri[strings.LastIndex(out.Uri, "/")+1:]
}

// wedgeOnGetBlocks runs a hook the first time a sync fetch goes out, which
// lands in exactly the window a concurrent actor can strike in: after
// DeepenRepo's entry check of the row, while its walk is in flight and the
// per-DID lock is held.
type wedgeOnGetBlocks struct {
	base http.RoundTripper
	once sync.Once
	hook func() error
	err  error
}

func (w *wedgeOnGetBlocks) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Path, "com.atproto.sync.getBlocks") {
		w.once.Do(func() { w.err = w.hook() })
	}
	return w.base.RoundTrip(req)
}

// TestDeepenSurvivesMidWalkRepair reproduces a production deadlock. DeepenRepo
// holds the per-DID lock for its whole window walk; a firehose gap detection
// marked the repo for repair mid-walk, blanking Version; the walk's visitor
// then re-entered SyncBlueskyRepoCached for the same DID, whose Version
// short-circuit fell through -- and the re-entry locked the mutex its own
// caller was holding. Forever: the lane and every later action for that user
// queued up behind a lock nothing would release. The in-flight mark is what
// makes the re-entry take the wedged row instead.
//
// It also pins down what happens to the wedge afterwards: the deepen's
// watermark write must not resurrect Version over a repair somebody has
// evidence for.
func TestDeepenSurvivesMidWalkRepair(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	ctx := context.Background()
	atsync, mod := backfillTestSynchronizer(t, dev)

	user := dev.CreateAccount(t)
	createBackfillRecord(t, user, "place.stream.chat.profile", "self", &placestream.ChatProfile{})
	// A chat message three days old: outside the shallow window, so it is the
	// deepening walk -- not the shallow sync -- that indexes it and re-enters.
	oldRkey := reposync.TIDForTime(time.Now().Add(-72 * time.Hour))
	createBackfillRecord(t, user, "place.stream.chat.message", oldRkey,
		chatMessageRecord(user.DID, "from the deep window"))

	_, err := atsync.SyncBlueskyRepoCached(ctx, user.DID)
	require.NoError(t, err)
	synced, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.NotEmpty(t, synced.Version)
	require.False(t, synced.BackfillDone, "the old message leaves history to deepen")

	// From here on the next sync fetch is the deepen's: wedge the repo there,
	// the way a live firehose gap detection does.
	wedge := &wedgeOnGetBlocks{base: SyncHTTPClient.Transport, hook: func() error {
		marked, err := mod.MarkRepoForRepair(ctx, user.DID, synced.Version)
		if err != nil {
			return fmt.Errorf("wedging repo: %w", err)
		}
		if !marked {
			return fmt.Errorf("wedging repo: CAS did not apply")
		}
		return nil
	}}
	SyncHTTPClient.Transport = wedge
	t.Cleanup(func() { SyncHTTPClient.Transport = wedge.base })

	type result struct {
		done bool
		err  error
	}
	got := make(chan result, 1)
	go func() {
		done, _, err := atsync.DeepenRepo(ctx, user.DID)
		got <- result{done, err}
	}()
	select {
	case r := <-got:
		require.NoError(t, r.err)
		require.False(t, r.done, "a wedged repo is not done deepening")
	case <-time.After(30 * time.Second):
		t.Fatal("DeepenRepo deadlocked on its own per-DID lock")
	}
	require.NoError(t, wedge.err)

	wedged, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.Empty(t, wedged.Version, "the repair wedge survives the deepen")
	require.Equal(t, synced.Version, wedged.RepairFrom, "and still knows where the missed span starts")
}

// TestDeepenSurvivesMidWalkRowDeletion is the second trigger of the same
// deadlock, performed in production by an operator: deleting a repo's row was
// the old system's way to force a full resync, and doing it while a deepen
// held the row's lock left the walk's visitor with no row at all -- which
// falls through SyncBlueskyRepoCached's guard (it needs a row to consult) into
// a full sync that locks the mutex its caller holds. The nil-row in-flight
// guard in SyncBlueskyRepo is what turns that into a per-record error the walk
// shrugs off. Afterwards, the delete does what the operator wanted all along:
// the next touch resyncs the repo from scratch.
func TestDeepenSurvivesMidWalkRowDeletion(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	ctx := context.Background()
	atsync, mod := backfillTestSynchronizer(t, dev)

	user := dev.CreateAccount(t)
	createBackfillRecord(t, user, "place.stream.chat.profile", "self", &placestream.ChatProfile{})
	oldRkey := reposync.TIDForTime(time.Now().Add(-72 * time.Hour))
	createBackfillRecord(t, user, "place.stream.chat.message", oldRkey,
		chatMessageRecord(user.DID, "from the deep window"))

	_, err := atsync.SyncBlueskyRepoCached(ctx, user.DID)
	require.NoError(t, err)
	synced, err := mod.GetRepo(user.DID)
	require.NoError(t, err)
	require.NotEmpty(t, synced.Version)
	require.False(t, synced.BackfillDone)

	// Mid-walk, the operator deletes the row to "force a resync".
	wedge := &wedgeOnGetBlocks{base: SyncHTTPClient.Transport, hook: func() error {
		return mod.(*model.DBModel).DB.Exec("DELETE FROM repos WHERE did = ?", user.DID).Error
	}}
	SyncHTTPClient.Transport = wedge
	t.Cleanup(func() { SyncHTTPClient.Transport = wedge.base })

	type result struct {
		done bool
		err  error
	}
	got := make(chan result, 1)
	go func() {
		done, _, err := atsync.DeepenRepo(ctx, user.DID)
		got <- result{done, err}
	}()
	select {
	case r := <-got:
		require.NoError(t, r.err)
		require.False(t, r.done, "a deleted row records nothing")
	case <-time.After(30 * time.Second):
		t.Fatal("DeepenRepo deadlocked on its own per-DID lock")
	}
	require.NoError(t, wedge.err)

	// The lock is free again, so the delete now means what it used to: the
	// next touch resyncs the account from scratch.
	resynced, err := atsync.SyncBlueskyRepoCached(ctx, user.DID)
	require.NoError(t, err)
	require.NotEmpty(t, resynced.Version)
}
