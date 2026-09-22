package model

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/placestream"
)

func chatRow(t *testing.T, cid, author, streamer, text string, at time.Time) *ChatMessage {
	t.Helper()
	rec := placestream.ChatMessage{LexiconTypeID: "place.stream.chat.message", Text: text, Streamer: streamer, CreatedAt: at.UTC().Format(time.RFC3339Nano)}
	var buf bytes.Buffer
	require.NoError(t, rec.MarshalCBOR(&buf))
	body := buf.Bytes()
	indexed := at
	return &ChatMessage{CID: cid, URI: "at://" + author + "/place.stream.chat.message/" + cid, CreatedAt: at, ChatMessage: &body,
		RepoDID: author, StreamerRepoDID: streamer, IndexedAt: &indexed}
}

// The replay window returns the streamer's visible messages between two
// times, oldest first, in pages that continue where the cursor left off.
func TestChatMessagesBetween(t *testing.T) {
	ctx := context.Background()
	mod := indexedTestDB(t)
	t0 := time.Date(2026, 9, 16, 7, 12, 0, 0, time.UTC)
	streamer := "did:plc:streamer"
	texts := map[string]string{}
	for i := 0; i < 7; i++ {
		cid := "bafym" + string(rune('a'+i))
		texts[cid] = "m" + string(rune('a'+i))
		require.NoError(t, mod.CreateChatMessage(ctx, chatRow(t, cid, "did:plc:viewer", streamer, texts[cid], t0.Add(time.Duration(i)*time.Minute))))
	}
	// Before the window, another streamer's chat, and a deleted one inside it.
	require.NoError(t, mod.CreateChatMessage(ctx, chatRow(t, "bafyearly", "did:plc:viewer", streamer, "early", t0.Add(-time.Hour))))
	require.NoError(t, mod.CreateChatMessage(ctx, chatRow(t, "bafyother", "did:plc:viewer", "did:plc:other", "other", t0.Add(time.Minute))))
	gone := t0.Add(2 * time.Minute)
	require.NoError(t, mod.CreateChatMessage(ctx, chatRow(t, "bafygone", "did:plc:viewer", streamer, "gone", t0.Add(90*time.Second))))
	require.NoError(t, mod.DeleteChatMessage(ctx, "at://did:plc:viewer/place.stream.chat.message/bafygone", &gone))

	from, to := t0, t0.Add(4*time.Minute) // ma..me
	page1, cur, err := mod.ChatMessagesBetween(ctx, streamer, from, to, "", 3)
	require.NoError(t, err)
	require.NotEmpty(t, cur, "more to come")
	page2, cur2, err := mod.ChatMessagesBetween(ctx, streamer, from, to, cur, 3)
	require.NoError(t, err)
	require.Empty(t, cur2, "last page")
	var got []string
	for _, m := range append(page1, page2...) {
		got = append(got, m.Record.Val.(*placestream.ChatMessage).Text)
	}
	require.Equal(t, []string{"ma", "mb", "mc", "md", "me"}, got, "in order, only the window, only this streamer, not the deleted one")

	_, _, err = mod.ChatMessagesBetween(ctx, streamer, from, to, "garbage", 3)
	require.Error(t, err)
}
