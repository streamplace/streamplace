package websocketrep

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	official "moq.dev/moq"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/viewlog"
	"stream.place/streamplace/pkg/vod"
)

// fakeVideos resolves the videos it holds, keyed "did/rkey/sid", and
// refuses the rest.
type fakeVideos map[string]*vod.ServableVideo

func (f fakeVideos) ResolveServableVideo(_ context.Context, did, rkey, sid string) (*vod.ServableVideo, error) {
	if video, ok := f[did+"/"+rkey+"/"+sid]; ok {
		return video, nil
	}
	return nil, errors.New("no such video")
}

// vodFixture stores a content blob and returns a video over it: two
// segments on track 1, one on track 2.
func vodFixture(t *testing.T, ctx context.Context) (blob.Store, []byte, *vod.ServableVideo) {
	t.Helper()
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	content := make([]byte, 256<<10)
	for i := range content {
		content[i] = byte(i ^ i>>8)
	}
	w, err := store.NewWriter(ctx, vod.BlobsPrefix+"bafyvideo.mp4", "video/mp4")
	require.NoError(t, err)
	_, err = w.Write(content)
	require.NoError(t, err)
	require.NoError(t, w.Complete())
	return store, content, &vod.ServableVideo{
		OwnerDID: "did:plc:owner",
		SID:      "sid1",
		BlobCID:  "bafyvideo",
		Tracks: map[string][]vod.ByteRange{
			"1": {{Offset: 16, Size: 1000}, {Offset: 1016, Size: 200 << 10}},
			"2": {{Offset: 250000, Size: 512}},
		},
	}
}

// fetchVODGroup fetches one group and returns its frames' payloads.
func fetchVODGroup(ctx context.Context, broadcast *official.BroadcastConsumer, track string, sequence uint64) ([][]byte, error) {
	group, err := broadcast.FetchGroup(ctx, track, sequence, nil)
	if err != nil {
		return nil, err
	}
	defer group.Cancel()
	var frames [][]byte
	for frame, err := range group.Frames(ctx) {
		if err != nil {
			return nil, err
		}
		frames = append(frames, frame.Payload)
	}
	return frames, nil
}

// A viewer fetching a video's segments through the listener gets, as group
// i of a track, one frame holding exactly segment i's byte range of the
// content blob, logged as a segment request of the session. A path that is
// not vod/<did>/<rkey>/<sid>, a video the resolver refuses, and a track or
// segment the video does not have are refused.
func TestMoqServesVOD(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	store, content, video := vodFixture(t, ctx)
	viewLog, err := viewlog.NewWriter(viewlog.Config{Store: store, NodeDID: "did:web:node.test"})
	require.NoError(t, err)
	go viewLog.Run(ctx)
	r := &WebsocketReplicator{
		bus:         bus.NewBus(),
		cli:         &config.CLI{VODMoQ: true},
		VODResolver: fakeVideos{"did:plc:owner/rkey/sid1": video},
		VODStore:    store,
		ViewLog:     viewLog,
	}
	client := listenMoq(t, ctx, r)

	const path = "vod/did:plc:owner/rkey/sid1"
	// The refusal reaches a viewer by the time it fetches.
	for _, refused := range []string{"vod/did:plc:owner/rkey", path + "/extra", "vod/did:plc:owner/rkey/revoked"} {
		broadcast, err := requestMoqBroadcast(ctx, client, refused)
		if err == nil {
			_, err = fetchVODGroup(ctx, broadcast, "1", 0)
		}
		require.Error(t, err, refused)
	}
	require.NoError(t, ctx.Err(), "refused, not waited on")

	broadcast, err := requestMoqBroadcast(ctx, client, path)
	require.NoError(t, err)
	var logged []viewlog.Event
	for _, fetch := range []struct {
		track    string
		sequence uint64
	}{{"1", 1}, {"2", 0}, {"1", 0}} {
		segment := video.Tracks[fetch.track][fetch.sequence]
		frames, err := fetchVODGroup(ctx, broadcast, fetch.track, fetch.sequence)
		require.NoError(t, err)
		require.Len(t, frames, 1, "a segment is one frame")
		require.True(t, bytes.Equal(content[segment.Offset:segment.Offset+segment.Size], frames[0]),
			"track %s group %d is its segment's bytes", fetch.track, fetch.sequence)
		logged = append(logged, viewlog.Event{
			Type:       viewlog.EventTypeSegmentRequest,
			SID:        "sid1",
			CID:        "bafyvideo",
			OwnerDID:   "did:plc:owner",
			RangeStart: segment.Offset,
			RangeEnd:   segment.Offset + segment.Size - 1,
		})
	}
	_, err = fetchVODGroup(ctx, broadcast, "1", 2)
	require.Error(t, err, "past the last segment")
	_, err = fetchVODGroup(ctx, broadcast, "3", 0)
	require.Error(t, err, "a track the video does not have")
	require.NoError(t, ctx.Err(), "refused, not waited on")

	require.NoError(t, viewLog.Close())
	keys, err := store.List(ctx, "view-logs/")
	require.NoError(t, err)
	require.Len(t, keys, 1)
	f, err := store.Open(ctx, keys[0])
	require.NoError(t, err)
	defer f.Close()
	gz, err := gzip.NewReader(io.NewSectionReader(f, 0, f.Size()))
	require.NoError(t, err)
	var got []viewlog.Event
	for lines := bufio.NewScanner(gz); lines.Scan(); {
		var ev viewlog.Event
		require.NoError(t, json.Unmarshal(lines.Bytes(), &ev))
		ev.Ts = time.Time{}
		got = append(got, ev)
	}
	require.Equal(t, logged, got)
}

// A broadcast nothing is requested from ends on its own, so a viewer who
// left does not hold it open, and its tracks are no longer served.
func TestMoqVODEndsWhenIdle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	timeout := vodIdleTimeout
	vodIdleTimeout = 200 * time.Millisecond
	t.Cleanup(func() { vodIdleTimeout = timeout })
	store, _, video := vodFixture(t, ctx)
	const path = "vod/did:plc:owner/rkey/sid1"
	r := &WebsocketReplicator{VODResolver: fakeVideos{"did:plc:owner/rkey/sid1": video}, VODStore: store}

	origin := official.NewOriginProducer()
	requests, err := origin.Dynamic("", official.Route{})
	require.NoError(t, err)
	defer requests.Cancel()
	requested := make(chan *official.BroadcastConsumer, 1)
	go func() {
		broadcast, _ := origin.Consume().RequestBroadcast(ctx, path)
		requested <- broadcast
	}()
	req, err := requests.RequestedBroadcast(ctx)
	require.NoError(t, err)
	ended := make(chan struct{})
	go func() {
		defer close(ended)
		r.serveVOD(ctx, req, strings.Split(path, "/"))
	}()
	broadcast := <-requested
	require.NotNil(t, broadcast)
	select {
	case <-ended:
	case <-ctx.Done():
		t.Fatal("an idle broadcast was served forever")
	}
	_, err = fetchVODGroup(ctx, broadcast, "1", 0)
	require.Error(t, err)
	require.NoError(t, ctx.Err(), "refused, not waited on")
}
