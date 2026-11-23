package websocketrep

import (
	"context"
	"fmt"
	"sync"
	"time"

	official "moq.dev/moq"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/viewlog"
	"stream.place/streamplace/pkg/vod"
)

// vodIdleTimeout ends a viewer's VOD broadcast once nothing has been
// requested from it for this long; a viewer who comes back requests it again.
var vodIdleTimeout = 5 * time.Minute

// serveVOD serves one viewer's video at vod/<did>/<rkey>/<sid> (see
// docs/vod-instant-start.md): a track per MUXL track, requested by name,
// whose group i is segment i's bytes, read from the blob store when fetched.
func (r *WebsocketReplicator) serveVOD(ctx context.Context, req *official.BroadcastRequest, parts []string) {
	if len(parts) != 4 || r.VODResolver == nil || r.VODStore == nil {
		_ = req.Reject(404)
		return
	}
	video, err := r.VODResolver.ResolveServableVideo(ctx, parts[1], parts[2], parts[3])
	if err != nil {
		_ = req.Reject(404)
		return
	}
	broadcast, err := official.NewBroadcastProducer()
	if err != nil {
		_ = req.Reject(500)
		return
	}
	defer broadcast.Close()
	tracks, err := broadcast.Dynamic()
	if err != nil {
		_ = req.Reject(500)
		return
	}
	defer tracks.Cancel()
	if err := req.Accept(broadcast); err != nil {
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	idle := time.AfterFunc(vodIdleTimeout, cancel)
	defer idle.Stop()
	var wg sync.WaitGroup
	defer wg.Wait()
	for treq, err := range tracks.Requests(ctx) {
		if err != nil {
			return
		}
		idle.Reset(vodIdleTimeout)
		name, err := treq.Name()
		ranges, ok := video.Tracks[name]
		if err != nil || !ok {
			_ = treq.Abort(404)
			continue
		}
		track, groups, err := acceptVODTrack(treq, uint64(len(ranges)))
		if err != nil {
			log.Warn(ctx, "moq: could not serve a vod track", "blob", video.BlobCID, "track", name, "error", err)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Held until the broadcast ends (a dropped producer ends its track),
			// then aborted so a late fetch fails instead of waiting.
			defer func() { _ = track.Abort(410) }()
			for greq, err := range groups.Requests(ctx) {
				if err != nil {
					return
				}
				idle.Reset(vodIdleTimeout)
				// A group per goroutine: one slow blob read holds up nothing else.
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := r.serveVODGroup(ctx, video, ranges, greq); err != nil {
						log.Warn(ctx, "moq: could not serve a vod group", "blob", video.BlobCID, "track", name, "group", greq.Sequence(), "error", err)
					}
				}()
			}
		}()
	}
}

// acceptVODTrack accepts a requested track as one that ends after its last
// segment and serves every group on demand, since the transport drops the
// groups it caches once they sit idle.
func acceptVODTrack(req *official.TrackRequest, segments uint64) (*official.TrackProducer, *official.TrackDynamic, error) {
	groups, err := req.Dynamic()
	if err != nil {
		_ = req.Abort(500)
		return nil, nil, err
	}
	// Segments are fetched in any order, so none is ever too old to deliver.
	forever := uint64(1<<62 - 1)
	track, err := req.Accept(&official.TrackInfo{Priority: 127, MaxAgeUs: &forever})
	if err != nil {
		groups.Cancel()
		return nil, nil, err
	}
	if err := track.FinishAt(segments); err != nil {
		groups.Cancel()
		_ = track.Abort(500)
		return nil, nil, err
	}
	return track, groups, nil
}

// serveVODGroup answers a fetch of one segment with a single frame, the
// segment's bytes, recorded in the view log like an HTTP range request.
func (r *WebsocketReplicator) serveVODGroup(ctx context.Context, video *vod.ServableVideo, ranges []vod.ByteRange, req *official.GroupRequest) error {
	if req.Sequence() >= uint64(len(ranges)) {
		return req.Abort(404)
	}
	segment := ranges[req.Sequence()]
	payload, err := r.readVODRange(ctx, video.BlobCID, segment)
	if err != nil {
		_ = req.Abort(500)
		return err
	}
	if r.ViewLog != nil {
		// No IP hash: the listener doesn't learn which session fetched a group.
		r.ViewLog.Log(ctx, viewlog.Event{
			Ts:         time.Now().UTC(),
			Type:       viewlog.EventTypeSegmentRequest,
			SID:        video.SID,
			CID:        video.BlobCID,
			OwnerDID:   video.OwnerDID,
			RangeStart: segment.Offset,
			RangeEnd:   segment.Offset + segment.Size - 1,
		})
	}
	group, err := req.Accept()
	if err != nil {
		return err
	}
	// The timestamp is 0: the player takes each segment's time from its tfdt.
	if err := group.WriteFrame(official.Frame{Payload: payload}); err != nil {
		_ = group.Abort(500)
		return err
	}
	return group.Finish()
}

func (r *WebsocketReplicator) readVODRange(ctx context.Context, cid string, segment vod.ByteRange) ([]byte, error) {
	content, err := r.VODStore.Open(ctx, vod.BlobsPrefix+cid+".mp4")
	if err != nil {
		return nil, err
	}
	defer content.Close()
	if segment.Offset < 0 || segment.Size <= 0 || segment.Offset+segment.Size > content.Size() {
		return nil, fmt.Errorf("segment %d+%d is outside the %d byte blob", segment.Offset, segment.Size, content.Size())
	}
	payload := make([]byte, segment.Size)
	// ReadAt may report io.EOF along with a full read at the end of the blob.
	if n, err := content.ReadAt(payload, segment.Offset); n < len(payload) {
		return nil, err
	}
	return payload, nil
}
