package indexdb

import (
	"context"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"stream.place/streamplace/pkg/placestream"
)

type VodGate struct {
	RKey          string    `gorm:"primaryKey;column:rkey"`
	CID           string    `gorm:"column:cid"`
	RepoDID       string    `json:"repoDID"              gorm:"column:repo_did"`
	Repo          *Repo     `json:"repo,omitempty"       gorm:"foreignKey:DID;references:RepoDID"`
	HiddenComment string    `gorm:"column:hidden_comment" json:"hiddenComment"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (g *VodGate) ToRecord() (placestream.VodGate, error) {
	return placestream.VodGate{
		LexiconTypeID: "place.stream.vod.gate",
		HiddenComment: g.HiddenComment,
	}, nil
}

// UpsertVodGate indexes a place.stream.vod.gate record, deriving the
// row (rkey, CID, timestamps) from the record and AT-URI.
func (m *DBModel) UpsertVodGate(ctx context.Context, rec placestream.VodGate, aturi syntax.ATURI) error {
	repoDID, cid, _, err := recordParts(aturi, &rec)
	if err != nil {
		return err
	}
	return m.DB.Create(&VodGate{
		RKey:          aturi.RecordKey().String(),
		RepoDID:       repoDID,
		HiddenComment: rec.HiddenComment,
		CID:           cid,
		CreatedAt:     time.Now().UTC(),
	}).Error
}

func (m *DBModel) DeleteVodGate(ctx context.Context, rkey string) error {
	return m.DB.Where("rkey = ?", rkey).Delete(&VodGate{}).Error
}
