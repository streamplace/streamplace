package statedb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	awss3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/s3"
)

// The state database tests run against whatever services the environment
// names, so one suite covers SQLite, a database server, and object storage:
//
//	STREAMPLACE_TEST_POSTGRES_COMMAND  shell command that starts postgres
//	STREAMPLACE_TEST_POSTGRES_URL      postgres the postgres tests run against
//	STREAMPLACE_TEST_RUSTFS_COMMAND    shell command that starts RustFS
//	STREAMPLACE_TEST_SLATEDB_URL       object store the SlateDB tests run against
//
// A command runs in the foreground and is killed when the tests finish; a URL
// with no command means its service is already running somewhere. The builder
// image sets all four (see docker/build.Dockerfile).
var (
	postgresURL string
	slatedbURL  string
)

// serviceStartupTimeout caps how long a service gets to accept connections
// before the run gives up and dumps its logs.
var serviceStartupTimeout = 30 * time.Second

func TestMain(m *testing.M) {
	postgresURL = os.Getenv("STREAMPLACE_TEST_POSTGRES_URL")
	slatedbURL = os.Getenv("STREAMPLACE_TEST_SLATEDB_URL")

	pg, err := startTestService("postgres", os.Getenv("STREAMPLACE_TEST_POSTGRES_COMMAND"), waitPostgresReady)
	if err != nil {
		os.Exit(1)
	}
	fs, err := startTestService("rustfs", os.Getenv("STREAMPLACE_TEST_RUSTFS_COMMAND"), waitRustFSReady)
	if err != nil {
		os.Exit(1)
	}

	code := m.Run()
	pg.stop()
	fs.stop()
	os.Exit(code)
}

// testService is a service the suite needs, running in the background.
type testService struct {
	name string
}

// startTestService runs a shell command that keeps a service in the
// foreground, and returns once ready says the service is up. A nil service
// with no error means no command was configured.
func startTestService(name, command string, ready func() error) (*testService, error) {
	if command == "" {
		return nil, nil
	}
	fmt.Printf("Starting %s process with command: %s\n", name, command)
	cmd := exec.Command("bash", "-c", command)
	// Capture the output as well as stream it: a service that fails at
	// startup (postgres can FATAL before it ever listens) otherwise leaves
	// the tests with a misleading "connection refused" and no clue why.
	var logs bytes.Buffer
	cmd.Stdout = io.MultiWriter(&logs, os.Stdout)
	cmd.Stderr = io.MultiWriter(&logs, os.Stderr)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %w", name, err)
	}
	if err := ready(); err != nil {
		fmt.Printf("%s did not become ready: %v\n", name, err)
		fmt.Printf("=== captured %s output ===\n%s\n=== end %s output ===\n", name, logs.String(), name)
		_ = cmd.Process.Kill()
		return nil, err
	}
	return &testService{name: name}, nil
}

// stop ends the service. The processes are found by name rather than by the
// shell that started them because neither service is the shell's only
// descendant: postgres is started through sudo, and RustFS may already have
// been started by hand.
func (s *testService) stop() {
	if s == nil {
		return
	}
	if err := exec.Command("pkill", s.name).Run(); err != nil {
		fmt.Printf("Failed to kill %s: %v\n", s.name, err)
	}
}

// waitReady waits for something to accept TCP connections at address.
func waitReady(address string) error {
	deadline := time.Now().Add(serviceStartupTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err
		time.Sleep(200 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timed out after %s", serviceStartupTimeout)
	}
	return fmt.Errorf("nothing accepting connections at %s: %w", address, lastErr)
}

// waitPostgresReady waits for the host and port the test postgres URL names.
func waitPostgresReady() error {
	host, port, err := postgresHostPort()
	if err != nil {
		return fmt.Errorf("could not parse postgres URL for readiness check: %w", err)
	}
	return waitReady(net.JoinHostPort(host, port))
}

// waitRustFSReady waits for the object store the test state URL names, which
// is where the state database's files are going.
func waitRustFSReady() error {
	endpoint, err := slatedbEndpoint()
	if err != nil {
		return err
	}
	return waitReady(endpoint)
}

// slatedbEndpoint is the host:port of the object store named by the test state
// URL.
func slatedbEndpoint() (string, error) {
	if slatedbURL == "" {
		return "", fmt.Errorf("STREAMPLACE_TEST_SLATEDB_URL is not set")
	}
	u, err := url.Parse(slatedbURL)
	if err != nil {
		return "", fmt.Errorf("could not parse STREAMPLACE_TEST_SLATEDB_URL: %w", err)
	}
	endpoint := u.Query().Get("endpoint")
	if endpoint == "" {
		return "", fmt.Errorf("STREAMPLACE_TEST_SLATEDB_URL names no endpoint to wait for")
	}
	eu, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("could not parse endpoint %q: %w", endpoint, err)
	}
	host := eu.Hostname()
	if host == "" {
		return "", fmt.Errorf("endpoint %q has no host", endpoint)
	}
	port := eu.Port()
	if port == "" {
		if eu.Scheme == "http" {
			port = "80"
		} else {
			port = "443"
		}
	}
	return net.JoinHostPort(host, port), nil
}

// postgresHostPort extracts the host and port from postgresURL, falling back to
// localhost:5432 when the URL is missing them.
func postgresHostPort() (string, string, error) {
	if postgresURL == "" {
		return "localhost", "5432", nil
	}
	u, err := url.Parse(postgresURL)
	if err != nil {
		return "", "", err
	}
	host := u.Hostname()
	if host == "" {
		host = "localhost"
	}
	port := u.Port()
	if port == "" {
		port = "5432"
	}
	return host, port, nil
}

// makePostgresURL is a database of its own on the test server, dropped when
// the test finishes.
func makePostgresURL(t *testing.T) string {
	u, err := url.Parse(postgresURL)
	if err != nil {
		panic(err)
	}
	uu, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	dbName := fmt.Sprintf("test_%s", strings.ReplaceAll(uu.String(), "-", "_"))
	u.Path = fmt.Sprintf("/%s", dbName)
	t.Cleanup(func() {
		u, err := url.Parse(postgresURL)
		if err != nil {
			panic(err)
		}
		u.Path = "/postgres"
		rootDial := postgres.Open(u.String())

		db, err := openDB(rootDial)
		if err != nil {
			t.Logf("Failed to open database: %v", err)
			return
		}

		// Drop the test database
		err = db.Exec(fmt.Sprintf("DROP DATABASE %s", dbName)).Error
		if err != nil {
			t.Logf("Failed to drop test database: %v", err)
		}
	})
	return u.String()
}

// makeTestDB opens a state database on dburl, closed when the test finishes:
// postgres will not drop a database a client is still connected to, and a
// slatedb-backed database holds its object store's writer claim until it
// closes.
func makeTestDB(t *testing.T, dburl string) *StatefulDB {
	t.Helper()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := MakeDB(t.Context(), &config.CLI{DBURL: dburl}, nil, mod)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, state.Close()) })
	return state
}

// makeSlateDBURL is a state database of its own on the test object store,
// deleted when the test finishes: the object-store counterpart of
// makePostgresURL's throwaway database. The bucket is created if the endpoint
// does not have it yet. It is empty when no object store is configured, in
// which case the caller skips.
func makeSlateDBURL(t *testing.T) string {
	t.Helper()
	raw := slatedbURL
	if raw == "" {
		raw = slatedbURLFromS3Env(t)
	}
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	require.NoError(t, err)
	u.Path = path.Join(u.Path, fmt.Sprintf("test-%08x", rand.Int64()))
	stateURL := u.String()
	ensureBucket(t, stateURL)
	t.Cleanup(func() { deletePrefix(t, stateURL) })
	return stateURL
}

// requireSlateDBURL is makeSlateDBURL for a test that cannot run without one.
func requireSlateDBURL(t *testing.T) string {
	t.Helper()
	stateURL := makeSlateDBURL(t)
	if stateURL == "" {
		t.Skip("no object store configured: set STREAMPLACE_TEST_SLATEDB_URL, or S3_ENDPOINT with S3_ACCESS_KEY_ID and S3_SECRET_ACCESS_KEY")
	}
	return stateURL
}

// slatedbURLFromS3Env builds a state URL from S3_ENDPOINT and friends, the way
// pkg/s3's live tests are pointed at an endpoint: the bucket is the endpoint's
// path, as the env files devs keep write it, or S3_BUCKET when the endpoint is
// a bare host. It is empty when those variables do not name an object store.
func slatedbURLFromS3Env(t *testing.T) string {
	t.Helper()
	endpoint := os.Getenv("S3_ENDPOINT")
	accessKey := os.Getenv("S3_ACCESS_KEY_ID")
	secret := os.Getenv("S3_SECRET_ACCESS_KEY")
	if endpoint == "" || accessKey == "" || secret == "" {
		return ""
	}
	u, err := url.Parse(endpoint)
	require.NoError(t, err)
	bucket := strings.Trim(u.Path, "/")
	if bucket == "" {
		bucket = os.Getenv("S3_BUCKET")
	}
	if bucket == "" {
		return ""
	}
	region := os.Getenv("S3_REGION")
	if region == "" {
		region = "auto"
	}
	query := url.Values{
		"endpoint":          {u.Scheme + "://" + u.Host},
		"region":            {region},
		"access_key_id":     {accessKey},
		"secret_access_key": {secret},
	}
	return (&url.URL{
		Scheme:   "slatedb+s3",
		Host:     bucket,
		Path:     "/state",
		RawQuery: query.Encode(),
	}).String()
}

// ensureBucket creates the bucket the state URL names when the endpoint does
// not have it yet, so a fresh object store needs no setup.
func ensureBucket(t *testing.T, stateURL string) {
	t.Helper()
	client, bucket := objectStoreClient(t, stateURL)
	_, err := client.HeadBucket(t.Context(), &awss3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err == nil {
		return
	}
	_, err = client.CreateBucket(t.Context(), &awss3.CreateBucketInput{Bucket: aws.String(bucket)})
	require.NoError(t, err, "creating bucket %s", bucket)
}

// listPrefix is the keys stored under the state database's path.
func listPrefix(t *testing.T, stateURL string) []string {
	t.Helper()
	client, bucket := objectStoreClient(t, stateURL)
	keys, err := listPrefixRaw(t.Context(), client, bucket, statePrefix(stateURL))
	require.NoError(t, err)
	return keys
}

// listPrefixRaw pages through every key under prefix.
func listPrefixRaw(ctx context.Context, client *awss3.Client, bucket, prefix string) ([]string, error) {
	var keys []string
	var token *string
	for {
		out, err := client.ListObjectsV2(ctx, &awss3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, err
		}
		for _, object := range out.Contents {
			keys = append(keys, aws.ToString(object.Key))
		}
		if out.IsTruncated == nil || !*out.IsTruncated {
			return keys, nil
		}
		token = out.NextContinuationToken
	}
}

// deletePrefix removes everything the state database stored. Object storage has
// no way to drop a database, so a test that wants its keys gone has to delete
// them itself. Failures are logged rather than reported: the test is over, and
// a state database too big to clean up does not mean it failed.
func deletePrefix(t *testing.T, stateURL string) {
	t.Helper()
	client, bucket := objectStoreClient(t, stateURL)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	keys, err := listPrefixRaw(ctx, client, bucket, statePrefix(stateURL))
	if err != nil {
		t.Logf("Failed to list state database objects: %v", err)
		return
	}
	for len(keys) > 0 {
		batch := keys
		if len(batch) > maxDeleteObjects {
			batch = batch[:maxDeleteObjects]
		}
		keys = keys[len(batch):]
		objects := make([]awss3types.ObjectIdentifier, 0, len(batch))
		for _, key := range batch {
			objects = append(objects, awss3types.ObjectIdentifier{Key: aws.String(key)})
		}
		_, err := client.DeleteObjects(ctx, &awss3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &awss3types.Delete{Objects: objects, Quiet: aws.Bool(true)},
		})
		if err != nil {
			t.Logf("Failed to delete state database objects: %v", err)
			return
		}
	}
}

// maxDeleteObjects is S3's ceiling on one delete request.
const maxDeleteObjects = 1000

// statePrefix is the object-store path the state URL's keys live under,
// slash-terminated so a neighbouring database is not swept up with it.
func statePrefix(stateURL string) string {
	u, err := url.Parse(stateURL)
	if err != nil {
		return ""
	}
	prefix := strings.TrimPrefix(u.Path, "/")
	if prefix == "" {
		return ""
	}
	return prefix + "/"
}

// objectStoreClient signs requests the same way the state URL does: the
// endpoint and credentials in its query string, and the bucket in its host.
func objectStoreClient(t *testing.T, stateURL string) (*awss3.Client, string) {
	t.Helper()
	u, err := url.Parse(stateURL)
	require.NoError(t, err)
	query := u.Query()
	cfg := s3.Config{
		Endpoint:        query.Get("endpoint"),
		Region:          query.Get("region"),
		AccessKeyID:     query.Get("access_key_id"),
		SecretAccessKey: query.Get("secret_access_key"),
		Bucket:          u.Host,
	}
	require.NotEmpty(t, cfg.Endpoint, "state URL needs an endpoint for the test to check its objects")
	return s3.NewClient(cfg), cfg.Bucket
}
