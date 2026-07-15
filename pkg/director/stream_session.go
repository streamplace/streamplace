package director

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/util"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/google/uuid"
	glex "github.com/streamplace/glex/runtime"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/livepeer"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/renditions"
	"stream.place/streamplace/pkg/replication"
	"stream.place/streamplace/pkg/s3"
	"stream.place/streamplace/pkg/spmetrics"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/thumbnail"
)

type StreamSession struct {
	mm                     *media.MediaManager
	mod                    model.Model
	cli                    *config.CLI
	bus                    *bus.Bus
	op                     *oatproxy.OATProxy
	lp                     *livepeer.LivepeerSession
	repoDID                string
	segmentChan            chan struct{}
	lastStatus             time.Time
	lastStatusCID          *string
	lastOriginTime         time.Time
	localDB                localdb.LocalDB
	streamReceivedNotified bool

	// Channels for background workers
	statusUpdateChan     chan struct{} // Signal to update status
	originUpdateChan     chan struct{} // Signal to update broadcast origin
	livestreamUpdateChan chan struct{} // Signal to update livestream
	viewCountUpdateChan  chan struct{} // Signal to update view count in server repo

	g          *errgroup.Group
	started    chan struct{}
	ctx        context.Context
	packets    []bus.PacketizedSegment
	statefulDB *statedb.StatefulDB
	replicator replication.Replicator
	atsync     *atproto.ATProtoSynchronizer

	lastLivestreamTime time.Time
	lastViewCountTime  time.Time
	s3Uploader         *s3.S3Uploader
}

// bitrateMargin is the wiggle room over the configured maximum before a stream
// is disconnected, so an occasional spiky GoP (e.g. a scene cut) doesn't kill an
// otherwise-compliant stream.
const bitrateMargin = 1.1

// exceedsMaxBitrate returns a segment's bitrate (bits/sec, from its emitted size
// and duration) and whether that exceeds maxBitrate (bits/sec) by more than
// bitrateMargin. maxBitrate <= 0 disables the check; a non-positive duration
// can't yield a meaningful rate, so it never counts as exceeding.
func exceedsMaxBitrate(dataLen int, durationNS int64, maxBitrate int) (int, bool) {
	if maxBitrate <= 0 || durationNS <= 0 {
		return 0, false
	}
	seconds := float64(durationNS) / float64(time.Second)
	bitrate := int(float64(dataLen) * 8 / seconds)
	return bitrate, float64(bitrate) > float64(maxBitrate)*bitrateMargin
}

func (ss *StreamSession) Start(ctx context.Context, notif *media.NewSegmentNotification) error {
	ctx, cancel := context.WithCancel(ctx)
	spmetrics.StreamSessions.WithLabelValues(notif.Segment.RepoDID).Inc()
	ss.g, ctx = errgroup.WithContext(ctx)
	sid := livepeer.RandomTrailer(8)
	ctx = log.WithLogValues(ctx, "sid", sid, "streamer", notif.Segment.RepoDID)
	ss.ctx = ctx
	log.Log(ctx, "starting stream session")
	defer cancel()
	spseg, err := notif.Segment.ToStreamplaceSegment()
	if err != nil {
		return fmt.Errorf("could not convert segment to streamplace segment: %w", err)
	}
	var allRenditions renditions.Renditions

	if ss.cli.LivepeerGatewayURL != "" {
		allRenditions, err = renditions.GenerateRenditions(spseg)
	} else {
		allRenditions = []renditions.Rendition{}
	}
	if err != nil {
		return err
	}
	if spseg.Duration == nil {
		return fmt.Errorf("segment duration is required to calculate bitrate")
	}
	dur := time.Duration(*spseg.Duration)
	byteLen := len(notif.Data)
	bitrate := int(float64(byteLen) / dur.Seconds() * 8)
	sourceRendition := renditions.Rendition{
		Name:    "source",
		Bitrate: bitrate,
		Width:   spseg.Video[0].Width,
		Height:  spseg.Video[0].Height,
	}
	allRenditions = append([]renditions.Rendition{sourceRendition}, allRenditions...)
	allRenditions = append(allRenditions, renditions.AudioRendition)

	ss.maybeStartS3Upload(ctx, notif.Segment.RepoDID)

	close(ss.started)

	// Start background workers for status, origin, and livestream updates
	ss.g.Go(func() error {
		return ss.statusUpdateLoop(ctx, spseg.Creator)
	})
	ss.g.Go(func() error {
		return ss.originUpdateLoop(ctx)
	})
	ss.g.Go(func() error {
		return ss.livestreamUpdateLoop(ctx, spseg.Creator)
	})
	ss.g.Go(func() error {
		return ss.viewCountUpdateLoop(ctx, spseg.Creator)
	})

	if notif.Local {
		ss.Go(ctx, func() error {
			return ss.HandleMultistreamTargets(ctx)
		})
	}

	for {
		select {
		case <-ss.segmentChan:
			// reset timer
		case <-ctx.Done():
			// Drain all in-flight session goroutines (including the per-segment
			// AddSegment senders) BEFORE closing the uploader, so no segment
			// send can race the uploader's channel close. s3Close then flushes
			// the final object on the uploader's own (still-live) context.
			err := ss.g.Wait()
			ss.s3Close(ctx)
			return err
		// case <-time.After(time.Minute * 1):
		case <-time.After(ss.cli.StreamSessionTimeout):
			log.Log(ctx, "stream session timeout, shutting down", "timeout", ss.cli.StreamSessionTimeout)
			spmetrics.StreamSessions.WithLabelValues(notif.Segment.RepoDID).Dec()
			for _, r := range allRenditions {
				ss.bus.EndSession(ctx, spseg.Creator, r.Name)
			}
			// Signal background workers to stop
			if notif.Local {
				ss.Go(ctx, func() error {
					return ss.DeleteStatus(spseg.Creator)
				})
			}
			cancel()
		}
	}
}

// Execute a goroutine in the context of the stream session. Errors are
// non-fatal; if you actually want to melt the universe on an error you
// should panic()
func (ss *StreamSession) Go(ctx context.Context, f func() error) {
	<-ss.started
	ss.g.Go(func() error {
		err := f()
		if err != nil {
			log.Error(ctx, "error in stream_session goroutine", "error", err)
		}
		return nil
	})
}

func (ss *StreamSession) NewSegment(ctx context.Context, notif *media.NewSegmentNotification) error {
	<-ss.started
	go func() {
		select {
		case <-ss.ctx.Done():
			return
		case ss.segmentChan <- struct{}{}:
		}
	}()
	aqt := aqtime.FromTime(notif.Segment.StartTime)
	ctx = log.WithLogValues(ctx, "segID", notif.Segment.ID, "repoDID", notif.Segment.RepoDID, "timestamp", aqt.FileSafeString())

	// Enforce the node's max live bitrate, inferred per emitted segment. A stream
	// over the limit (plus a margin for spiky GoPs) is kicked: the StreamKick both
	// tears the ingest down — via watchKeyRevocation, on every ingest path
	// including isolated workers — and surfaces a place.stream.error "problem" to
	// the streamer's dashboard explaining the disconnect.
	//
	// We kick on EVERY over-limit segment, not once per session: the kick only
	// ends the ingest connection, not this director session (which lingers until
	// StreamSessionTimeout). If we latched it, an encoder that auto-reconnects
	// within that window would reuse this session and stream on unchecked. The
	// early return keeps the over-limit segment from being distributed, so no
	// healthy segment overwrites the dashboard problem until the streamer fixes
	// their bitrate and reconnects clean — at which point findProblems clears it.
	// The client dedupes place.stream.error by code, so repeated kicks surface as
	// a single persistent problem.
	if bitrate, exceeded := exceedsMaxBitrate(len(notif.Data), notif.Segment.MediaData.Duration, ss.cli.MaximumLiveBitrate); exceeded {
		log.Log(ctx, "live bitrate exceeded maximum, disconnecting stream",
			"streamer", notif.Segment.RepoDID, "bitrate", bitrate, "max", ss.cli.MaximumLiveBitrate)
		ss.bus.Publish(notif.Segment.RepoDID, media.NewStreamKick(
			"bitrate",
			fmt.Sprintf("Your stream's bitrate (%d kbps) exceeds this server's maximum of %d kbps. Lower your encoder's bitrate to keep streaming.",
				bitrate/1000, ss.cli.MaximumLiveBitrate/1000),
		))
		return nil
	}

	notif.Segment.MediaData.Size = len(notif.Data)
	err := ss.localDB.CreateSegment(notif.Segment)
	if err != nil {
		return fmt.Errorf("could not add segment to database: %w", err)
	}
	spseg, err := notif.Segment.ToStreamplaceSegment()
	if err != nil {
		return fmt.Errorf("could not convert segment to streamplace segment: %w", err)
	}

	// stream.received is enqueued once per StreamSession when the first local media segment is accepted.
	if notif.Local && !ss.streamReceivedNotified {
		ss.streamReceivedNotified = true
		ss.Go(ctx, func() error {
			task := &statedb.StreamReceivedTask{
				StreamerDID: spseg.Creator,
			}
			_, err := ss.statefulDB.EnqueueTask(ctx, statedb.TaskStreamReceived, task, statedb.WithTaskKey(fmt.Sprintf("stream-received::%s::%s", spseg.Creator, notif.Segment.ID)))
			if err != nil {
				log.Error(ctx, "failed to enqueue stream.received task", "err", err)
			}
			return nil
		})
	}

	// Record to S3 for live-to-VOD only while the stream is live (an un-ended
	// place.stream.livestream record exists -> the segment is published).
	// Segments that arrive before "go live" or after the livestream ends are
	// NOT recorded: pushing them produced over-long VODs (the post-stop tail
	// got absorbed into the recording). The moment publishing stops we complete
	// the current object so it's immediately finalize-able instead of lingering
	// un-completed (which made finalize report "no recorded S3 segments").
	if notif.Metadata.Published {
		ss.s3Upload(ctx, notif)
	} else {
		ss.s3Cutover(ctx)
	}

	ss.bus.Publish(spseg.Creator, spseg)
	ss.Go(ctx, func() error {
		return ss.AddPlaybackSegment(ctx, spseg, "source", &bus.Seg{
			Filepath:  notif.Segment.ID,
			Data:      notif.Data,
			Muxl:      notif.Muxl,
			Published: notif.Metadata.Published,
		})
	})

	if notif.Local {
		ss.Go(ctx, func() error {
			return ss.statefulDB.UpsertBroadcastOrigin(spseg.Creator, ss.cli.ServerDID(), time.Now())
		})
	}

	if ss.cli.Thumbnail {
		ss.Go(ctx, func() error {
			return ss.Thumbnail(ctx, spseg.Creator, notif)
		})
	}

	// everything else is for published segments
	if !notif.Metadata.Published {
		return nil
	}

	if notif.Local {
		ss.UpdateStatus(ctx, spseg.Creator)
		ss.UpdateBroadcastOrigin(ctx)
		ss.UpdateLivestream(ctx, spseg.Creator)
	}
	ss.UpdateViewCount(ctx)

	if ss.cli.LivepeerGatewayURL != "" {
		ss.Go(ctx, func() error {
			start := time.Now()
			err := ss.Transcode(ctx, spseg, notif.Data)
			took := time.Since(start)
			spmetrics.QueuedTranscodeDuration.WithLabelValues(spseg.Creator).Set(float64(took.Milliseconds()))
			return err
		})
	}

	// trigger a notification blast if this is a new livestream
	if notif.Metadata.Livestream != nil {
		ss.Go(ctx, func() error {
			r, err := ss.mod.GetRepoByHandleOrDID(spseg.Creator)
			if err != nil {
				return fmt.Errorf("failed to get repo: %w", err)
			}
			livestreamModel, err := ss.mod.GetLatestLivestreamForRepo(spseg.Creator)
			if err != nil {
				return fmt.Errorf("failed to get latest livestream for repo: %w", err)
			}
			if livestreamModel == nil {
				log.Warn(ctx, "no livestream found, skipping notification blast", "repoDID", spseg.Creator)
				return nil
			}
			// Refresh the S3 uploader's livestream tag now that this stream's
			// own record is indexed (it may not have been when the uploader
			// started), so all objects are attributed to the right stream.
			if ss.s3Uploader != nil {
				ss.s3Uploader.SetLivestreamURI(livestreamModel.URI)
			}
			lsv, err := livestreamModel.ToLivestreamView()
			if err != nil {
				return fmt.Errorf("failed to convert livestream to streamplace livestream: %w", err)
			}
			if !shouldNotify(lsv) {
				log.Debug(ctx, "is not set to notify", "repoDID", spseg.Creator)
				return nil
			}
			task := &statedb.NotificationTask{
				Livestream: *lsv,
				PDSURL:     r.PDS,
			}
			cp, err := ss.mod.GetChatProfile(ctx, spseg.Creator)
			if err != nil {
				return fmt.Errorf("failed to get chat profile: %w", err)
			}
			if cp != nil {
				spcp, err := cp.ToStreamplaceChatProfile()
				if err != nil {
					return fmt.Errorf("failed to convert chat profile to streamplace chat profile: %w", err)
				}
				task.ChatProfile = spcp
			}

			_, err = ss.statefulDB.EnqueueTask(ctx, statedb.TaskNotification, task, statedb.WithTaskKey(fmt.Sprintf("notification-blast::%s", lsv.Uri)))
			if err != nil {
				log.Error(ctx, "failed to enqueue notification task", "err", err)
			}
			return nil
		})
	} else {
		log.Warn(ctx, "no livestream detected in stream, skipping notification blast", "repoDID", spseg.Creator)
	}

	return nil
}

func shouldNotify(lsv *placestream.Livestream_LivestreamView) bool {
	lsvr, ok := lsv.Record.Val.(*placestream.Livestream)
	if !ok {
		return true
	}
	if lsvr.NotificationSettings == nil {
		return true
	}
	settings := lsvr.NotificationSettings
	if settings.PushNotification == nil {
		return true
	}
	return *settings.PushNotification
}

// thumbnailInterval is how often we refresh a user's thumbnail while they're
// live. A missing or older thumbnail (e.g. the user just went live) is
// regenerated immediately on the next segment.
const thumbnailInterval = 12 * time.Second

func (ss *StreamSession) Thumbnail(ctx context.Context, repoDID string, not *media.NewSegmentNotification) error {
	lock := thumbnail.GetThumbnailLock(repoDID)
	if !lock.TryLock() {
		// we're already generating a thumbnail for this user, skip
		return nil
	}
	defer lock.Unlock()

	if mt, ok := ss.cli.ThumbnailModTime(repoDID); ok && time.Since(mt) < thumbnailInterval {
		// current thumbnail is still fresh, keep it
		return nil
	}

	return ss.cli.ThumbnailWrite(repoDID, func(w io.Writer) error {
		return media.Thumbnail(ctx, bytes.NewReader(not.Data), w, "jpeg")
	})
}

// UpdateStatus signals the background worker to update status (non-blocking)
func (ss *StreamSession) UpdateStatus(ctx context.Context, repoDID string) {
	select {
	case ss.statusUpdateChan <- struct{}{}:
	default:
		// Channel full, signal already pending
	}
}

// statusUpdateLoop runs as a background goroutine for the session lifetime
func (ss *StreamSession) statusUpdateLoop(ctx context.Context, repoDID string) error {
	ctx = log.WithLogValues(ctx, "func", "statusUpdateLoop")
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ss.statusUpdateChan:
			if time.Since(ss.lastStatus) < time.Minute {
				log.Debug(ctx, "not updating status, last status was less than 1 minute ago")
				continue
			}
			if err := ss.doUpdateStatus(ctx, repoDID); err != nil {
				log.Error(ctx, "failed to update status", "error", err)
			}
		}
	}
}

// doUpdateStatus performs the actual status update work
func (ss *StreamSession) doUpdateStatus(ctx context.Context, repoDID string) error {
	ctx = log.WithLogValues(ctx, "func", "doUpdateStatus")

	client, err := ss.GetClientByDID(repoDID, atproto.ScopeBskyActorStatus)
	if errors.Is(err, statedb.ErrNoSessionWithScope) {
		log.Debug(ctx, "user declined Bluesky permissions, skipping live status update", "repoDID", repoDID)
		ss.lastStatus = time.Now()
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not get xrpc client: %w", err)
	}

	ls, err := ss.mod.GetLatestLivestreamForRepo(repoDID)
	if err != nil {
		return fmt.Errorf("could not get latest livestream for repoDID: %w", err)
	}
	if ls == nil {
		log.Debug(ctx, "no livestream found, skipping status update", "repoDID", repoDID)
		return nil
	}
	lsv, err := ls.ToLivestreamView()
	if err != nil {
		return fmt.Errorf("could not convert livestream to streamplace livestream: %w", err)
	}

	lsvr, ok := lsv.Record.Val.(*placestream.Livestream)
	if !ok {
		return fmt.Errorf("livestream is not a streamplace livestream")
	}
	thumb := lsvr.Thumb

	repo, err := ss.mod.GetRepoByHandleOrDID(repoDID)
	if err != nil {
		return fmt.Errorf("could not get repo for repoDID: %w", err)
	}

	lsr, ok := lsv.Record.Val.(*placestream.Livestream)
	if !ok {
		return fmt.Errorf("livestream is not a streamplace livestream")
	}

	if ss.cli.BroadcasterHost == "" {
		log.Debug(ctx, "no broadcaster host configured, skipping status update", "repoDID", repoDID)
		return nil
	}

	canonicalUrl := fmt.Sprintf("https://%s/%s", ss.cli.BroadcasterHost, repo.Handle)

	if lsr.CanonicalUrl != nil {
		canonicalUrl = *lsr.CanonicalUrl
	}

	actorStatusEmbed := appbsky.ActorStatus_Embed{
		EmbedExternal: &appbsky.EmbedExternal{
			External: appbsky.EmbedExternal_External{
				Title:       lsr.Title,
				Uri:         canonicalUrl,
				Description: fmt.Sprintf("@%s is 🔴LIVE on %s", repo.Handle, ss.cli.BroadcasterHost),
				Thumb:       thumb,
			},
		},
	}

	duration := int64(10)
	status := appbsky.ActorStatus{
		Status:          "app.bsky.actor.status#live",
		DurationMinutes: &duration,
		Embed:           &actorStatusEmbed,
		CreatedAt:       time.Now().Format(time.RFC3339),
	}

	var swapRecord *string
	getOutput := comatproto.RepoGetRecord_Output{}
	err = client.Do(ctx, xrpc.Query, "application/json", "com.atproto.repo.getRecord", map[string]any{
		"repo":       repoDID,
		"collection": "app.bsky.actor.status",
		"rkey":       "self",
	}, nil, &getOutput)
	if err != nil {
		xErr, ok := err.(*xrpc.Error)
		if !ok {
			return fmt.Errorf("could not get record: %w", err)
		}
		if xErr.StatusCode != 400 { // yes, they return "400" for "not found"
			return fmt.Errorf("could not get record: %w", err)
		}
		log.Debug(ctx, "record not found, creating", "repoDID", repoDID)
	} else {
		log.Debug(ctx, "got record", "record", getOutput)
		swapRecord = getOutput.Cid
	}

	inp := comatproto.RepoPutRecord_Input{
		Collection: "app.bsky.actor.status",
		Record:     &glex.LexiconTypeDecoder{Val: &status},
		Rkey:       "self",
		Repo:       repoDID,
		SwapRecord: swapRecord,
	}
	out := comatproto.RepoPutRecord_Output{}

	ss.lastStatusCID = &out.Cid

	err = client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", map[string]any{}, inp, &out)
	if err != nil {
		return fmt.Errorf("could not create record: %w", err)
	}
	log.Debug(ctx, "created status record", "out", out)

	ss.lastStatus = time.Now()

	return nil
}

var livestreamUpdateInterval = time.Second * 30

// UpdateLivestream signals the background worker to update the livestream record (non-blocking)
func (ss *StreamSession) UpdateLivestream(ctx context.Context, repoDID string) {
	select {
	case ss.livestreamUpdateChan <- struct{}{}:
		log.Debug(ctx, "livestream update signal sent")
	default:
		log.Debug(ctx, "livestream update channel full, signal already pending")
		// Channel full, signal already pending
	}
}

// livestreamUpdateLoop runs as a background goroutine for the session lifetime
func (ss *StreamSession) livestreamUpdateLoop(ctx context.Context, repoDID string) error {
	ctx = log.WithLogValues(ctx, "func", "livestreamUpdateLoop")
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ss.livestreamUpdateChan:
			if time.Since(ss.lastLivestreamTime) < livestreamUpdateInterval {
				log.Debug(ctx, "not updating livestream, last livestream was less than 30 seconds ago")
				continue
			}
			if err := ss.doUpdateLivestream(ctx, repoDID); err != nil {
				log.Error(ctx, "failed to update livestream", "error", err)
			}
		}
	}
}

// doUpdateLivestream performs the actual livestream record update work
func (ss *StreamSession) doUpdateLivestream(ctx context.Context, repoDID string) error {
	ctx = log.WithLogValues(ctx, "func", "doUpdateLivestream")

	lastLivestream, err := ss.mod.GetLatestLivestreamForRepo(repoDID)
	if err != nil {
		return fmt.Errorf("could not get latest livestream for repoDID: %w", err)
	}
	if lastLivestream == nil {
		log.Debug(ctx, "no livestream found, skipping livestream update")
		return nil
	}
	lsv, err := lastLivestream.ToLivestreamView()
	if err != nil {
		return fmt.Errorf("could not convert livestream to streamplace livestream: %w", err)
	}
	lsvr, ok := lsv.Record.Val.(*placestream.Livestream)
	if !ok {
		return fmt.Errorf("livestream is not a streamplace livestream")
	}

	aturi, err := syntax.ParseATURI(lastLivestream.URI)
	if err != nil {
		return fmt.Errorf("could not parse livestream URI: %w", err)
	}

	client, err := ss.GetClientByDID(ss.repoDID)
	if err != nil {
		return fmt.Errorf("could not get xrpc client for repoDID: %w", err)
	}

	var swapRecord *string
	getOutput := comatproto.RepoGetRecord_Output{}
	err = client.Do(ctx, xrpc.Query, "application/json", "com.atproto.repo.getRecord", map[string]any{
		"repo":       repoDID,
		"collection": "place.stream.livestream",
		"rkey":       aturi.RecordKey().String(),
	}, nil, &getOutput)
	if err != nil {
		return fmt.Errorf("could not get record: %w", err)
	}
	log.Debug(ctx, "got record", "record", getOutput)
	swapRecord = getOutput.Cid

	now := time.Now().UTC().Format(util.ISO8601)
	lsvr.LastSeenAt = &now

	inp := comatproto.RepoPutRecord_Input{
		Collection: "place.stream.livestream",
		Record:     &glex.LexiconTypeDecoder{Val: lsvr},
		Rkey:       aturi.RecordKey().String(),
		Repo:       ss.repoDID,
		SwapRecord: swapRecord,
	}
	out := comatproto.RepoPutRecord_Output{}

	err = client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", map[string]any{}, inp, &out)
	if err != nil {
		return fmt.Errorf("could not update livestream record: %w", err)
	}

	log.Debug(ctx, "updated livestream record", "uri", lastLivestream.URI)
	ss.lastLivestreamTime = time.Now()

	return nil
}

func (ss *StreamSession) DeleteStatus(repoDID string) error {
	// need a special extra context because the stream session context is already cancelled
	// No lock needed - this runs during teardown after the background worker has exited
	ctx := log.WithLogValues(context.Background(), "func", "DeleteStatus", "repoDID", repoDID)
	if ss.lastStatusCID == nil {
		log.Debug(ctx, "no status cid to delete")
		return nil
	}
	inp := comatproto.RepoDeleteRecord_Input{
		Collection: "app.bsky.actor.status",
		Rkey:       "self",
		Repo:       repoDID,
	}
	inp.SwapRecord = ss.lastStatusCID
	out := comatproto.RepoDeleteRecord_Output{}

	client, err := ss.GetClientByDID(repoDID, atproto.ScopeBskyActorStatus)
	if err != nil {
		return fmt.Errorf("could not get xrpc client: %w", err)
	}

	err = client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.deleteRecord", map[string]any{}, inp, &out)
	if err != nil {
		return fmt.Errorf("could not delete record: %w", err)
	}

	ss.lastStatusCID = nil
	return nil
}

var originUpdateInterval = time.Second * 30

// UpdateBroadcastOrigin signals the background worker to update origin (non-blocking)
func (ss *StreamSession) UpdateBroadcastOrigin(ctx context.Context) {
	select {
	case ss.originUpdateChan <- struct{}{}:
	default:
		// Channel full, signal already pending
	}
}

// originUpdateLoop runs as a background goroutine for the session lifetime
func (ss *StreamSession) originUpdateLoop(ctx context.Context) error {
	ctx = log.WithLogValues(ctx, "func", "originUpdateLoop")
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ss.originUpdateChan:
			if time.Since(ss.lastOriginTime) < originUpdateInterval {
				log.Debug(ctx, "not updating origin, last origin was less than 30 seconds ago")
				continue
			}
			if err := ss.doUpdateBroadcastOrigin(ctx); err != nil {
				log.Error(ctx, "failed to update broadcast origin", "error", err)
			}
		}
	}
}

// doUpdateBroadcastOrigin performs the actual broadcast origin update work
func (ss *StreamSession) doUpdateBroadcastOrigin(ctx context.Context) error {
	ctx = log.WithLogValues(ctx, "func", "doUpdateBroadcastOrigin")

	broadcaster := fmt.Sprintf("did:web:%s", ss.cli.BroadcasterHost)
	origin := placestream.BroadcastOrigin{
		Streamer:    ss.repoDID,
		Server:      fmt.Sprintf("did:web:%s", ss.cli.ServerHost),
		Broadcaster: &broadcaster,
		UpdatedAt:   time.Now().UTC().Format(util.ISO8601),
	}
	err := ss.replicator.BuildOriginRecord(&origin)
	if err != nil {
		return fmt.Errorf("could not build origin record: %w", err)
	}

	client, err := ss.GetClientByDID(ss.repoDID)
	if err != nil {
		return fmt.Errorf("could not get xrpc client for repoDID: %w", err)
	}

	rkey := fmt.Sprintf("%s::did:web:%s", ss.repoDID, ss.cli.ServerHost)

	var swapRecord *string
	getOutput := comatproto.RepoGetRecord_Output{}
	err = client.Do(ctx, xrpc.Query, "application/json", "com.atproto.repo.getRecord", map[string]any{
		"repo":       ss.repoDID,
		"collection": "place.stream.broadcast.origin",
		"rkey":       rkey,
	}, nil, &getOutput)
	if err != nil {
		xErr, ok := err.(*xrpc.Error)
		if !ok {
			return fmt.Errorf("could not get record: %w", err)
		}
		if xErr.StatusCode != 400 { // yes, they return "400" for "not found"
			return fmt.Errorf("could not get record: %w", err)
		}
		log.Debug(ctx, "record not found, creating", "repoDID", ss.repoDID)
	} else {
		log.Debug(ctx, "got record", "record", getOutput)
		swapRecord = getOutput.Cid
	}

	inp := comatproto.RepoPutRecord_Input{
		Collection: "place.stream.broadcast.origin",
		Record:     &glex.LexiconTypeDecoder{Val: &origin},
		Rkey:       rkey,
		Repo:       ss.repoDID,
		SwapRecord: swapRecord,
	}
	out := comatproto.RepoPutRecord_Output{}

	err = client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", map[string]any{}, inp, &out)
	if err != nil {
		return fmt.Errorf("could not create record: %w", err)
	}

	ss.lastOriginTime = time.Now()
	return nil
}

var viewCountUpdateInterval = time.Second * 30

// UpdateViewCount signals the background worker to update the view count in the server repo (non-blocking)
func (ss *StreamSession) UpdateViewCount(ctx context.Context) {
	select {
	case ss.viewCountUpdateChan <- struct{}{}:
	default:
		// Channel full, signal already pending
	}
}

// viewCountUpdateLoop runs as a background goroutine for the session lifetime
func (ss *StreamSession) viewCountUpdateLoop(ctx context.Context, repoDID string) error {
	ctx = log.WithLogValues(ctx, "func", "viewCountUpdateLoop")
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ss.viewCountUpdateChan:
			if time.Since(ss.lastViewCountTime) < viewCountUpdateInterval {
				log.Debug(ctx, "not updating view count, last update was less than 30 seconds ago")
				continue
			}
			if err := ss.doUpdateViewCount(ctx, repoDID); err != nil {
				log.Error(ctx, "failed to update view count", "error", err)
			}
		}
	}
}

// doUpdateViewCount writes the current view count to the server's atproto repo
func (ss *StreamSession) doUpdateViewCount(ctx context.Context, repoDID string) error {
	ctx = log.WithLogValues(ctx, "func", "doUpdateViewCount")

	count := spmetrics.GetViewCount(repoDID)
	now := time.Now().UTC().Format(util.ISO8601)
	rkey := fmt.Sprintf("%s::%s", repoDID, ss.cli.ServerDID())

	vc := &placestream.LiveViewerCount{
		LexiconTypeID: constants.PLACE_STREAM_LIVE_VIEWERCOUNT,
		Count:         int64(count),
		Server:        ss.cli.ServerDID(),
		Streamer:      repoDID,
		UpdatedAt:     &now,
	}

	err := atproto.CommitServerRepoRecord(ctx, ss.cli, constants.PLACE_STREAM_LIVE_VIEWERCOUNT, rkey, vc)
	if err != nil {
		return fmt.Errorf("could not commit view count record: %w", err)
	}

	log.Debug(ctx, "updated view count", "streamer", repoDID, "count", count, "rkey", rkey)
	ss.lastViewCountTime = time.Now()
	return nil
}

func (ss *StreamSession) Transcode(ctx context.Context, spseg *placestream.Segment, data []byte) error {
	rs, err := renditions.GenerateRenditions(spseg)
	if err != nil {
		return fmt.Errorf("failed to generated renditions: %w", err)
	}

	if ss.lp == nil {
		var err error
		ss.lp, err = livepeer.NewLivepeerSession(ctx, ss.cli, spseg.Creator, ss.cli.LivepeerGatewayURL)
		if err != nil {
			return err
		}

	}
	spmetrics.TranscodeAttemptsTotal.Inc()
	segs, err := ss.lp.PostSegmentToGateway(ctx, data, spseg, rs)
	if err != nil {
		spmetrics.TranscodeErrorsTotal.Inc()
		return err
	}
	if len(rs) != len(segs) {
		spmetrics.TranscodeErrorsTotal.Inc()
		return fmt.Errorf("expected %d renditions, got %d", len(rs), len(segs))
	}
	spmetrics.TranscodeSuccessesTotal.Inc()
	aqt, err := aqtime.FromString(spseg.StartTime)
	if err != nil {
		return err
	}
	for i, seg := range segs {
		ctx := log.WithLogValues(ctx, "rendition", rs[i].Name)
		log.Debug(ctx, "publishing segment", "rendition", rs[i])
		fd, err := ss.cli.SegmentFileCreate(spseg.Creator, aqt, fmt.Sprintf("%s.mp4", rs[i].Name))
		if err != nil {
			return fmt.Errorf("failed to create transcoded segment file: %w", err)
		}
		defer fd.Close()
		_, err = fd.Write(seg)
		if err != nil {
			return fmt.Errorf("failed to write transcoded segment file: %w", err)
		}
		ss.Go(ctx, func() error {
			return ss.AddPlaybackSegment(ctx, spseg, rs[i].Name, &bus.Seg{
				Filepath: fd.Name(),
				Data:     seg,
			})
		})

	}
	return nil
}

func (ss *StreamSession) AddPlaybackSegment(ctx context.Context, spseg *placestream.Segment, rendition string, seg *bus.Seg) error {
	ss.Go(ctx, func() error {
		return ss.AddToWebRTC(ctx, spseg, rendition, seg)
	})
	return nil
}

func (ss *StreamSession) AddToWebRTC(ctx context.Context, spseg *placestream.Segment, rendition string, seg *bus.Seg) error {
	packet, err := media.Packetize(ctx, ss.cli, seg)
	if err != nil {
		return fmt.Errorf("failed to packetize segment: %w", err)
	}
	seg.PacketizedData = packet
	ss.bus.PublishSegment(ctx, spseg.Creator, rendition, seg)
	return nil
}

type XRPCClient interface {
	Do(ctx context.Context, method string, contentType string, path string, queryParams map[string]any, body any, out any) error
}

// GetClientByDID returns an XRPC client acting as the given user. If
// requiredScope values are passed, the user's sessions are scanned for one
// that was granted all of them; statedb.ErrNoSessionWithScope means the user
// declined those permissions everywhere they're logged in.
func (ss *StreamSession) GetClientByDID(did string, requiredScope ...string) (XRPCClient, error) {
	password, ok := ss.cli.DevAccountCreds[did]
	if ok {
		repo, err := ss.mod.GetRepoByHandleOrDID(did)
		if err != nil {
			return nil, fmt.Errorf("could not get repo by did: %w", err)
		}
		if repo == nil {
			return nil, fmt.Errorf("repo not found for did: %s", did)
		}
		anonXRPCC := &xrpc.Client{
			Host:   repo.PDS,
			Client: &aqhttp.Client,
		}
		session, err := comatproto.ServerCreateSession(context.Background(), anonXRPCC, &comatproto.ServerCreateSession_Input{
			Identifier: repo.DID,
			Password:   password,
		})
		if err != nil {
			return nil, fmt.Errorf("could not create session: %w", err)
		}

		log.Warn(context.Background(), "created session for dev account", "did", repo.DID, "handle", repo.Handle, "pds", repo.PDS)

		return &xrpc.Client{
			Host:   repo.PDS,
			Client: &aqhttp.Client,
			Auth: &xrpc.AuthInfo{
				Did:        repo.DID,
				AccessJwt:  session.AccessJwt,
				RefreshJwt: session.RefreshJwt,
				Handle:     repo.Handle,
			},
		}, nil
	}
	var session *oatproxy.OAuthSession
	var err error
	if len(requiredScope) > 0 {
		session, err = ss.statefulDB.GetSessionByDIDWithScope(ss.repoDID, strings.Join(requiredScope, " "))
	} else {
		session, err = ss.statefulDB.GetSessionByDID(ss.repoDID)
	}
	if err != nil {
		return nil, fmt.Errorf("could not get OAuth session for repoDID: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("no session found for repoDID: %s", ss.repoDID)
	}

	session, err = ss.op.RefreshIfNeeded(session)
	if err != nil {
		return nil, fmt.Errorf("could not refresh session for repoDID: %w", err)
	}

	client, err := ss.op.GetXrpcClient(session)
	if err != nil {
		return nil, fmt.Errorf("could not get xrpc client: %w", err)
	}

	return client, nil
}

type runningMultistream struct {
	cancel func()
	key    string
	pushID string
	url    string
}

func sanitizeMultistreamTargetURL(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	u.Path = "/redacted"
	return u.String()
}

// we're making an attempt here not to log (sensitive) stream keys, so we're
// referencing by atproto URI
func (ss *StreamSession) HandleMultistreamTargets(ctx context.Context) error {
	ctx = log.WithLogValues(ctx, "system", "multistreaming")
	isTrue := true
	// {target.Uri}:{rec.Url} -> runningMultistream
	// no concurrency issues, it's only used from this one loop
	running := map[string]*runningMultistream{}
	for {
		targets, err := ss.statefulDB.ListMultistreamTargets(ss.repoDID, 100, 0, &isTrue)
		if err != nil {
			return fmt.Errorf("failed to list multistream targets: %w", err)
		}
		currentRunning := map[string]bool{}
		for _, targetView := range targets {
			rec, ok := targetView.Record.Val.(*placestream.MultistreamTarget)
			if !ok {
				log.Error(ctx, "failed to convert multistream target to streamplace multistream target", "uri", targetView.Uri)
				continue
			}
			uu, err := uuid.NewV7()
			if err != nil {
				return err
			}
			ctx := log.WithLogValues(ctx, "url", sanitizeMultistreamTargetURL(rec.Url), "pushID", uu.String())
			key := fmt.Sprintf("%s:%s", targetView.Uri, rec.Url)
			if running[key] == nil {
				childCtx, childCancel := context.WithCancel(ctx)
				ss.Go(ctx, func() error {
					log.Log(ctx, "starting multistream target", "uri", targetView.Uri)
					err := ss.statefulDB.CreateMultistreamEvent(targetView.Uri, "starting multistream target", "pending")
					if err != nil {
						log.Error(ctx, "failed to create multistream event", "error", err)
					}
					return ss.StartMultistreamTarget(childCtx, &targetView)
				})
				running[key] = &runningMultistream{
					cancel: childCancel,
					key:    key,
					pushID: uu.String(),
					url:    sanitizeMultistreamTargetURL(rec.Url),
				}
			}
			currentRunning[key] = true
		}
		for key := range running {
			if !currentRunning[key] {
				log.Log(ctx, "stopping multistream target", "url", sanitizeMultistreamTargetURL(running[key].url), "pushID", running[key].pushID)
				running[key].cancel()
				delete(running, key)
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second * 5):
			continue
		}
	}
}

func (ss *StreamSession) StartMultistreamTarget(ctx context.Context, targetView *placestream.MultistreamDefs_TargetView) error {
	for {
		// Under --isolated-ingest the crash-prone native egress pipeline runs in a
		// worker subprocess (a gst fault there can't take the node down); otherwise
		// it runs in-process. The on/off + status flow is identical either way.
		var err error
		if ss.cli.IsolatedIngest {
			err = ss.mm.RTMPPushIsolated(ctx, ss.repoDID, "source", targetView)
		} else {
			err = ss.mm.RTMPPush(ctx, ss.repoDID, "source", targetView)
		}
		if err != nil {
			log.Error(ctx, "failed to push to RTMP server", "error", err)
			err := ss.statefulDB.CreateMultistreamEvent(targetView.Uri, err.Error(), "error")
			if err != nil {
				log.Error(ctx, "failed to create multistream event", "error", err)
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second * 5):
			continue
		}
	}
}
