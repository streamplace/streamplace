package websocketrep

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	official "moq.dev/moq"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/moq"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/pkg/spmetrics"
)

// moqSourceChannel is the bus channel a stream's signed source segments
// ride; its rendition addenda ride media.RenditionsChannel. A broadcast
// carries both as one CMAF track per MUXL track (see docs/moq.md).
const moqSourceChannel = "source"

// Subscriber priorities: under contention the source tracks, which carry
// the audio, go before the renditions.
const (
	moqSourcePriority    = uint8(200)
	moqRenditionPriority = uint8(150)
)

// moqMaxAgeUs is how long a group that is no longer the newest stays
// deliverable to a subscriber: one that falls behind by more skips ahead
// rather than validating stale segments.
const moqMaxAgeUs = uint64(10 * time.Second / time.Microsecond)

// moqLivenessInterval is how often a served broadcast checks that its
// stream is still live on the bus.
var moqLivenessInterval = time.Second

// moqTLSRetry is how often a listener without a certificate yet, a new
// node's ACME one still on its way, asks for it again.
var moqTLSRetry = 5 * time.Second

// serveMoQ runs the node's Media over QUIC listener on --moq-addr: every
// stream live on this node's bus, for the peers that pull it.
func (r *WebsocketReplicator) serveMoQ(ctx context.Context) error {
	if r.MoqTLS == nil {
		return fmt.Errorf("moq listener has no TLS files")
	}
	for {
		err := r.MoqTLS.Sync()
		if err == nil {
			break
		}
		log.Warn(ctx, "moq listener is waiting for its TLS certificate", "error", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(moqTLSRetry):
		}
	}
	origin := official.NewOriginProducer()
	requests, err := origin.Dynamic("", official.Route{})
	if err != nil {
		return err
	}
	defer requests.Cancel()
	// Go's ":port" means every interface, which the transport only takes
	// spelled out; its "[::]" socket is dual-stack.
	bind := r.cli.MoqAddr
	if host, port, err := net.SplitHostPort(bind); err == nil && host == "" {
		bind = net.JoinHostPort("::", port)
	}
	srv, err := official.Listen(ctx, bind,
		official.WithTLSCert(r.MoqTLS.CertPath), official.WithTLSKey(r.MoqTLS.KeyPath),
		official.WithServerPublishOrigin(origin))
	if err != nil {
		return fmt.Errorf("listening for moq: %w", err)
	}
	defer srv.Close()
	log.Log(ctx, "moq server starting", "addr", srv.LocalAddr())
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		r.MoqTLS.Refresh(gctx, func(err error) { log.Error(gctx, "refreshing moq TLS", "error", err) })
		return nil
	})
	g.Go(func() error { return srv.Serve(gctx) })
	g.Go(func() error {
		var wg sync.WaitGroup
		defer wg.Wait()
		for req, err := range requests.Requests(gctx) {
			if err != nil {
				return err
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				r.serveBroadcast(gctx, req)
			}()
		}
		return nil
	})
	return g.Wait()
}

// serveBroadcast answers a peer's request for a streamer's broadcast: a
// stream that is not live here is refused rather than waited on, and a
// live one is served until it ends on the bus.
func (r *WebsocketReplicator) serveBroadcast(ctx context.Context, req *official.BroadcastRequest) {
	streamer, err := req.Path()
	if err != nil {
		return
	}
	if r.cli.DisableSyndication || !r.bus.HasSegments(streamer, moqSourceChannel) {
		_ = req.Reject(404)
		return
	}
	broadcast, err := official.NewBroadcastProducer()
	if err != nil {
		_ = req.Reject(500)
		return
	}
	defer broadcast.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		ticker := time.NewTicker(moqLivenessInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !r.bus.HasSegments(streamer, moqSourceChannel) {
					cancel()
					return
				}
			}
		}
	}()
	r.serveTracks(ctx, streamer, broadcast, func() error { return req.Accept(broadcast) })
}

// serveTracks feeds broadcast from the streamer's bus until ctx ends or a
// segment can't be served: every MUXL track of the source segments and
// rendition addenda becomes a CMAF track in the broadcast's catalog, primed
// with the bus's recent segments like the websocket path. publish makes the
// broadcast reachable once the first source segment's tracks are all in the
// catalog: it grows a track at a time, and a subscriber that saw part of
// the set would reassemble partial segments.
func (r *WebsocketReplicator) serveTracks(ctx context.Context, streamer string, broadcast *official.BroadcastProducer, publish func() error) {
	source := r.bus.SubscribeSegmentBuf(ctx, streamer, moqSourceChannel, 2)
	defer r.bus.UnsubscribeSegment(context.Background(), streamer, moqSourceChannel, source)
	renditions := r.bus.SubscribeSegmentBuf(ctx, streamer, media.RenditionsChannel, 4)
	defer r.bus.UnsubscribeSegment(context.Background(), streamer, media.RenditionsChannel, renditions)
	tracks := moqTracks{}
	defer tracks.finish()
	// The tracks of one source segment share a group sequence, which is
	// how a peer puts the segment back together (see consumeSource).
	var sequence uint64
	for {
		var err error
		select {
		case <-ctx.Done():
			return
		case seg := <-source.C:
			if !seg.Published {
				continue
			}
			n := sequence
			sequence++
			err = tracks.publish(ctx, broadcast, seg.Muxl, func(t *official.CMAFProducer) (*official.CMAFGroupProducer, error) { return t.CreateGroup(n) })
		case seg := <-renditions.C:
			if !seg.Published {
				continue
			}
			err = tracks.publish(ctx, broadcast, seg.Muxl, (*official.CMAFProducer).AppendGroup)
		}
		if err == nil && publish != nil && sequence > 0 {
			if err = publish(); err != nil {
				err = fmt.Errorf("could not publish broadcast: %w", err)
			}
			publish = nil
		}
		if err != nil {
			log.Warn(ctx, "moq: ending the broadcast", "streamer", streamer, "error", err)
			return
		}
	}
}

// moqTrack is one MUXL track served as a CMAF track. Its catalog entry
// comes from the track's own init, so a segment whose MUXL catalog box (a
// function of the codec config alone) differs cannot ride it.
type moqTrack struct {
	*official.CMAFProducer
	catalog []byte
}

// moqTracks are a broadcast's tracks, by MUXL rendition name.
type moqTracks map[string]*moqTrack

func (ts moqTracks) finish() {
	for _, t := range ts {
		_ = t.Finish()
	}
}

// publish writes each track of seg, a bare canonical segment, as a new
// group of its CMAF track, publishing the track on first sight: one frame
// per moof+mdat, the track's leading boxes (its C2PA manifest and MUXL
// catalog) riding the first, so the group's frames concatenate back to
// the signed bytes.
func (ts moqTracks) publish(ctx context.Context, broadcast *official.BroadcastProducer, seg []byte, group func(*official.CMAFProducer) (*official.CMAFGroupProducer, error)) error {
	tracks, err := media.MuxlTracks(ctx, seg)
	if err != nil {
		return fmt.Errorf("segment is not MUXL: %w", err)
	}
	for name, data := range tracks {
		fragments, catalog, err := cmafFragments(data)
		if err != nil {
			return fmt.Errorf("track %s: %w", name, err)
		}
		t := ts[name]
		if t != nil && !bytes.Equal(t.catalog, catalog) {
			return fmt.Errorf("track %s changed its codec config", name)
		}
		if t == nil {
			var init bytes.Buffer
			if err := muxl.RunMuxlWrapInit(ctx, bytes.NewReader(data), &init); err != nil {
				return fmt.Errorf("track %s init: %w", name, err)
			}
			producer, err := broadcast.PublishCMAF(name, init.Bytes())
			if err != nil {
				return fmt.Errorf("publishing track %s: %w", name, err)
			}
			t = &moqTrack{CMAFProducer: producer, catalog: catalog}
			ts[name] = t
		}
		g, err := group(t.CMAFProducer)
		if err != nil {
			return fmt.Errorf("track %s group: %w", name, err)
		}
		for _, fragment := range fragments {
			if err := g.WriteFragment(fragment); err != nil {
				return fmt.Errorf("track %s: %w", name, err)
			}
		}
		if err := g.Finish(); err != nil {
			return fmt.Errorf("track %s: %w", name, err)
		}
	}
	return nil
}

// cmafFragments splits one track's canonical segment into its CMAF
// fragments, one per moof+mdat with the leading boxes riding the first,
// and returns its MUXL catalog box: the box right before the first moof.
func cmafFragments(track []byte) (fragments [][]byte, catalog []byte, err error) {
	start, prev := 0, -1
	for off := 0; off < len(track); {
		if len(track)-off < 8 {
			return nil, nil, errors.New("truncated box")
		}
		size := int(binary.BigEndian.Uint32(track[off:]))
		if size < 8 || size > len(track)-off {
			return nil, nil, fmt.Errorf("box size %d out of range", size)
		}
		switch string(track[off+4 : off+8]) {
		case "moof":
			if catalog == nil && prev >= 0 {
				catalog = track[prev:off]
			}
		case "mdat":
			fragments = append(fragments, track[start:off+size])
			start = off + size
		}
		prev = off
		off += size
	}
	if catalog == nil || start != len(track) {
		return nil, nil, errors.New("not a canonical MUXL track")
	}
	return fragments, catalog, nil
}

// dialMoq opens a session to a moqt:// URL, pinning the certificate its
// certhash parameter names when it has one.
func (r *WebsocketReplicator) dialMoq(ctx context.Context, rawURL string) (*official.Client, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse moq URL: %w", err)
	}
	opts := []official.ClientOption{
		official.WithPublishOrigin(official.NewOriginProducer()),
		official.WithSubscribeOrigin(official.NewOriginProducer()),
		// The pull and push loops own reconnects, so each attempt
		// rediscovers the URL and its certificate hash.
		official.WithReconnect(false),
		official.WithWebSocketEnabled(false),
	}
	if q := u.Query(); q.Has(moq.CertHashParam) {
		pin := q.Get(moq.CertHashParam)
		if decoded, err := hex.DecodeString(pin); err != nil || len(decoded) != 32 {
			return nil, fmt.Errorf("invalid moq certificate SHA-256 fingerprint")
		}
		opts = append(opts, official.WithTLSFingerprints(pin))
		q.Del(moq.CertHashParam)
		u.RawQuery = q.Encode()
	}
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	client, err := official.Dial(dialCtx, u.String(), opts...)
	if err != nil {
		spmetrics.ReplicationConnectErrorsTotal.Inc()
		return nil, fmt.Errorf("%w (%s): %w", errMoqUnreachable, rawURL, err)
	}
	return client, nil
}

// requestMoqBroadcast resolves streamer's broadcast on a session to a peer.
// The peer's route arrives just after the session opens, so this waits for
// it, but not forever; the peer then serves the broadcast or refuses it.
func requestMoqBroadcast(ctx context.Context, client *official.Client, streamer string) (*official.BroadcastConsumer, error) {
	announced, err := client.AnnouncedBroadcast(streamer)
	if err != nil {
		return nil, err
	}
	defer announced.Cancel()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return announced.Available(ctx)
}

// consume follows one streamer's broadcast catalog and feeds its tracks
// through validation until the broadcast or ctx ends; it always returns
// an error saying why. Anything that fails validation ends it: the wire
// carries MUXL and nothing else.
func (r *WebsocketReplicator) consume(ctx context.Context, broadcast *official.BroadcastConsumer, streamer string) error {
	spmetrics.ReplicationOutboundOpen.WithLabelValues(streamer).Inc()
	defer spmetrics.ReplicationOutboundOpen.WithLabelValues(streamer).Dec()
	return consumeBroadcast(ctx, broadcast,
		func(segment []byte) error {
			if err := r.mm.ValidateReplicatedMP4(context.Background(), bytes.NewReader(segment), streamer); err != nil {
				return fmt.Errorf("could not validate segment: %w", err)
			}
			return nil
		},
		func(ctx context.Context, addendum []byte) error {
			return r.feedRenditions(ctx, streamer, addendum)
		})
}

// consumeBroadcast hands each source segment the broadcast's catalog
// leads to, reassembled, to source, and each rendition track's segments to
// rendition, until something fails or the broadcast or ctx ends.
func consumeBroadcast(ctx context.Context, broadcast *official.BroadcastConsumer, source func([]byte) error, rendition func(context.Context, []byte) error) error {
	catalogs, err := broadcast.SubscribeCatalog(ctx)
	if err != nil {
		spmetrics.ReplicationConnectErrorsTotal.Inc()
		return fmt.Errorf("could not subscribe to the catalog: %w", err)
	}
	defer catalogs.Cancel()
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var sourceTracks []string
		renditions := map[string]bool{}
		for catalog, err := range catalogs.Updates(gctx) {
			if err != nil {
				return fmt.Errorf("catalog: %w", err)
			}
			src, ren := moqTrackSets(catalog)
			// The origin's first catalog already holds a source segment's
			// whole track set (see serveTracks); one that changes it
			// starts a new session, which reads the new set.
			if sourceTracks == nil && len(src) > 0 {
				sourceTracks = src
				g.Go(func() error { return consumeSource(gctx, broadcast, src, source) })
			} else if !slices.Equal(src, sourceTracks) {
				return fmt.Errorf("source tracks changed from %v to %v", sourceTracks, src)
			}
			for _, name := range ren {
				if renditions[name] {
					continue
				}
				renditions[name] = true
				g.Go(func() error {
					return consumeTrack(gctx, broadcast, name, moqRenditionPriority, func(addendum []byte) error {
						return rendition(gctx, addendum)
					})
				})
			}
		}
		return errors.New("catalog ended")
	})
	return g.Wait()
}

// moqTrackSets sorts a catalog's tracks, named by their MUXL rendition
// ("video1", "audio2"), into the source segment's, in track-id order, and
// the renditions' (track ids from media.RenditionTrackID(0) on). A track
// named otherwise is not Streamplace's and is ignored.
func moqTrackSets(catalog *official.Catalog) (source, renditions []string) {
	ids := map[string]uint64{}
	add := func(name, kind string) {
		if id, err := strconv.ParseUint(strings.TrimPrefix(name, kind), 10, 32); err == nil && strings.HasPrefix(name, kind) {
			ids[name] = id
			if id < uint64(media.RenditionTrackID(0)) {
				source = append(source, name)
			} else {
				renditions = append(renditions, name)
			}
		}
	}
	for name := range catalog.Video {
		add(name, "video")
	}
	for name := range catalog.Audio {
		add(name, "audio")
	}
	slices.SortFunc(source, func(a, b string) int { return int(ids[a]) - int(ids[b]) })
	return source, renditions
}

// consumeSource reassembles the source segments from their tracks (names,
// in track-id order) and hands each to handle. A segment's tracks share a
// group sequence, which the first track leads; a track with no group at a
// segment's sequence has no part in that segment.
func consumeSource(ctx context.Context, broadcast *official.BroadcastConsumer, names []string, handle func([]byte) error) error {
	tracks := make([]*official.TrackConsumer, len(names))
	for i, name := range names {
		track, err := subscribeMoqTrack(ctx, broadcast, name, moqSourcePriority)
		if err != nil {
			return err
		}
		defer track.Cancel()
		tracks[i] = track
	}
	// Groups already read past the segment being assembled.
	ahead := make([]*official.GroupConsumer, len(tracks))
	defer func() {
		for _, group := range ahead {
			if group != nil {
				group.Cancel()
			}
		}
	}()
	for {
		lead, err := nextMoqGroup(ctx, tracks[0], names[0])
		if err != nil {
			return err
		}
		sequence := lead.Sequence()
		segment, err := readMoqGroup(ctx, lead)
		if err != nil {
			return fmt.Errorf("%s track: %w", names[0], err)
		}
		for i := 1; i < len(tracks); i++ {
			group := ahead[i]
			ahead[i] = nil
			for group == nil || group.Sequence() < sequence {
				if group != nil {
					group.Cancel()
				}
				if group, err = nextMoqGroup(ctx, tracks[i], names[i]); err != nil {
					return err
				}
			}
			if group.Sequence() > sequence {
				ahead[i] = group
				continue
			}
			part, err := readMoqGroup(ctx, group)
			if err != nil {
				return fmt.Errorf("%s track: %w", names[i], err)
			}
			segment = append(segment, part...)
		}
		if err := handle(segment); err != nil {
			return err
		}
	}
}

// consumeTrack hands each group of a track, one track's canonical segment,
// to handle in sequence order, until the track ends, handle fails or ctx
// ends.
func consumeTrack(ctx context.Context, broadcast *official.BroadcastConsumer, name string, priority uint8, handle func([]byte) error) error {
	track, err := subscribeMoqTrack(ctx, broadcast, name, priority)
	if err != nil {
		return err
	}
	defer track.Cancel()
	for {
		group, err := nextMoqGroup(ctx, track, name)
		if err != nil {
			return err
		}
		segment, err := readMoqGroup(ctx, group)
		if err != nil {
			return fmt.Errorf("%s track: %w", name, err)
		}
		if err := handle(segment); err != nil {
			return err
		}
	}
}

func subscribeMoqTrack(ctx context.Context, broadcast *official.BroadcastConsumer, name string, priority uint8) (*official.TrackConsumer, error) {
	track, err := broadcast.SubscribeTrack(ctx, name, &official.Subscription{Priority: priority, MaxAgeUs: moqMaxAgeUs})
	if err != nil {
		spmetrics.ReplicationConnectErrorsTotal.Inc()
		return nil, fmt.Errorf("could not subscribe to %s: %w", name, err)
	}
	return track, nil
}

func nextMoqGroup(ctx context.Context, track *official.TrackConsumer, name string) (*official.GroupConsumer, error) {
	group, err := track.NextGroup(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s track: %w", name, err)
	}
	if group == nil {
		return nil, fmt.Errorf("%s track ended", name)
	}
	return group, nil
}

// moqMaxGroupBytes bounds a group read from a peer, which nothing has
// checked yet: one track's segment for one GoP, a few megabytes at the
// highest bitrate Streamplace recommends.
var moqMaxGroupBytes = 64 << 20

// readMoqGroup reads a group's frames, the CMAF fragments of one track's
// canonical segment, back into that segment.
func readMoqGroup(ctx context.Context, group *official.GroupConsumer) ([]byte, error) {
	defer group.Cancel()
	var segment []byte
	for {
		frame, err := group.ReadFrame(ctx)
		if err != nil {
			return nil, err
		}
		if frame == nil {
			break
		}
		if len(segment)+len(frame.Payload) > moqMaxGroupBytes {
			return nil, fmt.Errorf("group %d is over %d bytes", group.Sequence(), moqMaxGroupBytes)
		}
		segment = append(segment, frame.Payload...)
	}
	if !isMuxl(segment) {
		return nil, fmt.Errorf("group %d is not a signed MUXL segment", group.Sequence())
	}
	return segment, nil
}

// isMuxl recognises a bare canonical segment: it leads with the c2pa or
// muxl uuid box, never with ftyp/moov (a presentation MP4) or anything else.
func isMuxl(b []byte) bool {
	return len(b) >= 8 && string(b[4:8]) == "uuid"
}

// errMoqUnreachable is a peer whose MoQ listener could not be dialled.
var errMoqUnreachable = errors.New("could not dial moq")
