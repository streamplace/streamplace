package atproto

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	indigoatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/repo"
	glex "github.com/streamplace/glex/runtime"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/devenv"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

func indexingSession(did, handle string) *oatproxy.OAuthSession {
	expires := time.Now().UTC().Add(time.Hour)
	return &oatproxy.OAuthSession{
		DID: did, Handle: handle, DownstreamDPoPJKT: "jkt-" + did,
		DownstreamAccessToken: "local", UpstreamAccessToken: "station",
		UpstreamAccessTokenExp: &expires, UpstreamRefreshToken: "refresh",
	}
}

func TestUpstreamIndexingRejectsUnrelatedNetworkWork(t *testing.T) {
	ctx := t.Context()
	atsync, mod, _ := offlineSynchronizer(t)
	atsync.CLI.UpstreamStation = "did:web:station.test"
	var requests atomic.Int64
	pds := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer pds.Close()
	oldClient := SyncHTTPClient
	SyncHTTPClient = pds.Client()
	t.Cleanup(func() { SyncHTTPClient = oldClient })
	const did = "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	dir := identity.NewMockDirectory()
	dir.Insert(identity.Identity{
		DID: syntax.DID(did), Handle: syntax.Handle("unrelated.test"),
		Services: map[string]identity.ServiceEndpoint{
			"atproto_pds":     {Type: "AtprotoPersonalDataServer", URL: pds.URL},
			"atproto_labeler": {Type: "AtprotoLabeler", URL: pds.URL},
		},
	})
	atsync.PLCDirectory, atsync.CachedPLCDirectory = &dir, &dir
	// This repo was indexed before switching from station/server mode. A
	// cached row, even a complete one, does not make its owner a local user.
	require.NoError(t, mod.UpdateRepo(&model.Repo{
		DID: did, Handle: "unrelated.test", PDS: pds.URL,
		Version: "3lprev0000000", BackfillFloor: "3lprev0000000",
	}))
	require.NoError(t, atsync.StatefulDB.AddRepo(did))
	operations := []struct {
		name string
		run  func() error
	}{
		{"explicit DID", func() error { _, err := atsync.SyncBlueskyRepo(ctx, did, mod); return err }},
		{"explicit handle", func() error { _, err := atsync.SyncBlueskyRepo(ctx, "unrelated.test", mod); return err }},
		{"cached DID", func() error { _, err := atsync.SyncBlueskyRepoCached(ctx, did); return err }},
		{"cached handle", func() error { _, err := atsync.SyncBlueskyRepoCached(ctx, "unrelated.test"); return err }},
		{"history", func() error { _, _, err := atsync.DeepenRepo(ctx, did); return err }},
		{"head", func() error { _, _, err := atsync.headRev(ctx, did); return err }},
		{"identity", func() error { _, err := atsync.RefreshIdentity(ctx, did); return err }},
		{"verifier seed", func() error { return atsync.seedVerifier(ctx, did) }},
		{"label seed", func() error { return atsync.seedLabels(ctx, did, []string{"verified"}) }},
		{"label firehose", func() error { return atsync.StartLabelerFirehoseRetry(ctx, did) }},
		{"record event", func() error {
			return atsync.handleCreateUpdate(ctx, did, syntax.RecordKey("key"), nil, "", syntax.NSID("place.stream.chat.message"), false, false, "")
		}},
	}
	for _, op := range operations {
		t.Run(op.name, func(t *testing.T) {
			require.ErrorIs(t, op.run(), ErrRepoNotLocal)
			require.Zero(t, requests.Load(), "unauthorized work must never reach the PDS")
		})
	}
	// A filtered firehose commit would ordinarily advance the old repo's rev;
	// unrelated events must not mutate its index or schedule repair work.
	atsync.handleCommitEventOps(ctx, commitEvent(did, "3lprev0000000", "3lprev0000001"))
	atsync.handleIdentityEventOps(ctx, &indigoatproto.SyncSubscribeRepos_Identity{Did: did})
	stored, err := mod.GetRepo(did)
	require.NoError(t, err)
	require.Equal(t, "3lprev0000000", stored.Version)
	candidates, err := atsync.sweepCandidates(ctx)
	require.NoError(t, err)
	require.Empty(t, candidates)
	items, _, err := atsync.deepenCandidates(ctx)
	require.NoError(t, err)
	require.Empty(t, items)
	atsync.feedUnresolved(ctx, []sweepItem{{DID: did}}, func(sweepItem) { t.Error("unauthorized candidate was scheduled") })
	require.NoError(t, atsync.Sweep(ctx))
	require.Zero(t, requests.Load())

	// Persisting the completed login immediately changes the same gates,
	// without updating an in-memory allowlist or an indexed repo set.
	session := indexingSession(did, "unrelated.test")
	require.NoError(t, atsync.StatefulDB.CreateOAuthSession(session.DownstreamDPoPJKT, session))
	_, err = atsync.SyncBlueskyRepoCached(ctx, "unrelated.test")
	require.NoError(t, err)
	require.NoError(t, mod.UpdateRepoIdentity(did, "renamed.test", pds.URL))
	_, err = atsync.SyncBlueskyRepoCached(ctx, "renamed.test")
	require.NoError(t, err, "an indexed local user's handle may change before their OAuth session's handle")
	_, _, err = atsync.headRev(ctx, did)
	require.Error(t, err, "the test PDS intentionally rejects the request")
	require.NotErrorIs(t, err, ErrRepoNotLocal)
	require.EqualValues(t, 1, requests.Load(), "an authorized head check reaches the real HTTP server")
	candidates, err = atsync.sweepCandidates(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{did}, candidates)
	now := time.Now().UTC()
	require.NoError(t, atsync.StatefulDB.DB.Model(&oatproxy.OAuthSession{}).
		Where("repo_did = ?", did).Update("downstream_revoked_at", now).Error)
	_, err = atsync.SyncBlueskyRepoCached(ctx, did)
	require.ErrorIs(t, err, ErrRepoNotLocal)
	candidates, err = atsync.sweepCandidates(ctx)
	require.NoError(t, err)
	require.Empty(t, candidates)
	require.EqualValues(t, 1, requests.Load())
}

func TestUpstreamIndexingEligibilitySurvivesRestart(t *testing.T) {
	ctx := context.Background()
	cli := &config.CLI{
		DBURL:           "sqlite://" + filepath.Join(t.TempDir(), "state.sqlite"),
		UpstreamStation: "did:web:station.test",
	}
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(ctx, cli, nil, mod)
	require.NoError(t, err)
	const local = "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	const unrelated = "did:plc:bbbbbbbbbbbbbbbbbbbbbbbb"
	session := indexingSession(local, "local.test")
	// Pending PAR/authorization alone cannot let a public caller scrape a DID.
	session.DownstreamAccessToken = ""
	session.DownstreamAuthorizationCode = "pending-code"
	require.NoError(t, state.CreateOAuthSession(session.DownstreamDPoPJKT, session))
	atsync := &ATProtoSynchronizer{CLI: cli, StatefulDB: state, Model: mod}
	_, err = atsync.indexingDID(ctx, local)
	require.ErrorIs(t, err, ErrRepoNotLocal)
	// The successful local token exchange persists before its initial sync.
	session.DownstreamAccessToken = "local-token"
	require.NoError(t, state.UpdateOAuthSession(session.DownstreamDPoPJKT, session))
	require.NoError(t, state.AddRepo(unrelated))
	require.NoError(t, mod.UpdateRepo(syncedRepo(unrelated, "3lprev0000000")))
	sqlDB, err := state.DB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	state, err = statedb.MakeDB(ctx, cli, nil, mod)
	require.NoError(t, err)
	sqlDB, err = state.DB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	atsync = &ATProtoSynchronizer{CLI: cli, StatefulDB: state, Model: mod}
	did, err := atsync.indexingDID(ctx, "local.test")
	require.NoError(t, err)
	require.Equal(t, local, did)
	candidates, err := atsync.sweepCandidates(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{local}, candidates, "a persisted completed login is boot-eligible even if its initial sync never ran")
	_, err = atsync.indexingDID(ctx, unrelated)
	require.ErrorIs(t, err, ErrRepoNotLocal, "old server-mode index rows must not survive as indexing permissions")
}

type indexingRequestCounter struct {
	base     http.RoundTripper
	requests atomic.Int64
}

func (c *indexingRequestCounter) RoundTrip(req *http.Request) (*http.Response, error) {
	c.requests.Add(1)
	return c.base.RoundTrip(req)
}

// A real PDS CAR carrying a chat record from an unrelated account must never
// cause discovery. A completed local session, in contrast, enables a real
// verified backfill without replacing the indexed repo's actual PDS endpoint.
func TestUpstreamIndexingRealPDSAndUnrelatedFirehose(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	ctx := t.Context()
	atsync, mod := backfillTestSynchronizer(t, dev)
	atsync.CLI.UpstreamStation = "did:web:station.test"
	local, outsider := dev.CreateAccount(t), dev.CreateAccount(t)
	createBackfillRecord(t, local, "place.stream.chat.message", "", chatMessageRecord(local.DID, "local history"))
	path := createBackfillRecord(t, outsider, "place.stream.chat.message", "", chatMessageRecord(local.DID, "unrelated sender"))
	blocks, err := comatproto.SyncGetRepo(ctx, outsider.XRPC, outsider.DID, "")
	require.NoError(t, err)
	car, err := repo.ReadRepoFromCar(ctx, bytes.NewReader(blocks))
	require.NoError(t, err)
	recordCID, _, err := car.GetRecordBytes(ctx, path)
	require.NoError(t, err)
	wire := &comatproto.SyncSubscribeRepos_Commit{
		Repo: outsider.DID, Blocks: blocks, Rev: car.SignedCommit().Rev,
		Commit: glex.Link(recordCID), Time: time.Now().UTC().Format(time.RFC3339),
		Ops: []comatproto.SyncSubscribeRepos_RepoOp{{
			Action: "create", Path: path, Cid: glex.Link(recordCID),
		}},
	}
	var buf bytes.Buffer
	require.NoError(t, wire.MarshalCBOR(&buf))
	var evt indigoatproto.SyncSubscribeRepos_Commit
	require.NoError(t, evt.UnmarshalCBOR(bytes.NewReader(buf.Bytes())))

	counter := &indexingRequestCounter{base: SyncHTTPClient.Transport}
	SyncHTTPClient.Transport = counter
	t.Cleanup(func() { SyncHTTPClient.Transport = counter.base })
	atsync.handleCommitEventOps(ctx, &evt)
	outsiderRepo, err := mod.GetRepo(outsider.DID)
	require.NoError(t, err)
	require.Nil(t, outsiderRepo)
	require.Zero(t, counter.requests.Load())
	_, err = atsync.SyncBlueskyRepoCached(ctx, local.DID)
	require.ErrorIs(t, err, ErrRepoNotLocal)
	require.Zero(t, counter.requests.Load())

	session := indexingSession(local.DID, local.Handle)
	session.PDSUrl = "https://station.test"
	require.NoError(t, atsync.StatefulDB.CreateOAuthSession(session.DownstreamDPoPJKT, session))
	indexed, err := atsync.SyncBlueskyRepoCached(ctx, local.DID)
	require.NoError(t, err)
	require.NotEmpty(t, indexed.Version)
	require.Equal(t, dev.PDSURL, indexed.PDS)
	require.Positive(t, counter.requests.Load(), "completed local login permits actual PDS sync requests")
	before := counter.requests.Load()
	atsync.handleCommitEventOps(ctx, &evt)
	outsiderRepo, err = mod.GetRepo(outsider.DID)
	require.NoError(t, err)
	require.Nil(t, outsiderRepo)
	require.Equal(t, before, counter.requests.Load())
	messages, err := mod.MostRecentChatMessages(local.DID, time.Time{})
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, "local history", messages[0].Record.Val.(*placestream.ChatMessage).Text)
}
