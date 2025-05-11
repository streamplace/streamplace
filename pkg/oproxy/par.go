package oproxy

import "time"

type PAR struct {
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
}
