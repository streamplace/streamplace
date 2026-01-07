package aqhttp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// TrustedTransport is a basic transport that adds User-Agent headers.
// Use this for trusted infrastructure endpoints where SSRF is not a concern.
type TrustedTransport struct {
	Base http.RoundTripper
}

func (t *TrustedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("User-Agent", UserAgent)
	return t.Base.RoundTrip(req)
}

// NewTrustedTransport creates a transport for trusted endpoints.
func NewTrustedTransport() *TrustedTransport {
	return &TrustedTransport{
		Base: &http.Transport{
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

// UntrustedTransport validates destination IPs using DNS-over-HTTPS before connecting.
// Prevents SSRF attacks by blocking private, loopback, and bogon IP ranges.
type UntrustedTransport struct {
	Base     http.RoundTripper
	resolver *DoHResolver
}

func (t *UntrustedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("User-Agent", UserAgent)
	return t.Base.RoundTrip(req)
}

// NewUntrustedTransport creates a transport that validates all destination IPs.
func NewUntrustedTransport() *UntrustedTransport {
	resolver := NewDoHResolver("")

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse address: %w", err)
			}

			// Resolve IPv4 addresses using DoH
			ipv4Addrs, _ := resolver.Resolve(host, TypeA)
			var validIP string

			// Check IPv4 addresses first
			for _, ip := range ipv4Addrs {
				if !resolver.IsInvalidIP(ip) {
					validIP = ip
					break
				}
			}

			// Fall back to IPv6 if no valid IPv4
			if validIP == "" {
				ipv6Addrs, _ := resolver.Resolve(host, TypeAAAA)
				for _, ip := range ipv6Addrs {
					if !resolver.IsInvalidIP(ip) {
						validIP = ip
						break
					}
				}
			}

			if validIP == "" {
				return nil, fmt.Errorf("all resolved IPs for %s are private/invalid", host)
			}

			// Dial using the validated IP
			targetAddr := net.JoinHostPort(validIP, port)
			return dialer.DialContext(ctx, network, targetAddr)
		},
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &UntrustedTransport{
		Base:     transport,
		resolver: resolver,
	}
}
