package atproto

import (
	"context"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
)

func resetChatAccessCaches() {
	verifierMu.Lock()
	rulesCache = map[string]cachedRules{}
	subjectsAt = time.Time{}
	verifierMu.Unlock()
}

func chatAccessRecord(t *testing.T, streamer, rkey, action string, subject placestream.ChatAccess_Subject) *model.ChatAccessRule {
	t.Helper()
	rec := &placestream.ChatAccess{Action: action, Subject: subject, CreatedAt: "2026-09-17T00:00:00Z"}
	aturi, err := syntax.ParseATURI("at://" + streamer + "/place.stream.chat.access/" + rkey)
	require.NoError(t, err)
	row, err := model.ChatAccessRuleFromRecord(rec, aturi)
	require.NoError(t, err)
	return row
}

func TestChatAccessRules(t *testing.T) {
	ctx := context.Background()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	atsync := &ATProtoSynchronizer{Model: mod}
	resetChatAccessCaches()

	const (
		streamer = "did:plc:streamer"
		verifier = "did:plc:verifier"
		labeler  = "did:plc:labeler"
		alice    = "did:plc:alice"
		bob      = "did:plc:bob"
		carol    = "did:plc:carol"
	)
	// alice: vouched for by the verifier. bob: labelled partner-gold by the
	// labeler. carol: nothing.
	require.NoError(t, mod.CreateVerification(ctx, &model.Verification{
		URI: "at://" + verifier + "/app.bsky.graph.verification/1", IssuerDID: verifier, SubjectDID: alice, CreatedAt: time.Now(),
	}))
	require.NoError(t, mod.CreateVerification(ctx, &model.Verification{
		URI: labelVerificationURI(labeler, bob, "partner-gold"), IssuerDID: labeler, SubjectDID: bob, CreatedAt: time.Now(),
	}))

	// No rules: open chat, no badges.
	require.True(t, atsync.ChatAllowed(ctx, streamer, carol))
	require.Nil(t, atsync.VerificationState(ctx, streamer, alice))

	// allow verifier: only alice may chat, and wears the badge.
	allowV := chatAccessRecord(t, streamer, "a", "allow", placestream.ChatAccess_Subject{
		ChatAccess_Verifier: &placestream.ChatAccess_Verifier{Did: verifier},
	})
	require.NoError(t, mod.CreateChatAccessRule(ctx, allowV))
	atsync.NoteChatAccessRule(ctx, allowV)
	require.True(t, atsync.ChatAllowed(ctx, streamer, alice))
	require.False(t, atsync.ChatAllowed(ctx, streamer, bob))
	require.False(t, atsync.ChatAllowed(ctx, streamer, carol))
	state := atsync.VerificationState(ctx, streamer, alice)
	require.NotNil(t, state)
	require.Equal(t, verifier, state.Verifications[0].Issuer)
	require.Nil(t, atsync.VerificationState(ctx, streamer, bob))

	// allow label partner-*: bob may chat too, with a badge from the labeler.
	allowL := chatAccessRecord(t, streamer, "b", "allow", placestream.ChatAccess_Subject{
		ChatAccess_Label: &placestream.ChatAccess_Label{Labeler: labeler, Value: "partner-*"},
	})
	require.NoError(t, mod.CreateChatAccessRule(ctx, allowL))
	atsync.NoteChatAccessRule(ctx, allowL)
	require.True(t, atsync.ChatAllowed(ctx, streamer, bob))
	require.Equal(t, labeler, atsync.VerificationState(ctx, streamer, bob).Verifications[0].Issuer)
	require.False(t, atsync.ChatAllowed(ctx, streamer, carol))

	// deny verifier: deny wins over allow, so alice is out again.
	denyV := chatAccessRecord(t, streamer, "c", "deny", placestream.ChatAccess_Subject{
		ChatAccess_Verifier: &placestream.ChatAccess_Verifier{Did: verifier},
	})
	require.NoError(t, mod.CreateChatAccessRule(ctx, denyV))
	atsync.NoteChatAccessRule(ctx, denyV)
	require.False(t, atsync.ChatAllowed(ctx, streamer, alice))
	require.True(t, atsync.ChatAllowed(ctx, streamer, bob))

	// Another streamer's rules are their own: with none, everyone may chat.
	require.True(t, atsync.ChatAllowed(ctx, "did:plc:other", carol))
	require.Nil(t, atsync.VerificationState(ctx, "did:plc:other", alice))

	// The subjects the node indexes follow the rules.
	verifiers, labelers, err := mod.ChatAccessSubjects(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{verifier}, verifiers)
	require.Equal(t, map[string][]string{labeler: {"partner-*"}}, labelers)

	// Deleting the allow rules leaves only the deny: open chat except alice.
	require.NoError(t, mod.DeleteChatAccessRule(ctx, allowV.URI))
	require.NoError(t, mod.DeleteChatAccessRule(ctx, allowL.URI))
	atsync.NoteChatAccessRule(ctx, &model.ChatAccessRule{RepoDID: streamer})
	require.False(t, atsync.ChatAllowed(ctx, streamer, alice))
	require.True(t, atsync.ChatAllowed(ctx, streamer, carol))
	require.Nil(t, atsync.VerificationState(ctx, streamer, bob))
}

func TestChatAccessRuleFromRecord(t *testing.T) {
	aturi, err := syntax.ParseATURI("at://did:plc:streamer/place.stream.chat.access/x")
	require.NoError(t, err)
	_, err = model.ChatAccessRuleFromRecord(&placestream.ChatAccess{Action: "maybe", CreatedAt: "2026-09-17T00:00:00Z",
		Subject: placestream.ChatAccess_Subject{ChatAccess_Verifier: &placestream.ChatAccess_Verifier{Did: "did:plc:v"}}}, aturi)
	require.Error(t, err, "unknown action")
	_, err = model.ChatAccessRuleFromRecord(&placestream.ChatAccess{Action: "allow", CreatedAt: "2026-09-17T00:00:00Z",
		Subject: placestream.ChatAccess_Subject{ChatAccess_Verifier: &placestream.ChatAccess_Verifier{Did: "alice.example"}}}, aturi)
	require.Error(t, err, "verifier must be a DID")
	_, err = model.ChatAccessRuleFromRecord(&placestream.ChatAccess{Action: "allow", CreatedAt: "2026-09-17T00:00:00Z"}, aturi)
	require.ErrorIs(t, err, model.ErrChatAccessSubjectUnknown, "an unset (or unrecognised) subject is skipped, not failed")
	row, err := model.ChatAccessRuleFromRecord(&placestream.ChatAccess{Action: "deny", CreatedAt: "2026-09-17T00:00:00Z",
		Subject: placestream.ChatAccess_Subject{ChatAccess_Label: &placestream.ChatAccess_Label{Labeler: "did:plc:l", Value: "spam"}}}, aturi)
	require.NoError(t, err)
	require.Equal(t, model.ChatAccessSubjectLabel, row.SubjectType)
	require.Equal(t, "did:plc:l", row.SubjectDID)
	require.Equal(t, "spam", row.LabelValue)
	require.Equal(t, "did:plc:streamer", row.RepoDID)
}
