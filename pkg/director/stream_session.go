package director

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/util"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/livepeer"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/renditions"
	"stream.place/streamplace/pkg/replication"
	"stream.place/streamplace/pkg/spmetrics"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/streamplace"
	"stream.place/streamplace/pkg/thumbnail"
)

type StreamSession struct {
	mm             *media.MediaManager
	mod            model.Model
	cli            *config.CLI
	bus            *bus.Bus
	op             *oatproxy.OATProxy
	hls            *media.M3U8
	lp             *livepeer.LivepeerSession
	streamUrls     *livepeer.StreamUrls
	streamUrlsLock sync.Mutex
	aiDataCancel   context.CancelFunc
	repoDID        string
	segmentChan    chan struct{}
	lastStatus     time.Time
	lastStatusCID  *string
	lastStatusLock sync.Mutex
	lastOriginTime time.Time
	lastOriginLock sync.Mutex
	g              *errgroup.Group
	started        chan struct{}
	ctx            context.Context
	packets        []bus.PacketizedSegment
	statefulDB     *statedb.StatefulDB
	replicator     replication.Replicator
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
	defer ss.sendStopRequest()
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
	ss.hls = media.NewM3U8(allRenditions)

	// for _, r := range allRenditions {
	// 	g.Go(func() error {
	// 		for {
	// 			if ctx.Err() != nil {
	// 				return nil
	// 			}
	// 			err := ss.mm.ToHLS(ctx, spseg.Creator, r.Name, ss.hls)
	// 			if ctx.Err() != nil {
	// 				return nil
	// 			}
	// 			log.Warn(ctx, "hls failed, retrying in 5 seconds", "error", err)
	// 			time.Sleep(time.Second * 5)
	// 		}
	// 	})
	// }

	close(ss.started)

	for {
		select {
		case <-ss.segmentChan:
			// reset timer
		case <-ctx.Done():
			return ss.g.Wait()
		// case <-time.After(time.Minute * 1):
		case <-time.After(ss.cli.StreamSessionTimeout):
			log.Log(ctx, "stream session timeout, shutting down", "timeout", ss.cli.StreamSessionTimeout)
			spmetrics.StreamSessions.WithLabelValues(notif.Segment.RepoDID).Dec()
			for _, r := range allRenditions {
				ss.bus.EndSession(ctx, spseg.Creator, r.Name)
			}
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
	notif.Segment.MediaData.Size = len(notif.Data)
	err := ss.mod.CreateSegment(notif.Segment)
	if err != nil {
		return fmt.Errorf("could not add segment to database: %w", err)
	}
	spseg, err := notif.Segment.ToStreamplaceSegment()
	if err != nil {
		return fmt.Errorf("could not convert segment to streamplace segment: %w", err)
	}

	ss.bus.Publish(spseg.Creator, spseg)
	ss.Go(ctx, func() error {
		return ss.AddPlaybackSegment(ctx, spseg, "source", &bus.Seg{
			Filepath: notif.Segment.ID,
			Data:     notif.Data,
		})
	})

	if ss.cli.Thumbnail {
		ss.Go(ctx, func() error {
			return ss.Thumbnail(ctx, spseg.Creator, notif)
		})
	}

	if notif.Local {
		ss.Go(ctx, func() error {
			return ss.UpdateStatus(ctx, spseg.Creator)
		})

		ss.Go(ctx, func() error {
			return ss.UpdateBroadcastOrigin(ctx)
		})
	}

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
			lsv, err := livestreamModel.ToLivestreamView()
			if err != nil {
				return fmt.Errorf("failed to convert livestream to streamplace livestream: %w", err)
			}
			if !shouldNotify(lsv) {
				log.Debug(ctx, "is not set to notify", "repoDID", spseg.Creator)
				return nil
			}
			task := &statedb.NotificationTask{
				Livestream: lsv,
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
			return ss.UpdateStatus(ctx, spseg.Creator)
		})
	} else {
		log.Warn(ctx, "no livestream detected in stream, skipping notification blast", "repoDID", spseg.Creator)
	}

	return nil
}

func shouldNotify(lsv *streamplace.Livestream_LivestreamView) bool {
	lsvr, ok := lsv.Record.Val.(*streamplace.Livestream)
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

func (ss *StreamSession) Thumbnail(ctx context.Context, repoDID string, not *media.NewSegmentNotification) error {
	lock := thumbnail.GetThumbnailLock(not.Segment.RepoDID)
	locked := lock.TryLock()
	if !locked {
		// we're already generating a thumbnail for this user, skip
		return nil
	}
	defer lock.Unlock()
	oldThumb, err := ss.mod.LatestThumbnailForUser(not.Segment.RepoDID)
	if err != nil {
		return err
	}
	if oldThumb != nil && not.Segment.StartTime.Sub(oldThumb.Segment.StartTime) < time.Minute {
		// we have a thumbnail <60sec old, skip generating a new one
		return nil
	}
	r := bytes.NewReader(not.Data)
	aqt := aqtime.FromTime(not.Segment.StartTime)
	fd, err := ss.cli.SegmentFileCreate(not.Segment.RepoDID, aqt, "jpeg")
	if err != nil {
		return err
	}
	defer fd.Close()
	err = media.Thumbnail(ctx, r, fd, "jpeg")
	if err != nil {
		return err
	}
	thumb := &model.Thumbnail{
		Format:    "jpeg",
		SegmentID: not.Segment.ID,
	}
	err = ss.mod.CreateThumbnail(thumb)
	if err != nil {
		return err
	}
	return nil
}

func (ss *StreamSession) UpdateStatus(ctx context.Context, repoDID string) error {
	ctx = log.WithLogValues(ctx, "func", "UpdateStatus")
	ss.lastStatusLock.Lock()
	defer ss.lastStatusLock.Unlock()
	if time.Since(ss.lastStatus) < time.Minute {
		log.Debug(ctx, "not updating status, last status was less than 1 minute ago")
		return nil
	}

	client, err := ss.GetClientByDID(repoDID)
	if err != nil {
		return fmt.Errorf("could not get xrpc client: %w", err)
	}

	ls, err := ss.mod.GetLatestLivestreamForRepo(repoDID)
	if err != nil {
		return fmt.Errorf("could not get latest livestream for repoDID: %w", err)
	}
	lsv, err := ls.ToLivestreamView()
	if err != nil {
		return fmt.Errorf("could not convert livestream to streamplace livestream: %w", err)
	}

	lsvr, ok := lsv.Record.Val.(*streamplace.Livestream)
	if !ok {
		return fmt.Errorf("livestream is not a streamplace livestream")
	}
	thumb := lsvr.Thumb

	repo, err := ss.mod.GetRepoByHandleOrDID(repoDID)
	if err != nil {
		return fmt.Errorf("could not get repo for repoDID: %w", err)
	}

	lsr, ok := lsv.Record.Val.(*streamplace.Livestream)
	if !ok {
		return fmt.Errorf("livestream is not a streamplace livestream")
	}

	canonicalUrl := fmt.Sprintf("https://%s/%s", ss.cli.BroadcasterHost, repo.Handle)

	if lsr.CanonicalUrl != nil {
		canonicalUrl = *lsr.CanonicalUrl
	}

	actorStatusEmbed := bsky.ActorStatus_Embed{
		EmbedExternal: &bsky.EmbedExternal{
			External: &bsky.EmbedExternal_External{
				Title:       lsr.Title,
				Uri:         canonicalUrl,
				Description: fmt.Sprintf("@%s is 🔴LIVE on %s", repo.Handle, ss.cli.BroadcasterHost),
				Thumb:       thumb,
			},
		},
	}

	duration := int64(120)
	status := bsky.ActorStatus{
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
		Record:     &lexutil.LexiconTypeDecoder{Val: &status},
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

func (ss *StreamSession) DeleteStatus(repoDID string) error {
	// need a special extra context because the stream session context is already cancelled
	ctx := log.WithLogValues(context.Background(), "func", "DeleteStatus", "repoDID", repoDID)
	ss.lastStatusLock.Lock()
	defer ss.lastStatusLock.Unlock()
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

	client, err := ss.GetClientByDID(repoDID)
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

func (ss *StreamSession) UpdateBroadcastOrigin(ctx context.Context) error {
	ctx = log.WithLogValues(ctx, "func", "UpdateStatus")
	ss.lastOriginLock.Lock()
	defer ss.lastOriginLock.Unlock()
	if time.Since(ss.lastOriginTime) < originUpdateInterval {
		log.Debug(ctx, "not updating origin, last origin was less than 30 seconds ago")
		return nil
	}
	broadcaster := fmt.Sprintf("did:web:%s", ss.cli.BroadcasterHost)
	origin := streamplace.BroadcastOrigin{
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
		Record:     &lexutil.LexiconTypeDecoder{Val: &origin},
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

func (ss *StreamSession) Transcode(ctx context.Context, spseg *streamplace.Segment, data []byte) error {
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
	var segs [][]byte
	urls, segs, err := ss.lp.PostAISegmentToGateway(ctx, data, spseg, rs)
	if err != nil {
		spmetrics.TranscodeErrorsTotal.Inc()
		log.Error(ctx, "error posting segment to gateway", "error", err)
		return err
	}

	if urls != nil {
		// Store the stream URLs for cleanup
		ss.streamUrlsLock.Lock()
		ss.streamUrls = urls
		ss.streamUrlsLock.Unlock()

		if urls.DataURL != "" {
			log.Log(ctx, "✓ STARTING AI DATA OUTPUT CONSUMER", "data_url", urls.DataURL, "stream_id", urls.StreamID, "stop_url", urls.StopURL)
			// Run AI consumer on a long-lived context so it can retry even if the session context is canceled.
			aiCtx, cancel := context.WithCancel(context.Background())
			aiCtx = log.WithLogValues(aiCtx, "streamer", spseg.Creator)
			ss.streamUrlsLock.Lock()
			// Cancel any previous AI consumer to avoid leaks.
			if ss.aiDataCancel != nil {
				ss.aiDataCancel()
			}
			ss.aiDataCancel = cancel
			ss.streamUrlsLock.Unlock()

			ss.Go(ctx, func() error {
				return ss.ConsumeAIDataOutput(aiCtx, spseg.Creator, urls.DataURL)
			})
		} else {
			log.Debug(ctx, "streamUrls returned but no data_url", "stream_id", urls.StreamID, "stop_url", urls.StopURL)
		}
	} else if ss.cli.LivepeerAIProcessing {
		log.Debug(ctx, "no streamUrls returned from PostAISegmentToGateway")
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

func (ss *StreamSession) AddPlaybackSegment(ctx context.Context, spseg *streamplace.Segment, rendition string, seg *bus.Seg) error {
	ss.Go(ctx, func() error {
		return ss.AddToHLS(ctx, spseg, rendition, seg.Data)
	})
	ss.Go(ctx, func() error {
		return ss.AddToWebRTC(ctx, spseg, rendition, seg)
	})
	return nil
}

func (ss *StreamSession) AddToWebRTC(ctx context.Context, spseg *streamplace.Segment, rendition string, seg *bus.Seg) error {
	packet, err := media.Packetize(ctx, seg)
	if err != nil {
		return fmt.Errorf("failed to packetize segment: %w", err)
	}
	seg.PacketizedData = packet
	ss.bus.PublishSegment(ctx, spseg.Creator, rendition, seg)
	return nil
}

func (ss *StreamSession) ConsumeAIDataOutput(ctx context.Context, repoDID string, dataURL string) error {
	ctx = log.WithLogValues(ctx, "func", "ConsumeAIDataOutput", "dataURL", dataURL)
	log.Log(ctx, "starting AI data output consumer")

	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := ss.consumeAIDataOutputOnce(ctx, repoDID, dataURL)
		if err == nil {
			// Normal end of stream
			return nil
		}

		log.Warn(ctx, "AI data output stream ended; will retry", "error", err, "backoff", backoff)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			// exponential backoff up to 30s
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}

// consumeAIDataOutputOnce connects to the AI data SSE stream and pumps events until EOF/error/context cancel.
func (ss *StreamSession) consumeAIDataOutputOnce(ctx context.Context, repoDID string, dataURL string) error {
	// Gateway returns https:// URLs but may actually serve HTTP on non-443 ports
	// Rewrite https to http for non-standard ports
	if strings.HasPrefix(dataURL, "https://") && strings.Contains(dataURL, ":") {
		parts := strings.SplitN(dataURL[8:], "/", 2)
		if len(parts) >= 1 && strings.Contains(parts[0], ":") {
			port := strings.Split(parts[0], ":")[1]
			if port != "443" {
				dataURL = strings.Replace(dataURL, "https://", "http://", 1)
				log.Log(ctx, "rewrote https to http for non-443 port", "dataURL", dataURL)
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", dataURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := aqhttp.DoTrusted(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to connect to data URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("data URL returned non-OK status: %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	// AI SSE payloads (especially transcripts) can exceed bufio.Scanner's default 64K token limit,
	// which would cause the scanner to stop and the consumer to silently end.
	// Allow larger events.
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var eventData strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// SSE format: "data: {json}" or empty line to signal end of event
		if strings.HasPrefix(line, "data: ") {
			eventData.WriteString(strings.TrimPrefix(line, "data: "))
		} else if line == "" && eventData.Len() > 0 {
			// End of event, publish to bus
			dataStr := eventData.String()
			// Avoid logging huge transcript payloads.
			log.Debug(ctx, "received ai data event", "length", len(dataStr))

			// Try to parse as JSON and publish it
			var msg map[string]any
			if err := json.Unmarshal([]byte(dataStr), &msg); err == nil {
				// Add a type identifier so the frontend can recognize it
				msg["$type"] = "place.stream.ai#dataOutput"
				ss.bus.Publish(repoDID, msg)
			} else {
				// If not JSON, publish as a plain text message
				ss.bus.Publish(repoDID, map[string]any{
					"$type": "place.stream.ai#dataOutput",
					"text":  dataStr,
				})
			}

			eventData.Reset()
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("error reading data stream: %w", err)
	}

	log.Log(ctx, "AI data output consumer finished")
	return nil
}

func (ss *StreamSession) AddToHLS(ctx context.Context, spseg *streamplace.Segment, rendition string, data []byte) error {
	buf := bytes.Buffer{}
	dur, err := media.MP4ToMPEGTS(ctx, bytes.NewReader(data), &buf)
	if err != nil {
		return fmt.Errorf("failed to convert MP4 to MPEG-TS: %w", err)
	}
	// newSeg := &streamplace.Segment{
	// 	LexiconTypeID: "place.stream.segment",
	// 	Id:            spseg.Id,
	// 	Creator:       spseg.Creator,
	// 	StartTime:     spseg.StartTime,
	// 	Duration:      &dur,
	// 	Audio:         spseg.Audio,
	// 	Video:         spseg.Video,
	// 	SigningKey:    spseg.SigningKey,
	// }
	aqt, err := aqtime.FromString(spseg.StartTime)
	if err != nil {
		return fmt.Errorf("failed to parse segment start time: %w", err)
	}
	log.Debug(ctx, "transmuxed to mpegts, adding to hls", "rendition", rendition, "size", buf.Len())
	rend, err := ss.hls.GetRendition(rendition)
	if err != nil {
		return fmt.Errorf("failed to get rendition: %w", err)
	}
	if err := rend.NewSegment(&media.Segment{
		Buf:      &buf,
		Duration: time.Duration(dur),
		Time:     aqt.Time(),
	}); err != nil {
		return fmt.Errorf("failed to create new segment: %w", err)
	}

	return nil
}

type XRPCClient interface {
	Do(ctx context.Context, method string, contentType string, path string, queryParams map[string]any, body any, out any) error
}

func (ss *StreamSession) GetClientByDID(did string) (XRPCClient, error) {
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
	session, err := ss.statefulDB.GetSessionByDID(ss.repoDID)
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

func (ss *StreamSession) sendStopRequest() {
	ss.streamUrlsLock.Lock()
	urls := ss.streamUrls
	// cancel AI data consumer if running
	if ss.aiDataCancel != nil {
		ss.aiDataCancel()
		ss.aiDataCancel = nil
	}
	ss.streamUrlsLock.Unlock()

	if urls == nil {
		return
	}

	stopURL := urls.StopURL
	if stopURL == "" && urls.StreamID != "" && ss.cli.LivepeerGatewayURL != "" {
		gateway := strings.TrimSuffix(ss.cli.LivepeerGatewayURL, "/")
		stopURL = fmt.Sprintf("%s/process/stream/%s/stop", gateway, urls.StreamID)
	}
	if stopURL == "" {
		return
	}

	ctx := context.Background()
	ctx = log.WithLogValues(ctx, "func", "sendStopRequest", "streamer", ss.repoDID)
	log.Log(ctx, "sending POST stop request to gateway", "stop_url", stopURL)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", stopURL, nil)
	if err != nil {
		log.Error(ctx, "failed to create stop request", "error", err, "stop_url", stopURL)
		return
	}

	resp, err := aqhttp.DoTrusted(ctx, req)
	if err != nil {
		log.Error(ctx, "failed to send POST stop request", "error", err, "stop_url", stopURL)
		return
	}
	defer resp.Body.Close()

	log.Log(ctx, "successfully sent POST stop request to gateway", "stop_url", stopURL, "status", resp.StatusCode)
}
