package acme

import (
	"context"
	"io/fs"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
)

func newStorage(t *testing.T) *Storage {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cli := &config.CLI{BroadcasterHost: "node.example", DBURL: ":memory:"}
	cli.DataDir = t.TempDir()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(ctx, cli, nil, mod)
	require.NoError(t, err)
	return NewStorage(state)
}

func TestStorageRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newStorage(t)

	_, err := s.Load(ctx, "missing")
	require.ErrorIs(t, err, fs.ErrNotExist)
	require.False(t, s.Exists(ctx, "missing"))
	_, err = s.Stat(ctx, "missing")
	require.ErrorIs(t, err, fs.ErrNotExist)

	require.NoError(t, s.Store(ctx, "certificates/ca/example.com/example.com.crt", []byte("cert")))
	require.NoError(t, s.Store(ctx, "certificates/ca/example.com/example.com.key", []byte("key")))
	require.NoError(t, s.Store(ctx, "certificates/ca/other.example/other.example.crt", []byte("cert2")))
	require.NoError(t, s.Store(ctx, "acme/ca/users/me/me.json", []byte("{}")))

	v, err := s.Load(ctx, "certificates/ca/example.com/example.com.crt")
	require.NoError(t, err)
	require.Equal(t, "cert", string(v))

	// overwrite
	require.NoError(t, s.Store(ctx, "certificates/ca/example.com/example.com.crt", []byte("cert-v2")))
	v, err = s.Load(ctx, "certificates/ca/example.com/example.com.crt")
	require.NoError(t, err)
	require.Equal(t, "cert-v2", string(v))

	// directories exist implicitly
	require.True(t, s.Exists(ctx, "certificates"))
	require.True(t, s.Exists(ctx, "certificates/ca/example.com"))
	require.False(t, s.Exists(ctx, "certificates/ca/example"), "prefix must end at a path boundary")

	// Stat: terminal vs directory
	info, err := s.Stat(ctx, "certificates/ca/example.com/example.com.crt")
	require.NoError(t, err)
	require.True(t, info.IsTerminal)
	require.Equal(t, int64(len("cert-v2")), info.Size)
	require.WithinDuration(t, time.Now(), info.Modified, time.Minute)
	info, err = s.Stat(ctx, "certificates/ca")
	require.NoError(t, err)
	require.False(t, info.IsTerminal)
	require.WithinDuration(t, time.Now(), info.Modified, time.Minute)

	// List, non-recursive: immediate children only, directories included
	keys, err := s.List(ctx, "certificates/ca", false)
	require.NoError(t, err)
	require.Equal(t, []string{"certificates/ca/example.com", "certificates/ca/other.example"}, keys)
	keys, err = s.List(ctx, "certificates/ca/example.com", false)
	require.NoError(t, err)
	require.Equal(t, []string{"certificates/ca/example.com/example.com.crt", "certificates/ca/example.com/example.com.key"}, keys)

	// List, recursive: everything below, with the implicit directories
	keys, err = s.List(ctx, "certificates", true)
	require.NoError(t, err)
	require.Equal(t, []string{
		"certificates/ca",
		"certificates/ca/example.com",
		"certificates/ca/example.com/example.com.crt",
		"certificates/ca/example.com/example.com.key",
		"certificates/ca/other.example",
		"certificates/ca/other.example/other.example.crt",
	}, keys)

	// Root listing
	keys, err = s.List(ctx, "", false)
	require.NoError(t, err)
	require.Equal(t, []string{"acme", "certificates"}, keys)

	// Delete a directory removes the subtree and nothing else
	require.NoError(t, s.Delete(ctx, "certificates/ca/example.com"))
	require.False(t, s.Exists(ctx, "certificates/ca/example.com"))
	require.True(t, s.Exists(ctx, "certificates/ca/other.example/other.example.crt"))
	require.NoError(t, s.Delete(ctx, "certificates/ca/example.com"), "deleting what is already gone is not an error")

	// LIKE metacharacters in keys are literal
	require.NoError(t, s.Store(ctx, "weird/a_b%c", []byte("x")))
	require.True(t, s.Exists(ctx, "weird/a_b%c"))
	require.False(t, s.Exists(ctx, "weird/aXb"))
}

func TestStorageLock(t *testing.T) {
	ctx := context.Background()
	s := newStorage(t)

	require.NoError(t, s.Lock(ctx, "issue_cert_example.com"))
	require.Error(t, s.Unlock(ctx, "never-locked"))

	// A second holder waits until the first releases.
	acquired := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(t, s.Lock(ctx, "issue_cert_example.com"))
		close(acquired)
		require.NoError(t, s.Unlock(ctx, "issue_cert_example.com"))
	}()
	select {
	case <-acquired:
		t.Fatal("lock acquired while still held")
	case <-time.After(200 * time.Millisecond):
	}
	require.NoError(t, s.Unlock(ctx, "issue_cert_example.com"))
	select {
	case <-acquired:
	case <-time.After(5 * time.Second):
		t.Fatal("waiter never acquired the lock")
	}
	wg.Wait()

	// Locks are independent per name.
	require.NoError(t, s.Lock(ctx, "a"))
	require.NoError(t, s.Lock(ctx, "b"))
	require.NoError(t, s.Unlock(ctx, "a"))
	require.NoError(t, s.Unlock(ctx, "b"))
}

func TestDomainsAndCA(t *testing.T) {
	cli := &config.CLI{BroadcasterHost: "Example.com", ServerHost: "node1.example.com:443", ACMEDomains: []string{" rtmp.example.com ", "example.com"}}
	require.Equal(t, []string{"example.com", "node1.example.com", "rtmp.example.com"}, Domains(cli))

	cli = &config.CLI{BroadcasterHost: "example.com", ServerHost: "example.com"}
	require.Equal(t, []string{"example.com"}, Domains(cli), "one cert when station and server identities coincide")

	require.Equal(t, "https://acme-v02.api.letsencrypt.org/directory", ResolveCA(""))
	require.Equal(t, "https://acme-v02.api.letsencrypt.org/directory", ResolveCA("letsencrypt"))
	require.Equal(t, "https://acme-staging-v02.api.letsencrypt.org/directory", ResolveCA("staging"))
	require.Equal(t, "https://ca.internal/dir", ResolveCA("https://ca.internal/dir"))

	_, err := New(context.Background(), &config.CLI{BroadcasterHost: "127.0.0.1"}, nil)
	require.Error(t, err, "IPs cannot get public certificates")
	_, err = New(context.Background(), &config.CLI{}, nil)
	require.Error(t, err, "nothing to manage")
}
