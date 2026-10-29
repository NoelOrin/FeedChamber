package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/pkg/eventbus"
	"orinforum/internal/repository"

	"github.com/sirupsen/logrus"
)

// 暗区系统版面 slug（seed 幂等创建，见 server/seed_spaces.go）。
const DarkZoneSlug = "dark-zone"

// motionVoteWeight 把等级映射成动议票权：TL0–1 → 1 票 / TL2–3 → 2 票 / TL4+ → 3 票。
func motionVoteWeight(level int) int {
	switch {
	case level >= 4:
		return 3
	case level >= 2:
		return 2
	default:
		return 1
	}
}

type motionService struct {
	repo        repository.MotionRepository
	postRepo    repository.PostRepository
	contentRepo repository.ContentRepository
	userRepo    repository.UserRepository
	spaceRepo   repository.SpaceRepository
	reportSvc   ReportService
	permSvc     PermissionService
	banSvc      BannedService
	bus         *eventbus.EventBus
	cfg         MotionConfig
}

func NewMotionService(
	repo repository.MotionRepository,
	postRepo repository.PostRepository,
	contentRepo repository.ContentRepository,
	userRepo repository.UserRepository,
	spaceRepo repository.SpaceRepository,
	reportSvc ReportService,
	permSvc PermissionService,
	banSvc BannedService,
	bus *eventbus.EventBus,
) MotionService {
	return &motionService{
		repo:        repo,
		postRepo:    postRepo,
		contentRepo: contentRepo,
		userRepo:    userRepo,
		spaceRepo:   spaceRepo,
		reportSvc:   reportSvc,
		permSvc:     permSvc,
		banSvc:      banSvc,
		bus:         bus,
		cfg:         DefaultMotionConfig(),
	}
}

// ── Create ──────────────────────────────────────────────────────────────────

func (s *motionService) Create(ctx context.Context, initiatorID, targetType, targetID, motionType, reason, sourceMotionID string) (*model.Motion, error) {
	if targetType != model.MotionTargetPost && targetType != model.MotionTargetComment && targetType != model.MotionTargetUser {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "targetType 必须为 post/comment/user")
	}
	switch motionType {
	case model.MotionTypeFold, model.MotionTypeWatch, model.MotionTypeEscalate, model.MotionTypeExile, model.MotionTypeReverse:
	default:
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "不支持的动议类型")
	}
	// 反猎巫：受限用户不得发起动议
	if u, err := s.userRepo.FindByID(ctx, initiatorID); err == nil && u != nil {
		if u.MotionBannedUntil != nil && u.MotionBannedUntil.After(time.Now()) {
			return nil, pkg.NewAppError(pkg.FORBIDDEN, "您因多次发起被判定为滥诉的动议，暂被限制发起资格")
		}
	}
	// 同一目标不重复发起（pending 去重，避免刷屏）
	if existing, err := s.repo.FindPendingByTarget(ctx, targetType, targetID); err == nil && existing != nil && existing.Type == motionType {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "该目标已有进行中的同类动议")
	}

	m := &model.Motion{
		ID:             pkg.NewID(),
		TargetType:     targetType,
		TargetID:       targetID,
		Type:           motionType,
		Reason:         reason,
		Status:         model.MotionStatusPending,
		InitiatorID:    initiatorID,
		SourceMotionID: sourceMotionID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.publish(ctx, eventbus.EventMotionCreated, map[string]any{
		"motion_id": m.ID, "initiator_id": initiatorID, "target_type": targetType, "target_id": targetID, "type": motionType,
	})
	return m, nil
}

// ── Vote ────────────────────────────────────────────────────────────────────

func (s *motionService) Vote(ctx context.Context, motionID, voterID, side string) (*model.Motion, error) {
	if side != model.MotionVoteApprove && side != model.MotionVoteReject {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "side 必须为 approve 或 reject")
	}
	m, err := s.repo.FindByID(ctx, motionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "动议不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if m.Status != model.MotionStatusPending {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "该动议已结案，无法继续投票")
	}
	if _, err := s.repo.FindVote(ctx, motionID, voterID); err == nil {
		return m, pkg.NewAppError(pkg.INVALID_PARAMS, "您已对该动议投票")
	}

	weight := 1.0
	if u, err := s.userRepo.FindByID(ctx, voterID); err == nil && u != nil {
		weight = float64(motionVoteWeight(u.TrustLevel))
	}

	v := &model.MotionVote{
		ID:        pkg.NewID(),
		MotionID:  motionID,
		VoterID:   voterID,
		Side:      side,
		Weight:    weight,
		CreatedAt: time.Now(),
	}
	if err := s.repo.AddVote(ctx, v); err != nil {
		// 唯一约束冲突 = 并发重复投票
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "投票失败（可能已投过票）")
	}
	var dApprove, dReject float64
	if side == model.MotionVoteApprove {
		dApprove = weight
	} else {
		dReject = weight
	}
	if err := s.repo.IncrementTally(ctx, motionID, dApprove, dReject, 1); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	// 重新读取最新票数，达法定人数即结算
	latest, err := s.repo.FindByID(ctx, motionID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if latest.ApproveWeight+latest.RejectWeight >= s.quorumFor(latest) {
		return s.resolve(ctx, latest, latest.InitiatorID)
	}
	return latest, nil
}

// quorumFor 版面法定人数：user 类动议更高（默认 ×2）。
func (s *motionService) quorumFor(m *model.Motion) float64 {
	q := s.cfg.Quorum
	if m.TargetType == model.MotionTargetUser {
		q *= s.cfg.UserQuorumFactor
	}
	return q
}

// ── List / Get ──────────────────────────────────────────────────────────────

func (s *motionService) List(ctx context.Context, status, targetType string, page, pageSize int) ([]model.MotionListItem, int64, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	items, total, err := s.repo.List(ctx, status, targetType, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	out := make([]model.MotionListItem, 0, len(items))
	for i := range items {
		out = append(out, items[i].ToListItem())
	}
	return out, total, nil
}

func (s *motionService) GetByID(ctx context.Context, id string) (*model.Motion, error) {
	m, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "动议不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return m, nil
}

// ── Resolve ─────────────────────────────────────────────────────────────────

func (s *motionService) Resolve(ctx context.Context, id, actorID string) (*model.Motion, error) {
	m, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if m.Status != model.MotionStatusPending {
		return m, pkg.NewAppError(pkg.INVALID_PARAMS, "该动议已结案")
	}
	return s.resolve(ctx, m, actorID)
}

// resolve 结算并执行结果。只有达法定人数且赞成 > 反对才 passed。
func (s *motionService) resolve(ctx context.Context, m *model.Motion, actorID string) (*model.Motion, error) {
	passed := m.ApproveWeight > m.RejectWeight && (m.ApproveWeight+m.RejectWeight) >= s.quorumFor(m)
	now := time.Now()
	status := model.MotionStatusRejected
	if passed {
		status = model.MotionStatusPassed
	}
	if err := s.repo.UpdateStatus(ctx, m.ID, status, &now); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	m.Status = status
	m.ResolvedAt = &now

	if passed {
		if err := s.execute(ctx, m, actorID); err != nil {
			return nil, err
		}
	}
	s.publish(ctx, eventbus.EventMotionResolved, map[string]any{
		"motion_id": m.ID, "status": status, "type": m.Type, "initiator_id": m.InitiatorID,
		"target_type": m.TargetType, "target_id": m.TargetID,
	})
	return m, nil
}

// execute 执行 passed 动议的结果（全部为柔性措施）。
func (s *motionService) execute(ctx context.Context, m *model.Motion, actorID string) error {
	switch m.Type {
	case model.MotionTypeFold:
		return s.applyFold(ctx, m)
	case model.MotionTypeWatch:
		return s.applyWatch(ctx, m)
	case model.MotionTypeEscalate:
		// 移交版主：生成一条 Report，进入既有 report:manage 队列（不改 Report 行为）
		if s.reportSvc == nil {
			return nil
		}
		flag := "community_escalation"
		reason := fmt.Sprintf("社区动议移交：%s", m.Reason)
		_, err := s.reportSvc.Create(ctx, m.InitiatorID, m.TargetType, m.TargetID, flag, reason)
		return err
	case model.MotionTypeExile:
		return s.applyExile(ctx, m)
	case model.MotionTypeReverse:
		return s.applyReverse(ctx, m, actorID)
	}
	return nil
}

func (s *motionService) applyFold(ctx context.Context, m *model.Motion) error {
	switch m.TargetType {
	case model.MotionTargetPost:
		p, err := s.postRepo.FindByID(ctx, m.TargetID)
		if err != nil {
			return nil
		}
		p.Folded = true
		return s.postRepo.Update(ctx, p)
	case model.MotionTargetComment:
		c, err := s.contentRepo.FindByID(ctx, m.TargetID)
		if err != nil {
			return nil
		}
		c.Folded = true
		return s.contentRepo.Update(ctx, c)
	}
	return nil
}

func (s *motionService) applyWatch(ctx context.Context, m *model.Motion) error {
	until := time.Now().Add(7 * 24 * time.Hour)
	switch m.TargetType {
	case model.MotionTargetPost:
		p, err := s.postRepo.FindByID(ctx, m.TargetID)
		if err != nil {
			return nil
		}
		p.WatchedUntil = &until
		return s.postRepo.Update(ctx, p)
	case model.MotionTargetComment:
		c, err := s.contentRepo.FindByID(ctx, m.TargetID)
		if err != nil {
			return nil
		}
		c.WatchedUntil = &until
		return s.contentRepo.Update(ctx, c)
	}
	return nil
}

// applyExile 暗区软隔离：帖子路由到 dark-zone + 置 exiled；用户置 exiled_until。
// 不封号、不删内容，可翻案。
func (s *motionService) applyExile(ctx context.Context, m *model.Motion) error {
	switch m.TargetType {
	case model.MotionTargetPost:
		dz, err := s.darkZone(ctx)
		if err != nil {
			return err
		}
		p, err := s.postRepo.FindByID(ctx, m.TargetID)
		if err != nil {
			return nil
		}
		p.SpaceID = dz.ID
		p.Exiled = true
		return s.postRepo.Update(ctx, p)
	case model.MotionTargetComment:
		c, err := s.contentRepo.FindByID(ctx, m.TargetID)
		if err != nil {
			return nil
		}
		c.Folded = true
		return s.contentRepo.Update(ctx, c)
	case model.MotionTargetUser:
		u, err := s.userRepo.FindByID(ctx, m.TargetID)
		if err != nil {
			return nil
		}
		// 空 = 永久软隔离（仍可登录浏览，不等于封号）
		u.ExiledUntil = m.ExiledUntil
		return s.userRepo.Update(ctx, u)
	}
	return nil
}

// applyReverse 翻案：解除被翻案动议施加的软隔离。
func (s *motionService) applyReverse(ctx context.Context, m *model.Motion, actorID string) error {
	if m.SourceMotionID == "" {
		return nil
	}
	src, err := s.repo.FindByID(ctx, m.SourceMotionID)
	if err != nil {
		return nil
	}
	return s.lift(ctx, src)
}

// lift 解除一条动议施加的柔性措施。
func (s *motionService) lift(ctx context.Context, m *model.Motion) error {
	switch m.Type {
	case model.MotionTypeFold:
		switch m.TargetType {
		case model.MotionTargetPost:
			if p, err := s.postRepo.FindByID(ctx, m.TargetID); err == nil {
				p.Folded = false
				_ = s.postRepo.Update(ctx, p)
			}
		case model.MotionTargetComment:
			if c, err := s.contentRepo.FindByID(ctx, m.TargetID); err == nil {
				c.Folded = false
				_ = s.contentRepo.Update(ctx, c)
			}
		}
	case model.MotionTypeExile:
		switch m.TargetType {
		case model.MotionTargetPost:
			p, err := s.postRepo.FindByID(ctx, m.TargetID)
			if err != nil {
				logrus.Warnf("motion lift: post %s not found: %v", m.TargetID, err)
				return nil
			}
			p.Exiled = false
			if err := s.postRepo.Update(ctx, p); err != nil {
				return pkg.NewAppError(pkg.INTERNAL_ERROR, "解除软隔离失败: "+err.Error())
			}
		case model.MotionTargetComment:
			c, err := s.contentRepo.FindByID(ctx, m.TargetID)
			if err != nil {
				logrus.Warnf("motion lift: content %s not found: %v", m.TargetID, err)
				return nil
			}
			c.Folded = false
			if err := s.contentRepo.Update(ctx, c); err != nil {
				return pkg.NewAppError(pkg.INTERNAL_ERROR, "解除折叠失败: "+err.Error())
			}
		case model.MotionTargetUser:
			u, err := s.userRepo.FindByID(ctx, m.TargetID)
			if err != nil {
				logrus.Warnf("motion lift: user %s not found: %v", m.TargetID, err)
				return nil
			}
			u.ExiledUntil = nil
			if err := s.userRepo.Update(ctx, u); err != nil {
				return pkg.NewAppError(pkg.INTERNAL_ERROR, "解除用户软隔离失败: "+err.Error())
			}
		}
	}
	return nil
}

// ── UpdateStatus (motion:manage) ────────────────────────────────────────────

func (s *motionService) UpdateStatus(ctx context.Context, id, actorID, status string, abuse *bool, hardBan bool) (*model.Motion, error) {
	m, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 标记滥诉：rejected 的动议可被标记为恶意；达阈值限制发起者
	if abuse != nil && *abuse {
		if err := s.repo.MarkAbuse(ctx, m.ID, true); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		m.AbuseFlag = true
		if u, err := s.userRepo.FindByID(ctx, m.InitiatorID); err == nil && u != nil {
			u.AbuseCount++
			if u.AbuseCount >= s.cfg.AbuseThreshold {
				until := time.Now().Add(time.Duration(s.cfg.MotionBanDays) * 24 * time.Hour)
				u.MotionBannedUntil = &until
			}
			_ = s.userRepo.Update(ctx, u)
		}
	}

	// 翻案：解除软隔离
	if status == model.MotionStatusOverturned {
		now := time.Now()
		if err := s.repo.UpdateStatus(ctx, m.ID, model.MotionStatusOverturned, &now); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		m.Status = model.MotionStatusOverturned
		m.ResolvedAt = &now
		if err := s.lift(ctx, m); err != nil {
			return nil, err
		}
	}

	// 升级硬封禁/封号：人工裁量，显式传参才执行（不由状态机自动触发）
	if hardBan && m.TargetType == model.MotionTargetUser && s.banSvc != nil {
		if _, err := s.banSvc.Ban(ctx, m.TargetID, "社区动议升级为硬封禁", actorID, true, nil); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}

	return m, nil
}

// ── Delete ──────────────────────────────────────────────────────────────────

func (s *motionService) Delete(ctx context.Context, id, actorID string) error {
	m, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if m.InitiatorID != actorID {
		return pkg.NewAppError(pkg.FORBIDDEN, "只有发起者可以撤回动议")
	}
	if m.Status != model.MotionStatusPending {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "只能撤回进行中的动议")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// ── helpers ─────────────────────────────────────────────────────────────────

func (s *motionService) darkZone(ctx context.Context) (*model.Space, error) {
	sp, err := s.spaceRepo.FindBySlug(ctx, DarkZoneSlug)
	if err != nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "暗区版面尚未初始化")
	}
	return sp, nil
}

// StartResolveTicker 后台周期结算已过开放期的 pending 动议（design D1「开放期满即结算」）。
// 结算动作与投票触发走同一条 resolve 路径，因此票数门槛、事件与结果执行完全一致。
func (s *motionService) StartResolveTicker(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.resolveDue(context.Background())
			}
		}
	}()
	return cancel
}

// resolveDue 扫一批已过开放期的 pending 动议并逐个结算。
func (s *motionService) resolveDue(ctx context.Context) {
	before := time.Now().Add(-s.cfg.OpenDuration)
	items, err := s.repo.ListExpiredPending(ctx, before, 100)
	if err != nil {
		logrus.Warnf("motion resolve sweep: %v", err)
		return
	}
	for i := range items {
		m := &items[i]
		if _, err := s.resolve(ctx, m, m.InitiatorID); err != nil {
			logrus.Warnf("motion resolve sweep: settle %s: %v", m.ID, err)
			continue
		}
		logrus.Infof("motion resolve sweep: settled %s -> %s", m.ID, m.Status)
	}
}

func (s *motionService) publish(ctx context.Context, typ string, payload map[string]any) {
	if s.bus == nil {
		return
	}
	s.bus.Publish(eventbus.Event{Type: typ, Ctx: ctx, Payload: payload})
}
