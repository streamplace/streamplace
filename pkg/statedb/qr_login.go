package statedb

import (
	"context"
	"errors"
	"time"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"gorm.io/gorm"
)

const (
	QRLoginStatusPending  = "pending"
	QRLoginStatusApproved = "approved"
	QRLoginStatusDenied   = "denied"
	// QRLoginStatusExpired is never stored: it's how a pending attempt past
	// ExpiresAt reads.
	QRLoginStatusExpired = "expired"
)

// QRLogin is a browser's request to sign in by having the Streamplace app,
// already signed in to this node, scan a QR code and approve it. Lifecycle:
// created pending by createQRLogin, then approved or denied exactly once by
// completeQRLogin, unless it expires first.
type QRLogin struct {
	ID string `gorm:"column:id;primarykey"`
	// RequestURI is the request_uri of the browser's pushed authorization
	// request, which approval completes.
	RequestURI  string    `gorm:"column:request_uri"`
	UserAgent   string    `gorm:"column:user_agent"`
	IP          string    `gorm:"column:ip"`
	Status      string    `gorm:"column:status"`
	ApproverDID string    `gorm:"column:approver_did"`
	Code        string    `gorm:"column:code"`
	State       string    `gorm:"column:state"`
	Iss         string    `gorm:"column:iss"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	ExpiresAt   time.Time `gorm:"column:expires_at;index"`
}

func (QRLogin) TableName() string { return "qr_logins" }

// CurrentStatus is Status, except that a pending attempt past its expiry
// reads as expired.
func (l *QRLogin) CurrentStatus() string {
	if l.Status == QRLoginStatusPending && !time.Now().Before(l.ExpiresAt) {
		return QRLoginStatusExpired
	}
	return l.Status
}

// CreateQRLogin stores a new attempt. Attempts are created without
// authentication, so it also drops the ones that expired over an hour ago.
func (state *StatefulDB) CreateQRLogin(ctx context.Context, l *QRLogin) error {
	db := state.DB.WithContext(ctx)
	if err := db.Where("expires_at < ?", time.Now().UTC().Add(-time.Hour)).Delete(&QRLogin{}).Error; err != nil {
		return err
	}
	return db.Create(l).Error
}

// GetQRLogin returns the attempt, or nil if there is none.
func (state *StatefulDB) GetQRLogin(ctx context.Context, id string) (*QRLogin, error) {
	var l QRLogin
	err := state.DB.WithContext(ctx).Where("id = ?", id).First(&l).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &l, nil
}

// CompleteQRLogin approves (auth non-nil) or denies (auth nil) an attempt on
// approverDID's behalf. It reports false, changing nothing, unless the attempt
// is still pending and unexpired, so only one completion ever wins.
func (state *StatefulDB) CompleteQRLogin(ctx context.Context, id, approverDID string, auth *oatproxy.AuthorizationResponse) (bool, error) {
	updates := map[string]any{
		"status":       QRLoginStatusDenied,
		"approver_did": approverDID,
	}
	if auth != nil {
		updates["status"] = QRLoginStatusApproved
		updates["code"] = auth.Code
		updates["state"] = auth.State
		updates["iss"] = auth.Iss
	}
	res := state.DB.WithContext(ctx).Model(&QRLogin{}).
		Where("id = ? AND status = ? AND expires_at > ?", id, QRLoginStatusPending, time.Now().UTC()).
		Updates(updates)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}
