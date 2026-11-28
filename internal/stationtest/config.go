// Package stationtest locates the disposable S3 bucket for station tests.
package stationtest

import (
	"net/netip"
	"net/url"
	"os"
	"testing"

	"stream.place/streamplace/pkg/s3"
)

// Config skips only when the harness is absent. An incomplete or unsafe
// opt-in is a test failure, never a reason to use the live AWS environment.
func Config(t testing.TB) s3.Config {
	t.Helper()
	endpoint, present := os.LookupEnv("SP_TEST_S3_ENDPOINT")
	if !present {
		t.Skip("use hack/with-station-s3.sh for the isolated station bucket")
	}
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || endpoint != u.Scheme+"://"+u.Host {
		t.Fatal("SP_TEST_S3_ENDPOINT must be an HTTP(S) origin without credentials, path, query or fragment")
	}
	addr, err := netip.ParseAddrPort(u.Host)
	if err != nil || addr.Port() == 0 || addr.Addr().Zone() != "" || !addr.Addr().IsLoopback() {
		t.Fatal("SP_TEST_S3_ENDPOINT requires a literal loopback IP and nonzero port")
	}
	cfg := s3.Config{
		Endpoint:        endpoint,
		Bucket:          os.Getenv("SP_TEST_S3_BUCKET"),
		AccessKeyID:     os.Getenv("SP_TEST_S3_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("SP_TEST_S3_SECRET_ACCESS_KEY"),
		Region:          "us-east-1",
	}
	if cfg.Bucket != "station-test" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		t.Fatal("isolated S3 tests require the station-test bucket and explicit SP_TEST_S3 credentials")
	}
	return cfg
}
