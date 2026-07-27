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
		err = walker.WalkRanges(ctx, h.Root, backfillRanges(), func(path string, rcid cid.Cid, rec []byte) error {
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

	var rev, root string
	err = untilNoErrors(t, func() error {
		var err error
		rev, root, err = atsync.backfillRepo(ctx, ident, &xrpc.Client{Host: proxy.URL, Client: &aqhttp.Client})
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
	require.NotEmpty(t, rev, "the legacy path still reports the commit rev")
	require.Empty(t, root, "the legacy path has no verified MST root to record")
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

func TestBackfillRanges(t *testing.T) {
	ranges := backfillRanges()
	// One for place.stream., plus one per non-streamplace collection the
	// firehose accepts.
	want := 1
	for _, nsid := range CollectionFilter {
		if !strings.HasPrefix(nsid, placeStreamPrefix) {
			want++
		}
	}
	require.Len(t, ranges, want)

	inRange := func(key string) bool {
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
