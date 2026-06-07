package s3

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestS3UploaderCloseIdempotent exercises the lifecycle fix that re-enabled
// live S3 upload: Close must be safe to call repeatedly and concurrently (it
// was a plain close(segCh) before, which panicked on the second call), and
// AddSegment after Close must return an error rather than panic with
// "send on closed channel". No segments are added, so the upload loop completes
// without making any S3 calls — this stays a pure unit test.
func TestS3UploaderCloseIdempotent(t *testing.T) {
	u := NewS3Uploader(Config{
		Region:          "us-east-1",
		Endpoint:        "http://127.0.0.1:0",
		Bucket:          "test",
		AccessKeyID:     "k",
		SecretAccessKey: "s",
	}, "did:plc:test", "did:plc:test/", time.Minute, nil)

	const n = 4
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = u.Close(context.Background())
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("Close call %d returned error: %v", i, err)
		}
	}

	// A late AddSegment must be rejected, not panic on a closed channel.
	if err := u.AddSegment(context.Background(), []byte("late")); err == nil {
		t.Fatalf("AddSegment after Close should return an error")
	}
}
