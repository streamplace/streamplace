package websocketrep

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"sync"
	"time"

	official "moq.dev/moq"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/moq"
)

// moqBroadcast owns our validated inventory, not a peer's raw origin routes.
// The second producer advertises only locally ingested streams to push peers.
type moqBroadcast struct {
	mu              sync.Mutex
	streamer        string
	public          *official.BroadcastProducer
	local           *official.BroadcastProducer
	tracks          map[string]*moqPublishedTrack
	lastLocal       time.Time
	lastPublic      time.Time
	archiveRoute    *official.OriginDynamic
	archiveEligible bool
	catalog         moq.Catalog
}

type moqPublishedTrack struct {
	archive     *archive
	public      *official.TrackProducer
	local       *official.TrackProducer
	publicFetch *official.TrackDynamic
	localFetch  *official.TrackDynamic
	latest      *uint64
	groups      moqExpiringGroups
}

func (r *WebsocketReplicator) moqBroadcast(streamer string) (*moqBroadcast, error) {
	r.moqMu.Lock()
	defer r.moqMu.Unlock()
	if b := r.moqBroadcasts[streamer]; b != nil {
		return b, nil
	}
	pub, err := r.moqOrigin.CreateBroadcast(streamer)
	if err != nil {
		return nil, err
	}
	local, err := r.moqLocalOrigin.CreateBroadcast(streamer)
	if err != nil {
		_ = pub.Close()
		return nil, err
	}
	b := &moqBroadcast{streamer: streamer, public: pub, local: local, tracks: map[string]*moqPublishedTrack{}}
	r.moqBroadcasts[streamer] = b
	return b, nil
}

func (r *WebsocketReplicator) archivePath(streamer, track string) string {
	// Hash the namespace, not user supplied paths (including DID URL paths).
	return r.cli.DataFilePath([]string{"moq", fmt.Sprintf("%x", sha256.Sum256([]byte(streamer))), track})
}

func (r *WebsocketReplicator) publishedTrack(b *moqBroadcast, streamer, name string) (*moqPublishedTrack, error) {
	if t := b.tracks[name]; t != nil {
		return t, nil
	}
	if name != moq.SourceTrack {
		if _, ok := moq.ParseRenditionTrack(name); !ok {
			return nil, fmt.Errorf("invalid moq track %q", name)
		}
	}
	a, err := newArchive(r.archivePath(streamer, name))
	if err != nil {
		return nil, err
	}
	age := uint64(2_000_000)
	priority := uint8(150)
	if name == moq.SourceTrack {
		priority = 200
	}
	info := &official.TrackInfo{Priority: priority, MaxAgeUs: &age}
	public, err := b.public.PublishTrack(name, info)
	if err != nil {
		return nil, err
	}
	local, err := b.local.PublishTrack(name, info)
	if err != nil {
		_ = public.Abort(500)
		return nil, err
	}
	publicFetch, err := public.Dynamic()
	if err != nil {
		_ = public.Abort(500)
		_ = local.Abort(500)
		return nil, err
	}
	localFetch, err := local.Dynamic()
	if err != nil {
		publicFetch.Cancel()
		_ = public.Abort(500)
		_ = local.Abort(500)
		return nil, err
	}
	t := &moqPublishedTrack{archive: a, public: public, local: local, publicFetch: publicFetch, localFetch: localFetch}
	if seq, ok := a.latestSequence(); ok {
		t.latest = &seq
	}
	b.tracks[name] = t
	if err := r.persistMoqInventory(b); err != nil {
		delete(b.tracks, name)
		publicFetch.Cancel()
		localFetch.Cancel()
		_ = public.Abort(500)
		_ = local.Abort(500)
		return nil, err
	}
	return t, nil
}

// PublishMoqSource is the lossless media hook. The source includes required
// audio and signed control metadata, so it remains intact on the source track.
func (r *WebsocketReplicator) PublishMoqSource(ctx context.Context, streamer string, payload []byte, meta *media.SegmentMetadata, local bool) error {
	if r.cli.DisableSyndication {
		return nil
	}
	if meta == nil || !meta.Published || meta.Creator != streamer {
		return fmt.Errorf("MoQ source requires verified public stream metadata")
	}
	expiry, err := moqArchiveExpiry(meta, time.Now(), r.cli.SegmentArchiveRetention)
	if err != nil {
		return err
	}
	return r.publishMoqSegment(ctx, streamer, moq.SourceTrack, payload, uint64(meta.StartTime.Time().UnixMicro()), local, expiry)
}

// PublishMoqRenditions sends signed tracks independently, never an aggregate
// rendition subscription. MUXL's parser preserves every signed byte.
func (r *WebsocketReplicator) PublishMoqRenditions(ctx context.Context, streamer string, payload []byte, timestampUs uint64, local bool) error {
	if r.cli.DisableSyndication {
		return nil
	}
	pieces, err := moq.SplitRenditions(ctx, payload)
	if err != nil {
		return err
	}
	if len(pieces) == 0 {
		return fmt.Errorf("rendition addendum has no MUXL tracks")
	}
	for _, piece := range pieces {
		if err := r.publishMoqSegment(ctx, streamer, moq.RenditionTrack(piece.ID), piece.Payload, timestampUs, local, time.Time{}); err != nil {
			return err
		}
	}
	return nil
}

// publishMoqSegment durably commits a validated group before exposing it live.
// Source expiry comes from the already-validated metadata; renditions inherit it.
func (r *WebsocketReplicator) publishMoqSegment(ctx context.Context, streamer, name string, payload []byte, timestampUs uint64, local bool, expiresAt time.Time) error {
	if r.cli.DisableSyndication {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !moq.IsMuxl(payload) {
		return fmt.Errorf("moq publication is not MUXL")
	}
	b, err := r.moqBroadcast(streamer)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	t, err := r.publishedTrack(b, streamer, name)
	if err != nil {
		return err
	}
	var seq uint64
	if name == moq.SourceTrack {
		seq, err = t.archive.assignWithExpiry(timestampUs, payload, expiresAt)
	} else {
		source, sourceErr := r.publishedTrack(b, streamer, moq.SourceTrack)
		if sourceErr != nil {
			return sourceErr
		}
		var found bool
		seq, found = source.archive.sequenceAt(timestampUs)
		if !found {
			return fmt.Errorf("rendition has no archived source at timestamp %d", timestampUs)
		}
		sourceGroup, ok, sourceErr := source.archive.get(seq)
		if sourceErr != nil {
			return sourceErr
		}
		if !ok {
			return fmt.Errorf("rendition source group %d expired", seq)
		}
		err = t.archive.putWithExpiry(seq, timestampUs, payload, sourceGroup.expiresAt)
	}
	if err != nil {
		return err
	}
	b.archiveEligible = true
	if t.latest != nil && seq <= *t.latest {
		return nil
	}
	// Reuse the archive's timestamp when a replicated/deduplicated payload
	// was assigned earlier; FETCH and live must describe the same bytes.
	stored, ok, err := t.archive.get(seq)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("just-persisted moq group %d is missing", seq)
	}
	for _, producer := range [2]*official.TrackProducer{t.public, t.local} {
		if err := writeMoqGroup(producer, stored, &t.groups); err != nil {
			return err
		}
	}
	t.latest = &seq
	if name == moq.SourceTrack {
		b.catalog.SourceSequence = &seq
	} else {
		id, _ := moq.ParseRenditionTrack(name)
		found := false
		for _, item := range b.catalog.Renditions {
			if item.ID == id {
				found = true
				break
			}
		}
		if !found {
			b.catalog.Renditions = append(b.catalog.Renditions, moq.Rendition{Track: name, ID: id})
		}
		sort.Slice(b.catalog.Renditions, func(i, j int) bool { return b.catalog.Renditions[i].ID < b.catalog.Renditions[j].ID })
	}
	section, err := b.catalog.JSON()
	if err != nil {
		return err
	}
	if err := b.public.SetCatalogSection(moq.CatalogSection, section); err != nil {
		return err
	}
	if err := b.local.SetCatalogSection(moq.CatalogSection, section); err != nil {
		return err
	}
	b.lastPublic = time.Now()
	if err := b.public.Announce(official.Route{}); err != nil {
		return err
	}
	if local {
		b.lastLocal = time.Now()
		if err := b.local.Announce(official.Route{}); err != nil {
			return err
		}
	}
	return nil
}

func writeMoqGroup(track *official.TrackProducer, stored *archivedGroup, groups *moqExpiringGroups) error {
	group, err := track.CreateGroup(stored.seq)
	if err != nil {
		return err
	}
	return finishMoqGroup(group, stored, groups)
}

// runMoqPublisher serves retained groups without replaying them into live tracks.
// Both live inventories expire; exact-path archive requests remain available.
func (r *WebsocketReplicator) runMoqPublisher(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var wg sync.WaitGroup
	defer func() { cancel(); wg.Wait() }()
	routes := map[*moqBroadcast]bool{}
	started := map[*official.TrackProducer]bool{}
	defer func() {
		cancel()
		r.moqMu.Lock()
		defer r.moqMu.Unlock()
		for _, b := range r.moqBroadcasts {
			b.mu.Lock()
			if b.archiveRoute != nil {
				b.archiveRoute.Cancel()
			}
			for _, t := range b.tracks {
				t.publicFetch.Cancel()
				t.localFetch.Cancel()
				t.groups.close()
			}
			_ = b.public.Close()
			_ = b.local.Close()
			b.mu.Unlock()
		}
	}()
	if err := r.restoreMoqInventory(ctx); err != nil {
		return err
	}
	for {
		r.moqMu.Lock()
		for _, b := range r.moqBroadcasts {
			b.mu.Lock()
			if b.archiveEligible && b.archiveRoute == nil {
				var err error
				b.archiveRoute, err = r.moqOrigin.Dynamic(b.streamer, official.Route{})
				if err != nil {
					b.mu.Unlock()
					r.moqMu.Unlock()
					return err
				}
			}
			if b.archiveRoute != nil && !routes[b] {
				routes[b] = true
				wg.Add(1)
				go func(b *moqBroadcast, route *official.OriginDynamic) {
					defer wg.Done()
					serveMoqArchiveRoute(ctx, b, route)
				}(b, b.archiveRoute)
			}
			if !b.lastPublic.IsZero() && time.Since(b.lastPublic) > r.cli.StreamSessionTimeout {
				_ = b.public.Unannounce()
				_ = b.public.RemoveCatalogSection(moq.CatalogSection)
				b.catalog.SourceSequence = nil
				b.lastPublic = time.Time{}
			}
			if !b.lastLocal.IsZero() && time.Since(b.lastLocal) > r.cli.StreamSessionTimeout {
				_ = b.local.Unannounce()
				_ = b.local.RemoveCatalogSection(moq.CatalogSection)
				b.lastLocal = time.Time{}
			}
			for name, t := range b.tracks {
				if err := t.archive.prune(time.Now()); err != nil {
					b.mu.Unlock()
					r.moqMu.Unlock()
					return err
				}
				for _, pair := range [2]struct {
					producer *official.TrackProducer
					dynamic  *official.TrackDynamic
				}{{t.public, t.publicFetch}, {t.local, t.localFetch}} {
					producer, dynamic := pair.producer, pair.dynamic
					if started[producer] {
						continue
					}
					started[producer] = true
					wg.Add(1)
					go func(b *moqBroadcast, name string, t *moqPublishedTrack) {
						defer wg.Done()
						r.serveMoqFetches(ctx, dynamic, b, name, t)
					}(b, name, t)
				}
			}
			if source := b.tracks[moq.SourceTrack]; source != nil && b.lastPublic.IsZero() {
				if _, retained := source.archive.latestSequence(); !retained {
					if b.archiveRoute != nil {
						b.archiveRoute.Cancel()
						b.archiveRoute = nil
					}
					b.archiveEligible = false
					delete(routes, b)
				}
			}
			b.mu.Unlock()
		}
		r.moqMu.Unlock()
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (r *WebsocketReplicator) serveMoqFetches(ctx context.Context, dynamic *official.TrackDynamic, b *moqBroadcast, name string, t *moqPublishedTrack) {
	defer dynamic.Cancel()
	for request, err := range dynamic.Requests(ctx) {
		if err != nil {
			return
		}
		stored, ok, err := t.archive.get(request.Sequence())
		if err != nil {
			_ = request.Abort(500)
			continue
		}
		if !ok {
			_ = request.Abort(404)
			continue
		}
		b.mu.Lock()
		source := b.tracks[moq.SourceTrack]
		b.mu.Unlock()
		if source == nil {
			_ = request.Abort(404)
			continue
		}
		sourceGroup := stored
		if name != moq.SourceTrack {
			sourceGroup, ok, err = source.archive.get(request.Sequence())
			if err != nil || !ok {
				_ = request.Abort(404)
				continue
			}
		}
		meta, err := r.mm.ValidateArchivedMP4(ctx, bytes.NewReader(sourceGroup.payload), b.streamer)
		if err != nil {
			_ = request.Abort(403)
			continue
		}
		if name != moq.SourceTrack {
			if err := validateMoqRenditionTrack(ctx, name, stored.payload); err != nil {
				_ = request.Abort(403)
				continue
			}
			if err := r.mm.ValidateReplicatedRenditions(ctx, b.streamer, sourceGroup.payload, stored.payload); err != nil {
				_ = request.Abort(403)
				continue
			}
		}
		stored.expiresAt = earlierMoqExpiry(stored.expiresAt, sourceGroup.expiresAt)
		signedExpiry, err := moqArchiveExpiry(meta, time.Now(), 0)
		if err != nil {
			_ = request.Abort(403)
			continue
		}
		stored.expiresAt = earlierMoqExpiry(stored.expiresAt, signedExpiry)
		group, err := request.Accept()
		if err != nil {
			continue
		}
		_ = finishMoqGroup(group, stored, &t.groups)
	}
}
