package statedb

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

// TestSlateDBState runs the state database out of a real object store, which is
// what a slatedb+ URL buys: no database server, and state that outlives the
// process. It runs wherever the suite has an object store — the one the test
// environment starts (STREAMPLACE_TEST_SLATEDB_URL), or one named by the S3_*
// variables pkg/s3's live tests read:
//
//	set -a; . ~/rustfs.env; go test -count=1 -run TestSlateDBState -v ./pkg/statedb
//
// The bucket is created if it is missing, and the test's own keys are deleted
// when it finishes.
func TestSlateDBState(t *testing.T) {
	stateURL := requireSlateDBURL(t)

	ctx := t.Context()
	cli := &config.CLI{DBURL: stateURL}
	state, err := MakeDB(ctx, cli, nil, nil)
	require.NoError(t, err)
	require.Equal(t, DBTypeSQLite, state.Type)

	// A row in two tables of different shapes: a small key and a blob.
	require.NoError(t, state.AddRepo("did:plc:slatedbstore"))
	value := []byte("state in an object store")
	require.NoError(t, state.PutConfig("slatedb-test", value))

	// The prefix is fresh, so this can only be the row just written.
	repos, err := state.ListRepos(100, 0)
	require.NoError(t, err)
	require.Len(t, repos, 1)

	// Everything the database needs is supposed to be in the store by the time
	// a commit is acknowledged, so the process can let go of it and open it
	// again — which is also what a restart does.
	require.NoError(t, state.Close())
	state, err = MakeDB(ctx, cli, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, state.Close()) }()

	got, err := state.GetConfig("slatedb-test")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, value, got.Value)

	repos, err = state.ListRepos(100, 0)
	require.NoError(t, err)
	require.Len(t, repos, 1)
	require.Equal(t, "did:plc:slatedbstore", repos[0].DID)

	// The rows are supposed to be in the object store, not on a local disk.
	require.NotEmpty(t, listPrefix(t, stateURL))
}

// TestCopyStateSlateDB moves a state database into object storage and back out
// again: copying in opens the store as a target, copying out opens it as a
// source, and each open has to give the store's write claim back before the
// next one can take it.
func TestCopyStateSlateDB(t *testing.T) {
	stateURL := requireSlateDBURL(t)
	ctx := t.Context()
	dir := t.TempDir()
	sourceURL := "sqlite://" + filepath.Join(dir, "source.sqlite")
	backURL := "sqlite://" + filepath.Join(dir, "back.sqlite")

	// A row in two tables of different shapes: a small key and a blob.
	source, err := MakeDB(ctx, &config.CLI{DBURL: sourceURL}, nil, nil)
	require.NoError(t, err)
	require.NoError(t, source.AddRepo("did:plc:copyslatedb"))
	require.NoError(t, source.PutConfig("copy-test", []byte("object storage")))
	require.NoError(t, source.Close())

	reports, err := CopyState(ctx, sourceURL, stateURL, 100)
	require.NoError(t, err)
	inserted := map[string]int64{}
	for _, r := range reports {
		inserted[r.Table] = r.Inserted
	}
	require.Equal(t, int64(1), inserted["repos"])
	require.Equal(t, int64(1), inserted["configs"])

	_, err = CopyState(ctx, stateURL, backURL, 100)
	require.NoError(t, err)

	back, err := MakeDB(ctx, &config.CLI{DBURL: backURL}, nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, back.Close()) }()

	repos, err := back.ListRepos(100, 0)
	require.NoError(t, err)
	require.Len(t, repos, 1)
	require.Equal(t, "did:plc:copyslatedb", repos[0].DID)

	got, err := back.GetConfig("copy-test")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, []byte("object storage"), got.Value)
}
