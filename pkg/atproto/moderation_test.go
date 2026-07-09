package atproto

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/syntax"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/util"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/devenv"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/placestream"
)

func TestDelegatedModeration(t *testing.T) {
	dev := devenv.WithDevEnv(t)
	t.Logf("dev: %+v", dev)
	cli := config.CLI{
		BroadcasterHost: "example.com",
		DBURL:           ":memory:",
		RelayHost:       strings.ReplaceAll(dev.PDSURL, "http://", "ws://"),
		PLCURL:          dev.PLCURL,
	}
	t.Logf("cli: %+v", cli)
	b := bus.NewBus()
	cli.DataDir = t.TempDir()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	state, err := statedb.MakeDB(context.Background(), &cli, nil, mod)
	require.NoError(t, err)
	atsync := &ATProtoSynchronizer{
		CLI:        &cli,
		StatefulDB: state,
		Model:      mod,
		Bus:        b,
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})

	go func() {
		err := atsync.StartFirehose(ctx)
		require.NoError(t, err)
		close(done)
	}()

	streamer := dev.CreateAccount(t)
	moderator := dev.CreateAccount(t)
	user := dev.CreateAccount(t)

	// Test 1: Create delegation record with expiration time
	t.Log("Test 1: Creating delegation record with expiration time")
	expirationTime := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	delegationRecord := &streamplace.ModerationPermission{
		LexiconTypeID:  "place.stream.moderation.permission",
		Moderator:      moderator.DID,
		Permissions:    []string{"ban", "hide", "livestream.manage"},
		CreatedAt:      time.Now().Format(util.ISO8601),
		ExpirationTime: &expirationTime,
	}

	_, err = comatproto.RepoCreateRecord(ctx, streamer.XRPC, &comatproto.RepoCreateRecord_Input{
		Collection: constants.PLACE_STREAM_MODERATION_PERMISSION,
		Repo:       streamer.DID,
		Record:     &lexutil.LexiconTypeDecoder{Val: delegationRecord},
	})
	require.NoError(t, err)

	// Wait for firehose ingestion
	err = untilNoErrors(t, func() error {
		delegation, err := mod.GetModerationDelegation(ctx, streamer.DID, moderator.DID)
		if err != nil {
			return err
		}
		if delegation == nil {
			return fmt.Errorf("delegation not found")
		}
		return nil
	})
	require.NoError(t, err)
	t.Log("✓ Delegation record ingested successfully")

	// Verify delegation details including expiration time
	view, err := mod.GetModerationDelegation(ctx, streamer.DID, moderator.DID)
	require.NoError(t, err)
	require.NotNil(t, view)
	require.Equal(t, streamer.DID, view.Author.Did)

	delegation := view.Record.Val.(*streamplace.ModerationPermission)
	require.NotNil(t, delegation)
	require.Equal(t, moderator.DID, delegation.Moderator)
	require.NotNil(t, delegation.ExpirationTime, "expiration time should be set")

	exp, err := time.Parse(time.RFC3339, *delegation.ExpirationTime)
	require.NoError(t, err)
	require.True(t, exp.After(time.Now()), "expiration time should be in the future")
	t.Log("✓ Delegation details verified (including expiration time)")

	// Test 2: Create block (ban user)
	t.Log("Test 2: Creating block record")
	block := &bsky.GraphBlock{
		Subject:   user.DID,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	blockRec, err := comatproto.RepoCreateRecord(ctx, streamer.XRPC, &comatproto.RepoCreateRecord_Input{
		Collection: constants.APP_BSKY_GRAPH_BLOCK,
		Repo:       streamer.DID,
		Record:     &lexutil.LexiconTypeDecoder{Val: block},
	})
	require.NoError(t, err)
	t.Logf("✓ Block record created: %s", blockRec.Uri)

	// Wait for firehose to process block
	blockRkey := strings.TrimPrefix(blockRec.Uri, fmt.Sprintf("at://%s/%s/", streamer.DID, constants.APP_BSKY_GRAPH_BLOCK))
	err = untilNoErrors(t, func() error {
		block, err := mod.GetBlock(ctx, blockRkey)
		if err != nil {
			return err
		}
		if block == nil {
			return fmt.Errorf("block not found")
		}
		return nil
	})
	require.NoError(t, err)
	t.Log("✓ Block record ingested successfully")

	// Test 3: Create chat message and gate
	t.Log("Test 3: Creating chat message and gate")
	msg := &streamplace.ChatMessage{
		LexiconTypeID: "place.stream.chat.message",
		Text:          "Test message to be hidden",
		CreatedAt:     time.Now().Format(util.ISO8601),
		Streamer:      streamer.DID,
	}

	msgRec, err := comatproto.RepoCreateRecord(ctx, user.XRPC, &comatproto.RepoCreateRecord_Input{
		Collection: constants.PLACE_STREAM_CHAT_MESSAGE,
		Repo:       user.DID,
		Record:     &lexutil.LexiconTypeDecoder{Val: msg},
	})
	require.NoError(t, err)
	t.Logf("✓ Chat message created: %s", msgRec.Uri)

	// Create gate to hide the message
	gate := &streamplace.ChatGate{
		LexiconTypeID: "place.stream.chat.gate",
		HiddenMessage: msgRec.Uri,
	}

	gateRec, err := comatproto.RepoCreateRecord(ctx, streamer.XRPC, &comatproto.RepoCreateRecord_Input{
		Collection: constants.PLACE_STREAM_CHAT_GATE,
		Repo:       streamer.DID,
		Record:     &lexutil.LexiconTypeDecoder{Val: gate},
	})
	require.NoError(t, err)
	t.Logf("✓ Gate record created: %s", gateRec.Uri)

	// Wait for firehose to process gate
	gateRkey := strings.TrimPrefix(gateRec.Uri, fmt.Sprintf("at://%s/%s/", streamer.DID, constants.PLACE_STREAM_CHAT_GATE))
	err = untilNoErrors(t, func() error {
		gate, err := mod.GetGate(ctx, gateRkey)
		if err != nil {
			return err
		}
		if gate == nil {
			return fmt.Errorf("gate not found")
		}
		return nil
	})
	require.NoError(t, err)
	t.Log("✓ Gate record ingested successfully")

	// Test 4: Delete gate (unhide message)
	t.Log("Test 4: Deleting gate record")

	_, err = comatproto.RepoDeleteRecord(ctx, streamer.XRPC, &comatproto.RepoDeleteRecord_Input{
		Collection: constants.PLACE_STREAM_CHAT_GATE,
		Repo:       streamer.DID,
		Rkey:       gateRkey,
	})
	require.NoError(t, err)

	// Wait for firehose to process deletion
	err = untilNoErrors(t, func() error {
		gate, err := mod.GetGate(ctx, gateRkey)
		if err != nil {
			return err
		}
		if gate != nil {
			return fmt.Errorf("gate still exists")
		}
		return nil
	})
	require.NoError(t, err)
	t.Log("✓ Gate record deleted successfully")

	// Test 5: Delete delegation (revoke permissions)
	t.Log("Test 5: Deleting delegation record")
	uri, err := syntax.ParseATURI(view.Uri)
	require.NoError(t, err)

	_, err = comatproto.RepoDeleteRecord(ctx, streamer.XRPC, &comatproto.RepoDeleteRecord_Input{
		Collection: constants.PLACE_STREAM_MODERATION_PERMISSION,
		Repo:       streamer.DID,
		Rkey:       uri.RecordKey().String(),
	})
	require.NoError(t, err)

	// Wait for firehose to process deletion
	err = untilNoErrors(t, func() error {
		delegation, err := mod.GetModerationDelegation(ctx, streamer.DID, moderator.DID)
		if err != nil {
			return err
		}
		if delegation != nil {
			return fmt.Errorf("delegation still exists")
		}
		return nil
	})
	require.NoError(t, err)
	t.Log("✓ Delegation record deleted successfully")

	t.Log("All moderation tests passed!")
	cancel()
	<-done
}
