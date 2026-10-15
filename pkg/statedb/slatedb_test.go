package statedb

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/kvfs"
	"stream.place/streamplace/pkg/kvfs/slatedb"
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

// The two halves of the writer-fence test talk through the environment: the
// test runs this binary again, once per role, because both processes have to be
// alive at the same time for the second one's open to fence the first.
const (
	fenceURLEnv     = "STREAMPLACE_FENCE_TEST_URL"
	fenceRoleEnv    = "STREAMPLACE_FENCE_TEST_ROLE"
	fenceProceedEnv = "STREAMPLACE_FENCE_TEST_PROCEED"
)

// TestSlateDBWriterFence pins what a slatedb+ URL does when two nodes share one
// database, which is the reason the guide says object storage is a way to run a
// node without a disk rather than a way to share state between nodes: SlateDB
// has a single writer, and the newest opener takes the role. The second opener
// does not fail — it fences the first, whose next write fails.
func TestSlateDBWriterFence(t *testing.T) {
	stateURL := requireSlateDBURL(t)
	// The first process writes again when this file appears, which the parent
	// creates once the second process has taken the database over. Nothing here
	// waits on a clock, so a slow runner or a slow object store cannot decide
	// the outcome.
	proceed := filepath.Join(t.TempDir(), "proceed")

	// The process that holds the database first.
	hold := fenceChild(t, stateURL, "hold", proceed)
	held := &syncBuffer{}
	hold.Stdout = held
	hold.Stderr = held
	require.NoError(t, hold.Start())
	t.Cleanup(func() {
		if hold.Process != nil {
			_ = hold.Process.Kill()
		}
	})
	require.Eventually(t, func() bool {
		return strings.Contains(held.String(), "first write ok")
	}, 60*time.Second, 25*time.Millisecond, "the first process never wrote: %s", held.String())

	// A second process opens the same database while the first is between
	// writes, and takes the writer role.
	steal := fenceChild(t, stateURL, "steal", proceed)
	stealOut, err := steal.CombinedOutput()
	require.NoError(t, err, "second opener: %s", stealOut)
	require.Contains(t, string(stealOut), "write ok", "the second opener did not take over: %s", stealOut)
	require.NoError(t, os.WriteFile(proceed, nil, 0o600))

	// The first one finds out on its next write rather than at startup, which
	// is the operational shape of the tradeoff: a station with two nodes on one
	// database gets write failures from the older node, not a refusal to start.
	// The message ("detected newer DB client") is SlateDB's, so the assertion
	// is on the shape of it: the older writer is closed out, not told it fenced
	// itself.
	require.NoError(t, hold.Wait())
	require.Contains(t, held.String(), "second write failed", "the first writer was not fenced: %s", held.String())
	lower := strings.ToLower(held.String())
	require.True(t, strings.Contains(lower, "newer") || strings.Contains(lower, "fenced"),
		"the first writer's error does not name a fence: %s", held.String())
}

// TestSlateDBWriterFenceChild is one of the two processes TestSlateDBWriterFence
// needs. On its own it does nothing.
func TestSlateDBWriterFenceChild(t *testing.T) {
	url := os.Getenv(fenceURLEnv)
	if url == "" {
		t.Skip("run by TestSlateDBWriterFence, not on its own")
	}
	st, err := slatedb.Open(url, slatedb.Options{})
	if err != nil {
		fmt.Printf("open failed: %v\n", err)
		t.FailNow()
	}
	defer func() { _ = st.Close() }()
	put := func(key, value string) error {
		if err := st.Write([]kvfs.Mutation{{Key: []byte(key), Value: []byte(value)}}); err != nil {
			return err
		}
		return st.Flush()
	}

	switch role := os.Getenv(fenceRoleEnv); role {
	case "hold":
		if err := put("hold-1", "first"); err != nil {
			fmt.Printf("first write failed: %v\n", err)
			t.FailNow()
		}
		fmt.Println("first write ok")
		// The other process takes the writer role before this one writes
		// again, and the parent says when it has — a timing window here would
		// fail the assertion on a loaded machine even though the database
		// fenced correctly.
		if err := waitForSignal(os.Getenv(fenceProceedEnv), 60*time.Second); err != nil {
			fmt.Printf("no signal to write again: %v\n", err)
			t.FailNow()
		}
		if err := put("hold-2", "second"); err != nil {
			fmt.Printf("second write failed: %v\n", err)
			return
		}
		fmt.Println("second write ok")
	case "steal":
		if err := put("steal-1", "stolen"); err != nil {
			fmt.Printf("steal write failed: %v\n", err)
			t.FailNow()
		}
		fmt.Println("steal write ok")
	default:
		fmt.Printf("unknown role %q\n", role)
		t.FailNow()
	}
}

// waitForSignal returns once path exists, or an error after timeout.
func waitForSignal(path string, timeout time.Duration) error {
	if path == "" {
		return fmt.Errorf("no signal path configured")
	}
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s did not appear within %s", path, timeout)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// fenceChild is a fence test process: this binary, running only the child test,
// with a role and a database to share. The child does not start the suite's
// services — the parent is already running them.
func fenceChild(t *testing.T, stateURL, role, proceed string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestSlateDBWriterFenceChild")
	env := []string{}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "STREAMPLACE_TEST_RUSTFS_COMMAND=") ||
			strings.HasPrefix(e, "STREAMPLACE_TEST_POSTGRES_COMMAND=") {
			continue
		}
		env = append(env, e)
	}
	cmd.Env = append(env,
		fenceURLEnv+"="+stateURL,
		fenceRoleEnv+"="+role,
		fenceProceedEnv+"="+proceed)
	return cmd
}

// syncBuffer collects a child process's output for a test that reads it while
// the process is still writing it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
