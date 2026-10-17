package bunny

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/cdn"
	"stream.place/streamplace/pkg/log"
)

// Request is the provider-neutral record type this package emits.
// Aliased so the parser tests don't have to import pkg/cdn.
type Request = cdn.Request

// ParseLine decodes one line of bunny's pipe-delimited access-log
// format:
//
//	cache-status|status|timestamp-ms|bytes-sent|pull-zone-id|remote-ip|
//	referer|url|edge-location|user-agent|request-id|country-code
//
// Bunny strips '|' from the user-controllable fields, so a naive split
// is safe. Lines with fewer than 12 fields, or an unparseable status,
// timestamp or byte count, are rejected. Extra trailing fields (a
// future format extension) are ignored.
func ParseLine(line string) (Request, error) {
	line = strings.TrimRight(line, "\r\n")
	f := strings.Split(line, "|")
	if len(f) < 12 {
		return Request{}, fmt.Errorf("bunny log: %d fields, want 12", len(f))
	}
	status, err := strconv.Atoi(f[1])
	if err != nil {
		return Request{}, fmt.Errorf("bunny log: status %q: %w", f[1], err)
	}
	ms, err := strconv.ParseInt(f[2], 10, 64)
	if err != nil {
		return Request{}, fmt.Errorf("bunny log: timestamp %q: %w", f[2], err)
	}
	bytes, err := strconv.ParseInt(f[3], 10, 64)
	if err != nil {
		return Request{}, fmt.Errorf("bunny log: bytes %q: %w", f[3], err)
	}
	return Request{
		Time:      time.UnixMilli(ms).UTC(),
		Status:    status,
		BytesSent: bytes,
		RemoteIP:  f[5],
		URL:       f[7],
	}, nil
}

// LogStorage implements cdn.LogSource over a bunny Edge Storage zone
// that a pull zone's Permanent Log Storage writes into. Bunny files
// parts as
//
//	pullzone-logs/<pull-zone>/<YYYY>/<MM>/<dd>_<worker>-<part>-<rand>.gzip
//
// Each part is immutable once it lands (parts close on size, line
// count, age or the UTC midnight boundary and are never rewritten),
// which is what lets the ingester treat "seen this path" as "done".
type LogStorage struct {
	// Endpoint is the storage region's API base, e.g.
	// https://storage.bunnycdn.com or https://ny.storage.bunnycdn.com.
	Endpoint string
	// Zone is the storage zone name.
	Zone string
	// AccessKey is the storage zone's password (read-only is enough).
	AccessKey string
	// PullZone is the pull zone name as it appears in the log path.
	PullZone string
	// HTTPClient overrides the outbound client (tests). Defaults to
	// aqhttp.TrustedClient: the endpoint is operator-configured.
	HTTPClient *http.Client
	// Now overrides the clock (tests).
	Now func() time.Time
}

// storageObject is the subset of bunny's List Files response we read.
type storageObject struct {
	ObjectName  string `json:"ObjectName"`
	IsDirectory bool   `json:"IsDirectory"`
	Length      int64  `json:"Length"`
}

func (s *LogStorage) client() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return &aqhttp.TrustedClient
}

func (s *LogStorage) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// logPrefix is the directory all of this pull zone's parts live under.
func (s *LogStorage) logPrefix() string {
	return "pullzone-logs/" + s.PullZone
}

func (s *LogStorage) objectURL(p string) (string, error) {
	base, err := url.Parse(strings.TrimRight(s.Endpoint, "/"))
	if err != nil || base.Host == "" {
		return "", fmt.Errorf("bunny storage: bad endpoint %q", s.Endpoint)
	}
	base.Path = path.Join(base.Path, s.Zone, p)
	// Directory listings need the trailing slash; path.Join drops it.
	if strings.HasSuffix(p, "/") {
		base.Path += "/"
	}
	return base.String(), nil
}

func (s *LogStorage) get(ctx context.Context, p string) (*http.Response, error) {
	u, err := s.objectURL(p)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("AccessKey", s.AccessKey)
	req.Header.Set("Accept", "*/*")
	resp, err := s.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("bunny storage: GET %s: %w", p, err)
	}
	return resp, nil
}

// ListParts walks the month directories from since's UTC month through
// the current month and returns every part whose day-of-month prefix
// falls on or after since's UTC date.
func (s *LogStorage) ListParts(ctx context.Context, since time.Time) ([]cdn.Part, error) {
	since = since.UTC()
	sinceDay := time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, time.UTC)
	now := s.now().UTC()
	var parts []cdn.Part
	for m := time.Date(since.Year(), since.Month(), 1, 0, 0, 0, 0, time.UTC); !m.After(now); m = m.AddDate(0, 1, 0) {
		dir := fmt.Sprintf("%s/%04d/%02d/", s.logPrefix(), m.Year(), int(m.Month()))
		objs, err := s.list(ctx, dir)
		if err != nil {
			return nil, err
		}
		for _, o := range objs {
			if o.IsDirectory {
				continue
			}
			day, ok := partDay(m, o.ObjectName)
			if !ok {
				log.Debug(ctx, "bunny storage: skipping unrecognized object", "dir", dir, "name", o.ObjectName)
				continue
			}
			if day.Before(sinceDay) {
				continue
			}
			parts = append(parts, cdn.Part{
				ID:   dir + o.ObjectName,
				Day:  day,
				Size: o.Length,
			})
		}
	}
	return parts, nil
}

// partDay extracts the UTC date from a part filename like
// `06_abc123-0-1a2b3c4d.gzip` inside the given month directory.
func partDay(month time.Time, name string) (time.Time, bool) {
	if !strings.HasSuffix(name, ".gzip") && !strings.HasSuffix(name, ".gz") {
		return time.Time{}, false
	}
	i := strings.IndexByte(name, '_')
	if i <= 0 {
		return time.Time{}, false
	}
	d, err := strconv.Atoi(name[:i])
	if err != nil || d < 1 || d > 31 {
		return time.Time{}, false
	}
	return time.Date(month.Year(), month.Month(), d, 0, 0, 0, 0, time.UTC), true
}

func (s *LogStorage) list(ctx context.Context, dir string) ([]storageObject, error) {
	resp, err := s.get(ctx, dir)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		// A month with no logs yet (or a pull zone that hasn't
		// written anything) is not an error.
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bunny storage: list %s: HTTP %d", dir, resp.StatusCode)
	}
	var objs []storageObject
	if err := json.NewDecoder(resp.Body).Decode(&objs); err != nil {
		return nil, fmt.Errorf("bunny storage: decode listing %s: %w", dir, err)
	}
	return objs, nil
}

// maxLineBytes bounds a single log line. Bunny lines are a few hundred
// bytes; user-agent + referer are the only unbounded fields.
const maxLineBytes = 1 << 20

// ReadPart downloads the gzip part and streams parsed records. Lines
// that fail to parse are skipped and counted; the count is logged at
// the end so a format change is visible without failing the whole
// part.
func (s *LogStorage) ReadPart(ctx context.Context, part cdn.Part, emit func(Request) error) error {
	resp, err := s.get(ctx, part.ID)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bunny storage: read %s: HTTP %d", part.ID, resp.StatusCode)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("bunny storage: gunzip %s: %w", part.ID, err)
	}
	defer gz.Close()
	return readLines(ctx, gz, part.ID, emit)
}

func readLines(ctx context.Context, r io.Reader, name string, emit func(Request) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), maxLineBytes)
	var bad int
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		req, err := ParseLine(sc.Text())
		if err != nil {
			bad++
			continue
		}
		if err := emit(req); err != nil {
			return err
		}
	}
	if bad > 0 {
		log.Warn(ctx, "bunny storage: skipped unparseable log lines", "part", name, "lines", bad)
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("bunny storage: read %s: %w", name, err)
	}
	return nil
}
