//go:build tools

package tools

import (
	_ "github.com/99designs/gqlgen"
	_ "github.com/99designs/gqlgen/graphql/introspection"
	_ "github.com/bluesky-social/indigo/cmd/lexgen"
	_ "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
	_ "github.com/prometheus/client_model/go"
	_ "golang.org/x/tools/cmd/goimports"
)
