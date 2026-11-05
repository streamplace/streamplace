package atproto

import (
	"bytes"
	"context"
	"testing"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/ipfs/go-cid"
	glex "github.com/streamplace/glex/runtime"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
)

// indexBskyProfile stores an app.bsky.actor.profile the same way the firehose
// does: CBOR bytes keyed by the repo's self record and the go-live flag.
func indexBskyProfile(t *testing.T, mod model.Model, did string, wasStreamplace bool, rec *appbsky.ActorProfile) {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, rec.MarshalCBOR(&buf))
	uri := syntax.ATURI("at://" + did + "/app.bsky.actor.profile/self")
	require.NoError(t, mod.UpsertBskyProfile(context.Background(), uri, buf.Bytes(), wasStreamplace))
}

func ptr(s string) *string { return &s }

// TestDecorateAuthorProfileReadsBlueskySource: the index keeps two rows per
// DID, the real Bluesky profile and the transient copy Streamplace writes when
// a stream goes live. Only the real one may reach third parties.
func TestDecorateAuthorProfileReadsBlueskySource(t *testing.T) {
	ctx := context.Background()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	atsync := &ATProtoSynchronizer{Model: mod}

	const did = "did:plc:streamer"
	avatarCID := cid.MustParse("bafkreib2rxk3rybk3aobmv5cjuql3bm2twh4jo5uxyjfxzvjcamdmc76jm")
	indexBskyProfile(t, mod, did, false, &appbsky.ActorProfile{
		DisplayName: ptr("Real Name"),
		Pronouns:    ptr("they/them"),
		Avatar:      &glex.Blob{Ref: glex.Link(avatarCID), MimeType: "image/jpeg", Size: 4096},
	})
	indexBskyProfile(t, mod, did, true, &appbsky.ActorProfile{
		DisplayName: ptr("🔴 LIVE: Real Name"),
		Pronouns:    ptr("live/pronouns"),
	})

	author := appbsky.ActorDefs_ProfileViewBasic{Did: did, Handle: "streamer.test"}
	atsync.DecorateAuthorProfile(ctx, &author)
	require.Equal(t, "Real Name", *author.DisplayName)
	require.Equal(t, "they/them", *author.Pronouns)
	require.Equal(t, "https://cdn.bsky.app/img/avatar/plain/did:plc:streamer/"+avatarCID.String()+"@jpeg", *author.Avatar)

	// A DID with only the go-live copy indexed has no real profile to show.
	const liveOnly = "did:plc:liveonly"
	indexBskyProfile(t, mod, liveOnly, true, &appbsky.ActorProfile{DisplayName: ptr("🔴 LIVE")})
	bare := appbsky.ActorDefs_ProfileViewBasic{Did: liveOnly, Handle: "liveonly.test"}
	want := bare
	atsync.DecorateAuthorProfile(ctx, &bare)
	require.Equal(t, want, bare)
}

// TestDecorateAuthorProfileSparse: a profile record with none of the optional
// fields set must not invent them, and the identity fields the caller already
// resolved stay exactly as they were.
func TestDecorateAuthorProfileSparse(t *testing.T) {
	ctx := context.Background()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	atsync := &ATProtoSynchronizer{Model: mod}

	const did = "did:plc:sparse"
	indexBskyProfile(t, mod, did, false, &appbsky.ActorProfile{})

	author := appbsky.ActorDefs_ProfileViewBasic{
		Did:    did,
		Handle: "sparse.test",
		Verification: &appbsky.ActorDefs_VerificationState{
			VerifiedStatus:        "valid",
			TrustedVerifierStatus: "none",
		},
	}
	want := author
	atsync.DecorateAuthorProfile(ctx, &author)
	require.Equal(t, want, author)
	require.Nil(t, author.DisplayName)
	require.Nil(t, author.Pronouns)
	require.Nil(t, author.Avatar)
}

// TestDecorateChatProfilesIndexFailure: a corrupt record or an unreadable index
// must leave the message deliverable with its bare authors, including the one
// nested in the reply preview, rather than panicking or dropping identity.
func TestDecorateChatProfilesIndexFailure(t *testing.T) {
	ctx := context.Background()
	const did = "did:plc:broken"
	const parentDID = "did:plc:parent"

	newMessage := func() *placestream.ChatDefs_MessageView {
		return &placestream.ChatDefs_MessageView{
			Uri:    "at://" + did + "/place.stream.chat.message/reply",
			Author: appbsky.ActorDefs_ProfileViewBasic{Did: did, Handle: "broken.test"},
			ReplyTo: &placestream.ChatDefs_MessageView_ReplyTo{
				ChatDefs_MessageView: &placestream.ChatDefs_MessageView{
					Uri:    "at://" + parentDID + "/place.stream.chat.message/root",
					Author: appbsky.ActorDefs_ProfileViewBasic{Did: parentDID, Handle: "parent.test"},
				},
			},
		}
	}

	t.Run("corrupt record", func(t *testing.T) {
		mod, err := model.MakeDB(":memory:")
		require.NoError(t, err)
		garbage := []byte("not cbor at all")
		require.NoError(t, mod.(*model.DBModel).DB.Create(&model.BskyProfile{
			URI:     "at://" + did + "/app.bsky.actor.profile/self",
			CID:     "bafyreigarbage",
			RepoDID: did,
			Record:  &garbage,
		}).Error)
		indexBskyProfile(t, mod, parentDID, false, &appbsky.ActorProfile{DisplayName: ptr("Parent")})
		atsync := &ATProtoSynchronizer{Model: mod}

		msg := newMessage()
		want := *newMessage()
		atsync.DecorateChatProfiles(ctx, msg)
		require.Equal(t, want.Author, msg.Author, "a corrupt record leaves the author bare")
		// The healthy author in the same thread is still hydrated.
		require.Equal(t, "Parent", *msg.ReplyTo.ChatDefs_MessageView.Author.DisplayName)
		require.Equal(t, parentDID, msg.ReplyTo.ChatDefs_MessageView.Author.Did)
	})

	t.Run("unreadable index", func(t *testing.T) {
		mod, err := model.MakeDB(":memory:")
		require.NoError(t, err)
		require.NoError(t, mod.(*model.DBModel).DB.Migrator().DropTable(&model.BskyProfile{}))
		atsync := &ATProtoSynchronizer{Model: mod}

		msg := newMessage()
		want := newMessage()
		atsync.DecorateChatProfiles(ctx, msg)
		require.Equal(t, want, msg)
	})
}
