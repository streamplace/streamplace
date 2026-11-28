package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// QualifyConditional checks create-only and ETag-matched writes on a fresh key
// beneath a private probe prefix. Never probe a live head or lease. Reject
// providers that accept a failed precondition or change data despite rejecting
// the request; either behavior makes station ownership unsafe.
func QualifyConditional(ctx context.Context, client *awss3.Client, bucket, prefix string) (result error) {
	key := strings.TrimSuffix(prefix, "/") + "/" + uuid.NewString()
	const original, updated = "initial", "changed"
	put := func(value string, match, none *string) (string, error) {
		out, err := client.PutObject(ctx, &awss3.PutObjectInput{
			Bucket: &bucket, Key: &key, Body: strings.NewReader(value), IfMatch: match, IfNoneMatch: none,
		})
		if err != nil {
			return "", err
		}
		if aws.ToString(out.ETag) == "" {
			return "", errors.New("missing probe ETag")
		}
		return *out.ETag, nil
	}
	// Do not return a raw SDK error; S3-compatible servers can echo request
	// credentials in error bodies. Keep the stage and status for diagnosis.
	failed := func(stage string, err error) error {
		var response interface{ HTTPStatusCode() int }
		if errors.As(err, &response) {
			return fmt.Errorf("S3 conditional probe %s: HTTP %d", stage, response.HTTPStatusCode())
		}
		return fmt.Errorf("S3 conditional probe %s failed", stage)
	}
	rejected := func(err error) bool {
		var response interface{ HTTPStatusCode() int }
		return errors.As(err, &response) && response.HTTPStatusCode() == http.StatusPreconditionFailed
	}
	star := aws.String("*")
	first, err := put(original, nil, star)
	if err != nil {
		return failed("create", err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, err := client.DeleteObject(cleanup, &awss3.DeleteObjectInput{Bucket: &bucket, Key: &key})
		if err != nil {
			result = errors.Join(result, failed("delete", err))
		}
	}()
	if _, err := put(updated, nil, star); !rejected(err) {
		return failed("reject duplicate create", err)
	}
	if _, err := put(updated, &first, nil); err != nil {
		return failed("update with matching ETag", err)
	}
	if _, err := put(original, &first, nil); !rejected(err) {
		return failed("reject stale ETag", err)
	}
	out, err := client.GetObject(ctx, &awss3.GetObjectInput{Bucket: &bucket, Key: &key})
	if err != nil {
		return failed("read final value", err)
	}
	defer out.Body.Close()
	got, err := io.ReadAll(io.LimitReader(out.Body, int64(len(updated)+1)))
	if err != nil || string(got) != updated {
		return failed("verify final value", err)
	}
	return nil
}
