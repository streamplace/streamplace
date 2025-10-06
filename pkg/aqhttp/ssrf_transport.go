package aqhttp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// SSRFSafeTransport is an http.RoundTripper that validates destination IPs
// before connecting, preventing SSRF attacks via webhooks
type SSRFSafeTransport struct {
	Base *AddHeaderTransport
}

func (t *SSRFSafeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.Base.RoundTrip(req)
}

// NewSSRFSafeTransport creates a transport with custom dialer that validates IPs
func NewSSRFSafeTransport() *SSRFSafeTransport {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Resolve the address
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse address: %w", err)
			}

			// Resolve IPs for the hostname
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve hostname: %w", err)
			}

			if len(ips) == 0 {
				return nil, fmt.Errorf("no IP addresses found for %s", host)
			}

			// Validate each resolved IP
			var validIP net.IP
			for _, ip := range ips {
				if IsIPAllowed(ip) {
					validIP = ip
					break
				}
			}

			if validIP == nil {
				return nil, fmt.Errorf("all resolved IPs for %s are private/invalid", host)
			}

			// Dial using the validated IP
			targetAddr := net.JoinHostPort(validIP.String(), port)
			return dialer.DialContext(ctx, network, targetAddr)
		},
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &SSRFSafeTransport{
		Base: &AddHeaderTransport{T: transport},
	}
}
