package stationshard

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

// Only conditional PUT failures mean contention. Never turn permission, quota,
// throttling or transport failures into a missing object or a lost lease.
func s3Error(ctx context.Context, op string, err error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var response interface{ HTTPStatusCode() int }
	if errors.As(err, &response) {
		switch status := response.HTTPStatusCode(); {
		case op == "GET" && status == http.StatusNotFound:
			return ErrNotFound
		case op == "PUT" && (status == http.StatusConflict || status == http.StatusPreconditionFailed):
			return ErrConflict
		default:
			return fmt.Errorf("station S3 %s: HTTP %d", op, status)
		}
	}
	// S3 error bodies may echo request credentials. Do not pass them through
	// RPC or application logs.
	return fmt.Errorf("station S3 %s: transport failure (%T)", op, err)
}

func (s *Store) object(ctx context.Context, key string) ([]byte, string, error) {
	out, err := s.client.GetObject(ctx, &awss3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		return nil, "", s3Error(ctx, "GET", err)
	}
	defer out.Body.Close()
	if aws.ToString(out.ETag) == "" {
		return nil, "", errors.New("station S3 GET returned no ETag")
	}
	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, "", err
	}
	return data, *out.ETag, nil
}

// An absent expected ETag means create-only. There is deliberately no
// unconditional PUT in this package, even for immutable keys.
func (s *Store) swap(ctx context.Context, key, etag string, data []byte) (string, error) {
	in := &awss3.PutObjectInput{Bucket: &s.bucket, Key: &key, Body: bytes.NewReader(data)}
	if etag == "" {
		in.IfNoneMatch = aws.String("*")
	} else {
		in.IfMatch = &etag
	}
	out, err := s.client.PutObject(ctx, in)
	if err != nil {
		return "", s3Error(ctx, "PUT", err)
	}
	if aws.ToString(out.ETag) == "" {
		return "", errors.New("station S3 PUT returned no ETag")
	}
	return *out.ETag, nil
}

func (s *Store) scopedKey(kind, did, name string) string {
	return fmt.Sprintf("%s/%s/%x/%x", s.prefix, kind, sha256.Sum256([]byte(did)), sha256.Sum256([]byte(name)))
}

// PutImmutable writes an unpublished object. Attaching it to a workflow still
// requires a fenced manifest update. Retrying identical bytes is idempotent.
func (s *Store) PutImmutable(ctx context.Context, did, key string, data []byte) error {
	if err := s.begin(); err != nil {
		return err
	}
	defer s.active.Done()
	objectKey := s.scopedKey("immutable", did, key)
	if _, err := s.swap(ctx, objectKey, "", data); !errors.Is(err, ErrConflict) {
		return err
	}
	prev, _, err := s.object(ctx, objectKey)
	if err != nil {
		return err
	}
	if !bytes.Equal(prev, data) {
		return ErrConflict
	}
	return nil
}

func (s *Store) GetImmutable(ctx context.Context, did, key string) ([]byte, error) {
	if err := s.begin(); err != nil {
		return nil, err
	}
	defer s.active.Done()
	data, _, err := s.object(ctx, s.scopedKey("immutable", did, key))
	return data, err
}
