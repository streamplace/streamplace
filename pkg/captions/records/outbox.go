package records

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

// The outbox uses the existing idempotent PDS record key. An interrupted final
// flush therefore survives both the session and a node restart without a second
// transcript record. Files contain no OAuth credentials.
type outboxRecord struct {
	Target  Target                         `json:"target"`
	Rkey    string                         `json:"rkey"`
	Record  *placestream.CaptionTranscript `json:"record"`
	RetryAt time.Time                      `json:"retryAt"`
}

type outbox struct {
	cfg    Config
	mu     sync.Mutex
	owners map[string]*session
	cancel context.CancelFunc
	done   chan struct{}
	wake   chan struct{}
}

func newOutbox(cfg Config) (*outbox, error) {
	if err := os.MkdirAll(cfg.OutboxDir, 0700); err != nil {
		return nil, fmt.Errorf("create caption outbox: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	o := &outbox{cfg: cfg, owners: map[string]*session{}, cancel: cancel, done: make(chan struct{}), wake: make(chan struct{}, 1)}
	go func() { defer close(o.done); o.run(ctx) }()
	return o, nil
}

func (o *outbox) save(owner *session, r outboxRecord) error {
	o.mu.Lock()
	o.owners[r.Rkey] = owner
	o.mu.Unlock()
	return o.write(r)
}

func (o *outbox) write(r outboxRecord) error {
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("encode caption outbox: %w", err)
	}
	f, err := os.CreateTemp(o.cfg.OutboxDir, ".pending-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(o.cfg.OutboxDir, r.Rkey+".json"))
}

func (o *outbox) remove(key string) {
	if err := os.Remove(filepath.Join(o.cfg.OutboxDir, key+".json")); err != nil && !os.IsNotExist(err) {
		log.Warn(context.Background(), "remove delivered caption outbox record", "error", err)
	}
	o.mu.Lock()
	delete(o.owners, key)
	o.mu.Unlock()
}

func (o *outbox) release(owner *session) {
	o.mu.Lock()
	for key, current := range o.owners {
		if current == owner {
			delete(o.owners, key)
		}
	}
	o.mu.Unlock()
	select {
	case o.wake <- struct{}{}:
	default:
	}
}

func (o *outbox) run(ctx context.Context) {
	var limitedUntil time.Time
	for ctx.Err() == nil {
		delay := time.Minute
		files, err := os.ReadDir(o.cfg.OutboxDir)
		if err != nil {
			log.Warn(ctx, "read caption outbox", "error", err)
		}
		for _, file := range files {
			if ctx.Err() != nil {
				return
			}
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}
			key := strings.TrimSuffix(file.Name(), ".json")
			o.mu.Lock()
			_, owned := o.owners[key]
			if !owned {
				o.owners[key] = nil
			}
			o.mu.Unlock()
			if owned {
				continue
			}
			data, err := os.ReadFile(filepath.Join(o.cfg.OutboxDir, file.Name()))
			var r outboxRecord
			if err == nil {
				err = json.Unmarshal(data, &r)
			}
			if err != nil || r.Rkey != key || r.Record == nil || r.Target.Repo == "" {
				log.Warn(ctx, "invalid caption outbox record", "file", file.Name(), "error", err)
				o.mu.Lock()
				delete(o.owners, key)
				o.mu.Unlock()
				continue
			}
			if limitedUntil.After(r.RetryAt) {
				r.RetryAt = limitedUntil
				if err := o.write(r); err != nil {
					log.Warn(ctx, "retain shared caption rate limit", "error", err)
				}
			}
			wait := r.RetryAt.Sub(o.cfg.Now())
			if wait <= 0 {
				uri, err := o.cfg.Publisher.Publish(ctx, r.Target, r.Rkey, r.Record)
				if err == nil {
					if o.cfg.Index != nil {
						if err := o.cfg.Index(ctx, r.Record, uri); err != nil {
							log.Warn(ctx, "index recovered caption transcript", "error", err)
						}
					}
					o.remove(key)
					continue
				}
				wait, limited := retryAfter(err, o.cfg.Now())
				if !limited {
					wait = backoffMin
				}
				r.RetryAt = o.cfg.Now().Add(wait)
				if limited {
					limitedUntil = r.RetryAt
				}
				if err := o.write(r); err != nil {
					log.Warn(ctx, "retain caption outbox retry", "error", err)
				}
			}
			delay = min(delay, max(time.Millisecond, r.RetryAt.Sub(o.cfg.Now())))
			o.mu.Lock()
			delete(o.owners, key)
			o.mu.Unlock()
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-o.wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}
