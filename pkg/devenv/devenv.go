package devenv

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
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
	PDSURL string `json:"pds-url"`
	PLCURL string `json:"plc-url"`
}

func WithDevEnv(t *testing.T) *DevEnv {
	// Pick a random port for the proxy to listen on
	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start TLS proxy listener: %v", err)
	}
	proxyAddr := proxyListener.Addr().String()
	_, proxyPort, err := net.SplitHostPort(proxyAddr)
	if err != nil {
		t.Fatalf("failed to split host and port: %v", err)
	}

	_, filename, _, _ := runtime.Caller(0)
	cmd := exec.Command("node", "../../js/dev-env/run.mjs")
	cmd.Dir = filepath.Dir(filename)
	// INSERT_YOUR_CODE
	pdsPort := 30000 + rand.Intn(10000)
	plcPort := pdsPort + 1
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("TEST_ENV_PDS_PUBLIC_URL=https://localhost.iameli.xyz:%s", proxyPort),
		fmt.Sprintf("TEST_ENV_PDS_PORT=%d", pdsPort),
		fmt.Sprintf("TEST_ENV_PLC_PORT=%d", plcPort),
		fmt.Sprintf("TEST_ENV_PDS_HOSTNAME=localhost.iameli.xyz:%s", proxyPort),
	)

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

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			t.Logf("dev-env stdout: %s", scanner.Text())
			if scanner.Err() != nil {
				return
			}
		}
	}()

	// INSERT_YOUR_CODE

	// Load TLS cert and key
	certFile := filepath.Join("/shared", "tls", "tls.crt")
	keyFile := filepath.Join("/shared", "tls", "tls.key")
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("failed to load TLS cert: %v", err)
	}
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}

	// Forward requests to the dev PDS URL, including websocket support
	targetURL := fmt.Sprintf("http://127.0.0.1:%d", pdsPort)
	go func() {
		server := &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				u, err := url.Parse(targetURL)
				if err != nil {
					http.Error(w, "bad upstream: "+err.Error(), http.StatusInternalServerError)
					return
				}
				// Handle WebSockets upgrade
				if strings.EqualFold(r.Header.Get("Connection"), "Upgrade") &&
					strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
					// Extract underlying net.Conn from the ResponseWriter
					hj, ok := w.(http.Hijacker)
					if !ok {
						http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
						return
					}
					clientConn, clientBuf, err := hj.Hijack()
					if err != nil {
						http.Error(w, fmt.Sprintf("hijack failed: %v", err), http.StatusInternalServerError)
						return
					}
					defer clientConn.Close()

					backendHost := u.Host
					if !strings.Contains(backendHost, ":") {
						backendHost += ":80"
					}
					backendConn, err := net.Dial("tcp", backendHost)
					if err != nil {
						clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
						return
					}
					defer backendConn.Close()

					// Rewrite request line with backend host
					r.URL.Scheme = u.Scheme
					// r.URL.Host = u.Host
					r.RequestURI = "" // let Go set this
					// r.Host = u.Host

					// Copy the original request to backendConn
					err = r.Write(backendConn)
					if err != nil {
						return
					}

					// If clientBuf has unread buffered data, write it to backend
					if clientBuf.Reader.Buffered() > 0 {
						bufData, _ := clientBuf.Reader.Peek(clientBuf.Reader.Buffered())
						backendConn.Write(bufData)
					}

					// Proxy data between client and backend
					errc := make(chan error, 2)
					go func() {
						_, err := io.Copy(backendConn, clientConn)
						errc <- err
					}()
					go func() {
						_, err := io.Copy(clientConn, backendConn)
						errc <- err
					}()
					<-errc
					return
				}

				// HTTP request forwarding
				r.URL.Scheme = u.Scheme
				r.URL.Host = u.Host
				r.RequestURI = ""
				r.Host = u.Host

				// Copy the request to the backend server
				resp, err := http.DefaultTransport.RoundTrip(r)
				if err != nil {
					http.Error(w, "upstream error: "+err.Error(), http.StatusBadGateway)
					return
				}
				defer resp.Body.Close()

				for k, vv := range resp.Header {
					for _, v := range vv {
						w.Header().Add(k, v)
					}
				}
				w.WriteHeader(resp.StatusCode)
				io.Copy(w, resp.Body)
			}),
			TLSConfig: tlsConfig,
		}
		_ = server.Serve(tls.NewListener(proxyListener, tlsConfig))
	}()

	env.PDSURL = fmt.Sprintf("https://localhost.iameli.xyz:%s", proxyPort)

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

	return &DevEnvAccount{
		Handle:   out.Handle,
		Email:    email,
		Password: password,
		DID:      out.Did,
		XRPC:     xrpcc,
	}
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
