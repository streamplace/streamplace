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
	// BackfillFloor is a TID watermark for the collections a backfill reads by
	// time window (chat messages, feed posts): their history is contiguously
	// indexed from this TID up to now. Empty means no window has been recorded
	// -- either nothing is synced yet, or the row predates the watermark.
	BackfillFloor string `gorm:"column:backfill_floor" json:"backfillFloor,omitempty"`
	// BackfillDone reports that those windowed collections are indexed all the
	// way back to the start of the repo, so there is no history left to fetch.
	BackfillDone bool `gorm:"column:backfill_done" json:"backfillDone,omitempty"`
	// RepairFrom is the revision this repo was known good at when drift was
	// detected -- a firehose commit that did not follow our rev, or a head
	// check that disagreed with it. Marking a repo for repair clears Version
	// (the wedge every repair path already keys on), which would otherwise
	// throw away the one fact the repair needs: where the missed span starts.
	// Empty for a repo that has never been marked.
	RepairFrom string `gorm:"column:repair_from" json:"repairFrom,omitempty"`
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

// CountRepos reports how many repos the index has rows for. Zero means a
// fresh index, whose first sweep is the boot-critical work rather than
// background insurance.
func (m *DBModel) CountRepos() (int64, error) {
	var n int64
	err := m.DB.Model(&Repo{}).Count(&n).Error
	return n, err
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

// UpdateRepoIdentity writes just a repo's identity columns. Everything else on
// the row belongs to the sync engine — Version is CAS-advanced by the firehose,
// the backfill columns by the sweep — so a full-row Save here could stomp a
// concurrent advance; a two-column update cannot.
func (m *DBModel) UpdateRepoIdentity(did, handle, pds string) error {
	return m.DB.Model(&Repo{}).Where("did = ?", did).
		Select("Handle", "PDS").Updates(&Repo{Handle: handle, PDS: pds}).Error
}

// SetRepoStatus parks (or un-parks) a repo's account lifecycle state without
// touching the sync state in the rest of the row.
func (m *DBModel) SetRepoStatus(ctx context.Context, did string, status string) error {
	return m.DB.WithContext(ctx).Model(&Repo{}).Where("did = ?", did).Update("status", status).Error
}

// AdvanceRepoBackfill records the outcome of one deepening window: the repo is
// now indexed from floor forward (empty floor meaning all the way back), at the
// revision that window was read at. It reports whether the record applied.
//
// It writes exactly those four columns rather than the whole row, so a
// concurrent handle change or status update cannot be rolled back by a sweep
// that read the row minutes ago. Select names the fields explicitly, which is
// also what makes the zero values -- an empty floor, a false flag -- get
// written instead of skipped.
//
// A repo whose Version has been blanked is left alone, and the write reports
// false: an empty Version is a repair somebody has evidence for, marked while
// this window was being walked, and writing a Version here would quietly
// cancel it. The wedge wins; the window goes unrecorded and is re-walked
// after the repair.
func (m *DBModel) AdvanceRepoBackfill(ctx context.Context, did, version, rootCID, floor string, done bool) (bool, error) {
	res := m.DB.WithContext(ctx).Model(&Repo{}).Where("did = ? AND version <> ''", did).
		Select("Version", "RootCID", "BackfillFloor", "BackfillDone").
		Updates(Repo{
			Version:       version,
			RootCID:       rootCID,
			BackfillFloor: floor,
			BackfillDone:  done,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// AdvanceRepoVersion moves a repo's revision from one value to another, and
// only from that value: it is a compare-and-swap, and it reports whether it
// applied.
//
// The firehose hands events to a goroutine each, so nothing orders two commits
// on one repo. A CAS makes that harmless -- the event whose Since matches the
// stored rev is by definition the next one, and every other outcome is decided
// by re-reading the row rather than by whichever write landed last.
//
// An empty from is refused rather than executed: an empty Version is the wedge
// that means "this repo is being backfilled, or needs to be", and quietly
// filling it in from an event would un-wedge a repair nobody has done yet.
func (m *DBModel) AdvanceRepoVersion(ctx context.Context, did, from, to string) (bool, error) {
	if from == "" || to == "" {
		return false, nil
	}
	res := m.DB.WithContext(ctx).Model(&Repo{}).
		Where("did = ? AND version = ?", did, from).
		Select("Version").Updates(Repo{Version: to})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// MarkRepoForRepair records that this repo's index no longer matches its host:
// it clears Version -- the wedge that makes every existing repair path (the
// cached-sync fall-through, the sweep's plan) pick the repo up -- and remembers
// the rev it was last known good at in RepairFrom.
//
// Only Version and RepairFrom are written. The rest of the row is history the
// repair must not lose: the backfill watermark says how far back this repo is
// indexed, and a repair walks a recent window, so blanking it would send a
// completed repo back to the top of the deepening ladder.
//
// It is a compare-and-swap on from, so a repo somebody else has already wedged
// (or has since advanced past) is left alone, and it reports whether it applied.
func (m *DBModel) MarkRepoForRepair(ctx context.Context, did, from string) (bool, error) {
	if from == "" {
		return false, nil
	}
	res := m.DB.WithContext(ctx).Model(&Repo{}).
		Where("did = ? AND version = ?", did, from).
		Select("Version", "RepairFrom").
		Updates(Repo{Version: "", RepairFrom: from})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
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
