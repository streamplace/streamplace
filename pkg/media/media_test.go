package media

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	ct "stream.place/streamplace/pkg/config/configtesting"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
)

func getFixture(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	return filepath.Join(dir, "..", "..", "test", "fixtures", name)
}

func getStaticTestMediaManager(t *testing.T) (*MediaManager, MediaSigner) {
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	// signer, err := c2pa.MakeStaticSigner(eip712test.KeyBytes)
	require.NoError(t, err)
	if err != nil {
		panic(err)
	}
	cli := ct.CLI(t, &config.CLI{
		TAURL:          "http://timestamp.digicert.com",
		AllowedStreams: []string{"did:plc:2j2ounbiyi3ftihronlw5qhj"},
		DBURL:          ":memory:",
	})
	statedb, err := statedb.MakeDB(context.Background(), cli, nil, mod)
	require.NoError(t, err)
	atsync := &atproto.ATProtoSynchronizer{
		CLI:        cli,
		Model:      mod,
		StatefulDB: statedb,
		Bus:        bus.NewBus(),
	}
	mm, err := MakeMediaManager(context.Background(), cli, nil, mod, bus.NewBus(), atsync)
	require.NoError(t, err)
	// ms, err := MakeMediaSigner(context.Background(), cli, "test-person", signer)
	// require.NoError(t, err)
	return mm, nil
}
