package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PAR represents a Pushed Authorization Request for OAuth
type PAR struct {
	ID                  string    `gorm:"primaryKey"`
	ClientID            string    `json:"client_id" gorm:"column:client_id;index"`
	RedirectURI         string    `json:"redirect_uri" gorm:"column:redirect_uri"`
	CodeChallenge       string    `json:"code_challenge" gorm:"column:code_challenge;index"`
	CodeChallengeMethod string    `json:"code_challenge_method" gorm:"column:code_challenge_method"`
	State               string    `json:"state" gorm:"column:state"`
	LoginHint           string    `json:"login_hint" gorm:"column:login_hint"`
	ResponseMode        string    `json:"response_mode" gorm:"column:response_mode"`
	ResponseType        string    `json:"response_type" gorm:"column:response_type"`
	Scope               string    `json:"scope" gorm:"column:scope"`
	ExpiresAt           time.Time `json:"expires_at" gorm:"column:expires_at"`
	JKT                 string    `json:"jkt" gorm:"column:jkt"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           gorm.DeletedAt `gorm:"index"`
}

type PARResponse struct {
	RequestURI string `json:"request_uri"`
	ExpiresIn  int    `json:"expires_in"`
}

func (p *PAR) ToPARResponse() *PARResponse {
	return &PARResponse{
		RequestURI: p.ID,
		ExpiresIn:  int(p.ExpiresAt.Sub(time.Now()).Seconds()),
	}
}

// CreatePAR creates a new PAR record in the database
func (m *DBModel) CreatePAR(par *PAR) error {
	uu, err := uuid.NewV7()
	if err != nil {
		return err
	}
	par.ID = fmt.Sprintf("urn:ietf:params:oauth:request_uri:%s", uu.String())
	par.ExpiresAt = time.Now().Add(time.Minute * 10).UTC()
	return m.DB.Create(par).Error
}

// GetPAR retrieves a PAR by its ID
func (m *DBModel) GetPAR(id string) (*PAR, error) {
	var par PAR
	err := m.DB.Where("id = ?", id).First(&par).Error
	if err != nil {
		return nil, err
	}
	return &par, nil
}

func (m *DBModel) GetPARByCodeChallenge(codeChallenge string) (*PAR, error) {
	var par PAR
	err := m.DB.Where("code_challenge = ?", codeChallenge).First(&par).Error
	if err != nil {
		return nil, err
	}
	return &par, nil
}

// DeletePAR removes a PAR from the database
func (m *DBModel) DeletePAR(id string) error {
	return m.DB.Delete(&PAR{}, "id = ?", id).Error
}
