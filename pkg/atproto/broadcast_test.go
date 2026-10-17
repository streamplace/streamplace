package atproto

import (
	"bytes"
	"context"
	"testing"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
)

func TestLivestreamBroadcastReference(t *testing.T) {
	const did = "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	for _, tt := range []struct {
		name string
		uri  string
		want bool
	}{
		{name: "first chapter", want: true},
		{name: "continuing broadcast", uri: "at://" + did + "/place.stream.livestream/3lfirst00000", want: true},
		{name: "another streamer", uri: "at://did:plc:bbbbbbbbbbbbbbbbbbbbbbbb/place.stream.livestream/3lfirst00000"},
		{name: "another collection", uri: "at://" + did + "/place.stream.chat.message/3lfirst00000"},
		{name: "missing record key", uri: "at://" + did + "/place.stream.livestream"},
		{name: "malformed URI", uri: "not-an-at-uri"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			atsync, mod, _ := offlineSynchronizer(t)
			require.NoError(t, mod.UpdateRepo(&model.Repo{DID: did, Handle: "streamer.test", Version: "3lrev00000000"}))
			rec := &placestream.Livestream{
				LexiconTypeID: "place.stream.livestream",
				CreatedAt:     "2026-01-01T00:00:00Z",
				Title:         "chapter",
			}
			if tt.uri != "" {
				rec.Broadcast = &tt.uri
			}
			var buf bytes.Buffer
			require.NoError(t, rec.MarshalCBOR(&buf))
			body := buf.Bytes()
			cid, err := spid.GetCID(rec)
			require.NoError(t, err)
			err = atsync.handleCreateUpdate(ctx, did, syntax.RecordKey("3lchapter0000"), &body, cid.String(), syntax.NSID("place.stream.livestream"), false, true, "")
			if tt.want {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			ls, err := mod.GetLatestLivestreamForRepo(did)
			require.NoError(t, err)
			if tt.want {
				require.NotNil(t, ls)
			} else {
				require.Nil(t, ls)
			}
		})
	}
}

func TestPinnedBroadcastReference(t *testing.T) {
	const did = "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	for _, tt := range []struct {
		name string
		uri  string
		want bool
	}{
		{name: "own broadcast", uri: "at://" + did + "/place.stream.livestream/3lfirst00000", want: true},
		{name: "another streamer", uri: "at://did:plc:bbbbbbbbbbbbbbbbbbbbbbbb/place.stream.livestream/3lfirst00000"},
		{name: "another collection", uri: "at://" + did + "/place.stream.chat.message/3lfirst00000"},
		{name: "missing record key", uri: "at://" + did + "/place.stream.livestream"},
		{name: "malformed URI", uri: "not-an-at-uri"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			atsync, mod, _ := offlineSynchronizer(t)
			require.NoError(t, mod.UpdateRepo(&model.Repo{DID: did, Handle: "streamer.test", Version: "3lrev00000000"}))
			rec := &placestream.ChatPinnedRecord{
				LexiconTypeID: "place.stream.chat.pinnedRecord",
				CreatedAt:     "2026-01-01T00:00:00Z",
				PinnedMessage: "at://" + did + "/place.stream.chat.message/3lmessage000",
				Livestream:    &tt.uri,
			}
			var buf bytes.Buffer
			require.NoError(t, rec.MarshalCBOR(&buf))
			body := buf.Bytes()
			cid, err := spid.GetCID(rec)
			require.NoError(t, err)
			err = atsync.handleCreateUpdate(ctx, did, syntax.RecordKey("3lpin0000000"), &body, cid.String(), syntax.NSID("place.stream.chat.pinnedRecord"), false, true, "")
			if tt.want {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			pin, err := mod.GetPinnedRecord(ctx, "at://"+did+"/place.stream.chat.pinnedRecord/3lpin0000000")
			require.NoError(t, err)
			if tt.want {
				require.NotNil(t, pin)
				require.Equal(t, tt.uri, pin.LivestreamURI)
			} else {
				require.Nil(t, pin)
			}
		})
	}
}
