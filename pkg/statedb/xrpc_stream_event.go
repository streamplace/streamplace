package statedb

import (
	"bytes"
	"errors"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/util"
	"github.com/ipfs/go-cid"
	"gorm.io/gorm"
)

type XrpcStreamEvent struct {
	CID        string    `json:"cid" gorm:"column:cid;primaryKey"`
	RepoDID    string    `json:"repoDID" gorm:"index:idx_repo_timestamp,priority:1;index:idx_repo_seq,priority:1;column:repo_did"`
	Timestamp  time.Time `json:"timestamp" gorm:"index:idx_repo_timestamp,priority:2;column:timestamp"`
	Data       []byte    `json:"data" gorm:"column:data"`
	SignedData string    `json:"signedData" gorm:"column:signed_data"`
	Seq        int64     `json:"seq" gorm:"index:idx_repo_seq,priority:2;column:seq"`
}

func (ev *XrpcStreamEvent) ToCommitEvent() (*comatproto.SyncSubscribeRepos_Commit, error) {
	commit := &comatproto.SyncSubscribeRepos_Commit{}
	err := commit.UnmarshalCBOR(bytes.NewReader(ev.Data))
	if err != nil {
		return nil, err
	}
	return commit, nil
}

const CommitLockKey = "commit_lock"

func (state *StatefulDB) CreateCommitEvent(commit *comatproto.SyncSubscribeRepos_Commit, signedData string) error {
	unlock, err := state.GetNamedLock(CommitLockKey)
	if err != nil {
		return err
	}
	defer unlock()
	prev, err := state.GetMostRecentCommitEvent(commit.Repo)
	if err != nil {
		return err
	}
	if prev != nil {
		prevCommit, err := prev.ToCommitEvent()
		if err != nil {
			return err
		}
		commit.Seq = prevCommit.Seq + 1
		c, err := cid.Parse(prev.SignedData)
		if err != nil {
			return err
		}
		ll := lexutil.LexLink(c)
		commit.PrevData = &ll
		commit.Since = &prevCommit.Rev
	} else {
		commit.Seq = 1
	}
	buf := bytes.Buffer{}
	err = commit.MarshalCBOR(&buf)
	if err != nil {
		return err
	}
	timestamp, err := time.Parse(util.ISO8601, commit.Time)
	if err != nil {
		return err
	}
	event := &XrpcStreamEvent{
		CID:        commit.Commit.String(),
		RepoDID:    commit.Repo,
		Timestamp:  timestamp.UTC(),
		Data:       buf.Bytes(),
		Seq:        commit.Seq,
		SignedData: signedData,
	}
	return state.DB.Create(event).Error
}

func (state *StatefulDB) GetCommitEventsSince(repoDID string, t time.Time) ([]*XrpcStreamEvent, error) {
	var events []*XrpcStreamEvent
	query := state.DB.Where("repo_did = ?", repoDID)
	query = query.Where("timestamp > ?", t.UTC())
	err := query.Order("timestamp ASC").Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (state *StatefulDB) GetCommitEventsSinceSeq(repoDID string, seq int64) ([]*XrpcStreamEvent, error) {
	var events []*XrpcStreamEvent
	query := state.DB.Where("repo_did = ?", repoDID)
	query = query.Where("seq > ?", seq)
	err := query.Order("timestamp ASC").Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (state *StatefulDB) GetMostRecentCommitEvent(repoDID string) (*XrpcStreamEvent, error) {
	var event XrpcStreamEvent
	err := state.DB.Where("repo_did = ?", repoDID).
		Order("timestamp DESC").
		Limit(1).
		First(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &event, nil
}
