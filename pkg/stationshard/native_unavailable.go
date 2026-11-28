//go:build !linux || !cgo || !stationslate

package stationshard

import "errors"

// Single-node and cross-platform builds do not link a second native runtime.
// Selecting shared station state on such a binary must never fall back to a
// writable local database.
func openEngine(Config) (engine, error) {
	return nil, errors.New("station shards require a Linux cgo build with the stationslate tag (make dev-station)")
}
