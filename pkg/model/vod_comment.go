package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/api/bsky"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/rivo/uniseg"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/streamplace"
)

type VodComment struct {
	CID            string      `json:"cid"              gorm:"primaryKey;column:cid"`
	URI            string      `json:"uri"              gorm:"column:uri"`
	CreatedAt      time.Time   `json:"createdAt"        gorm:"column:created_at;index:idx_vod_comments_video,priority:2"`
	Comment        *[]byte     `json:"comment"          gorm:"column:comment"`
	RepoDID        string      `json:"repoDID"          gorm:"column:repo_did"`
	Repo           *Repo       `json:"repo,omitempty"   gorm:"foreignKey:DID;references:RepoDID"`
	IndexedAt      *time.Time  `json:"indexedAt"        gorm:"column:indexed_at"`
	VideoURI       string      `json:"videoURI"         gorm:"column:video_uri;index:idx_vod_comments_video,priority:1"`
	VideoAuthorDID string      `json:"videoAuthorDID"   gorm:"column:video_author_did"`
	ReplyToCID     *string     `json:"replyToCID"       gorm:"column:reply_to_cid"`
	ReplyTo        *VodComment `json:"replyTo,omitempty" gorm:"foreignKey:ReplyToCID;references:CID"`
	DeletedAt      *time.Time  `json:"deletedAt"        gorm:"column:deleted_at"`
}

func (c *VodComment) ToStreamplaceCommentView() (*streamplace.VodDefs_CommentView, error) {
	rec, err := lexutil.CborDecodeValue(*c.Comment)
	if err != nil {
		return nil, fmt.Errorf("error decoding comment: %w", err)
	}
	if msg, ok := rec.(*streamplace.VodComment); ok {
		graphemeCount := uniseg.GraphemeClusterCount(msg.Text)
		if graphemeCount > 300 {
			gr := uniseg.NewGraphemes(msg.Text)
			var result strings.Builder
			for count := 0; count < 300 && gr.Next(); count++ {
				result.WriteString(gr.Str())
			}
			msg.Text = result.String()
		}
	}
	commentView := &streamplace.VodDefs_CommentView{
		LexiconTypeID: "place.stream.vod.defs#commentView",
	}
	commentView.Uri = c.URI
	commentView.Cid = c.CID
	commentView.Author = &bsky.ActorDefs_ProfileViewBasic{Did: c.RepoDID}
	if c.Repo != nil {
		commentView.Author.Handle = c.Repo.Handle
	}
	commentView.Record = &lexutil.LexiconTypeDecoder{Val: rec}
	commentView.IndexedAt = c.IndexedAt.UTC().Format(time.RFC3339Nano)
	commentView.LikeCount = 0
	if c.ReplyTo != nil {
		replyTo, err := c.ReplyTo.ToStreamplaceCommentView()
		if err != nil {
			return nil, fmt.Errorf("error converting reply to comment view: %w", err)
		}
		commentView.ReplyTo = &streamplace.VodDefs_CommentView_ReplyTo{
			VodDefs_CommentView: replyTo,
		}
	}
	return commentView, nil
}

func (m *DBModel) CreateVodComment(ctx context.Context, comment *VodComment) error {
	return m.DB.Create(comment).Error
}

func (m *DBModel) DeleteVodComment(ctx context.Context, uri string, deletedAt *time.Time) error {
	tx := m.DB.Model(&VodComment{}).Where("uri = ?", uri).Update("deleted_at", deletedAt)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return fmt.Errorf("no VOD comment found for uri: %s", uri)
	}
	return nil
}

func (m *DBModel) GetVodComment(uri string) (*VodComment, error) {
	var comment VodComment
	err := m.DB.
		Preload("Repo").
		Preload("ReplyTo").
		Preload("ReplyTo.Repo").
		Where("uri = ?", uri).
		Where("deleted_at IS NULL").
		First(&comment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error retrieving VOD comment: %w", err)
	}
	return &comment, nil
}

func (m *DBModel) GetCommentsForVideo(ctx context.Context, videoURI string, limit int, cursor *time.Time) ([]*streamplace.VodDefs_CommentView, *time.Time, error) {
	dbcomments := []VodComment{}
	query := m.DB.
		Preload("Repo").
		Preload("ReplyTo").
		Preload("ReplyTo.Repo").
		Where("video_uri = ?", videoURI).
		Where("vod_comments.deleted_at IS NULL").
		Joins("LEFT JOIN blocks ON blocks.repo_did = vod_comments.video_author_did AND blocks.subject_did = vod_comments.repo_did").
		Where("blocks.rkey IS NULL").
		Joins("LEFT JOIN vod_gates ON vod_gates.hidden_comment = vod_comments.uri").
		Where("vod_gates.rkey IS NULL")
	if cursor != nil {
		query = query.Where("vod_comments.created_at < ?", *cursor)
	}
	err := query.
		Limit(limit + 1).
		Order("vod_comments.created_at DESC").
		Find(&dbcomments).Error
	if err != nil {
		return nil, nil, fmt.Errorf("error retrieving comments: %w", err)
	}
	var nextCursor *time.Time
	if len(dbcomments) > limit {
		nextCursor = &dbcomments[limit-1].CreatedAt
		dbcomments = dbcomments[:limit]
	}
	spcomments := []*streamplace.VodDefs_CommentView{}
	for _, c := range dbcomments {
		spcomment, err := c.ToStreamplaceCommentView()
		if err != nil {
			return nil, nil, fmt.Errorf("error converting comment to view: %w", err)
		}
		spcomments = append(spcomments, spcomment)
	}
	return spcomments, nextCursor, nil
}
