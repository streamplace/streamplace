package stationdb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"stream.place/streamplace/pkg/s3"
)

type s3Objects struct {
	client *awss3.Client
	bucket string
}

func (s s3Objects) qualifyConditional(ctx context.Context, prefix string) error {
	ctx, cancel := context.WithTimeout(ctx, objectTimeout)
	defer cancel()
	if err := s3.QualifyConditional(ctx, s.client, s.bucket, prefix+"/probes"); err != nil {
		return fmt.Errorf("qualify station database S3 conditional writes: %w", err)
	}
	return nil
}

func (s s3Objects) get(ctx context.Context, key string) ([]byte, string, error) {
	out, err := s.client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// NoSuchBucket and access failures must not initialize an empty database.
		var api smithy.APIError
		if errors.As(err, &api) && (api.ErrorCode() == "NoSuchKey" || api.ErrorCode() == "NotFound") {
			return nil, "", errMissing
		}
		return nil, "", err
	}
	defer out.Body.Close()
	body, err := io.ReadAll(out.Body)
	return body, aws.ToString(out.ETag), err
}

func (s s3Objects) empty(ctx context.Context, prefix string) (bool, error) {
	out, err := s.client.ListObjectsV2(ctx, &awss3.ListObjectsV2Input{
		Bucket:  aws.String(s.bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return false, err
	}
	if len(out.Contents) != 0 || aws.ToInt32(out.KeyCount) != 0 {
		return false, nil
	}
	if out.IsTruncated == nil || *out.IsTruncated {
		return false, errors.New("S3 listing did not establish that the prefix is empty")
	}
	return true, nil
}

func (s s3Objects) put(ctx context.Context, key string, body []byte, match string) (string, error) {
	in := &awss3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body),
	}
	if match == "" {
		in.IfNoneMatch = aws.String("*")
	} else {
		in.IfMatch = aws.String(match)
	}
	out, err := s.client.PutObject(ctx, in, func(o *awss3.Options) {
		// A retry after a lost success response can turn it into a false conflict.
		o.Retryer = aws.NopRetryer{}
	})
	if err != nil {
		var status interface{ HTTPStatusCode() int }
		var api smithy.APIError
		if (errors.As(err, &status) && status.HTTPStatusCode() == http.StatusPreconditionFailed) ||
			(errors.As(err, &api) && api.ErrorCode() == "ConditionalRequestConflict") {
			return "", errCAS
		}
		return "", fmt.Errorf("conditional S3 PUT: %w", err)
	}
	return aws.ToString(out.ETag), nil
}
