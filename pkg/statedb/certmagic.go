package statedb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CertmagicItem is one entry of the certmagic key/value store: ACME account
// keys, certificates, and in-flight challenge state. Living in statedb means
// every node in a station shares them, so one node obtains a certificate and
// the rest serve it, and any node can answer an HTTP-01 challenge another
// node started. Keys are slash-separated paths, as certmagic expects.
type CertmagicItem struct {
	Key       string    `gorm:"column:key;primaryKey"`
	Value     []byte    `gorm:"column:value;type:bytes"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (CertmagicItem) TableName() string {
	return "certmagic_items"
}

// CertmagicGet returns the item at key, or nil when absent.
func (state *StatefulDB) CertmagicGet(ctx context.Context, key string) (*CertmagicItem, error) {
	var item CertmagicItem
	err := state.DB.WithContext(ctx).Where("key = ?", key).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("certmagic get %q: %w", key, err)
	}
	return &item, nil
}

// CertmagicPut creates or overwrites key.
func (state *StatefulDB) CertmagicPut(ctx context.Context, key string, value []byte) error {
	item := CertmagicItem{Key: key, Value: value, UpdatedAt: time.Now()}
	err := state.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&item).Error
	if err != nil {
		return fmt.Errorf("certmagic put %q: %w", key, err)
	}
	return nil
}

// escapeLike makes s safe to use inside a LIKE pattern with ESCAPE '\'.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// certmagicScope matches key itself and everything under key + "/". An empty
// prefix matches everything.
func certmagicScope(db *gorm.DB, prefix string) *gorm.DB {
	if prefix == "" {
		return db
	}
	return db.Where(`key = ? OR key LIKE ? ESCAPE '\'`, prefix, escapeLike(prefix)+"/%")
}

// CertmagicDelete removes key and every key under it. Returns the number of
// rows removed.
func (state *StatefulDB) CertmagicDelete(ctx context.Context, key string) (int64, error) {
	res := certmagicScope(state.DB.WithContext(ctx), key).Delete(&CertmagicItem{})
	if res.Error != nil {
		return 0, fmt.Errorf("certmagic delete %q: %w", key, res.Error)
	}
	return res.RowsAffected, nil
}

// CertmagicListInfo is the metadata of one item, without its value.
type CertmagicListInfo struct {
	Key       string
	Size      int64
	UpdatedAt time.Time
}

// CertmagicList returns the metadata of prefix itself (when it is an item)
// and of every item under prefix + "/", ordered by key.
func (state *StatefulDB) CertmagicList(ctx context.Context, prefix string) ([]CertmagicListInfo, error) {
	var rows []CertmagicListInfo
	err := certmagicScope(state.DB.WithContext(ctx).Model(&CertmagicItem{}), prefix).
		Select("key, length(value) AS size, updated_at").
		Order("key ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("certmagic list %q: %w", prefix, err)
	}
	return rows, nil
}
