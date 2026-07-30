package indexdb

import (
	"context"
	"errors"
	"fmt"

	"github.com/bluesky-social/indigo/atproto/syntax"
	glex "github.com/streamplace/glex/runtime"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"stream.place/streamplace/pkg/appbsky"
)

type BskyProfile struct {
	URI            string  `json:"uri" gorm:"primaryKey;column:uri"`
	CID            string  `json:"cid" gorm:"column:cid"`
	RepoDID        string  `json:"repoDID" gorm:"column:repo_did"`
	Repo           *Repo   `json:"repo,omitempty" gorm:"foreignKey:DID;references:RepoDID"`
	Record         *[]byte `json:"record" gorm:"column:record"`
	WasStreamplace bool    `json:"wasStreamplace" gorm:"primaryKey;column:was_streamplace"`
}

// UpsertBskyProfile indexes an app.bsky.actor.profile record.
// wasStreamplace is the golive flag from the raw record (not part of the
// typed lexicon), so it stays caller-supplied; everything else derives
// from the record and AT-URI.
func (m *DBModel) UpsertBskyProfile(ctx context.Context, rec appbsky.ActorProfile, aturi syntax.ATURI, wasStreamplace bool) error {
	repoDID, cid, blob, err := recordParts(aturi, &rec)
	if err != nil {
		return err
	}
	dbProfile := &BskyProfile{
		URI:            aturi.String(),
		CID:            cid,
		RepoDID:        repoDID,
		Record:         &blob,
		WasStreamplace: wasStreamplace,
	}

	// Use GORM's OnConflict to handle unique/primary conflicts
	// If a conflict (same PK), then update the relevant fields
	return m.DB.
		Clauses(
			// Conflict columns: uri, was_streamplace (matching primaryKey in struct)
			clause.OnConflict{
				Columns:   []clause.Column{{Name: "uri"}, {Name: "was_streamplace"}},
				DoUpdates: clause.AssignmentColumns([]string{"cid", "repo_did", "record"}),
			},
		).
		Create(dbProfile).Error
}

func (m *DBModel) GetBskyProfile(ctx context.Context, did string, wasStreamplace bool) (*appbsky.ActorProfile, error) {
	var profile BskyProfile
	err := m.DB.Where("uri = ? AND was_streamplace = ?", fmt.Sprintf("at://%s/app.bsky.actor.profile/self", did), wasStreamplace).First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var bskyProfile appbsky.ActorProfile
	if err := glex.DecodeCBOR(*profile.Record, &bskyProfile); err != nil {
		return nil, fmt.Errorf("failed to decode profile record: %w", err)
	}
	return &bskyProfile, nil
}
