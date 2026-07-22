package config

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeS3 speaks just enough path-style multipart-upload S3 for UploadWriter:
// initiate → parts → complete. failComplete makes CompleteMultipartUpload 500,
// simulating an S3 that took the parts but won't commit.
type fakeS3 struct {
	mu           sync.Mutex
	parts        map[string][]byte
	objects      map[string][]byte
	failComplete bool
}

func newFakeS3() *fakeS3 {
	return &fakeS3{parts: map[string][]byte{}, objects: map[string][]byte{}}
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := strings.TrimPrefix(r.URL.Path, "/")
	q := r.URL.Query()
	switch {
	case r.Method == "POST" && q.Has("uploads"):
		fmt.Fprintf(w, `<InitiateMultipartUploadResult><UploadId>test-upload</UploadId></InitiateMultipartUploadResult>`)
	case r.Method == "PUT" && q.Has("partNumber"):
		body, _ := io.ReadAll(r.Body)
		f.parts[path+"#"+q.Get("partNumber")] = body
		w.Header().Set("ETag", `"part-`+q.Get("partNumber")+`"`)
	case r.Method == "POST" && q.Has("uploadId"):
		if f.failComplete {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var buf []byte
		for i := 1; ; i++ {
			part, ok := f.parts[fmt.Sprintf("%s#%d", path, i)]
			if !ok {
				break
			}
			buf = append(buf, part...)
		}
		f.objects[path] = buf
		fmt.Fprintf(w, `<CompleteMultipartUploadResult><Key>%s</Key></CompleteMultipartUploadResult>`, path)
	case r.Method == "DELETE":
		// AbortMultipartUpload
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (f *fakeS3) object(path string) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.objects[path]
	return b, ok
}

func newS3TestCLI(t *testing.T, endpoint string) *CLI {
	return &CLI{
		DataDir:           t.TempDir(),
		S3Endpoint:        endpoint,
		S3Bucket:          "bkt",
		S3AccessKeyID:     "test-access",
		S3SecretAccessKey: "test-secret",
		S3Region:          "auto",
	}
}

// TestDebugRecordingSpoolCommit: happy path — the recording streams to S3 (at
// the sanitized key mirroring the on-disk layout) and the local spool is
// removed once the upload commits.
func TestDebugRecordingSpoolCommit(t *testing.T) {
	fake := newFakeS3()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	cli := newS3TestCLI(t, srv.URL)

	fpath := []string{"debug-recordings", "did:key:zTest", "rec.rtmp.mkv"}
	f, err := cli.DebugRecordingCreate(context.Background(), fpath, "video/x-matroska", false)
	require.NoError(t, err)
	data := []byte("pretend this is mkv data")
	_, err = f.Write(data)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	got, ok := fake.object("bkt/debug-recordings/did-key-zTest/rec.rtmp.mkv")
	require.True(t, ok, "object committed at the sanitized key")
	require.Equal(t, data, got)
	_, err = os.Stat(cli.DataFilePath(fpath))
	require.True(t, os.IsNotExist(err), "spool removed after commit")
}

// TestDebugRecordingSpoolSurvivesFailedCommit: when S3 won't commit, Close is
// not an error — the local spool survives, intact, for the sweep.
func TestDebugRecordingSpoolSurvivesFailedCommit(t *testing.T) {
	fake := newFakeS3()
	fake.failComplete = true
	srv := httptest.NewServer(fake)
	defer srv.Close()
	cli := newS3TestCLI(t, srv.URL)

	fpath := []string{"debug-recordings", "did:key:zTest", "rec.rtmp.mkv"}
	f, err := cli.DebugRecordingCreate(context.Background(), fpath, "video/x-matroska", false)
	require.NoError(t, err)
	data := []byte("pretend this is mkv data")
	_, err = f.Write(data)
	require.NoError(t, err)
	require.NoError(t, f.Close(), "a failed S3 commit is not a recording error")

	got, err := os.ReadFile(cli.DataFilePath(fpath))
	require.NoError(t, err, "spool survives the failed commit")
	require.Equal(t, data, got)
	_, ok := fake.object("bkt/debug-recordings/did-key-zTest/rec.rtmp.mkv")
	require.False(t, ok, "no object committed")
}

// TestSweepDebugRecordings: idle leftovers get uploaded at the key their path
// dictates and deleted; a freshly-written file (possibly still being recorded)
// is left alone.
func TestSweepDebugRecordings(t *testing.T) {
	fake := newFakeS3()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	cli := newS3TestCLI(t, srv.URL)

	dir := cli.DataFilePath([]string{"debug-recordings", "did-key-zOld"})
	require.NoError(t, os.MkdirAll(dir, 0755))
	stale := filepath.Join(dir, "stale.rtmp.mkv")
	staleData := []byte("leftover recording bytes")
	require.NoError(t, os.WriteFile(stale, staleData, 0644))
	old := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(stale, old, old))

	fresh := filepath.Join(dir, "fresh.rtmp.mkv")
	require.NoError(t, os.WriteFile(fresh, []byte("still being written"), 0644))

	cli.SweepDebugRecordings(context.Background())

	got, ok := fake.object("bkt/debug-recordings/did-key-zOld/stale.rtmp.mkv")
	require.True(t, ok, "stale spool salvaged to S3")
	require.Equal(t, staleData, got)
	_, err := os.Stat(stale)
	require.True(t, os.IsNotExist(err), "salvaged spool removed")
	_, err = os.Stat(fresh)
	require.NoError(t, err, "fresh file untouched")
	_, ok = fake.object("bkt/debug-recordings/did-key-zOld/fresh.rtmp.mkv")
	require.False(t, ok, "fresh file not uploaded")
}
