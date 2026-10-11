package statedb

import (
	"time"
)

// Lease is a time-bounded exclusive claim on a job the nodes sharing this
// database must not do twice at once: recording a stream, driving its
// multistream targets. The holder renews it well inside its TTL; a holder
// that dies simply stops, and the next node to ask after the TTL takes the
// job over. Clocks are the nodes' own, so the TTL must dwarf their skew.
type Lease struct {
	Name   string `gorm:"column:name;primarykey"`
	Holder string `gorm:"column:holder"`
	// ExpiresAt is Unix milliseconds: an integer compares the same way in
	// every database.
	ExpiresAt int64 `gorm:"column:expires_at"`
}

func (Lease) TableName() string {
	return "leases"
}

// AcquireLease takes the lease for holder, or renews it, when it is free,
// expired, or already theirs; it reports whether holder now has it. One
// upsert, so two nodes asking at once cannot both win.
func (state *StatefulDB) AcquireLease(name, holder string, ttl time.Duration) (bool, error) {
	now := time.Now()
	res := state.DB.Exec(`
		INSERT INTO leases (name, holder, expires_at) VALUES (?, ?, ?)
		ON CONFLICT (name) DO UPDATE SET holder = excluded.holder, expires_at = excluded.expires_at
		WHERE leases.holder = excluded.holder OR leases.expires_at < ?`,
		name, holder, now.Add(ttl).UnixMilli(), now.UnixMilli())
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// ReleaseLease gives the lease up, if holder has it, so the next node to
// ask gets it now rather than after the TTL.
func (state *StatefulDB) ReleaseLease(name, holder string) error {
	return state.DB.Exec(`DELETE FROM leases WHERE name = ? AND holder = ?`, name, holder).Error
}
