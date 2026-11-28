//go:build linux && cgo && stationslate

package stationdb

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	compactTestSource      = "old-station/operational"
	compactTestDestination = "new-station/operational"
	compactTestOld         = compactTestSource + "/stationdb/v1/"
	compactTestNew         = compactTestDestination + "/stationdb/v1/"
	compactTestFirst       = "pages/10000000000000000000000000000000"
	compactTestSecond      = "pages/20000000000000000000000000000000"
)

type compactTestObject struct {
	body []byte
	etag string
}

type compactTestBucket struct {
	data       map[string]compactTestObject
	revision   uint64
	writes     []string
	beforeGet  func(string)
	afterPut   func(string) error
	listErrors map[string]error
	codec      cipher.AEAD
}

func newCompactTestBucket(t *testing.T) (*compactTestBucket, []byte) {
	t.Helper()
	key := bytes.Repeat([]byte{0x4b}, 32)
	block, err := aes.NewCipher(key)
	require.NoError(t, err)
	codec, err := cipher.NewGCM(block)
	require.NoError(t, err)
	return &compactTestBucket{
		data:       make(map[string]compactTestObject),
		listErrors: make(map[string]error),
		codec:      codec,
	}, key
}

func (b *compactTestBucket) get(_ context.Context, name string) ([]byte, string, error) {
	if b.beforeGet != nil {
		b.beforeGet(name)
	}
	obj, ok := b.data[name]
	if !ok {
		return nil, "", errMissing
	}
	return slices.Clone(obj.body), obj.etag, nil
}

func (b *compactTestBucket) put(_ context.Context, name string, body []byte, match string) (string, error) {
	b.writes = append(b.writes, name)
	if match != "" {
		return "", errors.New("compactor requested a replacing PUT")
	}
	if _, exists := b.data[name]; exists {
		return "", errCAS
	}
	etag := b.store(name, body)
	if b.afterPut != nil {
		if err := b.afterPut(name); err != nil {
			return "", err
		}
	}
	return etag, nil
}

func (b *compactTestBucket) empty(_ context.Context, prefix string) (bool, error) {
	if err := b.listErrors[prefix]; err != nil {
		return false, err
	}
	for name := range b.data {
		if strings.HasPrefix(name, prefix) {
			return false, nil
		}
	}
	return true, nil
}

func (b *compactTestBucket) list(_ context.Context, prefix string, visit func(string) error) error {
	if err := b.listErrors[prefix]; err != nil {
		return err
	}
	for _, name := range slices.Sorted(maps.Keys(b.data)) {
		if strings.HasPrefix(name, prefix) {
			if err := visit(name); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *compactTestBucket) store(name string, body []byte) string {
	b.revision++
	etag := fmt.Sprintf("\"revision-%d\"", b.revision)
	b.data[name] = compactTestObject{body: slices.Clone(body), etag: etag}
	return etag
}

// Seed with the v1 wire layout using ordinary GCM and a deterministic nonce.
func (b *compactTestBucket) seed(name string, plain []byte) {
	var nonce [12]byte
	binary.BigEndian.PutUint64(nonce[4:], b.revision+1)
	b.store(name, b.codec.Seal(nonce[:], nonce[:], plain, []byte(name)))
}

func (b *compactTestBucket) open(t *testing.T, name string) []byte {
	t.Helper()
	obj, ok := b.data[name]
	require.True(t, ok, "missing %s", name)
	require.GreaterOrEqual(t, len(obj.body), 12+b.codec.Overhead())
	plain, err := b.codec.Open(nil, obj.body[:12], obj.body[12:], []byte(name))
	require.NoError(t, err, "authenticate %s", name)
	return plain
}

func compactTestIndex(ids ...byte) []byte {
	index := make([]byte, 16*len(ids))
	for i, id := range ids {
		index[16*i] = id
	}
	return index
}

func compactTestSeedPages(b *compactTestBucket, index []byte) (first, second []byte) {
	first = make([]byte, 4096)
	copy(first, "SQLite format 3\x00")
	binary.BigEndian.PutUint16(first[16:18], 4096)
	second = bytes.Repeat([]byte{0xa5}, 4096)
	b.seed(compactTestOld+"head", index)
	b.seed(compactTestOld+compactTestFirst, first)
	b.seed(compactTestOld+compactTestSecond, second)
	return first, second
}

func TestCompactRawCopiesOnlyReferencedPages(t *testing.T) {
	b, key := newCompactTestBucket(t)
	index := compactTestIndex(0x10, 0x20, 0x20)
	first, second := compactTestSeedPages(b, index)
	b.seed(compactTestOld+"locks/released", nil)
	b.store(compactTestOld+"pages/orphan", []byte("not a valid page"))
	original := maps.Clone(b.data)

	count, err := compactObjects(t.Context(), b, compactTestSource+"/", compactTestDestination, key)
	require.NoError(t, err)
	require.Equal(t, 3, count)
	require.Equal(t, []string{compactTestNew + compactTestFirst, compactTestNew + compactTestSecond, compactTestNew + "head"}, b.writes)
	require.Equal(t, index, b.open(t, compactTestNew+"head"))
	require.Equal(t, first, b.open(t, compactTestNew+compactTestFirst))
	require.Equal(t, second, b.open(t, compactTestNew+compactTestSecond))
	for name, obj := range original {
		require.Equal(t, obj, b.data[name], "changed source object %s", name)
	}
}

func TestCompactRawCopiesAuthenticatedEmptyHead(t *testing.T) {
	b, key := newCompactTestBucket(t)
	b.seed(compactTestOld+"head", nil)

	count, err := compactObjects(t.Context(), b, compactTestSource, compactTestDestination, key)
	require.NoError(t, err)
	require.Zero(t, count)
	require.Empty(t, b.open(t, compactTestNew+"head"))
}

func TestCompactRawRejectsUnsafePrefixes(t *testing.T) {
	b, key := newCompactTestBucket(t)
	b.seed(compactTestOld+"head", nil)
	for _, tc := range []struct{ src, dst, want string }{
		{compactTestSource, compactTestSource + "/", "overlap"},
		{compactTestSource, compactTestSource + "/inner/operational", "overlap"},
		{compactTestSource + "/inner/operational", compactTestSource, "overlap"},
		{compactTestSource, "not-operational", "ending in /operational"},
	} {
		count, err := compactObjects(t.Context(), b, tc.src, tc.dst, key)
		require.ErrorContains(t, err, tc.want)
		require.Zero(t, count)
	}
	require.Empty(t, b.writes)
}

func TestCompactRawRejectsBadSourceOrDestination(t *testing.T) {
	listErr := errors.New("injected list failure")
	for _, tc := range []struct {
		name  string
		alter func(*compactTestBucket, []byte)
		want  string
		cause error
	}{
		{"missing head", func(b *compactTestBucket, _ []byte) { delete(b.data, compactTestOld+"head") }, "read source station database head", errMissing},
		{"wrong key", func(_ *compactTestBucket, key []byte) { key[0] ^= 1 }, "authenticate source station database head", nil},
		{"malformed index", func(b *compactTestBucket, _ []byte) { b.seed(compactTestOld+"head", []byte{1}) }, "page index", nil},
		{"held lock", func(b *compactTestBucket, _ []byte) { b.seed(compactTestOld+"locks/held", make([]byte, 16)) }, "check source station database locks", ErrLockBusy},
		{"malformed lock", func(b *compactTestBucket, _ []byte) { b.seed(compactTestOld+"locks/malformed", []byte{1}) }, "invalid source station database lock marker", nil},
		{"source list", func(b *compactTestBucket, _ []byte) { b.listErrors[compactTestOld+"locks/"] = listErr }, "check source station database locks", listErr},
		{"destination list", func(b *compactTestBucket, _ []byte) { b.listErrors[compactTestDestination+"/"] = listErr }, "check destination station database namespace", listErr},
		{"destination contents", func(b *compactTestBucket, _ []byte) { b.store(compactTestDestination+"/other", nil) }, "pick a fresh destination", nil},
		{"missing first page", func(b *compactTestBucket, _ []byte) { delete(b.data, compactTestOld+compactTestFirst) }, "read source station database page 1", errMissing},
		{"short first page", func(b *compactTestBucket, _ []byte) { b.seed(compactTestOld+compactTestFirst, []byte{1}) }, "page 1 length", nil},
		{"invalid SQLite header", func(b *compactTestBucket, _ []byte) { b.seed(compactTestOld+compactTestFirst, make([]byte, 4096)) }, "invalid source station SQLite header", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, key := newCompactTestBucket(t)
			compactTestSeedPages(b, compactTestIndex(0x10, 0x20))
			tc.alter(b, key)
			count, err := compactObjects(t.Context(), b, compactTestSource, compactTestDestination, key)
			require.ErrorContains(t, err, tc.want)
			if tc.cause != nil {
				require.ErrorIs(t, err, tc.cause)
			}
			require.Zero(t, count)
			require.Empty(t, b.writes)
		})
	}
}

func TestCompactRawCorruptPageNeverPublishes(t *testing.T) {
	b, key := newCompactTestBucket(t)
	compactTestSeedPages(b, compactTestIndex(0x10, 0x20))
	// Correct-size ciphertext transplanted from another object must not authenticate.
	b.store(compactTestOld+compactTestSecond, b.data[compactTestOld+compactTestFirst].body)

	count, err := compactObjects(t.Context(), b, compactTestSource, compactTestDestination, key)
	require.ErrorContains(t, err, "authenticate source station database page 2")
	require.ErrorContains(t, err, "pick a fresh destination")
	require.Zero(t, count)
	require.Equal(t, []string{compactTestNew + compactTestFirst}, b.writes)
	require.NotContains(t, b.data, compactTestNew+"head")

	_, err = compactObjects(t.Context(), b, compactTestSource, compactTestDestination, key)
	require.ErrorContains(t, err, "destination is not empty")
	require.Equal(t, []string{compactTestNew + compactTestFirst}, b.writes)
}

func TestCompactRawRechecksSourceBeforePublishing(t *testing.T) {
	b, key := newCompactTestBucket(t)
	index := compactTestIndex(0x10)
	compactTestSeedPages(b, index)
	b.beforeGet = func(name string) {
		if name == compactTestOld+"head" && len(b.writes) != 0 {
			b.seed(name, index)
		}
	}

	count, err := compactObjects(t.Context(), b, compactTestSource, compactTestDestination, key)
	require.ErrorIs(t, err, ErrConflict)
	require.ErrorContains(t, err, "source station database head changed")
	require.Zero(t, count)
	require.Equal(t, []string{compactTestNew + compactTestFirst}, b.writes)
	require.NotContains(t, b.data, compactTestNew+"head")
}

func TestCompactRawNeverReplacesConcurrentHead(t *testing.T) {
	b, key := newCompactTestBucket(t)
	compactTestSeedPages(b, compactTestIndex(0x10))
	b.beforeGet = func(name string) {
		if name == compactTestOld+"head" && len(b.writes) != 0 {
			b.seed(compactTestNew+"head", nil)
		}
	}

	count, err := compactObjects(t.Context(), b, compactTestSource, compactTestDestination, key)
	require.ErrorIs(t, err, ErrConflict)
	require.Zero(t, count)
	require.Empty(t, b.open(t, compactTestNew+"head"))
}

func TestCompactRawLostHeadResponseIsNotSuccess(t *testing.T) {
	b, key := newCompactTestBucket(t)
	index := compactTestIndex(0x10)
	compactTestSeedPages(b, index)
	lost := errors.New("injected lost PUT response")
	b.afterPut = func(name string) error {
		if name == compactTestNew+"head" {
			return lost
		}
		return nil
	}

	count, err := compactObjects(t.Context(), b, compactTestSource, compactTestDestination, key)
	require.ErrorIs(t, err, ErrCommitUnknown)
	require.ErrorIs(t, err, lost)
	require.ErrorContains(t, err, "pick a fresh destination")
	require.Zero(t, count)
	require.Equal(t, []string{compactTestNew + compactTestFirst, compactTestNew + "head"}, b.writes)
	require.Equal(t, index, b.open(t, compactTestNew+"head"))
}
