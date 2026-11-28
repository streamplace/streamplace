// Package stationshard stores station workflow state in fixed, independently
// recoverable SlateDB shards. S3 conditional writes, not node clocks, fence
// writers. Clocks only determine when another node is allowed to contend.
package stationshard

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"stream.place/streamplace/pkg/s3"
)

var (
	ErrHeld     = errors.New("station lease held")
	ErrFenced   = errors.New("station writer fenced")
	ErrNotFound = errors.New("station state not found")
	ErrConflict = errors.New("station state conflict")
)

type Config struct {
	S3     s3.Config
	Prefix string
	Owner  string // routing identity of this node, not the shared station DID
	TTL    time.Duration
}

// Lease is a capability scoped to a resource AND a process incarnation. A
// release never deletes the control object: a later owner gets a higher epoch.
type Lease struct {
	DID         string `json:"did"`
	Role        string `json:"role"`
	Owner       string `json:"owner"`
	Incarnation string `json:"incarnation"`
	Epoch       uint64 `json:"epoch"`
}

type engine interface {
	Epoch(path string) (uint64, error)
	Open(path string, expectedEpoch uint64) (database, error)
	Close() error
}

type database interface {
	Get([]byte) ([]byte, error)
	Put([]byte, []byte) error // returns only after WAL durability
	Close() error
}

type Store struct {
	client      *awss3.Client
	bucket      string
	prefix      string
	owner       string
	incarnation string
	ttl         time.Duration
	engine      engine

	mu       sync.Mutex // serializes admission with Close, never held in callbacks
	active   sync.WaitGroup
	shutdown sync.Once
	closeErr error
	closed   bool
	slots    [256]writer
}

func Open(cfg Config) (*Store, error) {
	if cfg.TTL == 0 {
		cfg.TTL = 30 * time.Second
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "station-state/v1"
	}
	if cfg.S3.Region == "" {
		cfg.S3.Region = "us-east-1"
	}
	if cfg.Owner == "" || cfg.TTL <= 0 || cfg.S3.Bucket == "" || cfg.S3.AccessKeyID == "" || cfg.S3.SecretAccessKey == "" {
		return nil, errors.New("station shard requires a node owner, positive TTL, bucket and explicit S3 credentials")
	}
	if path.Clean(cfg.Prefix) != cfg.Prefix || strings.HasPrefix(cfg.Prefix, "/") || cfg.Prefix == "." || cfg.Prefix == ".." || strings.HasPrefix(cfg.Prefix, "../") {
		return nil, errors.New("station state prefix must be a clean relative key prefix")
	}
	u, err := url.Parse(cfg.S3.Endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("station S3 endpoint must be an HTTP(S) origin without embedded credentials or bucket path")
	}
	native, err := openEngine(cfg)
	if err != nil {
		return nil, err
	}
	s := &Store{
		client: s3.NewClient(cfg.S3), bucket: cfg.S3.Bucket, prefix: cfg.Prefix,
		owner: cfg.Owner, incarnation: uuid.NewString(), ttl: cfg.TTL, engine: native,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s3.QualifyConditional(ctx, s.client, s.bucket, s.prefix+"/probes"); err != nil {
		return nil, errors.Join(err, native.Close())
	}
	return s, nil
}

// Slot is stable as the fleet changes. The first eight bits of SHA-256 are
// part of the on-bucket format; changing them requires migrating shards.
func Slot(did string) uint16 { return uint16(sha256.Sum256([]byte(did))[0]) }

func (s *Store) TTL() time.Duration { return s.ttl }

func (s *Store) begin() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	s.active.Add(1)
	return nil
}

func (s *Store) slotPath(did string) string {
	return fmt.Sprintf("%s/slots/%02x", s.prefix, Slot(did))
}

func (s *Store) Close() error {
	s.shutdown.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.active.Wait()
		s.closeErr = s.closeWriters()
	})
	return s.closeErr
}

func (s *Store) closeWriters() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var result error
	for i := range s.slots {
		w := &s.slots[i]
		w.mu.Lock()
		if w.timer != nil {
			w.timer.Stop()
		}
		if w.db != nil {
			result = errors.Join(result, w.close())
			_, err := s.change(ctx, w.controlKey, w.lease, nil, release)
			if !errors.Is(err, ErrFenced) {
				result = errors.Join(result, err)
			}
		}
		w.mu.Unlock()
	}
	return errors.Join(result, s.engine.Close())
}
