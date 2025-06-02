package director

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/livepeer"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/media/segchanman"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/renditions"
	"stream.place/streamplace/pkg/spmetrics"
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
	repoDID        string
	segmentChan    chan struct{}
	lastStatus     time.Time
	lastStatusLock sync.Mutex
}

func (ss *StreamSession) Start(ctx context.Context, not *media.NewSegmentNotification) error {

	sid := livepeer.RandomTrailer(8)
	ctx = log.WithLogValues(ctx, "sid", sid)
	ctx, cancel := context.WithCancel(ctx)
	log.Log(ctx, "starting stream session")
	defer cancel()
	spseg, err := not.Segment.ToStreamplaceSegment()
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
	byteLen := len(not.Data)
	bitrate := int(float64(byteLen) / dur.Seconds() * 8)
	sourceRendition := renditions.Rendition{
		Name:    "source",
		Bitrate: bitrate,
		Width:   spseg.Video[0].Width,
		Height:  spseg.Video[0].Height,
	}
	allRenditions = append([]renditions.Rendition{sourceRendition}, allRenditions...)
	ss.hls = media.NewM3U8(allRenditions)

	g, ctx := errgroup.WithContext(ctx)

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

	for {
		select {
		case <-ss.segmentChan:
			// reset timer
		case <-ctx.Done():
			return g.Wait()
		// case <-time.After(time.Minute * 1):
		case <-time.After(time.Second * 60):
			log.Log(ctx, "no new segments for 1 minute, shutting down")
			cancel()
		}
	}
}

func (ss *StreamSession) NewSegment(ctx context.Context, not *media.NewSegmentNotification) error {
	if ctx.Err() != nil {
		return nil
	}
	ss.segmentChan <- struct{}{}
	aqt := aqtime.FromTime(not.Segment.StartTime)
	ctx = log.WithLogValues(ctx, "segID", not.Segment.ID, "repoDID", not.Segment.RepoDID, "timestamp", aqt.FileSafeString())
	err := ss.mod.CreateSegment(not.Segment)
	if err != nil {
		return fmt.Errorf("could not add segment to database: %w", err)
	}
	spseg, err := not.Segment.ToStreamplaceSegment()
	if err != nil {
		return fmt.Errorf("could not convert segment to streamplace segment: %w", err)
	}

	ss.bus.Publish(spseg.Creator, spseg)
	go ss.TryAddToHLS(ctx, spseg, "source", not.Data)

	if ss.cli.Thumbnail {
		go func() {
			err := ss.Thumbnail(ctx, spseg.Creator, not)
			if err != nil {
				log.Error(ctx, "could not create thumbnail", "error", err)
			}
		}()
	}

	go func() {
		err := ss.UpdateStatus(ctx, spseg.Creator)
		if err != nil {
			log.Error(ctx, "could not update status", "error", err)
		}
	}()

	if ss.cli.LivepeerGatewayURL != "" {
		go func() {
			start := time.Now()
			err := ss.Transcode(ctx, spseg, not.Data)
			took := time.Since(start)
			if err != nil {
				log.Error(ctx, "could not transcode", "error", err, "took", took)
			} else {
				log.Log(ctx, "transcoded segment", "took", took)
			}
			spmetrics.QueuedTranscodeDuration.WithLabelValues(spseg.Creator).Set(float64(time.Since(start).Milliseconds()))
		}()
	}

	return nil
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

func getThumbnailCID(pv *bsky.FeedDefs_PostView) (*util.LexBlob, error) {
	if pv == nil {
		return nil, fmt.Errorf("post view is nil")
	}
	rec, ok := pv.Record.Val.(*bsky.FeedPost)
	if !ok {
		return nil, fmt.Errorf("post view record is not a feed post")
	}
	if rec.Embed == nil {
		return nil, fmt.Errorf("post view embed is nil")
	}
	if rec.Embed.EmbedExternal == nil {
		return nil, fmt.Errorf("post view embed external view is nil")
	}
	if rec.Embed.EmbedExternal.External == nil {
		return nil, fmt.Errorf("post view embed external is nil")
	}
	if rec.Embed.EmbedExternal.External.Thumb == nil {
		return nil, fmt.Errorf("post view embed external thumb is nil")
	}
	return rec.Embed.EmbedExternal.External.Thumb, nil
}

func (ss *StreamSession) UpdateStatus(ctx context.Context, repoDID string) error {
	ctx = log.WithLogValues(ctx, "func", "UpdateStatus")
	ss.lastStatusLock.Lock()
	defer ss.lastStatusLock.Unlock()
	if time.Since(ss.lastStatus) < time.Minute {
		log.Debug(ctx, "not updating status, last status was less than 1 minute ago")
		return nil
	}

	session, err := ss.mod.GetSessionByDID(repoDID)
	if err != nil {
		return fmt.Errorf("could not get session for repoDID: %w", err)
	}
	if session == nil {
		return fmt.Errorf("no session found for repoDID: %s", repoDID)
	}

	session, err = ss.op.RefreshIfNeeded(session)
	if err != nil {
		return fmt.Errorf("could not refresh session for repoDID: %w", err)
	}

	ls, err := ss.mod.GetLatestLivestreamForRepo(repoDID)
	if err != nil {
		return fmt.Errorf("could not get latest livestream for repoDID: %w", err)
	}
	lsv, err := ls.ToLivestreamView()
	if err != nil {
		return fmt.Errorf("could not convert livestream to streamplace livestream: %w", err)
	}

	post, err := ss.mod.GetFeedPost(ls.PostCID)
	if err != nil {
		return fmt.Errorf("could not get feed post: %w", err)
	}
	if post == nil {
		return fmt.Errorf("feed post not found for livestream: %w", err)
	}
	postView, err := post.ToBskyPostView()
	if err != nil {
		return fmt.Errorf("could not convert feed post to bsky post view: %w", err)
	}
	thumb, err := getThumbnailCID(postView)
	if err != nil {
		return fmt.Errorf("could not get thumbnail cid: %w", err)
	}

	repo, err := ss.mod.GetRepoByHandleOrDID(repoDID)
	if err != nil {
		return fmt.Errorf("could not get repo for repoDID: %w", err)
	}

	lsr, ok := lsv.Record.Val.(*streamplace.Livestream)
	if !ok {
		return fmt.Errorf("livestream is not a streamplace livestream")
	}

	actorStatusEmbed := bsky.ActorStatus_Embed{
		EmbedExternal: &bsky.EmbedExternal{
			External: &bsky.EmbedExternal_External{
				Title:       lsr.Title,
				Uri:         fmt.Sprintf("https://%s/%s", ss.cli.PublicHost, repo.Handle),
				Description: fmt.Sprintf("@%s is 🔴LIVE on %s", repo.Handle, ss.cli.PublicHost),
				Thumb:       thumb,
			},
		},
	}

	duration := int64(2)
	status := bsky.ActorStatus{
		Status:          "live",
		DurationMinutes: &duration,
		Embed:           &actorStatusEmbed,
		CreatedAt:       time.Now().Format(time.RFC3339),
	}

	client, err := ss.op.GetXrpcClient(session)
	if err != nil {
		return fmt.Errorf("could not get xrpc client: %w", err)
	}

	var swapRecord *string
	getOutput := atproto.RepoGetRecord_Output{}
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

	inp := atproto.RepoPutRecord_Input{
		Collection: "app.bsky.actor.status",
		Record:     &util.LexiconTypeDecoder{Val: &status},
		Rkey:       "self",
		Repo:       repoDID,
		SwapRecord: swapRecord,
	}
	out := atproto.RepoPutRecord_Output{}

	err = client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", map[string]any{}, inp, &out)
	if err != nil {
		return fmt.Errorf("could not create record: %w", err)
	}
	log.Debug(ctx, "created status record", "out", out)

	ss.lastStatus = time.Now()

	return nil
}

func (ss *StreamSession) Transcode(ctx context.Context, spseg *streamplace.Segment, data []byte) error {
	rs, err := renditions.GenerateRenditions(spseg)
	if err != nil {
		return fmt.Errorf("failed to generated renditions: %w", err)
	}

	if ss.lp == nil {
		var err error
		ss.lp, err = livepeer.NewLivepeerSession(ctx, spseg.Creator, ss.cli.LivepeerGatewayURL)
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
		go ss.TryAddToHLS(ctx, spseg, rs[i].Name, seg)
		go ss.mm.PublishSegment(ctx, spseg.Creator, rs[i].Name, &segchanman.Seg{
			Filepath: fd.Name(),
			Data:     seg,
		})
	}
	return nil
}

func (ss *StreamSession) TryAddToHLS(ctx context.Context, spseg *streamplace.Segment, rendition string, data []byte) {
	ctx = log.WithLogValues(ctx, "rendition", rendition)
	err := ss.AddToHLS(ctx, spseg, rendition, data)
	if err != nil {
		log.Error(ctx, "could not add to hls", "error", err)
	}
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
	if err := ss.hls.GetRendition(rendition).NewSegment(&media.Segment{
		Buf:      &buf,
		Duration: time.Duration(dur),
		Time:     aqt.Time(),
	}); err != nil {
		return fmt.Errorf("failed to create new segment: %w", err)
	}

	return nil
}
