package websocketrep

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	official "moq.dev/moq"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/moq"
)

// This inventory identifies a bounded transport spool, not a complete recording.
// Expired groups are intentionally unavailable; neither restart nor FETCH emits
// a live media notification or rebuilds a live catalog from retained sequences.
type moqArchiveInventory struct {
	Streamer string   `json:"streamer"`
	Tracks   []string `json:"tracks"`
}

func readMoqInventory(dir string) (*moqArchiveInventory, error) {
	data, err := os.ReadFile(filepath.Join(dir, "inventory.json"))
	if err != nil {
		return nil, err
	}
	var inventory moqArchiveInventory
	if err := json.Unmarshal(data, &inventory); err != nil {
		return nil, err
	}
	if _, err := syntax.ParseDID(inventory.Streamer); err != nil {
		return nil, fmt.Errorf("invalid archived DID: %w", err)
	}
	if filepath.Base(dir) != fmt.Sprintf("%x", sha256.Sum256([]byte(inventory.Streamer))) {
		return nil, fmt.Errorf("MoQ archive namespace hash mismatch")
	}
	for _, name := range inventory.Tracks {
		if name != moq.SourceTrack {
			if _, ok := moq.ParseRenditionTrack(name); !ok {
				return nil, fmt.Errorf("invalid archived track %q", name)
			}
		}
	}
	return &inventory, nil
}

// Called with b.mu held. Merge the durable inventory so reopening tracks one at
// a time cannot lose the remainder if the process stops during recovery.
func (r *WebsocketReplicator) persistMoqInventory(b *moqBroadcast) error {
	dir := filepath.Dir(r.archivePath(b.streamer, moq.SourceTrack))
	inventory, err := readMoqInventory(dir)
	if errors.Is(err, os.ErrNotExist) {
		if _, err := syntax.ParseDID(b.streamer); err != nil {
			return err
		}
		inventory = &moqArchiveInventory{Streamer: b.streamer}
	} else if err != nil {
		return err
	}
	tracks := make(map[string]bool, len(inventory.Tracks)+len(b.tracks))
	for _, name := range inventory.Tracks {
		tracks[name] = true
	}
	for name := range b.tracks {
		tracks[name] = true
	}
	inventory.Tracks = inventory.Tracks[:0]
	for name := range tracks {
		inventory.Tracks = append(inventory.Tracks, name)
	}
	sort.Strings(inventory.Tracks)
	data, err := json.Marshal(inventory)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".inventory-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return commitMoqArchive(f.Name(), filepath.Join(dir, "inventory.json"))
}

func (r *WebsocketReplicator) restoreMoqInventory(ctx context.Context) error {
	root := r.cli.DataFilePath([]string{"moq"})
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		inventory, err := readMoqInventory(filepath.Join(root, entry.Name()))
		if errors.Is(err, os.ErrNotExist) {
			continue // A write interrupted before its inventory was committed.
		}
		if err != nil {
			return err
		}
		allowed := r.cli.StreamIsAllowed(inventory.Streamer) == nil
		b, err := r.moqBroadcast(inventory.Streamer)
		if err != nil {
			return err
		}
		b.mu.Lock()
		for _, name := range inventory.Tracks {
			if _, err = r.publishedTrack(b, inventory.Streamer, name); err != nil {
				break
			}
		}
		if err == nil {
			for _, track := range b.tracks {
				if err = track.archive.prune(time.Now()); err != nil {
					break
				}
			}
		}
		// Disk identity alone is not evidence of a valid signed stream. Only
		// install the exact DID route after validating retained source bytes.
		if err == nil && allowed {
			if source := b.tracks[moq.SourceTrack]; source != nil {
				if seq, ok := source.archive.latestSequence(); ok {
					var stored *archivedGroup
					stored, ok, err = source.archive.get(seq)
					if err == nil && ok {
						_, err = r.mm.ValidateArchivedMP4(ctx, bytes.NewReader(stored.payload), b.streamer)
						if err == nil {
							b.archiveRoute, err = r.moqOrigin.Dynamic(b.streamer, official.Route{})
						}
					}
				}
			}
		}
		b.mu.Unlock()
		if err != nil {
			return fmt.Errorf("restore MoQ archive %s: %w", inventory.Streamer, err)
		}
	}
	return nil
}

func serveMoqArchiveRoute(ctx context.Context, b *moqBroadcast, route *official.OriginDynamic) {
	defer route.Cancel()
	for request, err := range route.Requests(ctx) {
		if err != nil {
			return
		}
		path, err := request.Path()
		if err != nil || path != b.streamer {
			_ = request.Reject(404)
			continue
		}
		_ = request.Accept(b.public)
	}
}

func earlierMoqExpiry(a, b time.Time) time.Time {
	if a.IsZero() || (!b.IsZero() && b.Before(a)) {
		return b
	}
	return a
}

func moqArchiveExpiry(meta *media.SegmentMetadata, now time.Time, retention time.Duration) (time.Time, error) {
	var expires time.Time
	if retention > 0 {
		expires = now.Add(retention)
	}
	if meta.DistributionPolicy != nil && meta.DistributionPolicy.DeleteAfterSeconds != nil {
		seconds := *meta.DistributionPolicy.DeleteAfterSeconds
		if seconds >= 0 {
			// Avoid signed duration multiplication wrapping into a permissive
			// expiry. A policy outside the representable range fails closed.
			if seconds > int64(^uint64(0)>>1)/int64(time.Second) {
				return time.Time{}, fmt.Errorf("MoQ delete-after exceeds supported duration")
			}
			expires = earlierMoqExpiry(expires, meta.StartTime.Time().Add(time.Duration(seconds)*time.Second))
		}
	}
	if !expires.IsZero() && !now.Before(expires) {
		return time.Time{}, fmt.Errorf("MoQ archival permission expired")
	}
	return expires, nil
}

// Native MaxAge is relative to the live edge, not a wall-clock deletion timer.
// Keep actual producers until expiry, including dynamic FETCH cache fills. The
// explicit limit bounds metadata even under repeated concurrent FETCH misses;
// exhaustion fails the request rather than dropping an expiry obligation.
const maxMoqExpiryHandles = 65536

type moqExpiringGroups struct {
	mu      sync.Mutex
	pending map[*official.GroupProducer]*time.Timer
	closed  bool
}

func (g *moqExpiringGroups) retain(group *official.GroupProducer, expires time.Time) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return fmt.Errorf("MoQ group lifecycle closed")
	}
	if expires.IsZero() {
		return nil // Explicit operator opt-out, and no signed deletion deadline.
	}
	if !time.Now().Before(expires) {
		return fmt.Errorf("MoQ archival permission expired")
	}
	if len(g.pending) >= maxMoqExpiryHandles {
		return fmt.Errorf("MoQ expiry handle capacity exhausted")
	}
	if g.pending == nil {
		g.pending = make(map[*official.GroupProducer]*time.Timer)
	}
	g.pending[group] = time.AfterFunc(time.Until(expires), func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		_ = group.Abort(403)
		delete(g.pending, group)
	})
	return nil
}

func (g *moqExpiringGroups) close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.closed = true
	for group, timer := range g.pending {
		timer.Stop()
		_ = group.Abort(503)
		delete(g.pending, group)
	}
}

func finishMoqGroup(group *official.GroupProducer, stored *archivedGroup, groups *moqExpiringGroups) error {
	if err := groups.retain(group, stored.expiresAt); err != nil {
		_ = group.Abort(403)
		return err
	}
	err := group.WriteFrame(official.Frame{Payload: stored.payload, TimestampUs: stored.timestampUs})
	if err == nil {
		err = group.Finish()
	}
	if err != nil {
		_ = group.Abort(500)
	}
	return err
}
