package acme

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/caddyserver/certmagic"
	"go.uber.org/zap"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/statedb"
)

// CA shorthands accepted by --acme-ca.
const (
	CALetsEncrypt        = "letsencrypt"
	CALetsEncryptStaging = "letsencrypt-staging"
)

// ResolveCA turns a shorthand or URL into an ACME directory URL.
func ResolveCA(ca string) string {
	switch strings.ToLower(strings.TrimSpace(ca)) {
	case "", CALetsEncrypt, "production", "prod":
		return certmagic.LetsEncryptProductionCA
	case CALetsEncryptStaging, "staging":
		return certmagic.LetsEncryptStagingCA
	}
	return ca
}

// Manager owns the certmagic configuration for this node.
type Manager struct {
	cfg     *certmagic.Config
	issuer  *certmagic.ACMEIssuer
	domains []string
}

// Domains lists the hostnames a node must hold certificates for: the
// broadcaster (station) host, the server host when it is a distinct
// identity, and any extra names the operator asked for. Ports are stripped.
func Domains(cli *config.CLI) []string {
	var out []string
	seen := map[string]struct{}{}
	add := func(h string) {
		h = strings.ToLower(strings.TrimSpace(h))
		if host, _, err := net.SplitHostPort(h); err == nil {
			h = host
		}
		if h == "" {
			return
		}
		if _, dup := seen[h]; dup {
			return
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}
	add(cli.BroadcasterHost)
	add(cli.ServerHost)
	for _, d := range cli.ACMEDomains {
		add(d)
	}
	return out
}

// New builds the manager. It does not talk to the CA; call Manage for that.
func New(ctx context.Context, cli *config.CLI, state *statedb.StatefulDB) (*Manager, error) {
	domains := Domains(cli)
	if len(domains) == 0 {
		return nil, fmt.Errorf("acme: no domains to manage; set --broadcaster-host")
	}
	for _, d := range domains {
		if ip := net.ParseIP(d); ip != nil || d == "localhost" {
			return nil, fmt.Errorf("acme: %q is not a public hostname a CA can validate", d)
		}
	}

	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("acme: build logger: %w", err)
	}
	storage := NewStorage(state)

	m := &Manager{domains: domains}
	cache := certmagic.NewCache(certmagic.CacheOptions{
		GetConfigForCert: func(certmagic.Certificate) (*certmagic.Config, error) {
			return m.cfg, nil
		},
		Logger: logger,
	})
	m.cfg = certmagic.New(cache, certmagic.Config{
		Storage:           storage,
		Logger:            logger,
		DefaultServerName: domains[0],
	})
	m.issuer = certmagic.NewACMEIssuer(m.cfg, certmagic.ACMEIssuer{
		CA:     ResolveCA(cli.ACMECA),
		Email:  cli.ACMEEmail,
		Agreed: true,
		Logger: logger,
	})
	m.cfg.Issuers = []certmagic.Issuer{m.issuer}
	return m, nil
}

// Domains returns the managed hostnames.
func (m *Manager) Domains() []string {
	return append([]string(nil), m.domains...)
}

// Manage obtains any missing certificates in the background and keeps them
// renewed until ctx ends. Failures are retried by certmagic; the node keeps
// serving meanwhile, so a misconfigured DNS record shows up as TLS handshake
// errors and log lines rather than a node that refuses to start.
func (m *Manager) Manage(ctx context.Context) error {
	log.Log(ctx, "acme: managing certificates", "domains", strings.Join(m.domains, ","), "ca", m.issuer.CA)
	if err := m.cfg.ManageAsync(ctx, m.domains); err != nil {
		return fmt.Errorf("acme: manage %v: %w", m.domains, err)
	}
	<-ctx.Done()
	return nil
}

// TLSConfig serves the managed certificates and answers the TLS-ALPN-01
// challenge. Callers append their own application protocols.
func (m *Manager) TLSConfig() *tls.Config {
	cfg := m.cfg.TLSConfig()
	cfg.MinVersion = tls.VersionTLS12
	return cfg
}

// HTTPChallengeHandler answers HTTP-01 challenges (for this node or any node
// sharing its statedb) on the plain-HTTP listener, passing everything else
// to next.
func (m *Manager) HTTPChallengeHandler(next http.Handler) http.Handler {
	return m.issuer.HTTPChallengeHandler(next)
}
