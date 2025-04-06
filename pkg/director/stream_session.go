package director

import (
	"bytes"
	"context"
	"fmt"
	"time"

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
	mm          *media.MediaManager
	mod         model.Model
	cli         *config.CLI
	bus         *bus.Bus
	hls         *media.M3U8
	lp          *livepeer.LivepeerSession
	repoDID     string
	segmentChan chan struct{}
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

	for _, r := range allRenditions {
		g.Go(func() error {
			for {
				if ctx.Err() != nil {
					return nil
				}
				err := ss.mm.ToHLS(ctx, spseg.Creator, r.Name, ss.hls)
				if ctx.Err() != nil {
					return nil
				}
				log.Warn(ctx, "hls failed, retrying in 5 seconds", "error", err)
				time.Sleep(time.Second * 5)
			}
		})
	}

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

	go func() {
		err := ss.Thumbnail(ctx, spseg.Creator, not)
		if err != nil {
			log.Error(ctx, "could not create thumbnail", "error", err)
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
	fd, err := ss.cli.SegmentFileCreate(not.Segment.RepoDID, aqt, "png")
	if err != nil {
		return err
	}
	defer fd.Close()
	err = ss.mm.Thumbnail(ctx, r, fd)
	if err != nil {
		return err
	}
	thumb := &model.Thumbnail{
		Format:    "png",
		SegmentID: not.Segment.ID,
	}
	err = ss.mod.CreateThumbnail(thumb)
	if err != nil {
		return err
	}
	return nil
}

func (ss *StreamSession) Transcode(ctx context.Context, spseg *streamplace.Segment, data []byte) error {
	rs, err := renditions.GenerateRenditions(spseg)
	if ss.lp == nil {
		var err error
		ss.lp, err = livepeer.NewLivepeerSession(ctx, spseg.Creator, ss.cli.LivepeerGatewayURL)
		if err != nil {
			return err
		}

	}
	spmetrics.TranscodeAttemptsTotal.Inc()
	segs, err := ss.lp.PostSegmentToGateway(ctx, data, spseg)
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
			return err
		}
		defer fd.Close()
		fd.Write(seg)
		// go ss.TryAddToHLS(ctx, spseg, rs[i].Name, seg)
		go ss.mm.PublishSegment(ctx, spseg.Creator, rs[i].Name, &segchanman.Seg{
			Filepath: fd.Name(),
			Data:     seg,
		})
	}
	return nil
}

// func (ss *StreamSession) TryAddToHLS(ctx context.Context, spseg *streamplace.Segment, rendition string, data []byte) {
// 	ctx = log.WithLogValues(ctx, "rendition", rendition)
// 	err := ss.AddToHLS(ctx, spseg, rendition, data)
// 	if err != nil {
// 		log.Error(ctx, "could not add to hls", "error", err)
// 	}
// }

// func (ss *StreamSession) AddToHLS(ctx context.Context, spseg *streamplace.Segment, rendition string, data []byte) error {
// 	buf := bytes.Buffer{}
// 	dur, err := media.MP4ToMPEGTS(ctx, bytes.NewReader(data), &buf)
// 	if err != nil {
// 		return err
// 	}
// 	newSeg := &streamplace.Segment{
// 		LexiconTypeID: "place.stream.segment",
// 		Id:            spseg.Id,
// 		Creator:       spseg.Creator,
// 		StartTime:     spseg.StartTime,
// 		Duration:      &dur,
// 		Audio:         spseg.Audio,
// 		Video:         spseg.Video,
// 		SigningKey:    spseg.SigningKey,
// 	}
// 	log.Debug(ctx, "transmuxed to mpegts, adding to hls", "rendition", rendition, "size", buf.Len())
// 	ss.hls.NewSegment(newSeg, rendition, buf.Bytes())
// 	return nil
// }
