package statedb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"gorm.io/driver/postgres"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
)

var postgresURL string

// postgresStartupTimeout caps how long we wait for postgres to accept SQL
// before giving up and dumping its logs.
var postgresStartupTimeout = 30 * time.Second

func TestMain(m *testing.M) {
	postgresCommand := os.Getenv("STREAMPLACE_TEST_POSTGRES_COMMAND")
	postgresURL = os.Getenv("STREAMPLACE_TEST_POSTGRES_URL")
	if postgresCommand != "" {
		// Start postgres process. Capture stdout+stderr so a startup
		// FATAL (which exits before ever listening on a port) is not
		// silently discarded — without this, tests just fail with a
		// misleading "connection refused" and no clue why postgres died.
		fmt.Printf("Starting postgres process with command: %s\n", postgresCommand)
		cmd := exec.Command("bash", "-c", postgresCommand)
		var pgLog bytes.Buffer
		// Tee to os.Stdout so the logs are also streamed live to the
		// test output, not just surfaced on failure.
		cmd.Stdout = io.MultiWriter(&pgLog, os.Stdout)
		cmd.Stderr = io.MultiWriter(&pgLog, os.Stderr)
		err := cmd.Start()
		if err != nil {
			fmt.Printf("Failed to start postgres: %v\n", err)
			os.Exit(1)
		}

		// Wait for postgres to actually accept connections rather than
		// sleeping a fixed amount. A bare sleep masks startup failures
		// (postgres can FATAL within milliseconds) and is flaky on slow
		// CI runners.
		if err := waitPostgresReady(); err != nil {
			fmt.Printf("postgres did not become ready: %v\n", err)
			fmt.Println("=== captured postgres output ===")
			fmt.Println(pgLog.String())
			fmt.Println("=== end postgres output ===")
			_ = cmd.Process.Kill()
			os.Exit(1)
		}

		// Run tests
		exitCode := m.Run()

		// Clean up postgres process
		if cmd.Process != nil {
			cmd2 := exec.Command("pkill", "postgres")
			err := cmd2.Run()
			if err != nil {
				fmt.Printf("Failed to kill postgres: %v\n", err)
			}
		}

		os.Exit(exitCode)
		return
	}
	os.Exit(m.Run())
}

// waitPostgresReady waits until postgres will actually run SQL, not merely
// accept a TCP connection. During startup and crash recovery the postmaster
// listens while rejecting every connection with FATAL 57P03 ("the database
// system is starting up"), which is exactly the window in which a dial-only
// check let tests start and then fail in MakeDB. It retries with exponential
// backoff, on the maintenance database since the test databases do not exist
// yet, up to postgresStartupTimeout.
func waitPostgresReady() error {
	u, err := url.Parse(postgresURL)
	if postgresURL == "" {
		u = &url.URL{Scheme: "postgresql", Host: "localhost:5432"}
	} else if err != nil {
		return fmt.Errorf("could not parse postgres URL for readiness check: %w", err)
	}
	if u.Host == "" {
		u.Host = "localhost:5432"
	}
	u.Path = "/postgres"
	readyURL := u.String()

	err = retryPostgresConnect(context.Background(), postgresStartupTimeout, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, err := pgx.Connect(ctx, readyURL)
		if err != nil {
			return err
		}
		defer conn.Close(ctx)
		_, err = conn.Exec(ctx, "SELECT 1")
		return err
	})
	if err != nil {
		return fmt.Errorf("postgres not accepting SQL at %s: %w", redactDBURL(readyURL), err)
	}
	return nil
}

func makePostgresURL(t *testing.T) string {
	u, err := url.Parse(postgresURL)
	if err != nil {
		panic(err)
	}
	uu, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	dbName := fmt.Sprintf("test_%s", strings.ReplaceAll(uu.String(), "-", "_"))
	u.Path = fmt.Sprintf("/%s", dbName)
	t.Cleanup(func() {
		u, err := url.Parse(postgresURL)
		if err != nil {
			panic(err)
		}
		u.Path = "/postgres"
		rootDial := postgres.Open(u.String())

		db, err := openDB(rootDial)
		if err != nil {
			t.Logf("Failed to open database: %v", err)
			return
		}

		// Drop the test database
		err = db.Exec(fmt.Sprintf("DROP DATABASE %s", dbName)).Error
		if err != nil {
			t.Logf("Failed to drop test database: %v", err)
		}
	})
	return u.String()
}

var lockRuns = 100
var nodeCount = 25

func TestPostgresLocks(t *testing.T) {
	if postgresURL == "" {
		t.Skip("no postgres url, skipping postgres tests")
		return
	}
	dburl := makePostgresURL(t)
	cli := config.CLI{
		DBURL: dburl,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var g errgroup.Group
	var count atomic.Uint64
	start := make(chan struct{})
	for i := 0; i < nodeCount; i++ {
		mod, err := model.MakeDB(":memory:")
		require.NoError(t, err)
		state, err := MakeDB(ctx, &cli, nil, mod)
		require.NoError(t, err)

		defer func() {
			sqlDB, err := state.DB.DB()
			require.NoError(t, err)
			err = sqlDB.Close()
			require.NoError(t, err)
		}()

		doLock := func() error {
			<-start
			unlock, err := state.GetNamedLock("test")
			require.NoError(t, err)
			defer unlock()
			count.Add(1)
			return nil
		}

		for i := 0; i < lockRuns; i++ {
			g.Go(doLock)
		}
	}
	close(start)

	err := g.Wait()
	require.NoError(t, err)
	require.Equal(t, int(count.Load()), int(uint64(lockRuns*nodeCount)))

}
