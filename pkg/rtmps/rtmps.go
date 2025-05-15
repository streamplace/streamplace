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
func ServeRTMPS(ctx context.Context, cli *config.CLI) error {
	if cli.RTMPServerAddon == "" {
		return fmt.Errorf("RTMP server address not configured")
	}

	cert, err := tls.LoadX509KeyPair(cli.TLSCertPath, cli.TLSKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load TLS certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	listener, err := tls.Listen("tcp", cli.RtmpsAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to create RTMPS listener: %w", err)
	}

	log.Log(ctx, "rtmps server starting",
		"addr", cli.RtmpsAddr,
		"forwarding_to", cli.RTMPServerAddon)

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

			rtmpConn, err := net.Dial("tcp", cli.RTMPServerAddon)
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
				_, err := io.Copy(rtmpConn, clientConn)
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
