package statedb

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"gorm.io/driver/postgres"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
)

var postgresURL string

func TestMain(m *testing.M) {
	postgresCommand := os.Getenv("STREAMPLACE_TEST_POSTGRES_COMMAND")
	postgresURL = os.Getenv("STREAMPLACE_TEST_POSTGRES_URL")
	if postgresCommand != "" {
		// Start postgres process
		fmt.Printf("Starting postgres process with command: %s\n", postgresCommand)
		cmd := exec.Command("bash", "-c", postgresCommand)
		err := cmd.Start()
		if err != nil {
			fmt.Printf("Failed to start postgres: %v\n", err)
			os.Exit(1)
		}

		// Give postgres time to start up
		time.Sleep(2 * time.Second)

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
