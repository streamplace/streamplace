package atproto

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"stream.place/streamplace/pkg/comatproto"
	glexrt "github.com/streamplace/glex/runtime"
	"github.com/bluesky-social/indigo/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/devenv"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/placestream"
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

	dedupedBefore := dedupedCommitTotal(t)

	go func() {
		_ = atsync.StartFirehose(ctx)
	}()

	user := dev.CreateAccount(t)
	msg := placestream.ChatMessage{
		LexiconTypeID: "place.stream.chat.message",
		Text:          "Hello from two relays!",
		CreatedAt:     time.Now().Add(-time.Second).Format(util.ISO8601),
		Streamer:      user.DID,
	}
	_, err = comatproto.RepoCreateRecord(ctx, user.XRPC, &comatproto.RepoCreateRecord_Input{
		Collection: "place.stream.chat.message",
		Repo:       user.DID,
		Record:     &glexrt.LexiconTypeDecoder{Val: msg},
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
		deduped := dedupedCommitTotal(t)
		if deduped <= dedupedBefore {
			return fmt.Errorf("expected commit dedup counter to climb (before=%v now=%v)", dedupedBefore, deduped)
		}
		return nil
	})
	require.NoError(t, err)
}

// dedupedCommitTotal sums the commit dedup counter across every relay/protocol
// series (a duplicate is attributed to whichever relay delivered it second).
func dedupedCommitTotal(t *testing.T) float64 {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	var sum float64
	for _, mf := range mfs {
		if mf.GetName() != "streamplace_firehose_events_deduped_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "kind" && l.GetValue() == "commit" {
					sum += m.GetCounter().GetValue()
				}
			}
		}
	}
	return sum
}
