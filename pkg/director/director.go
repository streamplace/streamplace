package director

import (
	"context"
	"sync"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/indexdb"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/replication"
	"stream.place/streamplace/pkg/statedb"
)

// director is responsible for managing the lifecycle of a stream, making business
// logic decisions about when to do things like
// - size of the in-memory segment cache
// - transcoding
// - thumbnail generation

// directorStore is the subset of the index database that pkg/director
// reads, declared consumer-side so director depends on five methods
// instead of the full indexdb.Model. Repo is indexer state; the rest are
// placestream view types.
type directorStore interface {
	GetRepoByHandleOrDID(arg string) (*indexdb.Repo, error)
	GetLatestLivestreamForRepo(repoDID string) (*placestream.Livestream_LivestreamView, error)
	GetChatProfile(ctx context.Context, repoDID string) (*placestream.ChatProfile, error)
	GetServerSettings(ctx context.Context, server, repoDID string) (*placestream.ServerSettings, error)
	HasBetaInvite(ctx context.Context, fromRepoDID, subjectDID, feature string) (bool, error)
}

type Director struct {
	mm               *media.MediaManager
	mod              directorStore
	cli              *config.CLI
	bus              *bus.Bus
	streamSessions   map[string]*StreamSession
	streamSessionsMu sync.Mutex
	op               *oatproxy.OATProxy
	statefulDB       *statedb.StatefulDB
	replicator       replication.Replicator
	localDB          localdb.LocalDB
	atsync           *atproto.ATProtoSynchronizer
}

// Params carries the dependencies NewDirector needs, so adding a
// dependency is a one-line change here instead of a signature change at
// every call site.
type Params struct {
	MediaManager *media.MediaManager
	Store        directorStore
	CLI          *config.CLI
	Bus          *bus.Bus
	OATProxy     *oatproxy.OATProxy
	StatefulDB   *statedb.StatefulDB
	Replicator   replication.Replicator
	LocalDB      localdb.LocalDB
	ATSync       *atproto.ATProtoSynchronizer
}

func NewDirector(p Params) *Director {
	return &Director{
		mm:               p.MediaManager,
		mod:              p.Store,
		cli:              p.CLI,
		bus:              p.Bus,
		streamSessions:   make(map[string]*StreamSession),
		streamSessionsMu: sync.Mutex{},
		op:               p.OATProxy,
		statefulDB:       p.StatefulDB,
		replicator:       p.Replicator,
		localDB:          p.LocalDB,
		atsync:           p.ATSync,
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
					lp:          nil,
					repoDID:     not.Segment.RepoDID,
					mm:          d.mm,
					mod:         d.mod,
					cli:         d.cli,
					bus:         d.bus,
					segmentChan: make(chan struct{}),
					op:          d.op,
					packets:     make([]bus.PacketizedSegment, 0),
					started:     make(chan struct{}),
					statefulDB:  d.statefulDB,
					replicator:  d.replicator,
					// Initialize notification channels (buffered size 1 for coalescing)
					statusUpdateChan:     make(chan struct{}, 1),
					originUpdateChan:     make(chan struct{}, 1),
					livestreamUpdateChan: make(chan struct{}, 1),
					viewCountUpdateChan:  make(chan struct{}, 1),
					localDB:              d.localDB,
					atsync:               d.atsync,
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
