package rtmps

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
)

// passthrough RTMPS TLS terminator to external RTMP server
// tlsConfig may be nil, in which case the certificate files from the CLI
// are used; the ACME manager passes its own.
func ServeRTMPSAddon(ctx context.Context, cli *config.CLI, tlsConfig *tls.Config) error {
	if cli.RTMPServerAddon == "" {
		return fmt.Errorf("RTMP server address not configured")
	}

	if tlsConfig == nil {
		cert, err := tls.LoadX509KeyPair(cli.TLSCertPath, cli.TLSKeyPath)
		if err != nil {
			return fmt.Errorf("failed to load TLS certificate: %w", err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
	}

	listener, err := tls.Listen("tcp", cli.RTMPSAddonAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to create RTMPS listener: %w", err)
	}

	log.Log(ctx, "rtmps server starting",
		"addr", cli.RTMPSAddonAddr,
		"forwarding_to", cli.RTMPServerAddon)

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	opts := shadowOptions{}
	// Reaps every shadow worker before we return on shutdown; each worker is
	// killed when ctx is canceled.
	var shadows sync.WaitGroup
	defer shadows.Wait()
	opts.release = shadows.Done
	if cli.DuplicateMistTest {
		log.Warn(ctx, shadowTag+": ENABLED, teeing decrypted client RTMP bytes to a shadow parser worker for every connection; Mist remains the only responder")
	}
	var connID uint64

	for {
		conn, err := listener.Accept()
		if err != nil {
			// Check if the context was canceled, which means we're shutting down
			select {
			case <-ctx.Done():
				return nil
			default:
				log.Error(ctx, "error accepting RTMPS connection", "error", err)
				continue
			}
		}

		if cli.DuplicateMistTest {
			connID++
			shadows.Add(1)
			// Count before starting the goroutine so shutdown cannot race Add.
		}
		id := connID
		go func(clientConn net.Conn) {
			defer clientConn.Close()
			var sh *shadow
			if cli.DuplicateMistTest {
				// Complete TLS before allocating a ring or spawning a parser.
				// HandshakeContext also unblocks idle clients on shutdown.
				if err := clientConn.(*tls.Conn).HandshakeContext(ctx); err != nil {
					shadows.Done()
					if ctx.Err() == nil {
						log.Error(ctx, "error completing RTMPS handshake", "error", err)
					}
					return
				}
				sh = startShadow(ctx, opts, id, clientConn.RemoteAddr().String())
			}
			proxyConn(ctx, clientConn, cli.RTMPServerAddon, sh)
		}(conn)
	}
}

// proxyConn copies bytes between the client and the RTMP server until either
// side closes. sh, if non-nil, receives a copy of the client-to-server bytes
// only; it never writes to the client or the server.
func proxyConn(ctx context.Context, clientConn net.Conn, rtmpAddr string, sh *shadow) {
	rtmpConn, err := (&net.Dialer{}).DialContext(ctx, "tcp", rtmpAddr)
	if err != nil {
		log.Error(ctx, "failed to connect to RTMP server", "error", err)
		sh.finish(ctx)
		return
	}
	defer rtmpConn.Close()
	stop := context.AfterFunc(ctx, func() {
		_ = clientConn.Close()
		_ = rtmpConn.Close()
	})
	defer stop()

	// Create a wait group to wait for both copy operations to complete
	var wg sync.WaitGroup
	wg.Add(2)

	// Copy from client to RTMP server
	go func() {
		defer wg.Done()
		_, err := forwardClientToMist(ctx, rtmpConn, clientConn, sh)
		if err != nil && !errors.Is(err, io.EOF) {
			log.Error(ctx, "error copying from client to RTMP server", "error", err)
		}
		// Signal the other goroutine to stop by closing the connection
		rtmpConn.Close()
	}()

	// Copy from RTMP server to client
	go func() {
		defer wg.Done()
		_, err := io.Copy(clientConn, rtmpConn)
		if err != nil && !errors.Is(err, io.EOF) {
			log.Error(ctx, "error copying from RTMP server to client", "error", err)
		}
		// Signal the other goroutine to stop by closing the connection
		clientConn.Close()
	}()

	// Wait for both copy operations to complete
	wg.Wait()
}
