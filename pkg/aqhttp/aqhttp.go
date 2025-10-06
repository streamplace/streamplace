package aqhttp

import (
	"net"
	"net/http"
)

var Client http.Client
var UserAgent string = "streamplace/unknown"
var bogonRanges []*net.IPNet

type AddHeaderTransport struct {
	T http.RoundTripper
}

func (adt *AddHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("User-Agent", UserAgent)
	return adt.T.RoundTrip(req)
}

func init() {
	// Smokescreen doesn't provide a complete list of bogon IP ranges, so we define our own.
	ipv4Bogons := []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8",
		"169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24",
		"192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24",
		"224.0.0.0/4", "240.0.0.0/4", "255.255.255.255/32",
	}

	ipv6Bogons := []string{
		"::/128", "::1/128", "::ffff:0:0/96", "100::/64", "2001::/32",
		"2001:10::/28", "2001:db8::/32", "fc00::/7", "fe80::/10", "ff00::/8",
	}

	// Combine and parse the ranges
	cidrs := append(ipv4Bogons, ipv6Bogons...)
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			bogonRanges = append(bogonRanges, network)
		}
	}

	Client = http.Client{
		Transport: NewSSRFSafeTransport(),
	}
}

// IsIPAllowed checks if an IP is a bogon. It returns true if the IP is allowed.
func IsIPAllowed(ip net.IP) bool {
	if ip == nil {
		return false
	}

	for _, network := range bogonRanges {
		if network.Contains(ip) {
			return false // It's a bogon, so it's not allowed.
		}
	}

	return true // Not found in bogon list, so it's allowed.
}
