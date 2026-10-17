package bunny

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const sampleLine = `HIT|206|1725580800123|524288|123456|203.0.113.7|https://stream.place/|https://eli-vod-dev.b-cdn.net/blobs/bafyblob.mp4?token=abc&did=did%3Aplc%3Aabc&sid=tid123&expires=1725600000|NY|Mozilla/5.0|req-1|US`

func TestParseLine(t *testing.T) {
	req, err := ParseLine(sampleLine + "\r\n")
	require.NoError(t, err)
	require.Equal(t, time.UnixMilli(1725580800123).UTC(), req.Time)
	require.Equal(t, 206, req.Status)
	require.EqualValues(t, 524288, req.BytesSent)
	require.Equal(t, "203.0.113.7", req.RemoteIP)
	require.Equal(t, "https://eli-vod-dev.b-cdn.net/blobs/bafyblob.mp4?token=abc&did=did%3Aplc%3Aabc&sid=tid123&expires=1725600000", req.URL)
}

func TestParseLineRejectsShortAndMalformed(t *testing.T) {
	_, err := ParseLine("HIT|200|123")
	require.Error(t, err)
	_, err = ParseLine(strings.Replace(sampleLine, "|206|", "|xx|", 1))
	require.Error(t, err)
	_, err = ParseLine(strings.Replace(sampleLine, "|1725580800123|", "|soon|", 1))
	require.Error(t, err)
}

// fakeStorage serves a bunny-shaped listing + gzip parts.
func fakeStorage(t *testing.T, listings map[string][]storageObject, files map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "zone-key", r.Header.Get("AccessKey"))
		p := strings.TrimPrefix(r.URL.Path, "/logs-zone/")
		if objs, ok := listings[p]; ok {
			_ = json.NewEncoder(w).Encode(objs)
			return
		}
		if body, ok := files[p]; ok {
			var buf bytes.Buffer
			gz := gzip.NewWriter(&buf)
			_, _ = gz.Write([]byte(body))
			_ = gz.Close()
			_, _ = w.Write(buf.Bytes())
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

func TestLogStorageListAndRead(t *testing.T) {
	srv := fakeStorage(t,
		map[string][]storageObject{
			"pullzone-logs/eli-vod-dev/2025/08/": {
				{ObjectName: "30_w1-0-aaaaaaaa.gzip", Length: 10},
				{ObjectName: "31_w1-0-bbbbbbbb.gzip", Length: 20},
			},
			"pullzone-logs/eli-vod-dev/2025/09/": {
				{ObjectName: "01_w1-0-cccccccc.gzip", Length: 30},
				{ObjectName: "01_w2-0-dddddddd.gzip", Length: 40},
				{ObjectName: "notes", IsDirectory: true},
				{ObjectName: "junk.txt"},
			},
		},
		map[string]string{
			"pullzone-logs/eli-vod-dev/2025/09/01_w1-0-cccccccc.gzip": sampleLine + "\n" + "garbage line\n" + sampleLine + "\n",
		},
	)
	defer srv.Close()

	s := &LogStorage{
		Endpoint:  srv.URL,
		Zone:      "logs-zone",
		AccessKey: "zone-key",
		PullZone:  "eli-vod-dev",
		Now:       func() time.Time { return time.Date(2025, 9, 6, 12, 0, 0, 0, time.UTC) },
	}
	ctx := context.Background()

	// since = Aug 31 12:00 → the Aug 30 part is skipped, Aug 31 and
	// both Sep 1 parts are listed; the Oct directory (404) is never
	// reached because Now is in September.
	parts, err := s.ListParts(ctx, time.Date(2025, 8, 31, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	var ids []string
	for _, p := range parts {
		ids = append(ids, p.ID)
	}
	require.Equal(t, []string{
		"pullzone-logs/eli-vod-dev/2025/08/31_w1-0-bbbbbbbb.gzip",
		"pullzone-logs/eli-vod-dev/2025/09/01_w1-0-cccccccc.gzip",
		"pullzone-logs/eli-vod-dev/2025/09/01_w2-0-dddddddd.gzip",
	}, ids)
	require.Equal(t, time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC), parts[1].Day)
	require.EqualValues(t, 30, parts[1].Size)

	var got []Request
	err = s.ReadPart(ctx, parts[1], func(r Request) error {
		got = append(got, r)
		return nil
	})
	require.NoError(t, err)
	require.Len(t, got, 2, "the garbage line is skipped, not fatal")
	require.Equal(t, 206, got[0].Status)

	// A missing part is an error, not silently empty.
	err = s.ReadPart(ctx, parts[2], func(Request) error { return nil })
	require.Error(t, err)
}

func TestLogStorageMissingMonthIsEmpty(t *testing.T) {
	srv := fakeStorage(t, nil, nil)
	defer srv.Close()
	s := &LogStorage{Endpoint: srv.URL, Zone: "logs-zone", AccessKey: "zone-key", PullZone: "pz",
		Now: func() time.Time { return time.Date(2025, 9, 6, 0, 0, 0, 0, time.UTC) }}
	parts, err := s.ListParts(context.Background(), time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Empty(t, parts)
}
