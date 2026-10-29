package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/repository"

	"github.com/sirupsen/logrus"
)

type pmService struct {
	repo     repository.ConversationRepository
	userRepo repository.UserRepository
	notif    NotificationService
	blockSvc UserBlockService
}

// NewPMService constructs a private-message service.
func NewPMService(repo repository.ConversationRepository, userRepo repository.UserRepository, notif NotificationService, blockSvc UserBlockService) PMService {
	return &pmService{repo: repo, userRepo: userRepo, notif: notif, blockSvc: blockSvc}
}

func (s *pmService) StartDirect(ctx context.Context, actorID, peerUserID, firstBody string) (*model.Conversation, *model.Message, error) {
	actorID = strings.TrimSpace(actorID)
	peerUserID = strings.TrimSpace(peerUserID)
	firstBody = strings.TrimSpace(firstBody)

	if actorID == "" || peerUserID == "" {
		return nil, nil, pkg.NewAppError(pkg.INVALID_PARAMS, "用户无效")
	}
	if actorID == peerUserID {
		return nil, nil, pkg.NewAppError(pkg.INVALID_PARAMS, "不能给自己发私信")
	}
	if s.blockSvc != nil {
		if blocked, err := s.blockSvc.EitherBlocked(ctx, actorID, peerUserID); err != nil {
			return nil, nil, err
		} else if blocked {
			return nil, nil, pkg.NewAppError(pkg.FORBIDDEN, "由于屏蔽关系无法发起私信")
		}
	}

	peer, err := s.userRepo.FindByID(ctx, peerUserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, pkg.NewAppError(pkg.NOT_FOUND, "用户不存在")
		}
		return nil, nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if peer == nil {
		return nil, nil, pkg.NewAppError(pkg.NOT_FOUND, "用户不存在")
	}

	// Existing direct conversation: open only, never auto-send.
	conv, err := s.repo.FindDirect(ctx, actorID, peerUserID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err == nil && conv != nil {
		return conv, nil, nil
	}

	// New conversation requires an initial message body.
	if firstBody == "" {
		return nil, nil, pkg.NewAppError(pkg.INVALID_PARAMS, "消息内容不能为空")
	}

	// create-then-reuse: concurrent creates are acceptable for MVP;
	// subsequent FindDirect returns the oldest direct conversation.
	conv = model.NewConversation()
	if err := s.repo.CreateConversation(ctx, conv, []string{actorID, peerUserID}); err != nil {
		// race: another request may have created the same pair — open only
		existing, findErr := s.repo.FindDirect(ctx, actorID, peerUserID)
		if findErr != nil {
			return nil, nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		return existing, nil, nil
	}

	msg, err := s.Send(ctx, actorID, conv.ID, firstBody)
	if err != nil {
		return nil, nil, err
	}
	// refresh conversation for last_message_at
	if updated, getErr := s.repo.FindByID(ctx, conv.ID); getErr == nil && updated != nil {
		conv = updated
	}
	return conv, msg, nil
}

func (s *pmService) ListConversations(ctx context.Context, userID string, page, pageSize int) ([]model.ConversationListItem, int, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	items, total, err := s.repo.ListForUser(ctx, userID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if items == nil {
		items = []model.ConversationListItem{}
	}
	return items, total, nil
}

func (s *pmService) ListMessages(ctx context.Context, userID, conversationID string, page, pageSize int) ([]model.Message, int, error) {
	if err := s.requireMember(ctx, conversationID, userID); err != nil {
		return nil, 0, err
	}
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	// Repo returns newest-first pages; reverse to chronological oldest→newest for chat UI.
	msgs, total, err := s.repo.ListMessages(ctx, conversationID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if msgs == nil {
		msgs = []model.Message{}
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, total, nil
}

func (s *pmService) Send(ctx context.Context, userID, conversationID, body string) (*model.Message, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "消息内容不能为空")
	}
	if conversationID == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "会话无效")
	}
	if err := s.requireMember(ctx, conversationID, userID); err != nil {
		return nil, err
	}

	// ensure conversation exists
	if _, err := s.repo.FindByID(ctx, conversationID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "会话不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	msg := model.NewMessage(conversationID, userID, body)
	if err := s.repo.CreateMessage(ctx, msg); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	// notify peer (best-effort)
	peerID, peerErr := s.findPeerID(ctx, userID, conversationID)
	if peerErr == nil && peerID != "" && peerID != userID {
		senderName := userID
		if sender, err := s.userRepo.FindByID(ctx, userID); err == nil {
			if name := displayNameOf(sender); name != "" {
				senderName = name
			}
		}
		content := senderName + " 给你发来私信"
		if err := s.notif.Create(ctx, peerID, "pm", content, conversationID, userID); err != nil {
			logrus.Warnf("pm: create notification failed: %v", err)
		}
	}

	return msg, nil
}

func (s *pmService) MarkRead(ctx context.Context, userID, conversationID string) error {
	if err := s.requireMember(ctx, conversationID, userID); err != nil {
		return err
	}
	if err := s.repo.MarkRead(ctx, conversationID, userID, time.Now()); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *pmService) requireMember(ctx context.Context, conversationID, userID string) error {
	ok, err := s.repo.IsMember(ctx, conversationID, userID)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if !ok {
		return pkg.NewAppError(pkg.FORBIDDEN, "非会话成员")
	}
	return nil
}

// findPeerID locates the other participant of a direct conversation for the given user.
// Paginates ListForUser so large inboxes still resolve correctly.
func (s *pmService) findPeerID(ctx context.Context, userID, conversationID string) (string, error) {
	const pageSize = 50
	page := 1
	for {
		items, total, err := s.repo.ListForUser(ctx, userID, (page-1)*pageSize, pageSize)
		if err != nil {
			return "", err
		}
		for _, it := range items {
			if it.ID == conversationID {
				return it.PeerID, nil
			}
		}
		if page*pageSize >= total || len(items) == 0 {
			break
		}
		page++
	}
	return "", repository.ErrNotFound
}
