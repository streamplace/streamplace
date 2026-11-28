package statedb

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"stream.place/streamplace/pkg/s3"
	"stream.place/streamplace/pkg/stationdb"
)

func openStationDB(ctx context.Context, cfg s3.Config, prefix string, key []byte) (*gorm.DB, *stationdb.Store, error) {
	sqlDB, store, err := stationdb.Open(ctx, cfg, prefix, key)
	if err != nil {
		return nil, nil, err
	}
	db, err := gorm.Open(sqlite.New(sqlite.Config{Conn: sqlDB}), &gorm.Config{
		SkipDefaultTransaction: true,
		TranslateError:         true,
		Logger:                 logger.Discard,
		DisableAutomaticPing:   true,
	})
	if err == nil {
		err = sqlDB.PingContext(ctx)
	}
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, err
	}
	return db, store, nil
}

// Only a rejected head PUT proves that a station SQL write did not commit.
func retryStationWrite(ctx context.Context, attempt int, err error) bool {
	if attempt >= 9 || !errors.Is(err, stationdb.ErrConflict) || errors.Is(err, stationdb.ErrCommitUnknown) {
		return false
	}
	delay := (5 * time.Millisecond) << min(attempt, 4)
	timer := time.NewTimer(delay + rand.N(delay))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return ctx.Err() == nil
	}
}
