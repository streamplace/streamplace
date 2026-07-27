package atproto

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/util"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/statedb"
)

// offlineSynchronizer builds a synchronizer with no network anywhere: the PLC
// URL points at a port nothing listens on, so any test that accidentally
// resolves an identity fails fast and loudly instead of reaching the internet.
func offlineSynchronizer(t *testing.T) (*ATProtoSynchronizer, model.Model, *bus.Bus) {
	t.Helper()
	cli := config.CLI{
		BroadcasterHost: "example.com",
		DBURL:           ":memory:",
		DataDir:         t.TempDir(),
		PLCURL:          "http://127.0.0.1:1",
	}
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(context.Background(), &cli, nil, mod)
	require.NoError(t, err)
	b := bus.NewBus()
	return &ATProtoSynchronizer{
		CLI:        &cli,
		StatefulDB: state,
		Model:      mod,
		Bus:        b,
	}, mod, b
}

// TestHandleCreateUpdateRedelivery is the whole point of the idempotent
// indexer: the same chat message delivered twice -- a cursor replay, a re-walk,
// two relays carrying the same commit -- must be indexed once and must reach
// the chat bus once. A second fanout would show the message twice in every
// viewer's chat.
func TestHandleCreateUpdateRedelivery(t *testing.T) {
	ctx := context.Background()
	atsync, mod, b := offlineSynchronizer(t)

	did := "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	// A finished repo row: SyncBlueskyRepoCached short-circuits on it, so
	// indexing never goes near the network.
	require.NoError(t, mod.UpdateRepo(&model.Repo{
		DID:     did,
		Handle:  "chatter.test",
		PDS:     "http://127.0.0.1:1",
		Version: "3lrev00000000",
	}))

	rec := &placestream.ChatMessage{
		LexiconTypeID: "place.stream.chat.message",
		Text:          "hello twice",
		CreatedAt:     time.Now().UTC().Format(util.ISO8601),
		Streamer:      did,
	}
	var buf bytes.Buffer
	require.NoError(t, rec.MarshalCBOR(&buf))
	recCBOR := buf.Bytes()
	rcid, err := spid.GetCID(rec)
	require.NoError(t, err)

	ch := b.Subscribe(did)
	defer b.Unsubscribe(did, ch)
	var mu sync.Mutex
	var published []bus.Message
	go func() {
		for msg := range ch {
			mu.Lock()
			published = append(published, msg)
			mu.Unlock()
		}
	}()
	countPublished := func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(published)
	}

	collection := syntax.NSID("place.stream.chat.message")
	rkey := syntax.RecordKey("3lmsg000000000")
	index := func() error {
		bs := recCBOR
		return atsync.handleCreateUpdate(ctx, did, rkey, &bs, rcid.String(), collection, false, false)
	}

	require.NoError(t, index())
	require.Eventually(t, func() bool { return countPublished() == 1 }, 5*time.Second, 10*time.Millisecond,
		"the first delivery should reach the chat bus")

	// The redelivery. Same path, same CID, byte-identical record.
	require.NoError(t, index())
	// Give the (asynchronous) publish a chance to be wrong.
	time.Sleep(250 * time.Millisecond)
	require.Equal(t, 1, countPublished(), "a redelivered chat message must not be published again")

	messages, err := mod.MostRecentChatMessages(did)
	require.NoError(t, err)
	require.Len(t, messages, 1, "a redelivered chat message must not be indexed again")
	require.Equal(t, "hello twice", messages[0].Record.Val.(*placestream.ChatMessage).Text)
}

func TestRepoStatusFromError(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, model.RepoStatusOK},
		{
			"deactivated",
			fmt.Errorf("failed to fetch verified head: %w", &xrpc.Error{
				StatusCode: http.StatusBadRequest,
				Wrapped:    &xrpc.XRPCError{ErrStr: "RepoDeactivated", Message: "Repo has been deactivated"},
			}),
			model.RepoStatusDeactivated,
		},
		{
			"not found",
			fmt.Errorf("getLatestCommit: %w", &xrpc.XRPCError{ErrStr: "RepoNotFound", Message: "Could not find repo"}),
			model.RepoStatusNotFound,
		},
		{
			"takendown",
			fmt.Errorf("getRepo: %w", &xrpc.Error{
				StatusCode: http.StatusBadRequest,
				Wrapped:    &xrpc.XRPCError{ErrStr: "RepoTakendown"},
			}),
			model.RepoStatusTakendown,
		},
		{
			// Through echo's default error handler the name arrives at the
			// front of Message with no ErrStr at all.
			"suspended, name only in the message",
			fmt.Errorf("getLatestCommit: %w", &xrpc.Error{
				StatusCode: http.StatusBadRequest,
				Wrapped:    &xrpc.XRPCError{Message: "RepoSuspended: account suspended"},
			}),
			model.RepoStatusSuspended,
		},
		// Everything below says nothing about the account and must stay
		// retryable: parking a repo because its host had a bad minute would
		// take it out of the index until it happened to commit again.
		{"block not found", fmt.Errorf("getBlocks: %w", &xrpc.XRPCError{ErrStr: "BlockNotFound"}), model.RepoStatusOK},
		{"throttled", fmt.Errorf("getBlocks: %w", &xrpc.Error{StatusCode: http.StatusTooManyRequests}), model.RepoStatusOK},
		{"server error", fmt.Errorf("getBlocks: %w", &xrpc.Error{StatusCode: http.StatusInternalServerError}), model.RepoStatusOK},
		{"dns", errors.New("dial tcp: no such host"), model.RepoStatusOK},
		{"ssrf", errors.New("request to private address blocked"), model.RepoStatusOK},
		{"canceled", context.Canceled, model.RepoStatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, repoStatusFromError(tc.err))
		})
	}
}

// TestParkTerminalRepo checks the write half: a terminal failure marks the row
// and hands back a typed error, a transient one leaves everything alone.
func TestParkTerminalRepo(t *testing.T) {
	ctx := context.Background()
	_, mod, _ := offlineSynchronizer(t)

	did := "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	require.NoError(t, mod.UpdateRepo(&model.Repo{DID: did, Version: "3lrev00000000", RootCID: "bafyroot"}))

	transient := fmt.Errorf("walking: %w", &xrpc.Error{StatusCode: http.StatusTooManyRequests})
	require.Nil(t, parkTerminalRepo(ctx, mod, did, transient))
	stored, err := mod.GetRepo(did)
	require.NoError(t, err)
	require.Equal(t, model.RepoStatusOK, stored.Status)

	gone := fmt.Errorf("head: %w", &xrpc.XRPCError{ErrStr: "RepoDeactivated"})
	parked := parkTerminalRepo(ctx, mod, did, gone)
	require.Error(t, parked)
	var terminal *TerminalRepoError
	require.ErrorAs(t, parked, &terminal)
	require.Equal(t, model.RepoStatusDeactivated, terminal.Status)
	require.ErrorIs(t, parked, gone, "the underlying failure stays wrapped")

	stored, err = mod.GetRepo(did)
	require.NoError(t, err)
	require.Equal(t, model.RepoStatusDeactivated, stored.Status)
	require.Equal(t, "3lrev00000000", stored.Version, "whatever we indexed before stays indexed")
	require.Equal(t, "bafyroot", stored.RootCID)
}

// TestSyncBlueskyRepoCachedSkipsTerminal: a parked row is served straight back.
// The PDS and PLC here are ports nothing listens on, so any network attempt
// would surface as an error rather than a returned row.
func TestSyncBlueskyRepoCachedSkipsTerminal(t *testing.T) {
	ctx := context.Background()
	atsync, mod, _ := offlineSynchronizer(t)

	did := "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	require.NoError(t, mod.UpdateRepo(&model.Repo{
		DID:    did,
		PDS:    "http://127.0.0.1:1",
		Status: model.RepoStatusDeactivated,
		// Deliberately no Version: without the status this row is a wedged
		// backfill and would be re-synced immediately.
	}))

	got, err := atsync.SyncBlueskyRepoCached(ctx, did)
	require.NoError(t, err)
	require.Equal(t, did, got.DID)
	require.Equal(t, model.RepoStatusDeactivated, got.Status)

	// Same row without the status: now it must try, and fail.
	require.NoError(t, mod.SetRepoStatus(ctx, did, model.RepoStatusOK))
	_, err = atsync.SyncBlueskyRepoCached(ctx, did)
	require.Error(t, err, "an un-parked placeholder row must actually attempt a sync")
}

// TestMigrateSkipsTerminalRepos: the boot sweep must not spend a request (or a
// log line) on accounts that are gone.
func TestMigrateSkipsTerminalRepos(t *testing.T) {
	ctx := context.Background()
	atsync, mod, _ := offlineSynchronizer(t)

	did := "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	require.NoError(t, mod.UpdateRepo(&model.Repo{DID: did, PDS: "http://127.0.0.1:1"}))
	require.NoError(t, atsync.StatefulDB.AddRepo(did))

	// Control: unparked, this DID resolves nowhere, so the sweep fails on it.
	require.Error(t, atsync.Migrate(ctx), "the only repo in the sweep should have failed")

	require.NoError(t, mod.SetRepoStatus(ctx, did, model.RepoStatusDeactivated))
	require.NoError(t, atsync.Migrate(ctx), "a terminal repo should never be dialed")
}

// TestReviveRepo: a commit proves the account is back.
func TestReviveRepo(t *testing.T) {
	ctx := context.Background()
	atsync, mod, _ := offlineSynchronizer(t)

	did := "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	require.NoError(t, mod.UpdateRepo(&model.Repo{DID: did, Status: model.RepoStatusTakendown, Version: "3lrev00000000"}))

	r, err := mod.GetRepo(did)
	require.NoError(t, err)
	atsync.reviveRepo(ctx, r)
	require.Equal(t, model.RepoStatusOK, r.Status, "the caller's copy is updated too")

	stored, err := mod.GetRepo(did)
	require.NoError(t, err)
	require.Equal(t, model.RepoStatusOK, stored.Status)
	require.Equal(t, "3lrev00000000", stored.Version)

	// A repo that was never parked, and a repo we have never heard of, are both
	// no-ops rather than writes.
	atsync.reviveRepo(ctx, stored)
	atsync.reviveRepo(ctx, nil)
}
