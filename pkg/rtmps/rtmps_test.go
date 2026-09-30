package rtmps

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/bluenviron/gortmplib"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/spmetrics"
)

func selfSignedTLSConfig(t *testing.T) *tls.Config {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "rtmps test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
		MinVersion:   tls.VersionTLS12,
	}
}

// fakeMist is a plain RTMP server standing in for MistServer: it accepts
// publishes and reports the URL each one published to.
func fakeMist(t *testing.T) (addr string, published chan *url.URL) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() })
	published = make(chan *url.URL, 10)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				sc := &gortmplib.ServerConn{RW: conn}
				if err := sc.Initialize(); err != nil {
					return
				}
				if err := sc.Accept(); err != nil {
					return
				}
				published <- sc.URL
				_, _ = io.Copy(io.Discard, conn)
			}()
		}
	}()
	return ln.Addr().String(), published
}

func TestRTMPSAddonTracksIngestHost(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend, published := fakeMist(t)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", selfSignedTLSConfig(t))
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	ingest := NewIngestHosts([]string{"stream.place", "localhost"})
	const streamer = "did:plc:streamer"
	resolve := func(ctx context.Context, key string) (string, error) {
		if key != "thekey" {
			return "", fmt.Errorf("unknown key %q", key)
		}
		return streamer, nil
	}
	done := make(chan error, 1)
	go func() { done <- serveRTMPSAddon(ctx, ln, backend, ingest, resolve) }()

	for _, tc := range []struct {
		name string
		// the server name the client sends, and the host it dials (which
		// becomes its tcUrl)
		sni, dialHost string
		want          string
	}{
		{name: "deprecated server name", sni: "stream.place", dialHost: "127.0.0.1", want: "stream.place"},
		{name: "deprecated tcUrl", sni: "rtmp.stream.place", dialHost: "localhost", want: "localhost"},
		{name: "current host", sni: "rtmp.stream.place", dialHost: "127.0.0.1", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gauge := spmetrics.RTMPIngestConnections.WithLabelValues(ListenerRTMPSMist, fmt.Sprint(tc.want != ""))
			base := gaugeValue(t, gauge)

			u, err := url.Parse(fmt.Sprintf("rtmps://%s/live/thekey", net.JoinHostPort(tc.dialHost, port)))
			require.NoError(t, err)
			c := &gortmplib.Client{
				URL:       u,
				TLSConfig: &tls.Config{ServerName: tc.sni, InsecureSkipVerify: true}, //nolint:gosec // self-signed test cert
				Publish:   true,
			}
			require.NoError(t, c.Initialize(ctx))
			defer c.Close()

			// the backend saw the publish intact through the terminator
			select {
			case got := <-published:
				require.Equal(t, "/live/thekey", got.Path)
			case <-time.After(10 * time.Second):
				t.Fatal("backend never saw the publish")
			}

			require.Eventually(t, func() bool {
				return gaugeValue(t, gauge) == base+1
			}, 5*time.Second, 10*time.Millisecond)
			// counted only once its streamer is recorded
			require.Equal(t, tc.want, ingest.StreamerDeprecatedHost(streamer))

			c.Close()
			require.Eventually(t, func() bool {
				return gaugeValue(t, gauge) == base && ingest.StreamerDeprecatedHost(streamer) == ""
			}, 5*time.Second, 10*time.Millisecond)
		})
	}

	cancel()
	require.NoError(t, <-done)
}

// A key nobody has indexed mustn't hold resources past its connection: the
// lookup is cancelled when the client goes, and the connection isn't counted.
func TestRTMPSAddonCancelsResolveOnDisconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend, published := fakeMist(t)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", selfSignedTLSConfig(t))
	require.NoError(t, err)

	resolving := make(chan struct{})
	cancelled := make(chan struct{})
	resolve := func(ctx context.Context, key string) (string, error) {
		close(resolving)
		<-ctx.Done()
		close(cancelled)
		return "", ctx.Err()
	}
	ingest := NewIngestHosts([]string{"stream.place"})
	go func() { _ = serveRTMPSAddon(ctx, ln, backend, ingest, resolve) }()

	gauge := spmetrics.RTMPIngestConnections.WithLabelValues(ListenerRTMPSMist, "true")
	base := gaugeValue(t, gauge)

	u, err := url.Parse(fmt.Sprintf("rtmps://%s/live/unindexed", ln.Addr()))
	require.NoError(t, err)
	c := &gortmplib.Client{
		URL:       u,
		TLSConfig: &tls.Config{ServerName: "stream.place", InsecureSkipVerify: true}, //nolint:gosec // self-signed test cert
		Publish:   true,
	}
	require.NoError(t, c.Initialize(ctx))
	<-published
	<-resolving
	c.Close()

	select {
	case <-cancelled:
	case <-time.After(10 * time.Second):
		t.Fatal("stream key lookup outlived its connection")
	}
	require.Never(t, func() bool { return gaugeValue(t, gauge) != base }, 300*time.Millisecond, 10*time.Millisecond)
}
