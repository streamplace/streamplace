package director

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/livepeer"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/media/segchanman"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/renditions"
	"stream.place/streamplace/pkg/streamplace"
	"stream.place/streamplace/pkg/thumbnail"
)

// director is responsible for managing the lifecycle of a stream, making business
// logic decisions about when to do things like
// - size of the in-memory segment cache
// - transcoding
// - thumbnail generation

type Director struct {
	mm           *media.MediaManager
	mod          model.Model
	cli          *config.CLI
	bus          *bus.Bus
	lpSessions   map[string]*livepeer.LivepeerSession
	lpSessionsMu sync.Mutex
}

func NewDirector(mm *media.MediaManager, mod model.Model, cli *config.CLI, bus *bus.Bus) *Director {
	return &Director{
		mm:           mm,
		mod:          mod,
		cli:          cli,
		bus:          bus,
		lpSessions:   make(map[string]*livepeer.LivepeerSession),
		lpSessionsMu: sync.Mutex{},
	}
}

func (d *Director) Start(ctx context.Context) error {
	newSeg := d.mm.NewSegment()
	for {
		select {
		case <-ctx.Done():
			return nil
		case not := <-newSeg:
			err := d.mod.CreateSegment(not.Segment)
			if err != nil {
				log.Error(ctx, "could not add segment to database", "error", err)
			}
			spseg, err := not.Segment.ToStreamplaceSegment()
			if err != nil {
				log.Error(ctx, "could not convert segment to streamplace segment", "error", err)
				continue
			}
			d.bus.Publish(spseg.Creator, spseg)

			go func() {
				err := d.Thumbnail(ctx, spseg.Creator, not)
				if err != nil {
					log.Error(ctx, "could not create thumbnail", "error", err)
				}
			}()

			go func() {

				err := d.Transcode(ctx, spseg, not.Data)
				if err != nil {
					log.Error(ctx, "could not transcode", "error", err)
				}
			}()

		}
	}
}

func (d *Director) Thumbnail(ctx context.Context, repoDID string, not *media.NewSegmentNotification) error {
	lock := thumbnail.GetThumbnailLock(not.Segment.RepoDID)
	locked := lock.TryLock()
	if !locked {
		// we're already generating a thumbnail for this user, skip
		return nil
	}
	defer lock.Unlock()
	oldThumb, err := d.mod.LatestThumbnailForUser(not.Segment.RepoDID)
	if err != nil {
		return err
	}
	if oldThumb != nil && not.Segment.StartTime.Sub(oldThumb.Segment.StartTime) < time.Minute {
		// we have a thumbnail <60sec old, skip generating a new one
		return nil
	}
	r := bytes.NewReader(not.Data)
	aqt := aqtime.FromTime(not.Segment.StartTime)
	fd, err := d.cli.SegmentFileCreate(not.Segment.RepoDID, aqt, "png")
	if err != nil {
		return err
	}
	defer fd.Close()
	err = d.mm.Thumbnail(ctx, r, fd)
	if err != nil {
		return err
	}
	thumb := &model.Thumbnail{
		Format:    "png",
		SegmentID: not.Segment.ID,
	}
	err = d.mod.CreateThumbnail(thumb)
	if err != nil {
		return err
	}
	return nil
}

func (d *Director) getLPSession(ctx context.Context, spseg *streamplace.Segment) (*livepeer.LivepeerSession, error) {
	d.lpSessionsMu.Lock()
	defer d.lpSessionsMu.Unlock()
	var err error
	ls, ok := d.lpSessions[spseg.Creator]
	if !ok {
		ls, err = livepeer.NewLivepeerSession(ctx, spseg.Creator)
		if err != nil {
			return nil, err
		}
		d.lpSessions[spseg.Creator] = ls
	}
	return ls, nil
}

func (d *Director) Transcode(ctx context.Context, spseg *streamplace.Segment, data []byte) error {
	ls, err := d.getLPSession(ctx, spseg)
	if err != nil {
		return err
	}
	rs, err := renditions.GenerateRenditions(spseg)
	segs, err := ls.PostSegmentToGateway(ctx, data)
	if err != nil {
		return err
	}
	if len(rs) != len(segs) {
		return fmt.Errorf("expected %d renditions, got %d", len(rs), len(segs))
	}
	aqt, err := aqtime.FromString(spseg.StartTime)
	if err != nil {
		return err
	}
	for i, seg := range segs {
		log.Log(ctx, "transcoding segment", "rendition", rs[i])
		fd, err := d.cli.SegmentFileCreate(spseg.Creator, aqt, fmt.Sprintf("%s.mp4", rs[i].Name))
		if err != nil {
			return err
		}
		defer fd.Close()
		fd.Write(seg)
		go d.mm.PublishSegment(ctx, spseg.Creator, rs[i].Name, &segchanman.Seg{
			Filepath: fd.Name(),
			Data:     seg,
		})
	}
	return nil
}
