//go:build linux && cgo && stationslate

package stationdb

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"stream.place/streamplace/pkg/s3"
)

// These are v1 wire sizes, independent of how the store represents snapshots.
const (
	compactV1Root     = "/stationdb/v1/"
	compactV1PageSize = 4096
	compactV1IDSize   = 16
)

type compactObjectSet interface {
	objects
	list(ctx context.Context, prefix string, visit func(string) error) error
}

func (s s3Objects) list(ctx context.Context, prefix string, visit func(string) error) error {
	pager := awss3.NewListObjectsV2Paginator(s.client, &awss3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	var previousToken string
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return err
		}
		if page.IsTruncated == nil {
			return errors.New("S3 listing is missing its truncation flag")
		}
		if *page.IsTruncated {
			token := aws.ToString(page.NextContinuationToken)
			if token == "" || token == previousToken {
				return errors.New("S3 listing did not advance its continuation token")
			}
			previousToken = token
		}
		for _, entry := range page.Contents {
			name := aws.ToString(entry.Key)
			if !strings.HasPrefix(name, prefix) {
				return errors.New("S3 listing returned a key outside the requested prefix")
			}
			if err := visit(name); err != nil {
				return err
			}
		}
	}
	return nil
}

// Compact copies the live SQLite pages from sourcePrefix to an empty
// destinationPrefix in the same bucket. Both prefixes must end in /operational.
// The caller must first terminate or externally fence all station nodes and
// keep both prefixes exclusive to this operation until it returns. The bucket
// must provide strongly consistent reads and listings, and conditional PUTs.
// Compact never modifies the source. Do not open or reuse a destination from a
// failed copy; pick a fresh destination or inspect and clean it up offline.
func Compact(ctx context.Context, cfg s3.Config, sourcePrefix, destinationPrefix string, key []byte) (int, error) {
	if cfg.Bucket == "" {
		return 0, errors.New("station database requires an S3 bucket")
	}
	sourcePrefix, destinationPrefix, err := validateCompactInputs(sourcePrefix, destinationPrefix, key)
	if err != nil {
		return 0, err
	}
	backend := s3Objects{client: s3.NewClient(cfg), bucket: cfg.Bucket}
	if err := backend.qualifyConditional(ctx, destinationPrefix); err != nil {
		return 0, err
	}
	return compactValidatedObjects(ctx, backend, sourcePrefix, destinationPrefix, key)
}

func compactObjects(ctx context.Context, backend compactObjectSet, sourcePrefix, destinationPrefix string, key []byte) (int, error) {
	sourcePrefix, destinationPrefix, err := validateCompactInputs(sourcePrefix, destinationPrefix, key)
	if err != nil {
		return 0, err
	}
	return compactValidatedObjects(ctx, backend, sourcePrefix, destinationPrefix, key)
}

func validateCompactInputs(sourcePrefix, destinationPrefix string, key []byte) (string, string, error) {
	sourcePrefix = strings.TrimRight(sourcePrefix, "/")
	destinationPrefix = strings.TrimRight(destinationPrefix, "/")
	const suffix = "/operational"
	for _, prefix := range [...]string{sourcePrefix, destinationPrefix} {
		if len(prefix) <= len(suffix) || !strings.HasSuffix(prefix, suffix) {
			return "", "", errors.New("station compaction requires source and destination prefixes ending in /operational")
		}
		if strings.HasPrefix(prefix, "/") || strings.HasPrefix(prefix, "../") || path.Clean(prefix) != prefix {
			return "", "", errors.New("station compaction requires canonical relative S3 prefixes")
		}
	}
	if sourcePrefix == destinationPrefix || strings.HasPrefix(sourcePrefix, destinationPrefix+"/") ||
		strings.HasPrefix(destinationPrefix, sourcePrefix+"/") {
		return "", "", errors.New("station compaction source and destination prefixes overlap")
	}
	if len(key) != 32 {
		return "", "", errors.New("station database requires a 32-byte encryption key")
	}
	return sourcePrefix, destinationPrefix, nil
}

func compactValidatedObjects(ctx context.Context, backend compactObjectSet, sourcePrefix, destinationPrefix string, key []byte) (_ int, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, err
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return 0, err
	}

	source := sourcePrefix + compactV1Root
	destination := destinationPrefix + compactV1Root
	sourceHead := source + "head"
	body, sourceETag, err := backend.get(ctx, sourceHead)
	if err != nil {
		return 0, fmt.Errorf("read source station database head: %w", err)
	}
	if sourceETag == "" {
		return 0, errors.New("source station database head has no ETag")
	}
	index, err := aead.Open(body[:0], nil, body, []byte(sourceHead))
	if err != nil {
		return 0, fmt.Errorf("authenticate source station database head: %w", err)
	}
	defer clear(index)
	count := len(index) / compactV1IDSize
	if len(index)%compactV1IDSize != 0 || count > math.MaxInt/compactV1PageSize {
		return 0, errors.New("invalid source station database page index")
	}

	if err := backend.list(ctx, source+"locks/", func(name string) error {
		body, etag, err := backend.get(ctx, name)
		if err != nil {
			return fmt.Errorf("read source lock marker: %w", err)
		}
		plain, err := aead.Open(body[:0], nil, body, []byte(name))
		if err != nil {
			return fmt.Errorf("authenticate source lock marker: %w", err)
		}
		clear(plain)
		if etag == "" || (len(plain) != 0 && len(plain) != compactV1IDSize) {
			return errors.New("invalid source station database lock marker")
		}
		if len(plain) != 0 {
			return ErrLockBusy
		}
		return nil
	}); err != nil {
		return 0, fmt.Errorf("check source station database locks: %w", err)
	}
	if err := backend.list(ctx, destinationPrefix+"/", func(string) error {
		return errors.New("destination is not empty; pick a fresh destination")
	}); err != nil {
		return 0, fmt.Errorf("check destination station database namespace: %w", err)
	}

	destinationTouched := false
	defer func() {
		if err != nil && destinationTouched {
			err = fmt.Errorf("do not use the destination; pick a fresh destination or clean it up offline: %w", err)
		}
	}()
	put := func(name string, ciphertext []byte) error {
		destinationTouched = true
		etag, err := backend.put(ctx, name, ciphertext, "")
		if err == nil && etag == "" {
			return errors.New("station database PUT response has no ETag")
		}
		return err
	}

	// Preserve head order and shared references while keeping only one page in flight.
	copied := make(map[[compactV1IDSize]byte]struct{}, count)
	for i := range count {
		id := [compactV1IDSize]byte(index[i*compactV1IDSize : (i+1)*compactV1IDSize])
		if _, ok := copied[id]; ok {
			continue
		}
		relative := "pages/" + hex.EncodeToString(id[:])
		name := source + relative
		body, _, err := backend.get(ctx, name)
		if err != nil {
			return 0, fmt.Errorf("read source station database page %d: %w", i+1, err)
		}
		if len(body) != compactV1PageSize+aead.Overhead() {
			return 0, fmt.Errorf("invalid source station database page %d length", i+1)
		}
		page, err := aead.Open(body[:0], nil, body, []byte(name))
		if err != nil {
			return 0, fmt.Errorf("authenticate source station database page %d: %w", i+1, err)
		}
		if i == 0 && (!bytes.HasPrefix(page, []byte("SQLite format 3\x00")) || binary.BigEndian.Uint16(page[16:18]) != compactV1PageSize) {
			clear(page)
			return 0, errors.New("invalid source station SQLite header")
		}
		name = destination + relative
		if err := put(name, aead.Seal(page[:0], nil, page, []byte(name))); err != nil {
			return 0, fmt.Errorf("write destination station database page %d: %w", i+1, err)
		}
		copied[id] = struct{}{}
	}

	destinationHead := destination + "head"
	head := aead.Seal(index[:0], nil, index, []byte(destinationHead))
	_, currentETag, err := backend.get(ctx, sourceHead)
	if err != nil {
		return 0, fmt.Errorf("recheck source station database head: %w", err)
	}
	if currentETag != sourceETag {
		return 0, fmt.Errorf("source station database head changed: %w", ErrConflict)
	}
	if err := put(destinationHead, head); errors.Is(err, errCAS) {
		return 0, fmt.Errorf("destination station database head already exists or collided: %w", ErrConflict)
	} else if err != nil {
		return 0, fmt.Errorf("publish destination station database head: %w: %w", ErrCommitUnknown, err)
	}
	return count, nil
}
