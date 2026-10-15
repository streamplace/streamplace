package websocketrep

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	official "moq.dev/moq"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/moq"
	"stream.place/streamplace/pkg/muxl"
)

// Sign real fragmented media with a creator bound to its certificate. Unlike
// shadow-ingest fixtures this manifest explicitly authorizes publication.
func signedMoqArchiveSource(t *testing.T, deleteAfter int64) (string, []byte) {
	t.Helper()
	gstinit.InitGST()
	ms, err := media.NewEphemeralMediaSigner("archive-lifecycle")
	require.NoError(t, err)
	key, err := atproto.ParsePubKey(ms.Signer.Public().(*ecdsa.PublicKey))
	require.NoError(t, err)
	did := key.DIDKey()
	ms.PrebuiltManifest, err = json.Marshal(map[string]any{
		"title": "Archive lifecycle regression",
		"assertions": []any{
			map[string]any{"label": "c2pa.actions", "data": map[string]any{"actions": []any{map[string]any{"action": "c2pa.created"}, map[string]any{"action": "c2pa.published"}}}},
			map[string]any{"label": "cawg.metadata", "data": map[string]any{"@context": map[string]string{"dc": "http://purl.org/dc/elements/1.1/"}, "dc:creator": did, "dc:title": "Archive lifecycle regression", "dc:date": time.Now().UTC().Format(time.RFC3339Nano)}},
			map[string]any{"label": "place.stream.metadata.configuration", "data": map[string]any{"distributionPolicy": map[string]any{"deleteAfter": deleteAfter}}},
		},
	})
	require.NoError(t, err)
	_, filename, _, _ := runtime.Caller(0)
	fragment, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "..", "test", "fixtures", "h264-opus-frag.mp4"))
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	events := make(chan *muxl.MuxlEvent, 16)
	finished := make(chan error, 1)
	go func() {
		finished <- ms.SignSegmentStream(ctx, bytes.NewReader(fragment), events)
		close(events)
	}()
	var payload []byte
	for event := range events {
		if event.Type != "signed-segment" || payload != nil {
			continue
		}
		ids := make([]int, 0, len(event.Tracks))
		for id := range event.Tracks {
			n, err := strconv.Atoi(id)
			require.NoError(t, err)
			ids = append(ids, n)
		}
		sort.Ints(ids)
		for _, id := range ids {
			payload = append(payload, event.Tracks[strconv.Itoa(id)]...)
		}
	}
	require.NoError(t, <-finished)
	require.True(t, moq.IsMuxl(payload))
	return did, payload
}

func archiveTestReplicator(dir string) *WebsocketReplicator {
	cli := &config.CLI{DataDir: dir, WideOpen: true, SegmentArchiveRetention: time.Hour, StreamSessionTimeout: time.Second}
	return &WebsocketReplicator{
		cli: cli, mm: media.NewOffline(cli),
		moqOrigin: official.NewOriginProducer(), moqLocalOrigin: official.NewOriginProducer(),
		moqBroadcasts: make(map[string]*moqBroadcast),
	}
}

func closeArchiveTestReplicator(r *WebsocketReplicator) {
	for _, b := range r.moqBroadcasts {
		if b.archiveRoute != nil {
			b.archiveRoute.Cancel()
		}
		for _, track := range b.tracks {
			track.publicFetch.Cancel()
			track.localFetch.Cancel()
			track.groups.close()
		}
		_ = b.public.Close()
		_ = b.local.Close()
	}
}

func TestMoqRestartFetchDoesNotReplayOrPublishLive(t *testing.T) {
	did, payload := signedMoqArchiveSource(t, 3600)
	dir := t.TempDir()
	first := archiveTestReplicator(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	meta, err := first.mm.ValidateArchivedMP4(ctx, bytes.NewReader(payload), did)
	require.NoError(t, err)
	timestamp := uint64(meta.StartTime.Time().UnixMicro())
	require.NoError(t, first.storeMoqArchive(ctx, did, moq.SourceTrack, payload, meta))
	closeArchiveTestReplicator(first)

	restarted := archiveTestReplicator(dir)
	defer closeArchiveTestReplicator(restarted)
	require.NoError(t, restarted.restoreMoqInventory(ctx))
	b := restarted.moqBroadcasts[did]
	require.NotNil(t, b)
	require.NotNil(t, b.archiveRoute)
	require.True(t, b.lastLocal.IsZero())
	require.True(t, b.lastPublic.IsZero())
	require.Nil(t, b.catalog.SourceSequence)
	require.Nil(t, restarted.mm.GetLiveWindow(did))
	source := b.tracks[moq.SourceTrack]
	seq, ok := source.archive.sequenceAt(timestamp)
	require.True(t, ok)
	require.Equal(t, uint64(0), seq)
	go serveMoqArchiveRoute(ctx, b, b.archiveRoute)
	go restarted.serveMoqFetches(ctx, source.publicFetch, b, moq.SourceTrack, source)
	bc, err := restarted.moqOrigin.Consume().RequestBroadcast(ctx, did)
	require.NoError(t, err)
	frame, err := fetchMoqFrame(ctx, bc, moq.SourceTrack, seq)
	require.NoError(t, err)
	require.Equal(t, payload, frame.Payload)
	require.Equal(t, timestamp, frame.TimestampUs)
	require.Nil(t, restarted.mm.GetLiveWindow(did), "FETCH must not notify live validation")

	live, err := bc.SubscribeTrack(ctx, moq.SourceTrack, nil)
	require.NoError(t, err)
	defer live.Cancel()
	short, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	defer stop()
	group, err := live.RecvGroup(short)
	require.Error(t, err, "archived FETCH must not insert groups into the live track")
	require.Nil(t, group)
	_, err = restarted.moqOrigin.Consume().RequestBroadcast(ctx, did+"/other")
	require.Error(t, err, "DID prefix route must reject nonexact paths")
	localAnnouncements, err := restarted.moqLocalOrigin.Consume().Announced(official.AnnounceOptions{})
	require.NoError(t, err)
	defer localAnnouncements.Cancel()
	short, stop = context.WithTimeout(ctx, 100*time.Millisecond)
	defer stop()
	announcement, err := localAnnouncements.Next(short)
	require.Error(t, err)
	require.Nil(t, announcement, "retained media must not be advertised to push peers")
}

func TestMoqSignedExpiryAbortsNativeCacheHits(t *testing.T) {
	did, payload := signedMoqArchiveSource(t, 10)
	r := archiveTestReplicator(t.TempDir())
	defer closeArchiveTestReplicator(r)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	meta, err := r.mm.ValidateArchivedMP4(ctx, bytes.NewReader(payload), did)
	require.NoError(t, err)
	signedExpiry, err := moqArchiveExpiry(meta, time.Now(), 0)
	require.NoError(t, err)
	require.False(t, signedExpiry.IsZero(), "signed DeleteAfter must be extracted")
	expiry := signedExpiry
	broadcast, err := official.NewBroadcastProducer()
	require.NoError(t, err)
	defer broadcast.Close()
	track, err := broadcast.PublishTrack(moq.SourceTrack, nil)
	require.NoError(t, err)
	consumer, err := broadcast.Consume()
	require.NoError(t, err)
	var handles moqExpiringGroups
	defer handles.close()
	stored := &archivedGroup{seq: 1, timestampUs: uint64(meta.StartTime.Time().UnixMicro()), payload: payload, expiresAt: expiry}
	require.NoError(t, writeMoqGroup(track, stored, &handles))
	frame, err := fetchMoqFrame(ctx, consumer, moq.SourceTrack, 1)
	require.NoError(t, err)
	require.Equal(t, payload, frame.Payload)
	require.NoError(t, r.storeMoqArchive(ctx, did, moq.SourceTrack, payload, meta))
	b := r.moqBroadcasts[did]
	archivedTrack := b.tracks[moq.SourceTrack]
	go r.serveMoqFetches(ctx, archivedTrack.publicFetch, b, moq.SourceTrack, archivedTrack)
	archiveConsumer, err := b.public.Consume()
	require.NoError(t, err)
	archivedFrame, err := fetchMoqFrame(ctx, archiveConsumer, moq.SourceTrack, 0)
	require.NoError(t, err)
	require.Equal(t, payload, archivedFrame.Payload)
	// No newer group is published: MaxAge cannot be relied on to evict this.
	time.Sleep(time.Until(expiry) + 50*time.Millisecond)
	frame, err = fetchMoqFrame(ctx, consumer, moq.SourceTrack, 1)
	require.Error(t, err, "native cache hit after expiry must fail")
	require.Nil(t, frame)
	archivedFrame, err = fetchMoqFrame(ctx, archiveConsumer, moq.SourceTrack, 0)
	require.Error(t, err, "dynamic FETCH cache fill must also expire")
	require.Nil(t, archivedFrame)
	_, found, err := archivedTrack.archive.get(0)
	require.NoError(t, err)
	require.False(t, found, "expired disk group must not be refetched")
	handles.mu.Lock()
	pending := len(handles.pending)
	handles.mu.Unlock()
	require.Zero(t, pending, "expiry must release bounded native handle metadata")
}

func TestMoqInventoryRejectsNamespaceAndTrackTampering(t *testing.T) {
	r := archiveTestReplicator(t.TempDir())
	defer closeArchiveTestReplicator(r)
	did := "did:plc:2j2ounbiyi3ftihronlw5qhj"
	b, err := r.moqBroadcast(did)
	require.NoError(t, err)
	b.mu.Lock()
	_, err = r.publishedTrack(b, did, moq.SourceTrack)
	b.mu.Unlock()
	require.NoError(t, err)
	dir := filepath.Dir(r.archivePath(did, moq.SourceTrack))
	inventory, err := readMoqInventory(dir)
	require.NoError(t, err)
	inventory.Streamer = "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	data, err := json.Marshal(inventory)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "inventory.json"), data, 0600))
	_, err = readMoqInventory(dir)
	require.ErrorContains(t, err, "namespace hash mismatch")
	inventory.Streamer = did
	inventory.Tracks = []string{"../escape"}
	data, err = json.Marshal(inventory)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "inventory.json"), data, 0600))
	_, err = readMoqInventory(dir)
	require.ErrorContains(t, err, "invalid archived track")
}
