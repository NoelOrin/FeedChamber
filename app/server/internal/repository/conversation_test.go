package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"orinforum/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newConversationTestRepo(t *testing.T) *sqlConversationRepo {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Conversation{},
		&model.ConversationMember{},
		&model.Message{},
	))
	return &sqlConversationRepo{db: &mockSQLStore{db: db}}
}

func seedUser(t *testing.T, db *gorm.DB, id, username string) {
	t.Helper()
	u := model.User{
		ID:          id,
		Username:    username,
		DisplayName: username,
		Email:       username + "@example.com",
		Role:        "user",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	require.NoError(t, db.Create(&u).Error)
}

func TestConversation_CreateAndFindDirect(t *testing.T) {
	repo := newConversationTestRepo(t)
	ctx := context.Background()

	c := model.NewConversation()
	require.NoError(t, repo.CreateConversation(ctx, c, []string{"user-a", "user-b"}))

	found, err := repo.FindDirect(ctx, "user-a", "user-b")
	require.NoError(t, err)
	assert.Equal(t, c.ID, found.ID)

	// reverse order should still resolve the same direct conversation
	found2, err := repo.FindDirect(ctx, "user-b", "user-a")
	require.NoError(t, err)
	assert.Equal(t, c.ID, found2.ID)

	// unrelated pair
	_, err = repo.FindDirect(ctx, "user-a", "user-c")
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestConversation_CreateMessageAndList(t *testing.T) {
	repo := newConversationTestRepo(t)
	ctx := context.Background()

	c := model.NewConversation()
	require.NoError(t, repo.CreateConversation(ctx, c, []string{"user-a", "user-b"}))

	m1 := model.NewMessage(c.ID, "user-a", "hello")
	require.NoError(t, repo.CreateMessage(ctx, m1))
	m2 := model.NewMessage(c.ID, "user-b", "hi back")
	require.NoError(t, repo.CreateMessage(ctx, m2))

	msgs, total, err := repo.ListMessages(ctx, c.ID, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	require.Len(t, msgs, 2)
	// newest-first window
	assert.Equal(t, "hi back", msgs[0].Body)
	assert.Equal(t, "hello", msgs[1].Body)

	updated, err := repo.FindByID(ctx, c.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.LastMessageAt)
	assert.False(t, updated.LastMessageAt.IsZero())
}

func TestConversation_ListForUserUnreadAndMarkRead(t *testing.T) {
	repo := newConversationTestRepo(t)
	ctx := context.Background()
	db := repo.db.DB()

	seedUser(t, db, "user-a", "alice")
	seedUser(t, db, "user-b", "bob")

	c := model.NewConversation()
	require.NoError(t, repo.CreateConversation(ctx, c, []string{"user-a", "user-b"}))
	require.NoError(t, repo.CreateMessage(ctx, model.NewMessage(c.ID, "user-b", "ping")))

	items, total, err := repo.ListForUser(ctx, "user-a", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, items, 1)
	assert.Equal(t, c.ID, items[0].ID)
	assert.Equal(t, "user-b", items[0].PeerID)
	assert.Equal(t, "bob", items[0].PeerName)
	assert.Equal(t, "ping", items[0].LastMessage)
	assert.Equal(t, int64(1), items[0].UnreadCount)

	// mark read for user-a
	now := time.Now().Add(time.Second)
	require.NoError(t, repo.MarkRead(ctx, c.ID, "user-a", now))

	items, _, err = repo.ListForUser(ctx, "user-a", 0, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, int64(0), items[0].UnreadCount)

	ok, err := repo.IsMember(ctx, c.ID, "user-a")
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = repo.IsMember(ctx, c.ID, "user-z")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestConversation_FindDirectIgnoresLargerGroups(t *testing.T) {
	repo := newConversationTestRepo(t)
	ctx := context.Background()

	// three-member conversation should not match FindDirect(user-a,user-b)
	group := model.NewConversation()
	require.NoError(t, repo.CreateConversation(ctx, group, []string{"user-a", "user-b", "user-c"}))

	// true direct conversation
	direct := model.NewConversation()
	require.NoError(t, repo.CreateConversation(ctx, direct, []string{"user-a", "user-b"}))

	found, err := repo.FindDirect(ctx, "user-a", "user-b")
	require.NoError(t, err)
	assert.Equal(t, direct.ID, found.ID)
}

func TestConversation_CreateMessageLastMessageAtMonotonic(t *testing.T) {
	repo := newConversationTestRepo(t)
	ctx := context.Background()

	c := model.NewConversation()
	require.NoError(t, repo.CreateConversation(ctx, c, []string{"user-a", "user-b"}))

	newer := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	older := newer.Add(-2 * time.Hour)

	m1 := model.NewMessage(c.ID, "user-a", "newer first")
	m1.CreatedAt = newer
	require.NoError(t, repo.CreateMessage(ctx, m1))

	afterNewer, err := repo.FindByID(ctx, c.ID)
	require.NoError(t, err)
	require.NotNil(t, afterNewer.LastMessageAt)
	assert.True(t, afterNewer.LastMessageAt.Equal(newer))

	// Older message must not move last_message_at backwards.
	m2 := model.NewMessage(c.ID, "user-b", "older late insert")
	m2.CreatedAt = older
	require.NoError(t, repo.CreateMessage(ctx, m2))

	afterOlder, err := repo.FindByID(ctx, c.ID)
	require.NoError(t, err)
	require.NotNil(t, afterOlder.LastMessageAt)
	assert.True(t, afterOlder.LastMessageAt.Equal(newer), "last_message_at should stay at newer timestamp")
}

func TestConversation_ListForUserNoRowMultiplicationForMultiMember(t *testing.T) {
	repo := newConversationTestRepo(t)
	ctx := context.Background()
	db := repo.db.DB()

	seedUser(t, db, "user-a", "alice")
	seedUser(t, db, "user-b", "bob")
	seedUser(t, db, "user-c", "carol")
	seedUser(t, db, "user-d", "dave")

	group := model.NewConversation()
	require.NoError(t, repo.CreateConversation(ctx, group, []string{"user-a", "user-b", "user-c", "user-d"}))
	require.NoError(t, repo.CreateMessage(ctx, model.NewMessage(group.ID, "user-b", "group hello")))

	items, total, err := repo.ListForUser(ctx, "user-a", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, items, 1)
	assert.Equal(t, group.ID, items[0].ID)
	assert.NotEmpty(t, items[0].PeerID)
	assert.NotEqual(t, "user-a", items[0].PeerID)
	assert.Equal(t, "group hello", items[0].LastMessage)
	assert.Equal(t, int64(1), items[0].UnreadCount)
}

func TestConversation_CreateConversationRejectsEmptyAndDedupesMembers(t *testing.T) {
	repo := newConversationTestRepo(t)
	ctx := context.Background()

	err := repo.CreateConversation(ctx, model.NewConversation(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one member")

	err = repo.CreateConversation(ctx, model.NewConversation(), []string{"", ""})
	require.Error(t, err)

	c := model.NewConversation()
	require.NoError(t, repo.CreateConversation(ctx, c, []string{"user-a", "user-a", "user-b", ""}))

	var members []model.ConversationMember
	require.NoError(t, repo.db.DB().Where("conversation_id = ?", c.ID).Order("user_id ASC").Find(&members).Error)
	require.Len(t, members, 2)
	assert.Equal(t, "user-a", members[0].UserID)
	assert.Equal(t, "user-b", members[1].UserID)
}
