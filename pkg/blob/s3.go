package blob

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"

	s3pkg "stream.place/streamplace/pkg/s3"
)

// S3Store is a blob.Store backed by an S3-compatible object store.
//
// Writes go to a hidden staging prefix (.staging/<uuid>) so that
// in-progress multipart uploads can't collide with the final
// content-addressed key. Complete renames staging -> the configured
// key via CopyObject + DeleteObject.
type S3Store struct {
	client *awss3.Client
	bucket string
}

// stagingPrefix is the in-bucket prefix used for in-progress writes.
// Matches the FileStore convention (.staging at the root).
const s3StagingPrefix = ".staging/"

// NewS3Store wraps an existing S3 client + bucket as a blob.Store.
func NewS3Store(client *awss3.Client, bucket string) *S3Store {
	return &S3Store{client: client, bucket: bucket}
}

func (s *S3Store) URL(key string) string { return "s3://" + s.bucket + "/" + key }

func (s *S3Store) Bucket() string { return s.bucket }

func (s *S3Store) Open(ctx context.Context, key string) (Reader, error) {
	ra, err := s3pkg.NewReaderAt(ctx, s.client, s.bucket, key)
	if err != nil {
		// The AWS SDK returns *types.NoSuchKey wrapped in opaque
		// generic errors; sniff the message rather than relying on
		// type assertions that change between SDK versions.
		if isS3NotFound(err) {
			return nil, fmt.Errorf("%w: s3://%s/%s", ErrNotFound, s.bucket, key)
		}
		return nil, err
	}
	return ra, nil
}

func (s *S3Store) NewWriter(ctx context.Context, key, contentType string) (Writer, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("blob s3: generate staging id: %w", err)
	}
	stagingKey := s3StagingPrefix + id.String()

	mw, err := s3pkg.NewMultipartWriter(ctx, s.client, s.bucket, stagingKey, contentType)
	if err != nil {
		return nil, fmt.Errorf("blob s3: create staging multipart: %w", err)
	}
	return &s3Writer{
		store:      s,
		ctx:        ctx,
		mw:         mw,
		stagingKey: stagingKey,
		finalKey:   key,
	}, nil
}

func (s *S3Store) Move(ctx context.Context, srcKey, dstKey string) error {
	_, err := s.client.CopyObject(ctx, &awss3.CopyObjectInput{
		Bucket:     aws.String(s.bucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(s.bucket + "/" + srcKey),
	})
	if err != nil {
		if isS3NotFound(err) {
			// Idempotency: maybe a previous Move already renamed
			// source -> dest. If dest exists, we're done.
			if _, headErr := s.client.HeadObject(ctx, &awss3.HeadObjectInput{
				Bucket: aws.String(s.bucket),
				Key:    aws.String(dstKey),
			}); headErr == nil {
				return nil
			}
			return fmt.Errorf("%w: s3://%s/%s", ErrNotFound, s.bucket, srcKey)
		}
		return fmt.Errorf("blob s3: copy %s -> %s: %w", srcKey, dstKey, err)
	}
	if _, err := s.client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(srcKey),
	}); err != nil {
		// Non-fatal: the dst exists; staging is leftover but
		// eventually garbage-collected.
		return fmt.Errorf("blob s3: delete src %s after copy to %s: %w", srcKey, dstKey, err)
	}
	return nil
}

func (s *S3Store) List(ctx context.Context, prefix string) ([]string, error) {
	var out []string
	var continuation *string
	for {
		resp, err := s.client.ListObjectsV2(ctx, &awss3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: continuation,
		})
		if err != nil {
			return nil, fmt.Errorf("blob s3 list %s: %w", prefix, err)
		}
		for _, obj := range resp.Contents {
			if obj.Key == nil {
				continue
			}
			out = append(out, *obj.Key)
		}
		if resp.IsTruncated == nil || !*resp.IsTruncated {
			break
		}
		continuation = resp.NextContinuationToken
	}
	return out, nil
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil && !isS3NotFound(err) {
		return fmt.Errorf("blob s3 delete %s: %w", key, err)
	}
	return nil
}

// ParseLocation accepts "s3://bucket/key" URLs. The bucket must match
// the Store's bucket; otherwise we return ok=false to avoid silently
// reading from a different bucket than the Store is configured for.
func (s *S3Store) ParseLocation(location string) (string, bool) {
	if !strings.HasPrefix(location, "s3://") {
		return "", false
	}
	rest := strings.TrimPrefix(location, "s3://")
	slash := strings.IndexByte(rest, '/')
	if slash <= 0 || slash == len(rest)-1 {
		return "", false
	}
	bucket, key := rest[:slash], rest[slash+1:]
	if bucket != s.bucket {
		return "", false
	}
	return key, true
}

// s3Writer adapts the existing s3.MultipartWriter (which writes to one
// fixed key) to the blob.Writer interface that needs Complete to
// publish at the originally-requested final key. It does this with a
// staging-then-Move pattern that mirrors what pkg/vod already did
// inline.
type s3Writer struct {
	store      *S3Store
	ctx        context.Context
	mw         *s3pkg.MultipartWriter
	stagingKey string
	finalKey   string
	finalized  bool
}

func (w *s3Writer) Write(p []byte) (int, error) {
	if w.finalized {
		return 0, fmt.Errorf("blob s3: write after Complete/Abort on %s", w.finalKey)
	}
	return w.mw.Write(p)
}

func (w *s3Writer) Complete() error {
	if w.finalized {
		return fmt.Errorf("blob s3: Complete called twice on %s", w.finalKey)
	}
	if err := w.mw.Complete(); err != nil {
		return err
	}
	if err := w.store.Move(w.ctx, w.stagingKey, w.finalKey); err != nil {
		// We have a completed staging upload but the rename failed.
		// Try to remove the staging blob so it doesn't dangle; if that
		// fails the staging janitor (if/when one exists) will pick it
		// up later.
		_ = w.store.Delete(w.ctx, w.stagingKey)
		return fmt.Errorf("blob s3: move staging -> %s: %w", w.finalKey, err)
	}
	w.finalized = true
	return nil
}

func (w *s3Writer) Abort() error {
	if w.finalized {
		return nil
	}
	w.finalized = true
	return w.mw.Abort()
}

func (w *s3Writer) Close() error { return w.Abort() }

// isS3NotFound reports whether err looks like an S3 "the object you
// asked about doesn't exist" condition. The aws-sdk-go-v2 returns
// these as typed errors that vary across the API surface (NoSuchKey
// for GetObject, NotFound for HeadObject); string matching is the
// least painful way to cover both without dropping into reflect.
func isS3NotFound(err error) bool {
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "NoSuchKey") || strings.Contains(msg, "NotFound") || strings.Contains(msg, "status code: 404")
}

var (
	_ Store  = (*S3Store)(nil)
	_ Writer = (*s3Writer)(nil)
)
