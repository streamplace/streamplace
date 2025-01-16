package ct

import (
	"os"
	"testing"

	"stream.place/streamplace/pkg/config"
	"github.com/stretchr/testify/require"
)

func CLI(t *testing.T, cli *config.CLI) *config.CLI {
	dir, err := os.MkdirTemp("", "aq-testing-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	cli.DataDir = dir
	return cli
}
