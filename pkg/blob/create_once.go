package blob

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// CreateOnce atomically installs a small, immutable station control record. A
// concurrent writer winning the same key is success. Never emulate this with
// Open followed by Move: a late writer could replace the original authority.
func CreateOnce(ctx context.Context, store Store, key string, data []byte) error {
	switch s := store.(type) {
	case *S3Store:
		_, err := s.client.PutObject(ctx, &awss3.PutObjectInput{
			Bucket: aws.String(s.bucket), Key: aws.String(key), Body: bytes.NewReader(data), IfNoneMatch: aws.String("*"),
		})
		var api smithy.APIError
		if errors.As(err, &api) && api.ErrorCode() == "PreconditionFailed" {
			return nil
		}
		return err
	case *FileStore:
		if err := ctx.Err(); err != nil {
			return err
		}
		dest := s.absPath(key)
		dir := filepath.Dir(dest)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		f, err := os.CreateTemp(s.stagingDir, "station-*")
		if err != nil {
			return err
		}
		defer os.Remove(f.Name())
		if _, err = f.Write(data); err == nil {
			err = f.Sync()
		}
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
		if err := os.Link(f.Name(), dest); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		d, err := os.Open(dir)
		if err != nil {
			return err
		}
		defer d.Close()
		return d.Sync()
	default:
		return fmt.Errorf("blob store %T cannot conditionally create a station control record", store)
	}
}
