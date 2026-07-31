package atproto

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
)

// TestSelfRelayURLSecure pins the address the self-subscription dials. Under
// --secure the handler lives on HTTPSAddr and HTTPAddr serves only redirects,
// so a ws://<HTTPAddr> self-relay dials the redirect handler and every
// handshake fails ("websocket: bad handshake") — which silently stops the
// server repo's own place.stream.media.origin records from ever being indexed.
func TestSelfRelayURLSecure(t *testing.T) {
	for _, tt := range []struct {
		name   string
		cli    config.CLI
		expect string
	}{
		{
			name:   "plain http",
			cli:    config.CLI{HTTPAddr: ":38080", HTTPSAddr: ":38443"},
			expect: "ws://127.0.0.1:38080",
		},
		{
			name:   "secure uses the https listener",
			cli:    config.CLI{HTTPAddr: ":38080", HTTPSAddr: ":38443", Secure: true},
			expect: "wss://127.0.0.1:38443",
		},
		{
			// Behind a TLS-terminating proxy we really do serve the handler as
			// plain HTTP on HTTPAddr, so this must stay ws://.
			name:   "behind https proxy stays plain",
			cli:    config.CLI{HTTPAddr: ":38080", HTTPSAddr: ":38443", BehindHTTPSProxy: true},
			expect: "ws://127.0.0.1:38080",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			atsync := &ATProtoSynchronizer{CLI: &tt.cli}
			require.Equal(t, tt.expect, atsync.selfRelayURL())
		})
	}
}

// TestConnectRelaySelfDialsOwnTLSListener is the end-to-end regression for the
// --secure self-subscription: we must complete a wss handshake against our own
// listener even though its certificate is issued for ServerHost and we dial it
// by loopback IP, and we must still present Host: ServerHost so we land on the
// server-repo firehose rather than the broadcaster one.
func TestConnectRelaySelfDialsOwnTLSListener(t *testing.T) {
	const serverHost = "fairway-secure.example"

	// A certificate for serverHost and nothing else: no IP SAN, so verifying it
	// against the 127.0.0.1 we dial fails. This is what a real deployment has.
	cert := selfSignedCertFor(t, serverHost)

	gotHost := make(chan string, 1)
	upgrader := websocket.Upgrader{}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost <- r.Host
		con, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		// Handshake is all this test cares about; drop the stream immediately
		// so connectRelay returns instead of blocking on HandleRepoStream.
		con.Close()
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	srv.StartTLS()
	defer srv.Close()

	_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	require.NoError(t, err)

	cli := config.CLI{
		HTTPAddr:   ":38080",
		HTTPSAddr:  "127.0.0.1:" + port,
		Secure:     true,
		ServerHost: serverHost,
	}
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	atsync := &ATProtoSynchronizer{CLI: &cli, Model: mod}

	relay := atsync.selfRelayURL()
	require.Equal(t, "wss://127.0.0.1:"+port, relay)

	// Control: the stock dialer cannot verify this cert against 127.0.0.1, so
	// without the self-dial exemption the handshake never happens. Without this
	// the test would still pass if InsecureSkipVerify were dropped.
	_, _, err = websocket.DefaultDialer.Dial(relay+"/xrpc/com.atproto.sync.subscribeRepos", nil)
	require.Error(t, err, "cert must not verify against the dialed IP, else this test proves nothing")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// connectRelay returns an error once the (immediately closed) stream ends;
	// what matters is that it got past the dial.
	err = atsync.connectRelay(ctx, relay, atsync.newRelayCursor(ctx, relay))
	if err != nil {
		require.NotContains(t, err.Error(), "(dialing)",
			"self-subscription failed at the TLS/websocket handshake")
	}

	select {
	case host := <-gotHost:
		require.Equal(t, serverHost, host,
			"self-subscription must present Host: ServerHost to reach the server-repo firehose")
	default:
		t.Fatal("listener never saw the request")
	}
}

func selfSignedCertFor(t *testing.T, dnsName string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: dnsName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{dnsName}, // deliberately no IPAddresses
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}
