package multitest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/util"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/crypto/spkey"
	"stream.place/streamplace/pkg/devenv"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/streamplace"
)

func TestMultinodeSyndication(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	startStreamplaceNode(t, dev)
	// startStreamplaceNode(t, dev)
	acct := dev.CreateAccount(t)
	_, pub, err := spkey.GenerateStreamKeyForDID(acct.DID)
	require.NoError(t, err)
	createdBy := "multitest"
	streamKey := streamplace.Key{
		SigningKey: pub.DIDKey(),
		CreatedAt:  time.Now().Format(util.ISO8601),
		CreatedBy:  &createdBy,
	}
	_, err = comatproto.RepoCreateRecord(context.TODO(), acct.XRPC, &comatproto.RepoCreateRecord_Input{
		Collection: "place.stream.key",
		Repo:       acct.DID,
		Record:     &lexutil.LexiconTypeDecoder{Val: &streamKey},
	})
	require.NoError(t, err)
	log.Log(context.Background(), "created stream key", "did", acct.DID, "pub", pub.DIDKey())
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
