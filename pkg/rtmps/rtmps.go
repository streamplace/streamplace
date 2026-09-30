package rtmps

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
)

// handshakeTimeout bounds how long a client has to complete its TLS handshake.
const handshakeTimeout = 10 * time.Second

// resolveTimeout bounds how long the terminator waits to learn whose stream
// key a connection published with.
const resolveTimeout = 30 * time.Second

// StreamerForKey resolves an RTMP stream key to the DID of the streamer it
// belongs to.
type StreamerForKey func(ctx context.Context, streamKey string) (string, error)

// passthrough RTMPS TLS terminator to external RTMP server
// tlsConfig may be nil, in which case the certificate files from the CLI
// are used; the ACME manager passes its own.
//
// The external server only sees plain RTMP from us, so the TLS server name
// the encoder used is known only here. To report it in ingest (see
// IngestHosts), the terminator reads the client's connect and publish
// commands as they pass through, and resolves the stream key with
// streamerForKey.
func ServeRTMPSAddon(ctx context.Context, cli *config.CLI, tlsConfig *tls.Config, ingest *IngestHosts, streamerForKey StreamerForKey) error {
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

	return serveRTMPSAddon(ctx, listener, cli.RTMPServerAddon, ingest, streamerForKey)
}

// serveRTMPSAddon relays each connection accepted on the TLS listener to the
// RTMP server at backend until ctx ends.
func serveRTMPSAddon(ctx context.Context, listener net.Listener, backend string, ingest *IngestHosts, streamerForKey StreamerForKey) error {
	go func() {
		<-ctx.Done()
		listener.Close()
	}()

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

		go func(clientConn net.Conn) {
			defer clientConn.Close()

			// Handshake up front, rather than lazily on first read, so the
			// server name is known before any RTMP flows.
			var sni string
			if tc, ok := clientConn.(*tls.Conn); ok {
				hctx, cancel := context.WithTimeout(ctx, handshakeTimeout)
				err := tc.HandshakeContext(hctx)
				cancel()
				if err != nil {
					log.Debug(ctx, "RTMPS TLS handshake failed", "error", err, "remote", clientConn.RemoteAddr())
					return
				}
				sni = tc.ConnectionState().ServerName
			}

			rtmpConn, err := net.Dial("tcp", backend)
			if err != nil {
				log.Error(ctx, "failed to connect to RTMP server", "error", err)
				return
			}
			defer rtmpConn.Close()

			// Create a wait group to wait for both copy operations to complete
			var wg sync.WaitGroup
			wg.Add(2)

			// Copy from client to RTMP server
			go func() {
				defer wg.Done()
				// connCtx ends with the connection, and with it any key
				// lookup still running for it.
				connCtx, connDone := context.WithCancel(ctx)
				defer connDone()
				// Everything PeekPublish reads is forwarded as it goes, so
				// the copy below picks up exactly where it stopped.
				info, err := PeekPublish(io.TeeReader(clientConn, rtmpConn))
				if err != nil {
					log.Debug(ctx, "not tracking RTMPS connection", "error", err, "sni", sni)
				} else {
					// Resolving the key can wait on the streamer's repo
					// being indexed, so do it beside the relay rather than
					// in its way.
					go func() {
						var streamer string
						if streamerForKey != nil {
							rctx, cancel := context.WithTimeout(connCtx, resolveTimeout)
							var err error
							streamer, err = streamerForKey(rctx, info.StreamKey)
							cancel()
							if err != nil {
								log.Debug(ctx, "could not resolve RTMPS stream key", "error", err, "sni", sni)
							}
						}
						if connCtx.Err() != nil {
							return // gone before we knew whose it was
						}
						release := ingest.Open(ctx, ListenerRTMPSMist, sni, info.TCURL, streamer)
						<-connCtx.Done()
						release()
					}()
				}
				_, err = io.Copy(rtmpConn, clientConn)
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
		}(conn)
	}
}
