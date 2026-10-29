package service

import (
	"context"
	"errors"

	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
	"time"
)

type revisionService struct {
	repo        repository.RevisionRepository
	postRepo    repository.PostRepository
	contentRepo repository.ContentRepository
	txMgr       db.TransactionManager
	perms       PermissionChecker
}

func NewRevisionService(repo repository.RevisionRepository, postRepo repository.PostRepository, contentRepo repository.ContentRepository, txMgr db.TransactionManager, perms PermissionChecker) RevisionService {
	return &revisionService{repo: repo, postRepo: postRepo, contentRepo: contentRepo, txMgr: txMgr, perms: perms}
}

func (s *revisionService) List(ctx context.Context, targetType, targetID string, page, pageSize int) ([]model.Revision, int, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	return s.repo.ListByTarget(ctx, targetType, targetID, (page-1)*pageSize, pageSize)
}

func (s *revisionService) RestorePost(ctx context.Context, revisionID, actorID, actorRole string) (*model.Post, error) {
	var result *model.Post
	err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		rev, err := s.repo.GetByID(txCtx, revisionID)
		if err != nil {
			return err
		}
		if rev.TargetType != "post" {
			return pkg.NewAppError(pkg.INVALID_PARAMS, "revision 不是帖子版本")
		}
		post, err := s.postRepo.FindByID(txCtx, rev.TargetID)
		if err != nil {
			return err
		}
		if post.AuthorID != actorID && (s.perms == nil || !s.perms.HasPermissionInSpace(actorRole, post.SpaceID, model.PermPostUpdate)) {
			return pkg.NewAppError(pkg.FORBIDDEN, "无权恢复该帖子版本")
		}
		post.Title = rev.Title
		post.ContentDigest = model.DigestBody(rev.Body, 200)
		post.UpdatedAt = time.Now()
		if err := s.postRepo.Update(txCtx, post); err != nil {
			return err
		}
		content, err := s.contentRepo.FindByID(txCtx, post.ContentID)
		if err != nil {
			return err
		}
		content.Body = rev.Body
		content.UpdatedAt = time.Now()
		if err := s.contentRepo.Update(txCtx, content); err != nil {
			return err
		}
		result = post
		return nil
	})
	return result, err
}

func (s *revisionService) RestoreComment(ctx context.Context, revisionID, actorID, actorRole string) (*model.Content, error) {
	var result *model.Content
	err := s.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		rev, err := s.repo.GetByID(txCtx, revisionID)
		if err != nil {
			return err
		}
		if rev.TargetType != "comment" {
			return pkg.NewAppError(pkg.INVALID_PARAMS, "revision 不是评论版本")
		}
		content, err := s.contentRepo.FindByID(txCtx, rev.TargetID)
		if err != nil {
			return err
		}
		if content.AuthorID != actorID && (s.perms == nil || !s.perms.HasPermission(actorRole, model.PermCommentUpdate)) {
			return pkg.NewAppError(pkg.FORBIDDEN, "无权恢复该评论版本")
		}
		content.Body = rev.Body
		content.UpdatedAt = time.Now()
		if err := s.contentRepo.Update(txCtx, content); err != nil {
			return err
		}
		result = content
		return nil
	})
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "revision 不存在")
		}
		return nil, err
	}
	return result, nil
}
