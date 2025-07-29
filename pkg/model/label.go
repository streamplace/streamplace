package model

import "gorm.io/gorm/clause"

type Label struct {
	// cid: Optionally, CID specifying the specific version of 'uri' resource this label applies to.
	Cid *string `json:"cid,omitempty" cborgen:"cid,omitempty" gorm:"column:cid"`
	// cts: Timestamp when this label was created.
	Cts string `json:"cts" cborgen:"cts" gorm:"column:cts"`
	// exp: Timestamp at which this label expires (no longer applies).
	Exp *string `json:"exp,omitempty" cborgen:"exp,omitempty" gorm:"column:exp"`
	// neg: If true, this is a negation label, overwriting a previous label.
	Neg *bool `json:"neg,omitempty" cborgen:"neg,omitempty" gorm:"column:neg"`
	// sig: Signature of dag-cbor encoded label.
	Sig []byte `json:"sig,omitempty" cborgen:"sig,omitempty" gorm:"column:sig"`
	// src: DID of the actor who created this label.
	Src string `json:"src" cborgen:"src" gorm:"primaryKey;column:src"`
	// uri: AT URI of the record, repository (account), or other resource that this label applies to.
	Uri string `json:"uri" cborgen:"uri" gorm:"primaryKey;column:uri;index"`
	// val: The short string name of the value or type of this label.
	Val string `json:"val" cborgen:"val" gorm:"primaryKey;column:val"`
	// ver: The AT Protocol version of the label object.
	Ver *int64 `json:"ver,omitempty" cborgen:"ver,omitempty" gorm:"column:ver"`

	Record []byte `json:"record,omitempty" cborgen:"record,omitempty" gorm:"column:record"`
}

func (m *DBModel) CreateLabel(label *Label) error {
	return m.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "src"},
			{Name: "uri"},
			{Name: "val"},
		},
		UpdateAll: true,
	}).Create(label).Error
}
