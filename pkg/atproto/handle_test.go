package atproto

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/util"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/devenv"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/streamplace"
)

func TestHandleChange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dev := devenv.WithDevEnv(t)
	t.Logf("dev: %+v", dev)
	cli := config.CLI{
		PublicHost: "example.com",
		DBURL:      ":memory:",
		RelayHost:  strings.ReplaceAll(dev.PDSURL, "http://", "ws://"),
		PLCURL:     dev.PLCURL,
	}

	t.Logf("cli: %+v", cli)
	b := bus.NewBus()
	cli.DataDir = t.TempDir()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(context.Background(), &cli, nil, mod)
	require.NoError(t, err)
	atsync := &ATProtoSynchronizer{
		CLI:          &cli,
		StatefulDB:   state,
		Model:        mod,
		Bus:          b,
		PLCDirectory: dev.TestDirectory(),
	}

	done := make(chan struct{})

	go func() {
		err := atsync.StartFirehose(ctx)
		require.NoError(t, err)
		close(done)
	}()

	user := dev.CreateAccount(t)

	msg := &streamplace.ChatMessage{
		LexiconTypeID: "place.stream.chat.message",
		Text:          "Hello, world!",
		CreatedAt:     time.Now().Add(-time.Second).Format(util.ISO8601),
		Streamer:      user.DID,
	}

	_, err = comatproto.RepoCreateRecord(ctx, user.XRPC, &comatproto.RepoCreateRecord_Input{
		Collection: "place.stream.chat.message",
		Repo:       user.DID,
		Record:     &lexutil.LexiconTypeDecoder{Val: msg},
	})
	require.NoError(t, err)

	var message *streamplace.ChatDefs_MessageView
	err = untilNoErrors(t, func() error {
		messages, err := mod.MostRecentChatMessages(user.DID)
		if err != nil {
			return err
		}
		if len(messages) != 1 {
			return fmt.Errorf("expected 2 messages, got %d", len(messages))
		}
		message = messages[0]
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, user.Handle, message.Author.Handle)

	log.Log(ctx, "updating handle", "handle", "new-handle.test")
	err = comatproto.IdentityUpdateHandle(context.Background(), user.XRPC, &comatproto.IdentityUpdateHandle_Input{
		Handle: "new-handle.test",
	})
	require.NoError(t, err)

	err = untilNoErrors(t, func() error {
		messages, err := mod.MostRecentChatMessages(user.DID)
		if err != nil {
			return err
		}
		if len(messages) != 1 {
			return fmt.Errorf("expected 2 messages, got %d", len(messages))
		}
		message = messages[0]
		if message.Author.Handle != "new-handle.test" {
			return fmt.Errorf("expected new handle, got %s", message.Author.Handle)
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, message.Author.Handle, "new-handle.test")
}
