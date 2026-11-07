package localdb

import "fmt"

// LocalRecord is an atproto record that stays on this node instead of its
// author's repo, such as their configuration for this node (see
// place.stream.local.putRecord). Record is its CBOR encoding.
type LocalRecord struct {
	RepoDID    string `gorm:"column:repo_did;primaryKey"`
	Collection string `gorm:"column:collection;primaryKey"`
	Rkey       string `gorm:"column:rkey;primaryKey"`
	CID        string `gorm:"column:cid;not null"`
	Record     []byte `gorm:"column:record;not null"`
}

// URI is the record's at:// URI, as it would be in its author's repo.
func (r *LocalRecord) URI() string {
	return fmt.Sprintf("at://%s/%s/%s", r.RepoDID, r.Collection, r.Rkey)
}

// PutLocalRecord creates the record, or replaces the one at its key.
func (m *LocalDatabase) PutLocalRecord(record *LocalRecord) error {
	return m.DB.Save(record).Error
}

// ListLocalRecords returns repoDID's records of collection, in rkey order.
func (m *LocalDatabase) ListLocalRecords(repoDID, collection string) ([]LocalRecord, error) {
	var records []LocalRecord
	err := m.DB.Where("repo_did = ? AND collection = ?", repoDID, collection).Order("rkey").Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("list local records: %w", err)
	}
	return records, nil
}

// DeleteLocalRecord deletes the record at the key; deleting none is no error.
func (m *LocalDatabase) DeleteLocalRecord(repoDID, collection, rkey string) error {
	return m.DB.Where("repo_did = ? AND collection = ? AND rkey = ?", repoDID, collection, rkey).Delete(&LocalRecord{}).Error
}
