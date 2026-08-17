package model

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"stream.place/streamplace/pkg/log"
)

// ErrAlreadyIndexed reports that a record was handed to the indexer again with
// exactly the content the index already holds.
//
// Records reach us at least once, never exactly once: the firehose replays from
// a cursor after a restart, a backfill walk that loses a race restarts against a
// new head and re-emits everything in range, and the same commit can arrive from
// several relays. All of that is by design, so a redelivery is not an error --
// but it is not a fresh record either, and callers that fan a new record out to
// the bus or to a notification queue must not do it twice. Hence a distinct
// error rather than a plain nil: it says "nothing changed, stop here" loudly
// enough that a caller cannot forget to check, while `err != nil` handling that
// checks for it first stays quiet in the logs.
var ErrAlreadyIndexed = errors.New("record already indexed")

// indexedRow is one row holding one atproto record, which knows which version of
// that record (its CID) it holds and what it is called.
//
// Rows keyed by CID (ChatMessage, VodComment) satisfy this trivially: for them a
// primary-key conflict already implies the CIDs are equal, so every conflict is
// a redelivery and there is no update arm to take.
type indexedRow interface {
	recordCID() string
	recordURI() string
}

func (b *Block) recordCID() string { return b.CID }
func (b *Block) recordURI() string {
	return fmt.Sprintf("at://%s/app.bsky.graph.block/%s", b.RepoDID, b.RKey)
}

func (m *ChatMessage) recordCID() string { return m.CID }
func (m *ChatMessage) recordURI() string { return m.URI }

func (fp *FeedPost) recordCID() string { return fp.CID }
func (fp *FeedPost) recordURI() string { return fp.URI }

func (g *Gate) recordCID() string { return g.CID }
func (g *Gate) recordURI() string {
	return fmt.Sprintf("at://%s/place.stream.chat.gate/%s", g.RepoDID, g.RKey)
}

func (ls *Livestream) recordCID() string { return ls.CID }
func (ls *Livestream) recordURI() string { return ls.URI }

func (md *ModerationDelegation) recordCID() string { return md.CID }
func (md *ModerationDelegation) recordURI() string {
	return fmt.Sprintf("at://%s/place.stream.moderation.permission/%s", md.RepoDID, md.RKey)
}

func (p *PinnedRecord) recordCID() string { return p.CID }
func (p *PinnedRecord) recordURI() string { return p.Uri }

func (tp *Teleport) recordCID() string { return tp.CID }
func (tp *Teleport) recordURI() string { return tp.URI }

func (c *VodComment) recordCID() string { return c.CID }
func (c *VodComment) recordURI() string { return c.URI }

func (g *VodGate) recordCID() string { return g.CID }
func (g *VodGate) recordURI() string {
	return fmt.Sprintf("at://%s/place.stream.vod.gate/%s", g.RepoDID, g.RKey)
}

// createOrVerify writes one indexed record, idempotently.
//
// The insert is an ON CONFLICT DO NOTHING, so a redelivery costs one statement
// and -- unlike the failing INSERT it replaces -- never reaches GORM's SQL error
// logger. Only an actual conflict pays for the SELECT that decides which of the
// two things just happened:
//
//   - the stored CID matches: the same record arrived twice. Nothing to write,
//     ErrAlreadyIndexed to the caller so it skips its side effects.
//   - the stored CID differs: an update to the record slipped past us (or a
//     walk re-emitted a path whose record has since changed). Overwrite the row
//     with what we were given, and let the caller treat it as news.
//
// key must select the conflicting row -- in practice the row's primary key.
func createOrVerify[T any, PT interface {
	*T
	indexedRow
}](ctx context.Context, m *DBModel, row PT, key map[string]any) error {
	res := m.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(row)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		return nil
	}

	var existing T
	err := m.DB.WithContext(ctx).Where(key).Take(PT(&existing)).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// The insert was refused but nothing is there to have refused it:
			// the conflict was on some other constraint, or the row was deleted
			// underneath us. Either way this is not the benign case.
			return fmt.Errorf("insert of %s conflicted but no existing row matched %v", row.recordURI(), key)
		}
		return fmt.Errorf("failed to read conflicting row for %s: %w", row.recordURI(), err)
	}

	if PT(&existing).recordCID() == row.recordCID() {
		return ErrAlreadyIndexed
	}

	// Debug: an updated record arriving again with a new CID is routine --
	// every place.stream.livestream heartbeat does it -- not an anomaly worth
	// a line per occurrence.
	log.Debug(ctx, "record changed on redelivery, updated",
		"uri", row.recordURI(), "oldCid", PT(&existing).recordCID(), "newCid", row.recordCID())
	return m.DB.WithContext(ctx).Save(row).Error
}
