package devenv

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/cenkalti/backoff"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/log"
)

type DevEnv struct {
	PDSURL   string           `json:"pds-url"`
	PLCURL   string           `json:"plc-url"`
	Accounts []*DevEnvAccount `json:"accounts"`
}

func WithDevEnv(t *testing.T) *DevEnv {
	_, filename, _, _ := runtime.Caller(0)
	cmd := exec.Command("node", "../../js/dev-env/run.mjs")
	cmd.Dir = filepath.Dir(filename)

	// Start the command and get pipes for streaming output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Logf("Error getting stdout pipe: %v", err)
		t.FailNow()
	}

	if err := cmd.Start(); err != nil {
		t.Logf("Error starting dev env: %v", err)
		t.FailNow()
	}

	var env DevEnv

	scanner := bufio.NewScanner(stdout)
	scanner.Scan()
	err = json.Unmarshal(scanner.Bytes(), &env)
	if err != nil {
		t.Logf("Error unmarshalling dev-env stdout: %v", err)
		t.FailNow()
	}
	env.Accounts = []*DevEnvAccount{}

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			t.Logf("dev-env stdout: %s", scanner.Text())
			if scanner.Err() != nil {
				return
			}
		}
	}()

	// Ensure cleanup happens when test finishes
	t.Cleanup(func() {
		t.Logf("killing dev env")
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	return &env
}

type DevEnvAccount struct {
	Handle   string
	Email    string
	Password string
	DID      string
	XRPC     *xrpc.Client
}

func (d *DevEnv) CreateAccount(t *testing.T) *DevEnvAccount {

	xrpcc := &xrpc.Client{
		Host:   d.PDSURL,
		Client: &aqhttp.Client,
	}

	uu, err := uuid.NewRandom()
	require.NoError(t, err)

	handle := fmt.Sprintf("sp-%s.test", uu.String()[:8])
	email := fmt.Sprintf("%s@example.com", handle)
	password := "test"

	out, err := comatproto.ServerCreateAccount(context.Background(), xrpcc, &comatproto.ServerCreateAccount_Input{
		Handle:   handle,
		Email:    &email,
		Password: &password,
	})
	require.NoError(t, err)
	log.Log(context.Background(), "created account", "did", out.Did, "handle", out.Handle)

	session, err := comatproto.ServerCreateSession(context.Background(), xrpcc, &comatproto.ServerCreateSession_Input{
		Identifier: out.Handle,
		Password:   password,
	})
	require.NoError(t, err)

	xrpcc = &xrpc.Client{
		Host:   d.PDSURL,
		Client: &aqhttp.Client,
		Auth: &xrpc.AuthInfo{
			Did:        out.Did,
			AccessJwt:  session.AccessJwt,
			RefreshJwt: session.RefreshJwt,
			Handle:     out.Handle,
		},
	}
	acct := &DevEnvAccount{
		Handle:   out.Handle,
		Email:    email,
		Password: password,
		DID:      out.Did,
		XRPC:     xrpcc,
	}
	d.Accounts = append(d.Accounts, acct)
	return acct
}

// Custom RoundTripper for intercepting .test domain requests
type TestRoundTripper struct {
	DevEnv *DevEnv
}

func (d *DevEnv) TestHTTPClient() *http.Client {
	return &http.Client{
		Transport: d.TestRoundTripper(),
	}
}

func (d *DevEnv) TestRoundTripper() *TestRoundTripper {
	return &TestRoundTripper{DevEnv: d}
}

func (rt *TestRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.HasSuffix(req.URL.Hostname(), ".test") {
		log.Log(context.Background(), "intercepting .test domain request", "url", req.URL.String())
		upstreamURL := fmt.Sprintf("%s%s", rt.DevEnv.PDSURL, req.URL.Path)
		upstreamReq, err := http.NewRequest(req.Method, upstreamURL, req.Body)
		if err != nil {
			return nil, err
		}
		upstreamReq.Header = req.Header
		upstreamReq.Host = req.URL.Hostname()
		upstreamResp, err := http.DefaultTransport.RoundTrip(upstreamReq)
		if err != nil {
			return nil, err
		}
		return upstreamResp, nil
	}
	// For non-.test domains, use the default transport
	return http.DefaultTransport.RoundTrip(req)
}

func (d *DevEnv) TestDirectory() identity.Directory {
	// We need to create a new directory with our custom client
	base := identity.BaseDirectory{
		PLCURL:     d.PLCURL,
		HTTPClient: *d.TestHTTPClient(),
		Resolver: net.Resolver{
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: time.Second * 3}
				return d.DialContext(ctx, network, address)
			},
		},
		TryAuthoritativeDNS:   true,
		SkipDNSDomainSuffixes: []string{".bsky.social"},
	}
	return &base
}

// More aggressive backoff for tests
func NewExponentialBackOff() *backoff.ExponentialBackOff {
	b := &backoff.ExponentialBackOff{
		InitialInterval:     100 * time.Millisecond,
		RandomizationFactor: backoff.DefaultRandomizationFactor,
		Multiplier:          backoff.DefaultMultiplier,
		MaxInterval:         2 * time.Second,
		MaxElapsedTime:      10 * time.Second,
		Clock:               backoff.SystemClock,
	}
	b.Reset()
	return b
}
