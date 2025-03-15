//go:build tools

package tools

import (
	_ "github.com/99designs/gqlgen"
	_ "github.com/99designs/gqlgen/graphql/introspection"
	_ "github.com/bluesky-social/indigo/cmd/lexgen"
	_ "golang.org/x/tools/cmd/goimports"
)
