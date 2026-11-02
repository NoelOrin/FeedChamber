package service

import (
	"context"
	"testing"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg/testnats"
	"feedchamber/internal/repository"

	"github.com/stretchr/testify/assert"
)

func TestExtractMentions(t *testing.T) {
	got := ExtractMentions("hi @Alice and @bob and @Alice")
	assert.Equal(t, []string{"Alice", "bob"}, got)
}

func TestExtractMentions_Boundaries(t *testing.T) {
	got := ExtractMentions("@start mid @ok ( @paren) email@x.com @a")
	// @a is length 1 (<2) skipped; email@x.com is not a mention boundary
	assert.Equal(t, []string{"start", "ok", "paren"}, got)
}

func TestExtractMentions_Empty(t *testing.T) {
	assert.Nil(t, ExtractMentions(""))
	assert.Nil(t, ExtractMentions("no mentions here"))
}

// stubMentionUsers resolves only the usernames it was told about; the shared
// mock.UserRepository returns ErrNotFound for every lookup, which would make
// these assertions vacuous.
type stubMentionUsers struct {
	repository.UserRepository // panics if anything other than the lookups below is used
	byName                    map[string]*model.User
}

func (s *stubMentionUsers) FindByUsername(_ context.Context, name string) (*model.User, error) {
	if u, ok := s.byName[name]; ok {
		return u, nil
	}
	return nil, repository.ErrNotFound
}

// One comment that reaches the same person as post author, as an @-mention and as
// a follower of the post must produce exactly one notification. Before the shared
// `notified` set existed, each fanout step published independently and real
// traffic produced three notifications for a single comment.
func TestPublishUserMentions_SkipsAlreadyNotifiedRecipients(t *testing.T) {
	ctx := context.Background()
	bus := testnats.NewEventBus(t)
	bob := &model.User{ID: "u-mentioned", Username: "bob"}
	users := &stubMentionUsers{byName: map[string]*model.User{"bob": bob}}

	// The comment fanout already notified bob as the post author.
	notified := map[string]struct{}{bob.ID: {}}

	publishUserMentions(bus, users, ctx, "u-author", "post-1", "hi @bob", "alice", notified)

	// Nothing new may be recorded for bob.
	_, stillThere := notified[bob.ID]
	assert.True(t, stillThere)
	assert.Len(t, notified, 1, "an already-notified recipient must not be added again")
}

func TestPublishUserMentions_AddsRecipientToSharedSet(t *testing.T) {
	ctx := context.Background()
	bus := testnats.NewEventBus(t)
	carol := &model.User{ID: "u-target", Username: "carol"}
	users := &stubMentionUsers{byName: map[string]*model.User{"carol": carol}}

	notified := map[string]struct{}{}
	publishUserMentions(bus, users, ctx, "u-author", "post-1", "hi @carol", "alice", notified)

	_, ok := notified["u-target"]
	assert.True(t, ok, "mention must record its recipient so later fanout steps skip them")
}

func TestPublishUserMentions_SkipsSelfMention(t *testing.T) {
	ctx := context.Background()
	bus := testnats.NewEventBus(t)
	self := &model.User{ID: "u-author", Username: "alice"}
	users := &stubMentionUsers{byName: map[string]*model.User{"alice": self}}

	notified := map[string]struct{}{}
	publishUserMentions(bus, users, ctx, "u-author", "post-1", "note @alice", "alice", notified)

	assert.Empty(t, notified, "mentioning yourself must not notify or reserve a recipient")
}
