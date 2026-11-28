//go:build linux && cgo && stationslate

package statedb

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"stream.place/streamplace/internal/stationtest"
	"stream.place/streamplace/pkg/s3"
	"stream.place/streamplace/pkg/stationdb"
)

// TestStationOperationalS3 needs a disposable local S3 bucket; it leaves objects
// under a unique prefix for the fixture to discard.
func TestStationOperationalS3(t *testing.T) {
	cfg := stationtest.Config(t)

	redact := strings.NewReplacer(cfg.SecretAccessKey, "[redacted]", cfg.AccessKeyID, "[redacted]")
	check := func(action string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %s", action, redact.Replace(err.Error()))
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	prefix := "station-operational-test/" + uuid.NewString() + "/"
	var key [32]byte
	_, err := rand.Read(key[:])
	require.NoError(t, err)
	var sentinel [64]byte
	_, err = rand.Read(sentinel[:])
	require.NoError(t, err)

	open := func() (*gorm.DB, func()) {
		t.Helper()
		db, _, err := openStationDB(ctx, cfg, prefix, key[:])
		check("open station DB", err)
		sqlDB, err := db.DB()
		check("get station SQL pool", err)
		closeDB := sync.OnceFunc(func() {
			check("close station SQL pool", sqlDB.Close())
		})
		t.Cleanup(closeDB)
		return db.Session(&gorm.Session{Context: ctx, Logger: logger.Discard}), closeDB
	}
	read := func(db *gorm.DB) (int, []byte) {
		t.Helper()
		var count int
		var secret []byte
		check("read station row", db.Raw("SELECT n, secret FROM station_ops WHERE id = 1").Row().Scan(&count, &secret))
		return count, secret
	}
	const increment = "UPDATE station_ops SET n = n + 1 WHERE id = 1"

	a, closeA := open()
	b, closeB := open()
	check("commit schema and seed row on A", a.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("CREATE TABLE station_ops (id INTEGER PRIMARY KEY, n INTEGER NOT NULL, secret BLOB)").Error; err != nil {
			return err
		}
		return tx.Exec("INSERT INTO station_ops (id, n) VALUES (1, 0)").Error
	}))
	count, secret := read(b)
	require.Zero(t, count)
	require.True(t, secret == nil, "B should see the seed row without a session")

	rollback := errors.New("deliberate station transaction rollback")
	err = b.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("CREATE TABLE station_rollback (id INTEGER PRIMARY KEY)").Error; err != nil {
			return err
		}
		if err := tx.Exec(increment).Error; err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		check("execute transaction before deliberate rollback on B", err)
		t.Fatal("transaction did not return the rollback marker")
	}
	count, _ = read(a)
	require.Zero(t, count, "B's rolled-back increment leaked to A")
	check("read schema on A after rollback on B", a.Raw("SELECT COUNT(*) FROM sqlite_schema WHERE name = 'station_rollback'").Row().Scan(&count))
	require.Zero(t, count, "B's rolled-back DDL leaked to A")

	check("write private session on A", a.Exec("UPDATE station_ops SET secret = ? WHERE id = 1", sentinel[:]).Error)
	_, secret = read(b)
	require.True(t, bytes.Equal(sentinel[:], secret), "already-open B did not see A's private session")

	ready := make(chan struct{}, 2)
	start := make(chan struct{})
	results := make(chan *gorm.DB, 2)
	for _, db := range []*gorm.DB{a, b} {
		go func(db *gorm.DB) {
			ready <- struct{}{}
			<-start
			results <- db.Exec(increment)
		}(db)
	}
	<-ready
	<-ready
	close(start)
	acknowledged := 0
	conflicts := 0
	for _, result := range [2]*gorm.DB{<-results, <-results} {
		err := result.Error
		if errors.Is(err, stationdb.ErrConflict) && !errors.Is(err, stationdb.ErrCommitUnknown) {
			conflicts++
			continue
		}
		check("concurrent increment", err)
		require.EqualValues(t, 1, result.RowsAffected, "acknowledged increment must update exactly one row")
		acknowledged++
	}
	closeA()
	closeB()

	fresh, closeFresh := open()
	count, _ = read(fresh)
	require.Equal(t, acknowledged, count, "durable counter must match exactly the acknowledged writes before any retry")
	for range conflicts {
		result := fresh.Exec(increment)
		check("retry the rejected increment on a fresh client", result.Error)
		require.EqualValues(t, 1, result.RowsAffected)
	}
	closeFresh()

	wrongKey := key
	wrongKey[0] ^= 1
	wrong, _, err := openStationDB(ctx, cfg, prefix, wrongKey[:])
	if err == nil {
		wrong = wrong.Session(&gorm.Session{Context: ctx, Logger: logger.Discard})
		sqlDB, err := wrong.DB()
		check("get wrong-key SQL pool", err)
		// A lazy open must still reject the key when it first reads the schema.
		err = wrong.Raw("SELECT COUNT(*) FROM sqlite_schema").Row().Scan(&count)
		check("close wrong-key SQL pool", sqlDB.Close())
		require.Error(t, err, "wrong AES key could read the schema")
	}

	reopened, closeReopened := open()
	count, secret = read(reopened)
	require.Equal(t, 2, count, "both increment intents must survive a fresh reopen")
	require.True(t, bytes.Equal(sentinel[:], secret), "private session did not survive a fresh reopen")
	closeReopened()

	client := s3.NewClient(cfg)
	objects := awss3.NewListObjectsV2Paginator(client, &awss3.ListObjectsV2Input{
		Bucket: aws.String(cfg.Bucket),
		Prefix: aws.String(prefix),
	})
	inspected := 0
	for objects.HasMorePages() {
		page, err := objects.NextPage(ctx)
		check("list station objects", err)
		for _, object := range page.Contents {
			out, err := client.GetObject(ctx, &awss3.GetObjectInput{Bucket: aws.String(cfg.Bucket), Key: object.Key})
			check("get station object", err)
			body, err := io.ReadAll(out.Body)
			closeErr := out.Body.Close()
			check("read station object", err)
			check("close station object", closeErr)
			require.False(t, bytes.Contains(body, sentinel[:]), "S3 object contains the plaintext private session")
			require.False(t, bytes.Contains(body, []byte("SQLite format 3\x00")), "S3 object contains a plaintext SQLite header")
			inspected++
		}
	}
	require.Positive(t, inspected, "no station objects found under the test prefix")
}
