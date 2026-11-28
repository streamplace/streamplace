//go:build !linux || !cgo || !stationslate

package stationdb

import (
	"context"
	"database/sql"
	"errors"

	"stream.place/streamplace/pkg/s3"
)

var errUnsupported = errors.New("stationdb is unsupported: requires Linux, cgo (CGO_ENABLED=1), and the stationslate build tag")

func Open(ctx context.Context, cfg s3.Config, prefix string, key []byte) (*sql.DB, *Store, error) {
	return nil, nil, errUnsupported
}

func Compact(ctx context.Context, cfg s3.Config, sourcePrefix, destinationPrefix string, key []byte) (int, error) {
	return 0, errUnsupported
}
