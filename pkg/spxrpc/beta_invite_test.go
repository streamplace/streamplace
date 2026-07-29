package spxrpc

import (
	"context"
	"testing"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/indexdb"
	"stream.place/streamplace/pkg/placestream"
)

const (
	testInviteIssuer = "did:plc:streamplace"
	testInvitee      = "did:plc:invitee"
	testRando        = "did:plc:someoneelse"
)

func putInvite(t *testing.T, m indexdb.Model, repoDID, subjectDID, feature string) {
	t.Helper()
	rkey := feature + "-" + subjectDID
	aturi, err := syntax.ParseATURI("at://" + repoDID + "/place.stream.beta.invite/" + rkey)
	require.NoError(t, err)
	require.NoError(t, m.UpsertBetaInvite(context.Background(), placestream.BetaInvite{
		LexiconTypeID: "place.stream.beta.invite",
		Did:           subjectDID,
		Feature:       feature,
		CreatedAt:     "2026-01-01T00:00:00Z",
	}, aturi))
}

func TestAllowVODUpload_InviteMode(t *testing.T) {
	// With --beta-invite-did configured, only accounts holding a `vod`
	// invite from that exact issuer can upload.
	mkServer := func(t *testing.T) (*Server, indexdb.Model) {
		m, err := indexdb.MakeDB(":memory:")
		require.NoError(t, err)
		return &Server{
			model: m,
			cli:   &config.CLI{BetaInviteDID: testInviteIssuer},
		}, m
	}

	t.Run("invited account is allowed", func(t *testing.T) {
		s, m := mkServer(t)
		putInvite(t, m, testInviteIssuer, testInvitee, vodInviteFeature)
		require.NoError(t, s.allowVODUpload(context.Background(), testInvitee))
	})

	t.Run("uninvited account is rejected", func(t *testing.T) {
		s, _ := mkServer(t)
		require.Error(t, s.allowVODUpload(context.Background(), testInvitee))
	})

	t.Run("invite from a different repo is ignored", func(t *testing.T) {
		// A random user minting an invite under their own repo must
		// not get past the gate — we only trust the configured issuer.
		s, m := mkServer(t)
		putInvite(t, m, testRando, testInvitee, vodInviteFeature)
		require.Error(t, s.allowVODUpload(context.Background(), testInvitee))
	})

	t.Run("invite for a different feature is ignored", func(t *testing.T) {
		// Feature names are checked exactly; a `chat` invite doesn't
		// unlock VOD upload.
		s, m := mkServer(t)
		putInvite(t, m, testInviteIssuer, testInvitee, "chat")
		require.Error(t, s.allowVODUpload(context.Background(), testInvitee))
	})
}

func TestAllowVODUpload_AllowedStreamsFallback(t *testing.T) {
	// With BetaInviteDID unset we fall through to the same allowlist
	// livestreaming uses — including the "no allowlist ⇒ open server"
	// fallback for dev / single-node deployments.
	mkServer := func(allowed []string) *Server {
		m, err := indexdb.MakeDB(":memory:")
		require.NoError(t, err)
		return &Server{
			model: m,
			cli:   &config.CLI{AllowedStreams: allowed},
		}
	}

	t.Run("no allowlist configured ⇒ open server", func(t *testing.T) {
		s := mkServer(nil)
		require.NoError(t, s.allowVODUpload(context.Background(), testInvitee))
	})

	t.Run("DID in allowlist passes", func(t *testing.T) {
		s := mkServer([]string{testInvitee})
		require.NoError(t, s.allowVODUpload(context.Background(), testInvitee))
	})

	t.Run("DID outside allowlist rejected", func(t *testing.T) {
		s := mkServer([]string{testInviteIssuer})
		require.Error(t, s.allowVODUpload(context.Background(), testInvitee))
	})
}
