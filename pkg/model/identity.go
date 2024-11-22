package model

import "fmt"

// settings for publishing from a particular key. mostly node-local.
type Identity struct {
	ID     string `json:"id" gorm:"primaryKey"`
	Handle string `json:"handle"`
	DID    string `json:"did" gorm:"column:did"`
}

func (m *DBModel) GetIdentity(id string) (*Identity, error) {
	var identity Identity
	err := m.DB.Where("id = ?", id).FirstOrCreate(&identity, Identity{
		ID: id,
	}).Error
	if err != nil {
		return nil, fmt.Errorf("error getting settings: %w", err)
	}
	return &identity, nil
}

func (m *DBModel) UpdateIdentity(ident *Identity) error {
	err := m.DB.Where("id = ?", ident.ID).Save(ident).Error
	if err != nil {
		return fmt.Errorf("error updating settings: %w", err)
	}
	return nil
}
