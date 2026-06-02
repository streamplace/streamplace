package atproto

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/util"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/devenv"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/spmetrics"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/streamplace"
)

// TestMultiRelayDedup subscribes to the same dev PDS firehose twice (via two
// distinct-but-equivalent URL spellings, so they aren't collapsed as identical
// relays). Every commit is therefore delivered twice, and the deduper must drop
// the second copy: the record is indexed exactly once and the dedup counter
// climbs.
func TestMultiRelayDedup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dev := devenv.WithDevEnv(t)

	parsed, err := url.Parse(dev.PDSURL)
	require.NoError(t, err)
	port := parsed.Port()
	require.NotEmpty(t, port, "dev PDS URL should have a port")
	// Two spellings of the same server: relayHosts() keeps them distinct, so we
	// open two real connections to the one firehose.
	relayHost := fmt.Sprintf("ws://localhost:%s,ws://127.0.0.1:%s", port, port)

	cli := config.CLI{
		BroadcasterHost: "example.com",
		DBURL:           ":memory:",
		RelayHost:       relayHost,
		PLCURL:          dev.PLCURL,
	}
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

	// Sanity-check the fan-out: two relays, no self.
	require.Len(t, atsync.relayHosts(), 2)

	dedupedBefore := counterValue(t, spmetrics.FirehoseEventsDedupedTotal.WithLabelValues("commit"))

	go func() {
		_ = atsync.StartFirehose(ctx)
	}()

	user := dev.CreateAccount(t)
	msg := &streamplace.ChatMessage{
		LexiconTypeID: "place.stream.chat.message",
		Text:          "Hello from two relays!",
		CreatedAt:     time.Now().Add(-time.Second).Format(util.ISO8601),
		Streamer:      user.DID,
	}
	_, err = comatproto.RepoCreateRecord(ctx, user.XRPC, &comatproto.RepoCreateRecord_Input{
		Collection: "place.stream.chat.message",
		Repo:       user.DID,
		Record:     &lexutil.LexiconTypeDecoder{Val: msg},
	})
	require.NoError(t, err)

	// The message is indexed exactly once, even though two relays delivered it.
	err = untilNoErrors(t, func() error {
		messages, err := mod.MostRecentChatMessages(user.DID)
		if err != nil {
			return err
		}
		if len(messages) != 1 {
			return fmt.Errorf("expected exactly 1 message, got %d", len(messages))
		}
		return nil
	})
	require.NoError(t, err)

	// And the duplicates the second relay delivered were dropped by the deduper.
	err = untilNoErrors(t, func() error {
		deduped := counterValue(t, spmetrics.FirehoseEventsDedupedTotal.WithLabelValues("commit"))
		if deduped <= dedupedBefore {
			return fmt.Errorf("expected commit dedup counter to climb (before=%v now=%v)", dedupedBefore, deduped)
		}
		return nil
	})
	require.NoError(t, err)
}

func counterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	var m dto.Metric
	require.NoError(t, c.Write(&m))
	return m.GetCounter().GetValue()
}
