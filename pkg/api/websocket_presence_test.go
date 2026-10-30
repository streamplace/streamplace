package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
)

func TestViewerCountIncludesChannelPresence(t *testing.T) {
	b := bus.NewBus()
	a := &StreamplaceAPI{Bus: b}
	b.SetViewerCount("repo", "test", 7)
	viewer := b.SubscribeViewer("repo", nil)
	t.Cleanup(func() { b.Unsubscribe("repo", viewer) })

	message := a.viewerCountMessage(context.Background(), "repo")
	require.EqualValues(t, 7, message.Count)
	require.NotNil(t, message.Presence)
	require.EqualValues(t, 1, *message.Presence)

	b.Unsubscribe("repo", viewer)
	message = a.viewerCountMessage(context.Background(), "repo")
	require.EqualValues(t, 7, message.Count)
	require.NotNil(t, message.Presence)
	require.Zero(t, *message.Presence)
	data, err := json.Marshal(message)
	require.NoError(t, err)
	require.Contains(t, string(data), `"presence":0`)
}
