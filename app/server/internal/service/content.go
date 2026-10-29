package service

import (
	"context"
	"errors"
	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/eventbus"
	"feedchamber/internal/repository"
	"time"

	"github.com/sirupsen/logrus"
)

type contentService struct {
	repo          repository.ContentRepository
	postRepo      repository.PostRepository
	userRepo      repository.UserRepository
	bus           *eventbus.EventBus
	banSvc        BannedService
	txMgr         db.TransactionManager
	perms         PermissionChecker
	moderationSvc ModerationService
	settingSvc    SettingService
	watchSvc      WatchService
	revisions     repository.RevisionRepository
	trustSvc      TrustService
	gateSvc       GateService
	postView      PostViewer
}

// ContentServiceDeps 是 NewContentService 的依赖清单。
type ContentServiceDeps struct {
	Contents  repository.ContentRepository
	Posts     repository.PostRepository
	Users     repository.UserRepository
	Revisions repository.RevisionRepository

	Ban        BannedService
	Moderation ModerationService
	Settings   SettingService
	Watch      WatchService
	Perms      PermissionChecker
	Trust      TrustService
	Gate       GateService
	// PostView 是帖子可见性判定（由 PostService 实现）。评论是从属内容，
	// 必须复用帖子的门槛判定，否则受限帖的评论会从评论出口泄露。
	PostView PostViewer

	Tx  db.TransactionManager
	Bus *eventbus.EventBus
}

func NewContentService(d ContentServiceDeps) ContentService {
	return &contentService{
		repo:          d.Contents,
		postRepo:      d.Posts,
		userRepo:      d.Users,
		revisions:     d.Revisions,
		banSvc:        d.Ban,
		moderationSvc: d.Moderation,
		settingSvc:    d.Settings,
		watchSvc:      d.Watch,
		perms:         d.Perms,
		txMgr:         d.Tx,
		bus:           d.Bus,
		trustSvc:      d.Trust,
		gateSvc:       d.Gate,
		postView:      d.PostView,
	}
}

func (s *contentService) ListByPost(ctx context.Context, postID string, page, pageSize int, startTime, endTime *time.Time) ([]model.ContentListItem, int, error) {
	post, err := s.postRepo.FindByID(ctx, postID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, 0, pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
		}
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if s.postView != nil {
		if err := s.postView.CheckViewablePost(ctx, post); err != nil {
			return nil, 0, err
		}
	}
	items, total, err := s.repo.ListByRootWithTotal(ctx, post.ContentID, (page-1)*pageSize, pageSize, startTime, endTime)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *contentService) ListReplies(ctx context.Context, parentID string, page, pageSize int, startTime, endTime *time.Time) ([]model.ContentListItem, int, error) {
	// 回复挂在评论下，评论挂在帖子下：先把帖子解析出来，复用同一可见性门槛。
	if s.postView != nil {
		if parent, err := s.repo.FindByID(ctx, parentID); err == nil && parent != nil && parent.RootID != "" {
			if post, perr := s.postRepo.FindByContentID(ctx, parent.RootID); perr == nil && post != nil {
				if err := s.postView.CheckViewablePost(ctx, post); err != nil {
					return nil, 0, err
				}
			}
		}
	}
	items, total, err := s.repo.ListByParentWithTotal(ctx, parentID, (page-1)*pageSize, pageSize, startTime, endTime)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *contentService) GetByID(ctx context.Context, id string) (*model.ContentListItem, error) {
	c, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "评论不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	item := &model.ContentListItem{
		ID:        c.ID,
		AuthorID:  c.AuthorID,
		Body:      c.Body,
		RootID:    c.RootID,
		ParentID:  c.ParentID,
		LikeCount: c.LikeCount,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}

	// Enrich with author info
	if author, err := s.userRepo.FindByID(ctx, c.AuthorID); err == nil {
		item.AuthorName = displayNameOf(author)
		item.AuthorAvatar = author.Avatar
	}

	// Enrich with post info
	if post, err := s.postRepo.FindByContentID(ctx, c.RootID); err == nil {
		item.PostID = post.ID
		item.PostTitle = post.Title
	}

	return item, nil
}

func (s *contentService) Create(ctx context.Context, postID, authorID, body, parentID string) (*model.Content, error) {
	if s.settingSvc != nil && s.userRepo != nil {
		required := pkg.GetSettingOrDefault(ctx, s.settingSvc, "email_verification_required", "EMAIL_VERIFICATION_REQUIRED", "false")
		if required == "true" {
			if u, err := s.userRepo.FindByID(ctx, authorID); err == nil && u != nil && !u.EmailVerified {
				return nil, pkg.NewAppError(pkg.FORBIDDEN, "请先验证邮箱后再评论")
			}
		}
	}
	var post *model.Post
	var c *model.Content
	gateReview := false
	err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		p, err := s.postRepo.FindByID(txCtx, postID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return pkg.NewAppError(pkg.NOT_FOUND, "帖子不存在")
			}
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		post = p

		// 帖子锁定/关闭检查（在事务内，避免 TOCTOU）
		if post.Locked {
			return pkg.NewAppError(pkg.FORBIDDEN, "帖子已锁定，无法评论")
		}
		if post.Status == model.PostStatusClosed {
			return pkg.NewAppError(pkg.FORBIDDEN, "话题已关闭，无法评论")
		}
		// 读写一致：看不见的帖子不接受评论（与详情/列表同一可见性判定）。
		// 作者例外与 report:manage 审核者由 CheckViewablePost 内部放行。
		if s.postView != nil {
			if err := s.postView.CheckViewablePost(txCtx, post); err != nil {
				return err
			}
		}

		// 版面禁言检查（在事务内）
		if post.SpaceID != "" && s.banSvc != nil {
			if s.banSvc.IsBannedInSpace(txCtx, authorID, post.SpaceID) {
				return pkg.NewAppError(pkg.USER_BANNED, "您在该版面已被禁言，无法评论")
			}
		}
		// NGA 版面级评论权限（层级继承）
		if post.SpaceID != "" && s.perms != nil && s.userRepo != nil {
			if u, err := s.userRepo.FindByID(txCtx, authorID); err == nil && u != nil {
				if !s.perms.HasPermissionInSpace(u.Role, post.SpaceID, model.PermCommentCreate) {
					return pkg.NewAppError(pkg.FORBIDDEN, "无权限在此版面评论")
				}
			}
		}

		// 门控规则引擎：评论能力域（require_review 转待审，deny 拒绝，rate_limit 限流）。
		if s.gateSvc != nil && s.userRepo != nil {
			if u, err := s.userRepo.FindByID(txCtx, authorID); err == nil && u != nil {
				in := gateInputForUser(u, post.SpaceID, body)
				if d, gerr := s.gateSvc.Evaluate(txCtx, model.GateDomainCommentCreate, in); gerr == nil {
					switch d.Action {
					case model.GateActionDeny:
						return pkg.NewAppError(pkg.FORBIDDEN, gateDenyMessage(d, "当前条件不允许评论"))
					case model.GateActionRequireReview:
						gateReview = true
					case model.GateActionRateLimit:
						if err := s.gateSvc.EnforceRateLimit(txCtx, d, in); err != nil {
							return err
						}
					}
				}
				// 外链是独立能力域：只在正文确实含链接时求值，否则域默认 deny 会误伤纯文本。
				if in.ContainsExternalLink {
					if d, gerr := s.gateSvc.Evaluate(txCtx, model.GateDomainExternalLink, in); gerr == nil {
						switch d.Action {
						case model.GateActionDeny:
							return pkg.NewAppError(pkg.FORBIDDEN, gateDenyMessage(d, "当前等级不允许发布外链"))
						case model.GateActionRequireReview:
							gateReview = true
						case model.GateActionRateLimit:
							if err := s.gateSvc.EnforceRateLimit(txCtx, d, in); err != nil {
								return err
							}
						}
					}
				}
			}
		}

		c = model.NewContent(authorID, body, post.ContentID, parentID)
		if gateReview {
			c.Status = model.PostStatusPending
		}
		if err := s.repo.Create(txCtx, c); err != nil {
			return err
		}
		if err := s.postRepo.IncrementCommentCount(txCtx, postID); err != nil {
			return err
		}
		return s.postRepo.TouchActive(txCtx, postID, time.Now())
	})
	if err != nil {
		return nil, err
	}

	// 门控 require_review：留痕进入人工队列（与审核模式判定同一条队列）。
	if gateReview && c != nil && s.moderationSvc != nil {
		if err := s.moderationSvc.RecordPendingFrom(ctx, "content", c.ID, model.ModerationSceneComment, model.ModerationActionFlag, body, "gate"); err != nil {
			logrus.Warnf("content: record gate review for %s: %v", c.ID, err)
		}
	}

	// Moderation check after persistence
	if s.moderationSvc != nil && c != nil {
		role := "user"
		if s.userRepo != nil {
			if u, err := s.userRepo.FindByID(ctx, authorID); err == nil && u != nil && u.Role != "" {
				role = u.Role
			}
		}
		action, mErr := s.moderationSvc.CheckContent(ctx, "content", c.ID, body, model.ModerationSceneComment, role)
		if mErr == nil {
			switch action {
			case model.ModerationActionBlock:
				c.Status = model.PostStatusPending
				_ = s.repo.Update(ctx, c)
				_ = s.moderationSvc.RecordPending(ctx, "content", c.ID, model.ModerationSceneComment, action, body)
				return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "含违规内容")
			case model.ModerationActionFlag:
				c.Status = model.PostStatusPending
				_ = s.repo.Update(ctx, c)
				_ = s.moderationSvc.RecordPending(ctx, "content", c.ID, model.ModerationSceneComment, action, body)
			case model.ModerationActionReplace:
				_ = s.moderationSvc.RecordPending(ctx, "content", c.ID, model.ModerationSceneComment, action, body)
			}
		}
	}

	// 发通知（通过事件总线）
	sender, ue := s.userRepo.FindByID(ctx, authorID)
	senderName := ""
	if ue == nil {
		senderName = displayNameOf(sender)
		if parentID != "" {
			parent, pe := s.repo.FindByID(ctx, parentID)
			if pe == nil && parent.AuthorID != authorID && s.bus != nil {
				s.bus.Publish(eventbus.Event{
					Type: eventbus.EventContentReplied,
					Ctx:  ctx,
					Payload: map[string]any{
						"user_id":     parent.AuthorID,
						"sender_id":   authorID,
						"sender_name": senderName,
						"post_id":     postID,
						"post_title":  post.Title,
						"is_reply":    true,
					},
				})
			}
		} else if post.AuthorID != authorID && s.bus != nil {
			s.bus.Publish(eventbus.Event{
				Type: eventbus.EventContentReplied,
				Ctx:  ctx,
				Payload: map[string]any{
					"user_id":     post.AuthorID,
					"sender_id":   authorID,
					"sender_name": senderName,
					"post_id":     postID,
					"post_title":  post.Title,
					"is_reply":    false,
				},
			})
		}
	}

	s.recomputeTrust(ctx, authorID)
	if s.bus != nil {
		s.bus.Publish(eventbus.Event{Type: eventbus.EventContentCreated, Ctx: ctx, Payload: map[string]any{"content_id": c.ID, "post_id": postID, "author_id": authorID}})
	}

	// @提及通知
	publishUserMentions(s.bus, s.userRepo, ctx, authorID, postID, body, senderName)

	// Watch 扇出通知
	if s.watchSvc != nil {
		s.watchSvc.NotifyWatchers(ctx, postID, authorID, senderName, post.Title)
	}

	return c, nil
}

func (s *contentService) Update(ctx context.Context, postID, id, body, actorUUID, actorRole string) (*model.Content, error) {
	var result *model.Content
	err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		c, err := s.repo.FindByID(txCtx, id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return pkg.NewAppError(pkg.NOT_FOUND, "评论不存在")
			}
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}

		post, err := s.postRepo.FindByID(txCtx, postID)
		if err != nil {
			return err
		}
		if c.RootID != post.ContentID {
			return pkg.NewAppError(pkg.NOT_FOUND, "该帖子下不存在此评论")
		}
		// R-06b: 作者 OR 有权限
		isAuthor := c.AuthorID == actorUUID
		hasPerm := s.perms != nil && s.perms.HasPermissionInSpace(actorRole, post.SpaceID, model.PermCommentUpdate)
		if !isAuthor && !hasPerm {
			return pkg.NewAppError(pkg.FORBIDDEN, "无权编辑该评论")
		}

		if s.revisions != nil {
			_ = s.revisions.Create(txCtx, &model.Revision{
				TargetType: "comment",
				TargetID:   c.ID,
				Title:      "",
				Body:       c.Body,
				AuthorID:   actorUUID,
				CreatedAt:  time.Now(),
			})
		}
		c.Body = body
		c.UpdatedAt = time.Now()
		if err := s.repo.Update(txCtx, c); err != nil {
			return err
		}
		result = c
		return nil
	})
	return result, err
}

func (s *contentService) Delete(ctx context.Context, postID, id, actorUUID, actorRole string) error {
	authorID := ""
	err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		c, err := s.repo.FindByID(txCtx, id)
		if err != nil {
			return err
		}
		// R-06b: 作者 OR 有权限（按帖子所属版块解析）
		post, err := s.postRepo.FindByID(txCtx, postID)
		spaceID := ""
		if err == nil {
			spaceID = post.SpaceID
		}
		// post 找不到（如软删除）时 spaceID 为空，HasPermissionInSpace 回退全局
		isAuthor := c.AuthorID == actorUUID
		hasPerm := s.perms != nil && s.perms.HasPermissionInSpace(actorRole, spaceID, model.PermCommentDelete)
		if !isAuthor && !hasPerm {
			return pkg.NewAppError(pkg.FORBIDDEN, "无权删除该评论")
		}
		authorID = c.AuthorID
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		return s.postRepo.DecrementCommentCount(txCtx, postID)
	})
	if err != nil {
		return err
	}
	s.recomputeTrust(ctx, authorID)
	return nil
}

func (s *contentService) ListAll(ctx context.Context, page, pageSize int) ([]model.ContentListItem, int, error) {
	items, total, err := s.repo.ListAllWithTotal(ctx, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *contentService) ListTrash(ctx context.Context, page, pageSize int) ([]model.ContentListItem, int, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	return s.repo.ListTrash(ctx, (page-1)*pageSize, pageSize)
}

func (s *contentService) AdminDelete(ctx context.Context, id, actorRole string) error {
	authorID := ""
	err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		c, err := s.repo.FindByID(txCtx, id)
		if err != nil {
			return err
		}
		authorID = c.AuthorID
		post, err := s.postRepo.FindByContentID(txCtx, c.RootID)
		if err != nil {
			// 帖子不存在时仍删除评论（spaceID 未知则回退全局，不阻止）
			return s.repo.Delete(txCtx, id)
		}
		// 按帖子所属版块检查空间权限
		if s.perms != nil && !s.perms.HasPermissionInSpace(actorRole, post.SpaceID, model.PermCommentDelete) {
			return pkg.NewAppError(pkg.FORBIDDEN, "无权删除该评论")
		}
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		}
		return s.postRepo.DecrementCommentCount(txCtx, post.ID)
	})
	if err != nil {
		return err
	}
	s.recomputeTrust(ctx, authorID)
	return nil
}

func (s *contentService) recomputeTrust(ctx context.Context, userID string) {
	if s.trustSvc == nil || userID == "" {
		return
	}
	if _, err := s.trustSvc.Recompute(ctx, userID); err != nil {
		logrus.Warnf("trust: recompute %s: %v", userID, err)
	}
}

func (s *contentService) RestoreTrash(ctx context.Context, id string) error {
	if id == "" {
		return pkg.NewAppError(pkg.NOT_FOUND, "评论 ID 不能为空")
	}
	return s.repo.Restore(ctx, id)
}
