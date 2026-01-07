package aqhttp

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	TypeA    = 1  // IPv4
	TypeAAAA = 28 // IPv6
)

type dnsRecord struct {
	ips       []string
	expiresAt time.Time
}

type DoHResolver struct {
	Server        string
	Client        *http.Client
	invalidRanges []*net.IPNet
	cache         map[string]*dnsRecord
	mu            sync.RWMutex
}

func NewDoHResolver(server string) *DoHResolver {
	if server == "" {
		server = "https://1.1.1.1/dns-query"
	}
	ipv4Bogons := []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8",
		"169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24",
		"192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24",
		"224.0.0.0/4", "240.0.0.0/4", "255.255.255.255/32",
	}

	ipv6Bogons := []string{
		"::/128",        // Unspecified
		"::1/128",       // Loopback
		"100::/64",      // Discard prefix
		"2001::/32",     // TEREDO
		"2001:10::/28",  // Deprecated (ORCHID)
		"2001:db8::/32", // Documentation
		"fc00::/7",      // Unique local addresses (ULA)
		"fe80::/10",     // Link-local
		"ff00::/8",      // Multicast
	}

	ranges := append(ipv4Bogons, ipv6Bogons...)
	var invalidRanges []*net.IPNet
	for _, cidr := range ranges {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			invalidRanges = append(invalidRanges, network)
		}
	}

	return &DoHResolver{
		Server: server,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
		invalidRanges: invalidRanges,
		cache:         make(map[string]*dnsRecord),
	}
}

type DoHResponse struct {
	Status int `json:"Status"`
	Answer []struct {
		Name string `json:"name"`
		Type int    `json:"type"`
		TTL  int    `json:"TTL"`
		Data string `json:"data"`
	} `json:"Answer"`
}

func (r *DoHResolver) Resolve(domain string, recordType int) ([]string, error) {
	cacheKey := fmt.Sprintf("%s:%d", domain, recordType)

	r.mu.RLock()
	if record, ok := r.cache[cacheKey]; ok {
		if time.Now().Before(record.expiresAt) {
			defer r.mu.RUnlock()
			return record.ips, nil
		}
	}
	r.mu.RUnlock()

	reqURL := fmt.Sprintf("%s?name=%s&type=%d", r.Server, url.QueryEscape(domain), recordType)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/dns-json")

	resp, err := r.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var dohResp DoHResponse
	if err := json.Unmarshal(body, &dohResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	var results []string
	var minTTL = 3600
	for _, answer := range dohResp.Answer {
		if answer.Type == recordType {
			results = append(results, answer.Data)
			if answer.TTL < minTTL {
				minTTL = answer.TTL
			}
		}
	}

	if len(results) > 0 {
		r.mu.Lock()
		r.cache[cacheKey] = &dnsRecord{
			ips:       results,
			expiresAt: time.Now().Add(time.Duration(minTTL) * time.Second),
		}
		r.mu.Unlock()
	}

	return results, nil
}

// check if the given IP address is within known invalid ranges
func (r *DoHResolver) IsInvalidIP(ip string) bool {
	pip := net.ParseIP(ip)
	if pip == nil {
		return true // unparseable IPs are invalid
	}
	for _, nw := range r.invalidRanges {
		if nw.Contains(pip) {
			return true
		}
	}
	return false
}

// validates a HTTPS URL and returns a safe IP address to use for the request.
func (r *DoHResolver) ValidateAndGetIP(urlStr string) (string, *url.URL, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	if parsedURL.Scheme != "https" {
		return "", nil, fmt.Errorf("only HTTPS URLs are allowed, got: %s", parsedURL.Scheme)
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return "", nil, fmt.Errorf("URL has no hostname")
	}

	ipv4Addrs, err := r.Resolve(hostname, TypeA)
	if err == nil && len(ipv4Addrs) > 0 {
		for _, ip := range ipv4Addrs {
			if !r.IsInvalidIP(ip) {
				return ip, parsedURL, nil
			}
		}
	}

	ipv6Addrs, err := r.Resolve(hostname, TypeAAAA)
	if err == nil && len(ipv6Addrs) > 0 {
		for _, ip := range ipv6Addrs {
			if !r.IsInvalidIP(ip) {
				return ip, parsedURL, nil
			}
		}
	}

	return "", nil, fmt.Errorf("no valid IP addresses found for %s (all resolved to internal/bogon addresses)", hostname)
}
