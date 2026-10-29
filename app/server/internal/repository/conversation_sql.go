package repository

import (
	"context"
	"errors"
	"time"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"

	"gorm.io/gorm"
)

// ── SQL implementation ──────────────────────────────────────────────────────

type sqlConversationRepo struct{ db db.SQLStore }

func (r *sqlConversationRepo) CreateConversation(ctx context.Context, c *model.Conversation, memberUserIDs []string) error {
	if c == nil {
		return errors.New("conversation is nil")
	}
	// Dedupe while preserving first-seen order; empty IDs are ignored.
	seen := make(map[string]struct{}, len(memberUserIDs))
	members := make([]string, 0, len(memberUserIDs))
	for _, userID := range memberUserIDs {
		if userID == "" {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		members = append(members, userID)
	}
	if len(members) == 0 {
		return errors.New("conversation requires at least one member")
	}
	if c.ID == "" {
		c.ID = pkg.NewID()
	}
	now := time.Now()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = now
	}

	return r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(c).Error; err != nil {
			return err
		}
		for _, userID := range members {
			member := model.ConversationMember{
				ID:             pkg.NewID(),
				ConversationID: c.ID,
				UserID:         userID,
				CreatedAt:      now,
			}
			if err := tx.Create(&member).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *sqlConversationRepo) FindByID(ctx context.Context, id string) (*model.Conversation, error) {
	var c model.Conversation
	return &c, WrapDBErr(r.db.DB().WithContext(ctx).First(&c, "id = ?", id).Error)
}

// FindDirect locates the 1:1 conversation shared by exactly two users.
// Returns ErrNotFound when no such direct conversation exists.
func (r *sqlConversationRepo) FindDirect(ctx context.Context, userA, userB string) (*model.Conversation, error) {
	var c model.Conversation
	err := r.db.DB().WithContext(ctx).
		Table("conversations AS c").
		Select("c.*").
		Joins("JOIN conversation_members AS m1 ON m1.conversation_id = c.id AND m1.user_id = ?", userA).
		Joins("JOIN conversation_members AS m2 ON m2.conversation_id = c.id AND m2.user_id = ?", userB).
		Where(SQLSoftDeleteCondition("c")).
		Where(`(
			SELECT COUNT(*) FROM conversation_members cm
			WHERE cm.conversation_id = c.id
		) = 2`).
		Order("c.created_at ASC").
		Limit(1).
		Take(&c).Error
	return &c, WrapDBErr(err)
}

func (r *sqlConversationRepo) ListForUser(ctx context.Context, userID string, offset, limit int) ([]model.ConversationListItem, int, error) {
	gdb := r.db.DB().WithContext(ctx)

	var total int64
	if err := gdb.Table("conversation_members AS me").
		Joins("JOIN conversations AS c ON c.id = me.conversation_id").
		Where("me.user_id = ?", userID).
		Where(SQLSoftDeleteCondition("c")).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	type row struct {
		ID              string
		CreatedAt       time.Time
		UpdatedAt       time.Time
		LastMessageAt   *time.Time
		LastReadAt      *time.Time
		PeerID          string
		PeerName        string
		PeerAvatar      string
		LastMessageBody string
		UnreadCount     int64
	}

	var rows []row
	// Peer is resolved via scalar subqueries so multi-member conversations never multiply rows.
	// last_message: latest non-deleted message body; unread: messages after last_read_at not sent by self.
	err := gdb.Table("conversation_members AS me").
		Select(`
			c.id AS id,
			c.created_at AS created_at,
			c.updated_at AS updated_at,
			c.last_message_at AS last_message_at,
			me.last_read_at AS last_read_at,
			COALESCE((
				SELECT peer.user_id FROM conversation_members peer
				WHERE peer.conversation_id = c.id AND peer.user_id <> me.user_id
				ORDER BY peer.created_at ASC, peer.user_id ASC
				LIMIT 1
			), '') AS peer_id,
			COALESCE((
				SELECT u.username FROM conversation_members peer
				LEFT JOIN users AS u ON u.id = peer.user_id
				WHERE peer.conversation_id = c.id AND peer.user_id <> me.user_id
				ORDER BY peer.created_at ASC, peer.user_id ASC
				LIMIT 1
			), '') AS peer_name,
			COALESCE((
				SELECT u.avatar FROM conversation_members peer
				LEFT JOIN users AS u ON u.id = peer.user_id
				WHERE peer.conversation_id = c.id AND peer.user_id <> me.user_id
				ORDER BY peer.created_at ASC, peer.user_id ASC
				LIMIT 1
			), '') AS peer_avatar,
			COALESCE((
				SELECT m.body FROM messages m
				WHERE m.conversation_id = c.id AND m.deleted_at IS NULL
				ORDER BY m.created_at DESC
				LIMIT 1
			), '') AS last_message_body,
			(
				SELECT COUNT(*) FROM messages m
				WHERE m.conversation_id = c.id
				  AND m.deleted_at IS NULL
				  AND m.sender_id <> me.user_id
				  AND (me.last_read_at IS NULL OR m.created_at > me.last_read_at)
			) AS unread_count
		`).
		Joins("JOIN conversations AS c ON c.id = me.conversation_id").
		Where("me.user_id = ?", userID).
		Where(SQLSoftDeleteCondition("c")).
		Order("COALESCE(c.last_message_at, c.created_at) DESC").
		Offset(offset).Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	items := make([]model.ConversationListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, model.ConversationListItem{
			ID:            r.ID,
			PeerID:        r.PeerID,
			PeerName:      r.PeerName,
			PeerAvatar:    r.PeerAvatar,
			LastMessage:   r.LastMessageBody,
			LastMessageAt: r.LastMessageAt,
			UnreadCount:   r.UnreadCount,
			CreatedAt:     r.CreatedAt,
			UpdatedAt:     r.UpdatedAt,
		})
	}
	return items, int(total), nil
}

func (r *sqlConversationRepo) IsMember(ctx context.Context, conversationID, userID string) (bool, error) {
	var count int64
	err := r.db.DB().WithContext(ctx).
		Model(&model.ConversationMember{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *sqlConversationRepo) CreateMessage(ctx context.Context, m *model.Message) error {
	if m == nil {
		return errors.New("message is nil")
	}
	if m.ID == "" {
		m.ID = pkg.NewID()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	now := m.CreatedAt

	return r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		// Keep last_message_at monotonic: only advance when the new timestamp is newer.
		return tx.Model(&model.Conversation{}).
			Where("id = ?", m.ConversationID).
			Where("last_message_at IS NULL OR last_message_at < ?", now).
			Updates(map[string]any{
				"last_message_at": now,
				"updated_at":      now,
			}).Error
	})
}

func (r *sqlConversationRepo) ListMessages(ctx context.Context, conversationID string, offset, limit int) ([]model.Message, int, error) {
	gdb := r.db.DB().WithContext(ctx)

	var total int64
	if err := gdb.Model(&model.Message{}).
		Where("conversation_id = ?", conversationID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Newest-first paging: offset 0 is the latest window for chat UIs.
	var messages []model.Message
	err := gdb.Where("conversation_id = ?", conversationID).
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&messages).Error
	return messages, int(total), err
}

func (r *sqlConversationRepo) MarkRead(ctx context.Context, conversationID, userID string, at time.Time) error {
	result := r.db.DB().WithContext(ctx).
		Model(&model.ConversationMember{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Update("last_read_at", at)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
