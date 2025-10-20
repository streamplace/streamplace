package multitest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/devenv"
)

func TestMultinodeSyndication(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	startStreamplaceNode(t, dev)
	startStreamplaceNode(t, dev)
	time.Sleep(10 * time.Second)
}

var currentPort = 10000

func nextPort() int {
	currentPort++
	return currentPort
}

func startStreamplaceNode(t *testing.T, dev *devenv.DevEnv) {
	dataDir := t.TempDir()
	env := map[string]string{
		"SP_HTTP_ADDR":          fmt.Sprintf(":%d", nextPort()),
		"SP_HTTP_INTERNAL_ADDR": fmt.Sprintf(":%d", nextPort()),
		"SP_RELAY_HOST":         strings.ReplaceAll(dev.PDSURL, "http://", "ws://"),
		"SP_PLC_URL":            dev.PLCURL,
		"SP_DATA_DIR":           dataDir,
	}
	_, file, _, _ := runtime.Caller(0)
	abs, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", "..", "build-linux-amd64", "streamplace"))
	require.NoError(t, err)
	// Run the streamplace binary at abs with the environment env
	cmd := exec.Command(abs)
	cmd.Env = []string{}
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		err := cmd.Process.Kill()
		require.NoError(t, err)
		_, err = cmd.Process.Wait()
		require.NoError(t, err)
	})
}
