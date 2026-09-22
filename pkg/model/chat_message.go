package model

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/rivo/uniseg"
	glex "github.com/streamplace/glex/runtime"
	"strconv"

	"gorm.io/gorm"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/placestream"
)

type ChatMessage struct {
	CID             string       `json:"cid"                    gorm:"primaryKey;column:cid"`
	URI             string       `json:"uri"                    gorm:"column:uri"`
	CreatedAt       time.Time    `json:"createdAt"              gorm:"column:created_at;index:idx_recent_messages,priority:2"`
	ChatMessage     *[]byte      `json:"chatMessage"            gorm:"column:chat_message"`
	RepoDID         string       `json:"repoDID"                gorm:"column:repo_did"`
	Repo            *Repo        `json:"repo,omitempty"         gorm:"foreignKey:DID;references:RepoDID"`
	ChatProfile     *ChatProfile `json:"chatProfile,omitempty"  gorm:"foreignKey:RepoDID;references:RepoDID"`
	IndexedAt       *time.Time   `json:"indexedAt,omitempty"    gorm:"column:indexed_at"`
	StreamerRepoDID string       `json:"streamerRepoDID"        gorm:"column:streamer_repo_did;idx_recent_messages,priority:1"`
	StreamerRepo    *Repo        `json:"streamerRepo,omitempty" gorm:"foreignKey:DID;references:StreamerRepoDID"`
	ReplyToCID      *string      `json:"replyToCID,omitempty"   gorm:"column:reply_to_cid"`
	ReplyTo         *ChatMessage `json:"replyTo,omitempty"      gorm:"foreignKey:ReplyToCID;references:CID"`
	DeletedAt       *time.Time   `json:"deletedAt,omitempty"    gorm:"column:deleted_at"`
}

// hashString creates a hash from a string, used for deterministic color selection
func hashString(s string) int {
	h := fnv.New32a()
	h.Write([]byte(s))
	return int(h.Sum32())
}

func (m *ChatMessage) ToStreamplaceMessageView() (*placestream.ChatDefs_MessageView, error) {
	var msg placestream.ChatMessage
	if err := glex.DecodeCBOR(*m.ChatMessage, &msg); err != nil {
		return nil, fmt.Errorf("error decoding chat message: %w", err)
	}
	// Truncate overlong message text
	if uniseg.GraphemeClusterCount(msg.Text) > 300 {
		gr := uniseg.NewGraphemes(msg.Text)
		var result strings.Builder
		for count := 0; count < 300 && gr.Next(); count++ {
			result.WriteString(gr.Str())
		}
		msg.Text = result.String()
	}

	message := placestream.ChatDefs_MessageView{
		LexiconTypeID: "place.stream.chat.defs#messageView",
	}
	message.Uri = m.URI
	message.Cid = m.CID
	message.Author = appbsky.ActorDefs_ProfileViewBasic{
		Did: m.RepoDID,
	}
	if m.Repo != nil {
		message.Author.Handle = m.Repo.Handle
	}
	message.Record = &glex.LexiconTypeDecoder{Val: &msg}
	message.IndexedAt = m.IndexedAt.UTC().Format(time.RFC3339Nano)
	if m.ChatProfile != nil {
		scp, err := m.ChatProfile.ToStreamplaceChatProfile()
		if err != nil {
			return nil, fmt.Errorf("error converting chat profile to streamplace chat profile: %w", err)
		}
		message.ChatProfile = &scp
	} else {
		// If no chat profile exists, create a default one with a color based on the user's DID
		defaultColor := DefaultColors[hashString(m.RepoDID)%len(DefaultColors)]
		message.ChatProfile = &placestream.ChatProfile{
			Color: &defaultColor,
		}

	}
	if m.ReplyTo != nil {
		replyTo, err := m.ReplyTo.ToStreamplaceMessageView()
		if err != nil {
			return nil, fmt.Errorf("error converting reply to to streamplace message view: %w", err)
		}
		message.ReplyTo = &placestream.ChatDefs_MessageView_ReplyTo{
			ChatDefs_MessageView: replyTo,
		}
	}
	return &message, nil
}

// CreateChatMessage indexes one chat message. The table is keyed by record CID,
// so every conflict here is a redelivery of a message we already have; an edited
// message is a different CID and lands as its own row, as it always has.
func (m *DBModel) CreateChatMessage(ctx context.Context, message *ChatMessage) error {
	return createOrVerify(ctx, m, message, map[string]any{"cid": message.CID})
}

func (m *DBModel) DeleteChatMessage(ctx context.Context, uri string, deletedAt *time.Time) error {
	tx := m.DB.Model(&ChatMessage{}).Where("uri = ?", uri).Update("deleted_at", deletedAt)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return fmt.Errorf("no chat message found for uri: %s", uri)
	}
	return nil
}

func (m *DBModel) GetChatMessage(uri string) (*ChatMessage, error) {
	var message ChatMessage
	err := m.DB.
		Preload("Repo").
		Preload("ChatProfile").
		Preload("ReplyTo").
		Preload("ReplyTo.Repo").
		Preload("ReplyTo.ChatProfile").
		Where("uri = ?", uri).
		Where("deleted_at IS NULL").
		First(&message).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error retrieving chat message: %w", err)
	}
	return &message, nil
}

// visibleChatMessages is the query for a streamer's chat as a viewer may see
// it: the messages of the streamer's chat, minus those from accounts the
// streamer blocked, those a gate hid, those a labeler labeled and those that
// were deleted, with the author, chat profile and reply target loaded.
func (m *DBModel) visibleChatMessages(streamerDID string) *gorm.DB {
	return m.DB.
		Preload("Repo").
		Preload("ChatProfile").
		Preload("ReplyTo").
		Preload("ReplyTo.Repo").
		Preload("ReplyTo.ChatProfile").
		Where("streamer_repo_did = ?", streamerDID).
		// Exclude messages from users blocked by the streamer
		Joins("LEFT JOIN blocks ON blocks.repo_did = chat_messages.streamer_repo_did AND blocks.subject_did = chat_messages.repo_did").
		Where("blocks.rkey IS NULL"). // Only include messages where no block exists
		// Exclude gated messages
		Joins("LEFT JOIN gates ON gates.repo_did = chat_messages.streamer_repo_did AND gates.hidden_message = chat_messages.uri").
		Where("gates.hidden_message IS NULL"). // Only include messages where no gate exists
		// Exclude labeled messages
		Joins("LEFT JOIN labels ON labels.uri = chat_messages.uri").
		Where("labels.uri IS NULL"). // Only include messages where no label exists
		// Exclude deleted messages
		Where("chat_messages.deleted_at IS NULL")
}

func chatMessageViews(dbmessages []ChatMessage) ([]placestream.ChatDefs_MessageView, error) {
	spmessages := []placestream.ChatDefs_MessageView{}
	for _, m := range dbmessages {
		spmessage, err := m.ToStreamplaceMessageView()
		if err != nil {
			return nil, fmt.Errorf("error converting chat message to message view: %w", err)
		}
		spmessages = append(spmessages, *spmessage)
	}
	return spmessages, nil
}

func (m *DBModel) MostRecentChatMessages(repoDID string) ([]placestream.ChatDefs_MessageView, error) {
	dbmessages := []ChatMessage{}
	err := m.visibleChatMessages(repoDID).
		Limit(100).
		Order("chat_messages.created_at DESC").
		Find(&dbmessages).Error
	if err != nil {
		return nil, fmt.Errorf("error retrieving replies: %w", err)
	}
	return chatMessageViews(dbmessages)
}

// ChatMessagesBetween is a streamer's visible chat from one time to another,
// oldest first, for the replay of a recording: the messages sent while the
// livestream(s) it came from were on. Pages of limit with a cursor (the last
// message's time and CID); the returned cursor is empty on the last page.
func (m *DBModel) ChatMessagesBetween(ctx context.Context, streamerDID string, from, to time.Time, cursor string, limit int) ([]placestream.ChatDefs_MessageView, string, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	q := m.visibleChatMessages(streamerDID).WithContext(ctx).
		Where("chat_messages.created_at >= ? AND chat_messages.created_at <= ?", from, to)
	if cursor != "" {
		at, cid, err := decodeChatCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		q = q.Where("chat_messages.created_at > ? OR (chat_messages.created_at = ? AND chat_messages.cid > ?)", at, at, cid)
	}
	dbmessages := []ChatMessage{}
	if err := q.Order("chat_messages.created_at ASC, chat_messages.cid ASC").Limit(limit + 1).Find(&dbmessages).Error; err != nil {
		return nil, "", fmt.Errorf("error retrieving chat replay: %w", err)
	}
	next := ""
	if len(dbmessages) > limit {
		dbmessages = dbmessages[:limit]
		last := dbmessages[len(dbmessages)-1]
		next = encodeChatCursor(last.CreatedAt, last.CID)
	}
	views, err := chatMessageViews(dbmessages)
	return views, next, err
}

func encodeChatCursor(at time.Time, cid string) string {
	return strconv.FormatInt(at.UTC().UnixNano(), 10) + ":" + cid
}

func decodeChatCursor(cursor string) (time.Time, string, error) {
	i := strings.IndexByte(cursor, ':')
	if i <= 0 {
		return time.Time{}, "", fmt.Errorf("invalid cursor")
	}
	ns, err := strconv.ParseInt(cursor[:i], 10, 64)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid cursor")
	}
	return time.Unix(0, ns).UTC(), cursor[i+1:], nil
}
