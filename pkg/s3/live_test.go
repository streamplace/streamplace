package s3

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
)

// liveConfig builds a Config from the S3_* env vars, tolerating the bucket
// being embedded as the endpoint URL's path (the format of Eli's ~/r2.env /
// ~/rustfs.env files). Returns false unless endpoint AND both credentials are
// set — an environment (like CI) that defines S3_ENDPOINT for other purposes
// must not un-skip these tests only to fail with empty static credentials.
func liveConfig(t *testing.T) (Config, bool) {
	cfg := Config{
		Endpoint:        os.Getenv("S3_ENDPOINT"),
		Bucket:          os.Getenv("S3_BUCKET"),
		AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
		Region:          os.Getenv("S3_REGION"),
	}
	if cfg.Endpoint == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return Config{}, false
	}
	u, err := url.Parse(cfg.Endpoint)
	require.NoError(t, err)
	if p := strings.Trim(u.Path, "/"); p != "" {
		if cfg.Bucket == "" {
			cfg.Bucket = p
		}
		u.Path = ""
		cfg.Endpoint = u.String()
	}
	if cfg.Region == "" {
		cfg.Region = "auto"
	}
	return cfg, true
}

// keyRecorder captures the object keys the uploader creates so the test can
// fetch them back.
type keyRecorder struct {
	mu   sync.Mutex
	keys []string
}

func (r *keyRecorder) RecordStart(ctx context.Context, userDID, bucket, key, livestreamURI string, started time.Time) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys = append(r.keys, key)
	return key, nil
}

func (r *keyRecorder) RecordComplete(ctx context.Context, id string, parts int32, size int64) error {
	return nil
}

// TestLiveS3Uploader exercises the real multipart upload path against a real
// S3-compatible endpoint. Skipped unless S3_ENDPOINT / S3_ACCESS_KEY_ID /
// S3_SECRET_ACCESS_KEY are set, e.g.:
//
//	set -a; . ~/r2.env; go test -count=1 -run TestLiveS3Uploader -v ./pkg/s3
//
// Segment sizes are deliberately irregular so the flushed parts have unequal
// sizes, like real live segments do — R2 (unlike AWS/minio) rejects multipart
// completes whose non-final parts aren't all the same size.
func TestLiveS3Uploader(t *testing.T) {
	cfg, ok := liveConfig(t)
	if !ok {
		t.Skip("set S3_ENDPOINT etc. to run the live uploader test")
	}
	ctx := context.Background()
	rec := &keyRecorder{}
	up := NewS3Uploader(cfg, "did:test:live", "live-test/", time.Hour, rec)

	// Irregular segments: parts flush at >=5MB boundaries with different sizes.
	var want []byte
	for _, mb := range []int{2, 2, 2, 3, 3, 1, 2} { // parts: 6MB, 6MB(3+3), then 3MB tail
		seg := bytes.Repeat([]byte("streamplace-live-s3-test"), mb*1024*1024/24)
		seg = append(seg, bytes.Repeat([]byte{0xAB}, 137*mb)...) // knock sizes off round numbers
		require.NoError(t, up.AddSegment(ctx, seg))
		want = append(want, seg...)
	}
	require.NoError(t, up.Close(ctx))

	require.Len(t, rec.keys, 1)
	key := rec.keys[0]
	t.Logf("uploaded key %s", key)

	client := NewClient(cfg)
	out, err := client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
	})
	require.NoError(t, err)
	got, err := io.ReadAll(out.Body)
	require.NoError(t, err)
	require.NoError(t, out.Body.Close())
	require.Equal(t, len(want), len(got))
	require.True(t, bytes.Equal(want, got))

	_, err = client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
	})
	require.NoError(t, err)
}

// TestLiveCanary runs the canary probe against a real endpoint. Same env
// gating as TestLiveS3Uploader.
func TestLiveCanary(t *testing.T) {
	cfg, ok := liveConfig(t)
	if !ok {
		t.Skip("set S3_ENDPOINT etc. to run the live canary test")
	}
	require.NoError(t, RunCanary(context.Background(), NewClient(cfg), cfg.Bucket))
}

// TestLiveConcatWithHeader exercises the live-to-VOD finalize assembly
// (header + UploadPartCopy concat) against a real endpoint. Same env gating
// as TestLiveS3Uploader.
func TestLiveConcatWithHeader(t *testing.T) {
	cfg, ok := liveConfig(t)
	if !ok {
		t.Skip("set S3_ENDPOINT etc. to run the live concat test")
	}
	ctx := context.Background()
	client := NewClient(cfg)

	put := func(key string, size int) []byte {
		body := bytes.Repeat([]byte("streamplace-live-concat!"), size/24)
		_, err := client.PutObject(ctx, &awss3.PutObjectInput{
			Bucket: aws.String(cfg.Bucket),
			Key:    aws.String(key),
			Body:   bytes.NewReader(body),
		})
		require.NoError(t, err)
		return body
	}
	keys := []string{"live-test/concat-src-0", "live-test/concat-src-1", "live-test/concat-src-2"}
	cleanup := append([]string(nil), keys...)
	defer func() {
		for _, k := range cleanup {
			_, _ = client.DeleteObject(ctx, &awss3.DeleteObjectInput{
				Bucket: aws.String(cfg.Bucket),
				Key:    aws.String(k),
			})
		}
	}()

	header := bytes.Repeat([]byte{0x42}, 1024)
	want := append([]byte(nil), header...)
	// Deliberately unequal source sizes, like real 10-minute cutover objects.
	// The first two exceed concatPartSize so interior windows exercise real
	// server-side UploadPartCopy; the boundaries exercise the mixed windows.
	for i, size := range []int{40 * 1024 * 1024, 33 * 1024 * 1024, 7 * 1024 * 1024} {
		want = append(want, put(keys[i], size)...)
	}

	dstKey := "live-test/concat-dst.m4s"
	cleanup = append(cleanup, dstKey)
	require.NoError(t, ConcatWithHeader(ctx, client, cfg.Bucket, header, keys, dstKey, "video/mp4"))

	out, err := client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(dstKey),
	})
	require.NoError(t, err)
	got, err := io.ReadAll(out.Body)
	require.NoError(t, err)
	require.NoError(t, out.Body.Close())
	require.Equal(t, len(want), len(got))
	require.True(t, bytes.Equal(want, got))
}
