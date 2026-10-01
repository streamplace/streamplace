package media

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

func TestCaptionMasterEngineDiscovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mm := NewOffline(&config.CLI{DataDir: t.TempDir()})
	stop, err := mm.StartCaptionEngineProxy(ctx)
	require.NoError(t, err)
	defer stop()
	dir, err := mm.ingestWorkerSocketDir()
	require.NoError(t, err)
	framePath := filepath.Join(dir, "worker.sock")
	frame, err := net.Listen("unix", framePath)
	require.NoError(t, err)
	defer frame.Close()
	push, err := net.Listen("unix", framePath+".captions")
	require.NoError(t, err)
	defer push.Close()
	require.NoError(t, writeWorkerMeta(framePath, workerMeta{}))
	resumable, err := DiscoverWorkerSockets(dir)
	require.NoError(t, err)
	require.Equal(t, []string{framePath}, resumable, "engine and caption-control sockets are not frame transports")
	removeWorkerFiles(framePath)
	_, err = os.Stat(framePath + ".captions")
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(workerMetaPath(framePath))
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(mm.CaptionEngineSocket)
	require.NoError(t, err, "worker crash cleanup must not unlink the node scheduler")
}
