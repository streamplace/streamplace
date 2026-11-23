package statedb

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	notificationpkg "stream.place/streamplace/pkg/notifications"
)

// NotificationType is re-exported from pkg/notifications so callers of this
// package don't need a second import to name the transport.
type NotificationType = notificationpkg.NotificationType

// Re-export the transport constants for the same reason.
const (
	NotificationTypeFirebase = notificationpkg.NotificationTypeFirebase
	NotificationTypeWeb      = notificationpkg.NotificationTypeWeb
)

type Notification struct {
	Token            string           `gorm:"column:token;primarykey"`
	RepoDID          string           `json:"repoDID,omitempty" gorm:"column:repo_did;index"`
	Type             NotificationType `json:"type,omitempty" gorm:"column:type;default:firebase"`
	AuthenticatedDID string           `json:"-" gorm:"column:authenticated_did"`
	CreatedAt        time.Time        `gorm:"column:created_at"`
	UpdatedAt        time.Time        `gorm:"column:updated_at"`
}

// CreateNotification registers (or refreshes) a device's push token. When a
// repoDID is supplied we upsert it onto the token's row so livestream blasts
// can target the user's followers. When repoDID is empty we make sure the
// token row exists but never clobber an existing repoDID association.
//
// notifType selects the push transport ("firebase" or "web"). An empty value
// defaults to "firebase" so existing callers and pre-migration rows keep
// working. The type is only written when a row is created or a repoDID is
// being upserted; a DID-less re-registration deliberately leaves the type
// untouched (mirroring the repoDID-preservation behavior below).
//
// This deliberately avoids DB.Save(): Save issues a full-row UPDATE including
// zero-value columns, so a re-registration with no repoDID (e.g. the client
// posts before its OAuth session has restored) would blank out repo_did and
// silently drop the user from follower notifications.
func (state *StatefulDB) CreateNotification(token string, repoDID string, notifType NotificationType) error {
	return state.createNotification(token, repoDID, notifType, "")
}

// CreateAuthenticatedNotification associates a token with the DID verified by OAuth.
func (state *StatefulDB) CreateAuthenticatedNotification(token, did string, notifType NotificationType) error {
	return state.createNotification(token, did, notifType, did)
}

func (state *StatefulDB) createNotification(token, repoDID string, notifType NotificationType, authenticatedDID string) error {
	if notifType == "" {
		notifType = NotificationTypeFirebase
	}
	if repoDID != "" {
		not := Notification{Token: token, RepoDID: repoDID, Type: notifType, AuthenticatedDID: authenticatedDID}
		columns := []string{"repo_did", "type", "updated_at"}
		if authenticatedDID != "" {
			columns = append(columns, "authenticated_did")
		}
		return state.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "token"}},
			DoUpdates: clause.AssignmentColumns(columns),
		}).Create(&not).Error
	}
	not := Notification{Token: token, Type: notifType}
	return state.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token"}},
		DoNothing: true,
	}).Create(&not).Error
}

func (state *StatefulDB) ListNotifications() ([]Notification, error) {
	nots := []Notification{}
	err := state.DB.Find(&nots).Error
	if err != nil {
		return nil, fmt.Errorf("error retrieving notifications: %w", err)
	}
	return nots, nil
}

func (state *StatefulDB) ListUserNotifications(userDID string) ([]Notification, error) {
	nots := []Notification{}
	err := state.DB.Where("repo_did = ?", userDID).Find(&nots).Error
	if err != nil {
		return nil, fmt.Errorf("error retrieving notifications: %w", err)
	}
	return nots, nil
}

// GetManyNotificationTokens returns the raw token strings for the given user
// DIDs, across all notification types. Kept for backwards compatibility with
// callers that only need the token list (e.g. the legacy blast endpoint).
func (state *StatefulDB) GetManyNotificationTokens(userDIDs []string) ([]string, error) {
	tokens := []string{}
	err := state.DB.Model(&Notification{}).
		Where("repo_did IN (?)", userDIDs).
		Pluck("token", &tokens).
		Error
	if err != nil {
		return nil, fmt.Errorf("error retrieving notification tokens: %w", err)
	}
	return tokens, nil
}

// GetManyNotifications returns the full notification rows for the given user
// DIDs, including each token's Type so the notifier can route it to the
// correct transport (firebase vs web).
func (state *StatefulDB) GetManyNotifications(userDIDs []string) ([]Notification, error) {
	return getManyNotifications(state.DB, userDIDs)
}

// GetManyAuthenticatedNotifications excludes legacy, caller-asserted DID associations.
func (state *StatefulDB) GetManyAuthenticatedNotifications(userDIDs []string) ([]Notification, error) {
	return getManyNotifications(state.DB.Where("authenticated_did = repo_did"), userDIDs)
}

func getManyNotifications(db *gorm.DB, userDIDs []string) ([]Notification, error) {
	nots := []Notification{}
	err := db.Where("repo_did IN (?)", userDIDs).Find(&nots).Error
	if err != nil {
		return nil, fmt.Errorf("error retrieving notifications: %w", err)
	}
	return nots, nil
}

// DeleteNotification removes a token row, used when a web client unsubscribes
// (or a mobile token is revoked). Missing rows are not an error.
func (state *StatefulDB) DeleteNotification(token string) error {
	return state.DB.Where("token = ?", token).Delete(&Notification{}).Error
}
