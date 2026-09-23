package cmd

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"stream.place/streamplace/pkg/log"
)

// e2eHTTPS gives the e2e harness real, locally trusted HTTPS, so the atproto
// OAuth flow can run against the local PDS and node unmodified. Neither end
// accepts less: the Go OAuth client wants a PDS issuer that is https:// with
// no port, the PDS's OAuth provider refuses client IDs and redirect URIs on
// .test/.local/.localhost names, and oatproxy and the browser OAuth client
// both resolve did:plc against a hardcoded https://plc.directory.
//
// So, given public DNS records pointing a PDS hostname, everything under it
// (account handles) and a station (broadcaster) hostname at 127.0.0.1, the
// harness does the following. The two must be on different registrable
// domains, as they are in production: the PDS refuses to show its sign-in
// page to a same-site navigation (Sec-Fetch-Site: same-site).
//
//   - mints a throwaway CA and one leaf for all of those and plc.directory;
//   - terminates TLS on 127.0.0.1:443 and routes by SNI: the PDS hostname and
//     handles to the dev-env PDS, the station hostname to the node's
//     plain-HTTP listener (it runs with --behind-https-proxy), plc.directory
//     to the dev-env PLC;
//   - has the PDS resolve lexicons, the OAuth permission sets among them,
//     from a local lexicon authority account that the harness publishes this
//     build's lexicons into (see publishLexicons in e2e.go);
//   - runs a CONNECT proxy that sends the names above to that listener and
//     everything else where it was going. The node (HTTPS_PROXY) and the
//     browser use it, which is how their hardcoded plc.directory lookups land
//     on the local PLC.
//
// Nothing is installed system-wide: each process is told to trust the CA on
// its own (SSL_CERT_DIR for Go, NODE_EXTRA_CA_CERTS for Node, the leaf's SPKI
// for Chromium), and the CA's key is deleted with the run.
type e2eHTTPS struct {
	pdsHost     string
	stationHost string
	dir         string
	caPath      string
	trustDir    string
	cert        tls.Certificate
	spki        string

	frontLn net.Listener // 127.0.0.1:443, or whatever loopback 443 redirects to
	proxyLn net.Listener // CONNECT proxy
}

const e2ePLCHost = "plc.directory"

// newE2EHTTPS mints the certificates and claims every listener up front, so a
// harness that cannot bind its port fails before it has started anything else.
//
// The front end listens on 127.0.0.1:port. The URLs it serves stay portless
// (https://<host>), so any port other than 443 only works on a machine that
// redirects loopback 443 to it, e.g. with an iptables REDIRECT rule.
func newE2EHTTPS(pdsHost, stationHost string, port int) (*e2eHTTPS, error) {
	h := &e2eHTTPS{pdsHost: pdsHost, stationHost: stationHost}
	var err error
	h.dir, err = os.MkdirTemp("", "streamplace-e2e-tls-*")
	if err != nil {
		return nil, err
	}
	if err := h.mintCerts(); err != nil {
		h.Close()
		return nil, fmt.Errorf("mint e2e certificates: %w", err)
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	h.frontLn, err = net.Listen("tcp", addr)
	if err != nil {
		h.Close()
		fix := "run as root (e.g. in the build container)"
		if errors.Is(err, syscall.EACCES) && runtime.GOOS == "linux" {
			fix = fmt.Sprintf("run as root, or let unprivileged processes bind it until the next reboot with `sudo sysctl -w net.ipv4.ip_unprivileged_port_start=%d`", port)
		}
		return nil, fmt.Errorf("listen on %s: %w\nOAuth needs portless https hosts: %s; or leave the --https-* hostnames empty (E2E_HTTPS_PDS_HOSTNAME= for the hack/ runners) to skip HTTPS", addr, err, fix)
	}
	if h.proxyLn, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
		h.Close()
		return nil, err
	}
	return h, nil
}

func (h *e2eHTTPS) Close() {
	for _, ln := range []net.Listener{h.frontLn, h.proxyLn} {
		if ln != nil {
			ln.Close() //nolint:errcheck
		}
	}
	if h.dir != "" {
		os.RemoveAll(h.dir) //nolint:errcheck
	}
}

// StationURL and PDSURL are the public https URLs users visit.
func (h *e2eHTTPS) StationURL() string { return "https://" + h.stationHost }
func (h *e2eHTTPS) PDSURL() string     { return "https://" + h.pdsHost }

func (h *e2eHTTPS) ProxyURL() string { return "http://" + h.proxyLn.Addr().String() }

// DevEnvEnv is the extra environment for js/dev-env: the PDS's public
// hostname, and trust in our CA for what it fetches over https (the node's
// OAuth client metadata, lexicons from its own public URL).
func (h *e2eHTTPS) DevEnvEnv() []string {
	return []string{
		"DEV_ENV_PDS_HOSTNAME=" + h.pdsHost,
		"NODE_EXTRA_CA_CERTS=" + h.caPath,
	}
}

// NodeEnv is the extra environment for the forked node, whose broadcaster
// host is the station hostname. Go reads SSL_CERT_DIR on top of the system
// bundle file, so public roots still work.
func (h *e2eHTTPS) NodeEnv() []string {
	return []string{
		"SP_BEHIND_HTTPS_PROXY=true",
		"SSL_CERT_DIR=" + h.trustDir,
		"HTTPS_PROXY=" + h.ProxyURL(),
	}
}

// Serve starts the TLS listeners and the proxy. The backends are the dev-env
// PDS and PLC and the node's plain-HTTP listener, as host:port.
func (h *e2eHTTPS) Serve(ctx context.Context, pdsAddr, plcAddr, nodeAddr string) {
	route := func(name string) string {
		name = strings.ToLower(strings.TrimSuffix(name, "."))
		switch {
		case name == e2ePLCHost:
			return plcAddr
		case name == h.stationHost:
			return nodeAddr
		case name == h.pdsHost || strings.HasSuffix(name, "."+h.pdsHost):
			return pdsAddr
		}
		return ""
	}
	go serveTLSPipe(ctx, h.frontLn, h.cert, route)
	go serveConnectProxy(ctx, h.proxyLn, func(hostport string) string {
		host, port, err := net.SplitHostPort(hostport)
		if err == nil && port == "443" && route(host) != "" {
			return h.frontLn.Addr().String()
		}
		return hostport
	})
}

func (h *e2eHTTPS) mintCerts() error {
	now := time.Now()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          randomSerial(),
		Subject:               pkix.Name{CommonName: "Streamplace e2e throwaway CA"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(7 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return err
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		return err
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: randomSerial(),
		Subject:      pkix.Name{CommonName: h.pdsHost},
		DNSNames:     []string{h.pdsHost, "*." + h.pdsHost, h.stationHost, e2ePLCHost},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(7 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, ca, &leafKey.PublicKey, caKey)
	if err != nil {
		return err
	}
	leaf, err := x509.ParseCertificate(leafDER)
	if err != nil {
		return err
	}
	h.cert = tls.Certificate{Certificate: [][]byte{leafDER}, PrivateKey: leafKey, Leaf: leaf}
	sum := sha256.Sum256(leaf.RawSubjectPublicKeyInfo)
	h.spki = base64.StdEncoding.EncodeToString(sum[:])

	// The trust dir holds the CA and nothing else: Go parses every file in an
	// SSL_CERT_DIR entry.
	h.trustDir = filepath.Join(h.dir, "trust")
	if err := os.Mkdir(h.trustDir, 0o700); err != nil {
		return err
	}
	h.caPath = filepath.Join(h.trustDir, "ca.pem")
	return os.WriteFile(h.caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0o600)
}

func randomSerial() *big.Int {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		panic(err)
	}
	return n
}

// serveTLSPipe terminates TLS on ln and pipes the plaintext, byte for byte, to
// the backend route picks from the SNI name. Piping rather than proxying HTTP
// keeps websockets working and leaves requests exactly as the client sent
// them, Host header included (the PDS answers handle lookups by Host).
func serveTLSPipe(ctx context.Context, ln net.Listener, cert tls.Certificate, route func(sni string) string) {
	tlsLn := tls.NewListener(ln, &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})
	for {
		conn, err := tlsLn.Accept()
		if err != nil {
			if ctx.Err() == nil && !errors.Is(err, net.ErrClosed) {
				log.Log(ctx, "e2e tls: accept failed", "addr", ln.Addr(), "err", err)
			}
			return
		}
		go func() {
			tc := conn.(*tls.Conn)
			hsCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			if err := tc.HandshakeContext(hsCtx); err != nil {
				conn.Close() //nolint:errcheck
				return
			}
			sni := tc.ConnectionState().ServerName
			backend := route(sni)
			if backend == "" {
				log.Log(ctx, "e2e tls: no backend for server name", "sni", sni, "addr", ln.Addr())
				conn.Close() //nolint:errcheck
				return
			}
			upstream, err := net.Dial("tcp", backend)
			if err != nil {
				log.Log(ctx, "e2e tls: backend dial failed", "sni", sni, "backend", backend, "err", err)
				conn.Close() //nolint:errcheck
				return
			}
			pipeConns(conn, upstream)
		}()
	}
}

// serveConnectProxy is an HTTP CONNECT proxy whose only trick is resolve,
// which may send a host:port somewhere other than DNS would.
func serveConnectProxy(ctx context.Context, ln net.Listener, resolve func(hostport string) string) {
	srv := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodConnect {
				http.Error(w, "this proxy only speaks CONNECT", http.StatusMethodNotAllowed)
				return
			}
			upstream, err := net.DialTimeout("tcp", resolve(r.Host), 10*time.Second)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			hj, ok := w.(http.Hijacker)
			if !ok {
				upstream.Close() //nolint:errcheck
				http.Error(w, "cannot hijack", http.StatusInternalServerError)
				return
			}
			conn, buf, err := hj.Hijack()
			if err != nil {
				upstream.Close() //nolint:errcheck
				return
			}
			if _, err := conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n")); err != nil {
				conn.Close()     //nolint:errcheck
				upstream.Close() //nolint:errcheck
				return
			}
			pipeConns(&bufferedConn{Conn: conn, r: buf.Reader}, upstream)
		}),
	}
	go func() {
		<-ctx.Done()
		srv.Close() //nolint:errcheck
	}()
	err := srv.Serve(ln)
	if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) && ctx.Err() == nil {
		log.Log(ctx, "e2e proxy: serve failed", "err", err)
	}
}

// bufferedConn reads through the bufio.Reader a hijacked connection came
// with, in case the client pipelined bytes behind its CONNECT.
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

// pipeConns copies both ways until either side is done, then closes both.
func pipeConns(a, b net.Conn) {
	var once sync.Once
	closeBoth := func() {
		a.Close() //nolint:errcheck
		b.Close() //nolint:errcheck
	}
	go func() {
		io.Copy(a, b) //nolint:errcheck
		once.Do(closeBoth)
	}()
	io.Copy(b, a) //nolint:errcheck
	once.Do(closeBoth)
}
