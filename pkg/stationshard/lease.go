package stationshard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

type control struct {
	Lease
	Expires  time.Time `json:"expires"`
	Revision uint64    `json:"revision"` // prevents ETag ABA when a payload changes back
	Payload  []byte    `json:"payload,omitempty"`
	etag     string
}

func (s *Store) load(ctx context.Context, key string) (control, error) {
	payload, etag, err := s.object(ctx, key)
	if err != nil {
		return control{}, err
	}
	var c control
	if err := json.Unmarshal(payload, &c); err != nil {
		return c, fmt.Errorf("invalid station control object: %w", err)
	}
	if c.Epoch == 0 || c.Revision == 0 || c.Role == "" || c.DID == "" || (c.Owner == "") != (c.Incarnation == "") {
		return c, errors.New("invalid station control fields")
	}
	c.etag = etag
	return c, nil
}

func (s *Store) save(ctx context.Context, key string, c *control) error {
	if c.Revision == math.MaxUint64 {
		return errors.New("station control revision exhausted")
	}
	c.Revision++
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	c.etag, err = s.swap(ctx, key, c.etag, data)
	return err
}

func (s *Store) claim(ctx context.Context, key, did, role string, reuse bool) (control, error) {
	if did == "" || role == "" {
		return control{}, errors.New("station claim lacks a DID or role")
	}
	for {
		c, err := s.load(ctx, key)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return c, err
		}
		if err == nil && (c.DID != did || c.Role != role) {
			return c, errors.New("station claim key and resource disagree")
		}
		now := time.Now()
		if c.Owner != "" && now.Before(c.Expires) {
			if reuse && c.Owner == s.owner && c.Incarnation == s.incarnation {
				return c, nil
			}
			return c, ErrHeld
		}
		if c.Epoch == math.MaxUint64 {
			return c, errors.New("station claim epoch exhausted")
		}
		c.Lease = Lease{DID: did, Role: role, Owner: s.owner, Incarnation: s.incarnation, Epoch: c.Epoch + 1}
		c.Expires = now.Add(s.ttl).UTC()
		err = s.save(ctx, key, &c)
		if errors.Is(err, ErrConflict) {
			continue
		}
		return c, err
	}
}

type leaseChange uint8

const (
	renew leaseChange = iota
	release
	// Recovery can take longer than a TTL. A candidate may finish opening
	// after expiry only if CAS proves nobody took a newer assignment. This
	// exception is not available to callbacks or role holders.
	confirmOpener
)

// Predicate, lease, and manifest live in the very same conditional PUT. A
// preflight check against a separate lease followed by PUT would not fence
// a paused worker. Retry conflicts only while the exact capability survives.
func (s *Store) change(ctx context.Context, key string, lease Lease, update func([]byte) ([]byte, error), kind leaseChange) (control, error) {
	if lease.Owner != s.owner || lease.Incarnation != s.incarnation {
		return control{}, ErrFenced
	}
	for {
		c, err := s.load(ctx, key)
		if errors.Is(err, ErrNotFound) {
			return c, ErrFenced
		}
		if err != nil {
			return c, err
		}
		if c.Lease != lease || (kind != confirmOpener && !time.Now().Before(c.Expires)) {
			return c, ErrFenced
		}
		if update != nil {
			c.Payload, err = update(c.Payload)
			if err != nil {
				return c, err
			}
		}
		now := time.Now()
		if kind != confirmOpener && !now.Before(c.Expires) {
			return c, ErrFenced
		}
		if kind == release {
			c.Owner, c.Incarnation = "", ""
			c.Expires = time.Time{}
		} else {
			c.Expires = now.Add(s.ttl).UTC()
		}
		err = s.save(ctx, key, &c)
		if errors.Is(err, ErrConflict) {
			continue
		}
		return c, err
	}
}

func (s *Store) ClaimRole(ctx context.Context, did, role string) (Lease, error) {
	if err := s.begin(); err != nil {
		return Lease{}, err
	}
	defer s.active.Done()
	c, err := s.claim(ctx, s.scopedKey("roles", did, role), did, role, false)
	if err != nil {
		return Lease{}, err
	}
	return c.Lease, nil
}

func (s *Store) changeRole(ctx context.Context, lease Lease, update func([]byte) ([]byte, error), kind leaseChange) error {
	if err := s.begin(); err != nil {
		return err
	}
	defer s.active.Done()
	_, err := s.change(ctx, s.scopedKey("roles", lease.DID, lease.Role), lease, update, kind)
	return err
}

func (s *Store) RenewRole(ctx context.Context, lease Lease) error {
	return s.changeRole(ctx, lease, nil, renew)
}

func (s *Store) ReleaseRole(ctx context.Context, lease Lease) error {
	return s.changeRole(ctx, lease, nil, release)
}

// UpdateRole may invoke update again on contention within the same lease.
// update must not perform non-idempotent side effects. Release and takeover
// preserve the payload; the consumer decides which epoch's opens are valid.
func (s *Store) UpdateRole(ctx context.Context, lease Lease, update func([]byte) ([]byte, error)) error {
	if update == nil {
		return errors.New("missing role update")
	}
	return s.changeRole(ctx, lease, update, renew)
}

func (s *Store) ReadRole(ctx context.Context, did, role string) ([]byte, error) {
	payload, _, err := s.ReadRoleState(ctx, did, role)
	return payload, err
}

// ReadRoleState returns one consistent snapshot of the manifest and its
// current holder. Completed entries survive expiry; open entries are only
// authoritative if they carry the returned nonzero lease.
func (s *Store) ReadRoleState(ctx context.Context, did, role string) ([]byte, Lease, error) {
	if err := s.begin(); err != nil {
		return nil, Lease{}, err
	}
	defer s.active.Done()
	c, err := s.load(ctx, s.scopedKey("roles", did, role))
	if err != nil {
		return nil, Lease{}, err
	}
	if c.Owner == "" || !time.Now().Before(c.Expires) {
		return c.Payload, Lease{}, nil
	}
	return c.Payload, c.Lease, nil
}
