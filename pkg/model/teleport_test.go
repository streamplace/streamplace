package model

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetTeleportForViewer(t *testing.T) {
	db := indexedTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	teleports := []Teleport{
		{
			URI:      "at://did:plc:streamer/place.stream.live.teleport/3lteleport0",
			CID:      "expired-cid",
			RepoDID:  "did:plc:streamer",
			StartsAt: now.Add(-time.Minute),
		},
		{
			URI:      "at://did:plc:streamer/place.stream.live.teleport/3lteleport1",
			RepoDID:  "did:plc:streamer",
			StartsAt: now.Add(2 * time.Minute),
		},
		{
			URI:      "at://did:plc:streamer/place.stream.live.teleport/3lteleport2",
			RepoDID:  "did:plc:streamer",
			StartsAt: now.Add(time.Minute),
		},
		{
			URI:      "at://did:plc:streamer/place.stream.live.teleport/denied",
			RepoDID:  "did:plc:streamer",
			StartsAt: now.Add(2 * time.Minute),
			Denied:   true,
		},
	}
	for i := range teleports {
		require.NoError(t, db.CreateTeleport(ctx, &teleports[i]))
	}

	got, err := db.GetTeleportForViewer("did:plc:streamer", "", "")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, teleports[2].URI, got.URI)

	got, err = db.GetTeleportForViewer("did:plc:streamer", teleports[0].URI, teleports[0].CID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, teleports[2].URI, got.URI, "a newer pending teleport supersedes the known record")
}

func TestGetTeleportForViewerReturnsNilWhenNoneArePending(t *testing.T) {
	db := indexedTestDB(t)

	got, err := db.GetTeleportForViewer("did:plc:streamer", "", "")
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestGetTeleportForViewerReconnect(t *testing.T) {
	const repo = "did:plc:streamer"
	const uri = "at://did:plc:streamer/place.stream.live.teleport/1"
	for _, tt := range []struct {
		name       string
		requestURI string
		requestCID string
		repo       string
		denied     bool
		deleted    bool
		want       bool
	}{
		{name: "known teleport has started", requestURI: uri, requestCID: "cid", repo: repo, want: true},
		{name: "new viewer gets no historical teleport", repo: repo},
		{name: "missing CID", requestURI: uri, repo: repo},
		{name: "different version", requestURI: uri, requestCID: "old-cid", repo: repo},
		{name: "different stream", requestURI: uri, requestCID: "cid", repo: "did:plc:other"},
		{name: "denied", requestURI: uri, requestCID: "cid", repo: repo, denied: true},
		{name: "deleted", requestURI: uri, requestCID: "cid", repo: repo, deleted: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db := indexedTestDB(t)
			ctx := context.Background()
			require.NoError(t, db.CreateTeleport(ctx, &Teleport{
				URI: uri, CID: "cid", RepoDID: repo,
				StartsAt: time.Now().Add(-time.Second), Denied: tt.denied,
			}))
			if tt.deleted {
				require.NoError(t, db.DeleteTeleport(ctx, uri))
			}

			got, err := db.GetTeleportForViewer(tt.repo, tt.requestURI, tt.requestCID)
			require.NoError(t, err)
			if tt.want {
				require.NotNil(t, got)
				require.Equal(t, uri, got.URI)
			} else {
				require.Nil(t, got)
			}
		})
	}
}
