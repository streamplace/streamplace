package indexdb

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	glex "github.com/streamplace/glex/runtime"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
)

type BroadcastOrigin struct {
	URI             string    `gorm:"primaryKey;column:uri"`
	CID             string    `gorm:"column:cid"`
	RepoDID         string    `gorm:"column:repo_did"`
	Repo            *Repo     `gorm:"foreignKey:DID;references:RepoDID"`
	StreamerRepoDID string    `gorm:"column:streamer_repo_did;index:idx_streamer_repo_did_indexed_at,priority:1"`
	StreamerRepo    *Repo     `gorm:"foreignKey:DID;references:StreamerRepoDID"`
	ServerRepoDID   string    `gorm:"column:server_repo_did;index:idx_server_repo_did_indexed_at,priority:1"`
	IndexedAt       time.Time `gorm:"column:indexed_at;index:idx_streamer_repo_did_indexed_at,priority:2;index:idx_server_repo_did_indexed_at,priority:2"`
	Record          []byte    `gorm:"column:record"`
}

func (bo *BroadcastOrigin) TableName() string {
	return "broadcast_origins"
}

func (bo *BroadcastOrigin) ToBroadcastOriginView() (placestream.BroadcastDefs_BroadcastOriginView, error) {
	rec, err := glex.CborDecodeValue(bo.Record)
	if err != nil {
		return placestream.BroadcastDefs_BroadcastOriginView{}, fmt.Errorf("error decoding broadcast origin: %w", err)
	}
	return placestream.BroadcastDefs_BroadcastOriginView{
		Author: appbsky.ActorDefs_ProfileViewBasic{
			Did: bo.StreamerRepoDID,
		},
		Cid:    bo.CID,
		Record: &glex.LexiconTypeDecoder{Val: rec},
		Uri:    bo.URI,
	}, nil
}

func (m *DBModel) UpdateBroadcastOrigin(ctx context.Context, aturi syntax.ATURI, origin placestream.BroadcastOrigin) error {
	repoDID := aturi.Authority().String()
	cid, err := spid.GetCID(&origin)
	if err != nil {
		return fmt.Errorf("failed to get CID: %w", err)
	}
	validATURI := fmt.Sprintf("at://%s/place.stream.broadcast.origin/%s::%s", repoDID, origin.Streamer, origin.Server)
	if validATURI != aturi.String() {
		return fmt.Errorf("invalid ATURI: %s != %s", validATURI, aturi.String())
	}
	buf := bytes.Buffer{}
	err = origin.MarshalCBOR(&buf)
	if err != nil {
		return fmt.Errorf("failed to marshal origin: %w", err)
	}
	aqt := aqtime.FromTime(time.Now().UTC())

	bo := &BroadcastOrigin{
		URI:             validATURI,
		CID:             cid.String(),
		StreamerRepoDID: origin.Streamer,
		ServerRepoDID:   origin.Server,
		IndexedAt:       aqt.Time().UTC(),
		Record:          buf.Bytes(),
	}
	return m.DB.Save(bo).Error
}

func (m *DBModel) GetRecentBroadcastOrigins(ctx context.Context) ([]placestream.BroadcastDefs_BroadcastOriginView, error) {
	now := time.Now()
	oneMinuteAgo := now.Add(-1 * time.Minute)

	var origins []*BroadcastOrigin
	err := m.DB.
		Where("indexed_at >= ?", oneMinuteAgo.UTC()).
		Find(&origins).Error
	if err != nil {
		return nil, err
	}
	views := make([]placestream.BroadcastDefs_BroadcastOriginView, len(origins))
	for i, o := range origins {
		view, err := o.ToBroadcastOriginView()
		if err != nil {
			return nil, err
		}
		views[i] = view
	}
	return views, nil
}
