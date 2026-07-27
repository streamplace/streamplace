package model

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// Account lifecycle states a repo row can be parked in. Anything other than
// RepoStatusOK is terminal: the account is gone, hidden, or turned off, so
// retrying its backfill on every boot only burns requests. A live commit on the
// firehose is what proves the account is back and clears it.
const (
	RepoStatusOK          = ""
	RepoStatusDeactivated = "deactivated"
	RepoStatusNotFound    = "notfound"
	RepoStatusTakendown   = "takendown"
	RepoStatusSuspended   = "suspended"
)

type Repo struct {
	DID     string `gorm:"primaryKey;column:did" json:"did"`
	Handle  string `gorm:"index" json:"handle"`
	PDS     string `json:"pds"`
	Version string `json:"version"`
	RootCID string `json:"rootCid"`
	// Status is one of the RepoStatus* constants; empty for a normal account.
	Status string `gorm:"column:status" json:"status,omitempty"`
}

// TerminalStatus reports whether this repo is in an account state no amount of
// retrying will get us past.
func (r *Repo) TerminalStatus() bool {
	return r != nil && r.Status != RepoStatusOK
}

func (Repo) TableName() string {
	return "repos"
}

func (m *DBModel) GetRepo(did string) (*Repo, error) {
	var repoModel Repo
	res := m.DB.Where("did = ?", did).First(&repoModel)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if res.Error != nil {
		return nil, res.Error
	}
	return &repoModel, nil
}

func (m *DBModel) GetAllRepos() ([]Repo, error) {
	var repos []Repo
	res := m.DB.Find(&repos)
	if res.Error != nil {
		return nil, res.Error
	}
	return repos, nil
}

func (m *DBModel) GetRepoByHandle(handle string) (*Repo, error) {
	var repoModel Repo
	res := m.DB.Where("handle = ?", handle).First(&repoModel)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if res.Error != nil {
		return nil, res.Error
	}
	return &repoModel, nil
}

func (m *DBModel) GetRepoBySigningKey(signingKey string) (*Repo, error) {
	var repoModel Repo
	res := m.DB.Where("signing_key = ?", signingKey).First(&repoModel)
	if errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if res.Error != nil {
		return nil, res.Error
	}
	return &repoModel, nil
}

func (m *DBModel) GetRepoByHandleOrDID(arg string) (*Repo, error) {
	repo, err := m.GetRepoByHandle(arg)
	if err != nil {
		return nil, err
	}
	if repo != nil {
		return repo, nil
	}
	return m.GetRepo(arg)
}

func (m *DBModel) UpdateRepo(repo *Repo) error {
	return m.DB.Save(repo).Error
}

// SetRepoStatus parks (or un-parks) a repo's account lifecycle state without
// touching the sync state in the rest of the row.
func (m *DBModel) SetRepoStatus(ctx context.Context, did string, status string) error {
	return m.DB.WithContext(ctx).Model(&Repo{}).Where("did = ?", did).Update("status", status).Error
}

// TerminalRepoDIDs lists the repos parked in a terminal account state, so the
// boot-time sync sweep can skip them in one query instead of failing on each.
func (m *DBModel) TerminalRepoDIDs(ctx context.Context) ([]string, error) {
	var dids []string
	err := m.DB.WithContext(ctx).Model(&Repo{}).
		Where("status IS NOT NULL AND status != ?", RepoStatusOK).
		Pluck("did", &dids).Error
	if err != nil {
		return nil, err
	}
	return dids, nil
}

func (m *DBModel) SearchReposByHandle(query string, limit int) ([]Repo, error) {
	var repos []Repo
	// Search for repos where handle starts with the query (case-insensitive)
	// Use LIKE with LOWER for sqlite/postgres compatibility
	res := m.DB.Where("LOWER(handle) LIKE LOWER(?)", query+"%").Limit(limit).Find(&repos)
	if res.Error != nil {
		return nil, res.Error
	}
	return repos, nil
}
