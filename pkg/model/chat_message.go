package model

import (
	"context"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/api/bsky"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"stream.place/streamplace/pkg/streamplace"
)

type ChatMessage struct {
	CID             string     `json:"cid"                    gorm:"primaryKey;column:cid"`
	URI             string     `json:"uri"                    gorm:"column:uri"`
	CreatedAt       time.Time  `json:"createdAt"              gorm:"column:created_at;index:idx_recent_messages,priority:2"`
	ChatMessage     *[]byte    `json:"chatMessage"            gorm:"column:chat_message"`
	RepoDID         string     `json:"repoDID"                gorm:"column:repo_did"`
	Repo            *Repo      `json:"repo,omitempty"         gorm:"foreignKey:DID;references:RepoDID"`
	IndexedAt       *time.Time `json:"indexedAt,omitempty"    gorm:"column:indexed_at"`
	StreamerRepoDID string     `json:"streamerRepoDID"        gorm:"column:streamer_repo_did;idx_recent_messages,priority:1"`
	StreamerRepo    *Repo      `json:"streamerRepo,omitempty" gorm:"foreignKey:DID;references:StreamerRepoDID"`
}

func (m *ChatMessage) ToStreamplaceMessageView() (*streamplace.ChatDefs_MessageView, error) {
	rec, err := lexutil.CborDecodeValue(*m.ChatMessage)
	if err != nil {
		return nil, fmt.Errorf("error decoding feed post: %w", err)
	}
	message := &streamplace.ChatDefs_MessageView{
		LexiconTypeID: "place.stream.chat.defs#messageView",
	}
	message.Uri = m.URI
	message.Cid = m.CID
	message.Author = &bsky.ActorDefs_ProfileViewBasic{
		Did:    m.RepoDID,
		Handle: m.Repo.Handle,
	}
	message.Record = &lexutil.LexiconTypeDecoder{Val: rec}
	message.IndexedAt = m.IndexedAt.UTC().Format(time.RFC3339Nano)
	return message, nil
}

func (m *DBModel) CreateChatMessage(ctx context.Context, message *ChatMessage) error {
	return m.DB.Create(message).Error
}

func (m *DBModel) MostRecentChatMessages(repoDID string) ([]*streamplace.ChatDefs_MessageView, error) {
	dbmessages := []ChatMessage{}
	err := m.DB.
		Preload("Repo").
		Where("streamer_repo_did = ?", repoDID).
		Limit(100).
		Order("created_at DESC").
		Find(&dbmessages).Error
	if err != nil {
		return nil, fmt.Errorf("error retrieving replies: %w", err)
	}
	spmessages := []*streamplace.ChatDefs_MessageView{}
	for _, m := range dbmessages {
		spmessage, err := m.ToStreamplaceMessageView()
		if err != nil {
			return nil, fmt.Errorf("error converting feed post to bsky post view: %w", err)
		}
		spmessages = append(spmessages, spmessage)
	}
	return spmessages, nil
}
