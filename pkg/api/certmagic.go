package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/caddyserver/certmagic"
	"stream.place/streamplace/pkg/log"
)

// serve with CertMagic
func (a *StreamplaceAPI) ServeHTTPSWithCertMagic(ctx context.Context) error {
	if a.CLI.PublicHost == "" {
		return fmt.Errorf("public-host must be set when using CertMagic")
	}

	if a.CLI.CertMagicCAURL != "" {
		certmagic.DefaultACME.CA = a.CLI.CertMagicCAURL
	}

	certmagic.DefaultACME.Agreed = true
	handler, err := a.Handler(ctx)
	if err != nil {
		return err
	}

	return a.ServerWithShutdown(ctx, handler, func(s *http.Server) error {
		s.Addr = a.CLI.HTTPSAddr

		tlsConfig := certmagic.Default.TLSConfig()
		tlsConfig.NextProtos = append([]string{"h2", "http/1.1"}, tlsConfig.NextProtos...)
		s.TLSConfig = tlsConfig

		log.Log(ctx, "https server starting with CertMagic",
			"addr", s.Addr,
			"domain", a.CLI.PublicHost,
			"ca", certmagic.DefaultACME.CA,
		)

		err := certmagic.ManageAsync(ctx, []string{a.CLI.PublicHost})
		if err != nil {
			return fmt.Errorf("failed to start certificate management: %w", err)
		}

		return s.ListenAndServeTLS("", "")
	})
}
