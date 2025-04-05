package director

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/model"
)

// director is responsible for managing the lifecycle of a stream, making business
// logic decisions about when to do things like
// - size of the in-memory segment cache
// - transcoding
// - thumbnail generation

type Director struct {
	mm               *media.MediaManager
	mod              model.Model
	cli              *config.CLI
	bus              *bus.Bus
	streamSessions   map[string]*StreamSession
	streamSessionsMu sync.Mutex
}

func NewDirector(mm *media.MediaManager, mod model.Model, cli *config.CLI, bus *bus.Bus) *Director {
	return &Director{
		mm:               mm,
		mod:              mod,
		cli:              cli,
		bus:              bus,
		streamSessions:   make(map[string]*StreamSession),
		streamSessionsMu: sync.Mutex{},
	}
}

func (d *Director) Start(ctx context.Context) error {
	newSeg := d.mm.NewSegment()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)
	for {
		select {
		case <-ctx.Done():
			cancel()
			return g.Wait()
		case not := <-newSeg:
			d.streamSessionsMu.Lock()
			ss, ok := d.streamSessions[not.Segment.RepoDID]
			if !ok {
				ss = &StreamSession{
					hls:         nil,
					lp:          nil,
					repoDID:     not.Segment.RepoDID,
					mm:          d.mm,
					mod:         d.mod,
					cli:         d.cli,
					bus:         d.bus,
					segmentChan: make(chan struct{}),
				}
				d.streamSessions[not.Segment.RepoDID] = ss
				g.Go(func() error {
					err := ss.Start(ctx, not)
					if err != nil {
						log.Error(ctx, "could not start stream session", "error", err)
					}
					d.streamSessionsMu.Lock()
					delete(d.streamSessions, not.Segment.RepoDID)
					d.streamSessionsMu.Unlock()
					return nil
				})
			}
			d.streamSessionsMu.Unlock()
			err := ss.NewSegment(ctx, not)
			if err != nil {
				log.Error(ctx, "could not add segment to stream session", "error", err)
			}
		}
	}
}

func (d *Director) GetM3U8(ctx context.Context, repoDID string) (*media.M3U8, error) {
	d.streamSessionsMu.Lock()
	defer d.streamSessionsMu.Unlock()
	ss, ok := d.streamSessions[repoDID]
	if !ok {
		return nil, fmt.Errorf("stream session not found")
	}
	return ss.hls, nil
}
