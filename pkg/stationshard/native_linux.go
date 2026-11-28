//go:build linux && stationslate

package stationshard

/*
#cgo CFLAGS: -I${SRCDIR}/../../rust/station-slate
#cgo !static LDFLAGS: -lstation_slate
#cgo static LDFLAGS: -lstreamplace_native
#include "station_slate.h"
*/
import "C"

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"unsafe"
)

var errNativeClosed = errors.New("station SlateDB handle closed")

type nativeEngine struct {
	mu  sync.RWMutex
	ptr *C.station_slate_engine
}

type nativeDB struct {
	mu  sync.RWMutex
	ptr *C.station_slate_db
}

func openEngine(cfg Config) (engine, error) {
	s3 := cfg.S3
	var ptr *C.station_slate_engine
	status := C.station_slate_engine_new(
		nativeString(s3.Endpoint), nativeString(s3.Bucket), nativeString(s3.AccessKeyID),
		nativeString(s3.SecretAccessKey), nativeString(s3.Region), &ptr,
	)
	if err := nativeError("configure S3", status); err != nil {
		return nil, err
	}
	return &nativeEngine{ptr: ptr}, nil
}

func (e *nativeEngine) Epoch(path string) (uint64, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.ptr == nil {
		return 0, errNativeClosed
	}
	var epoch C.uint64_t
	status := C.station_slate_epoch(e.ptr, nativeString(path), &epoch)
	return uint64(epoch), nativeError("read writer epoch", status)
}

func (e *nativeEngine) Open(path string, expectedEpoch uint64) (database, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.ptr == nil {
		return nil, errNativeClosed
	}
	var ptr *C.station_slate_db
	status := C.station_slate_open(e.ptr, nativeString(path), C.uint64_t(expectedEpoch), &ptr)
	if err := nativeError("open writer", status); err != nil {
		return nil, err
	}
	return &nativeDB{ptr: ptr}, nil
}

func (e *nativeEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ptr != nil {
		C.station_slate_engine_free(e.ptr)
		e.ptr = nil
	}
	return nil
}

func (db *nativeDB) Get(key []byte) ([]byte, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if db.ptr == nil {
		return nil, errNativeClosed
	}
	var value C.station_slate_value
	status := C.station_slate_get(db.ptr, nativeBytes(key), &value)
	if err := nativeError("get", status); err != nil {
		return nil, err
	}
	defer C.station_slate_value_free(value.owner)
	return bytes.Clone(unsafe.Slice((*byte)(unsafe.Pointer(value.data.data)), int(value.data.len))), nil
}

func (db *nativeDB) Put(key, value []byte) error {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if db.ptr == nil {
		return errNativeClosed
	}
	return nativeError("put durably", C.station_slate_put(db.ptr, nativeBytes(key), nativeBytes(value)))
}

func (db *nativeDB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.ptr == nil {
		return nil
	}
	status := C.station_slate_close(db.ptr)
	db.ptr = nil
	return nativeError("close writer", status)
}

func nativeString(s string) C.station_slate_slice {
	return nativeSlice(unsafe.StringData(s), len(s))
}

func nativeBytes(b []byte) C.station_slate_slice {
	return nativeSlice(unsafe.SliceData(b), len(b))
}

func nativeSlice(data *byte, n int) C.station_slate_slice {
	return C.station_slate_slice{data: (*C.uint8_t)(unsafe.Pointer(data)), len: C.size_t(n)}
}

func nativeError(op string, status C.station_slate_status) error {
	switch status {
	case C.STATION_SLATE_OK:
		return nil
	case C.STATION_SLATE_NOT_FOUND:
		return ErrNotFound
	case C.STATION_SLATE_FENCED:
		return fmt.Errorf("station SlateDB %s: %w", op, ErrFenced)
	case C.STATION_SLATE_INVALID:
		return fmt.Errorf("station SlateDB %s: invalid input", op)
	default:
		return fmt.Errorf("station SlateDB %s failed", op)
	}
}
