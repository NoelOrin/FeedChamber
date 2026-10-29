package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"feedchamber/internal/pkg/eventbus"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/repository"
)

// moderationExternalTimeout 是发布前调用外部自动化审核服务的硬超时：
// 外部服务慢不能把发帖请求一起拖死，超时按「不可用」处理并回退异步规则。
const moderationExternalTimeout = 3 * time.Second

type moderationService struct {
	repo        repository.ModerationRepository
	postRepo    repository.PostRepository
	contentRepo repository.ContentRepository
	settingSvc  SettingService
	bus         *eventbus.EventBus
	trustSvc    TrustService
}

// SetTrustService 注入等级评估服务（审核结论会改变内容的 approved 状态）。
func (s *moderationService) SetTrustService(svc TrustService) { s.trustSvc = svc }

func NewModerationService(repo repository.ModerationRepository, postRepo repository.PostRepository, contentRepo repository.ContentRepository, settingSvc SettingService, bus *eventbus.EventBus) ModerationService {
	return &moderationService{repo: repo, postRepo: postRepo, contentRepo: contentRepo, settingSvc: settingSvc, bus: bus}
}

// Mode 返回当前文章审核模式。设置缺失/非法时由 ParseModerationMode 回落默认值。
func (s *moderationService) Mode(ctx context.Context) string {
	if s.settingSvc == nil {
		return model.DefaultModerationMode
	}
	raw, err := s.settingSvc.Get(ctx, model.SettingModerationMode)
	if err != nil {
		return model.DefaultModerationMode
	}
	return model.ParseModerationMode(raw)
}

func (s *moderationService) ListPending(ctx context.Context, page, pageSize int) ([]model.ContentModeration, int64, error) {
	offset := (page - 1) * pageSize
	return s.repo.ListModerations(ctx, model.ModerationPending, offset, pageSize)
}

func (s *moderationService) Approve(ctx context.Context, id, reviewerID string) error {
	m, err := s.repo.GetModeration(ctx, id)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "moderation record not found")
	}
	if err := s.repo.UpdateModerationStatus(ctx, id, model.ModerationApproved, reviewerID); err != nil {
		return err
	}
	if err := s.applyTargetStatus(ctx, m.TargetType, m.TargetID, model.PostStatusApproved); err != nil {
		return err
	}
	s.recomputeAuthor(ctx, m.TargetType, m.TargetID)
	if s.bus != nil {
		s.bus.Publish(eventbus.Event{Type: eventbus.EventModerationDone, Payload: map[string]any{"moderation_id": id, "decision": "approved", "target_type": m.TargetType, "target_id": m.TargetID}})
	}
	return nil
}

func (s *moderationService) Reject(ctx context.Context, id, reviewerID, reason string) error {
	m, err := s.repo.GetModeration(ctx, id)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "moderation record not found")
	}
	if err := s.repo.UpdateModerationStatus(ctx, id, model.ModerationRejected, reviewerID); err != nil {
		return err
	}
	if err := s.applyTargetStatus(ctx, m.TargetType, m.TargetID, model.PostStatusRejected); err != nil {
		return err
	}
	s.recomputeAuthor(ctx, m.TargetType, m.TargetID)
	if s.bus != nil {
		s.bus.Publish(eventbus.Event{Type: eventbus.EventModerationDone, Payload: map[string]any{"moderation_id": id, "decision": "rejected", "target_type": m.TargetType, "target_id": m.TargetID}})
	}
	return nil
}

func (s *moderationService) RecordPending(ctx context.Context, targetType, targetID string, scene model.ModerationScene, action model.ModerationAction, body string) error {
	return s.RecordPendingFrom(ctx, targetType, targetID, scene, action, body, "auto")
}

// RecordPendingFrom 与 RecordPending 相同，但额外标注判定来源（async / external /
// manual / tl0 / auto）。审核队列靠这一列区分「谁判的」，人工复核时先看外部的结论。
func (s *moderationService) RecordPendingFrom(ctx context.Context, targetType, targetID string, scene model.ModerationScene, action model.ModerationAction, body, strategy string) error {
	if s.repo == nil {
		return nil
	}
	if strategy == "" {
		strategy = "auto"
	}
	now := time.Now()
	m := &model.ContentModeration{
		TargetType:   targetType,
		TargetID:     targetID,
		Scene:        scene,
		Status:       model.ModerationPending,
		Action:       action,
		Strategy:     strategy,
		OriginalBody: body,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return s.repo.CreateModeration(ctx, m)
}

// recomputeAuthor 在审核结论落库后重算内容作者的 XP / 等级：只有 approved 的
// 内容计 XP，通过或拒绝都会改变聚合结果。
func (s *moderationService) recomputeAuthor(ctx context.Context, targetType, targetID string) {
	if s.trustSvc == nil {
		return
	}
	authorID := ""
	switch targetType {
	case "post":
		if s.postRepo != nil {
			if p, err := s.postRepo.FindByID(ctx, targetID); err == nil && p != nil {
				authorID = p.AuthorID
			}
		}
	case "content":
		if s.contentRepo != nil {
			if c, err := s.contentRepo.FindByID(ctx, targetID); err == nil && c != nil {
				authorID = c.AuthorID
			}
		}
	}
	if authorID == "" {
		return
	}
	if _, err := s.trustSvc.Recompute(ctx, authorID); err != nil {
		logrus.Warnf("trust: recompute %s after moderation: %v", authorID, err)
	}
}

func (s *moderationService) applyTargetStatus(ctx context.Context, targetType, targetID, status string) error {
	switch targetType {
	case "post":
		if s.postRepo == nil {
			return nil
		}
		post, err := s.postRepo.FindByID(ctx, targetID)
		if err != nil {
			return err
		}
		post.Status = status
		post.UpdatedAt = time.Now()
		return s.postRepo.Update(ctx, post)
	case "content":
		if s.contentRepo == nil {
			return nil
		}
		c, err := s.contentRepo.FindByID(ctx, targetID)
		if err != nil {
			return err
		}
		c.Status = status
		c.UpdatedAt = time.Now()
		return s.contentRepo.Update(ctx, c)
	default:
		return nil
	}
}

// externalVerdict 是外部自动化审核服务的响应契约（JSON）。
type externalVerdict struct {
	Action string `json:"action"` // allow | flag | block | replace
	Reason string `json:"reason"`
}

// errExternalModerationUnavailable 表示外部审核服务未配置、超时或返回了无法
// 理解的结论。调用方据此回退「发布后异步审核」，绝不因此阻塞发帖。
var errExternalModerationUnavailable = errors.New("moderation: external service unavailable")

// CheckExternal 在发布前同步调用外部自动化审核服务（moderation_mode=external）。
//
// 请求：POST <moderation_ai_endpoint>，JSON {target_type,target_id,scene,title,body}，
// 配置了 moderation_ai_api_key 时带 Authorization: Bearer <key>。
// 响应：200 + {"action":"allow|flag|block|replace","reason":"..."}。
// 任何超时 / 非 2xx / 解析失败都返回 errExternalModerationUnavailable。
func (s *moderationService) CheckExternal(ctx context.Context, targetType, targetID, title, body string, scene model.ModerationScene) (model.ModerationAction, error) {
	configs, err := s.GetConfigs(ctx)
	if err != nil {
		return "", errExternalModerationUnavailable
	}
	endpoint := strings.TrimSpace(configs[model.ModerationConfigAIEndpoint])
	provider := strings.TrimSpace(configs[model.ModerationConfigAIProvider])
	if endpoint == "" || provider == "" || provider == "none" {
		return "", errExternalModerationUnavailable
	}

	payload, err := json.Marshal(map[string]string{
		"target_type": targetType,
		"target_id":   targetID,
		"scene":       string(scene),
		"title":       title,
		"body":        body,
	})
	if err != nil {
		return "", errExternalModerationUnavailable
	}

	reqCtx, cancel := context.WithTimeout(ctx, moderationExternalTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", errExternalModerationUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	if key := strings.TrimSpace(configs[model.ModerationConfigAIAPIKey]); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logrus.Warnf("moderation: external check %s: %v", endpoint, err)
		return "", errExternalModerationUnavailable
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logrus.Warnf("moderation: external check %s: status=%d err=%v", endpoint, resp.StatusCode, err)
		return "", errExternalModerationUnavailable
	}

	var verdict externalVerdict
	if err := json.Unmarshal(raw, &verdict); err != nil {
		logrus.Warnf("moderation: external check %s: bad payload: %v", endpoint, err)
		return "", errExternalModerationUnavailable
	}
	switch model.ModerationAction(strings.ToLower(strings.TrimSpace(verdict.Action))) {
	case model.ModerationActionAllow:
		return model.ModerationActionAllow, nil
	case model.ModerationActionFlag, model.ModerationActionBlock, model.ModerationActionReplace:
		return model.ModerationAction(strings.ToLower(strings.TrimSpace(verdict.Action))), nil
	default:
		// 外部服务给了没见过的结论：按不可用处理，交给异步规则兜底。
		logrus.Warnf("moderation: external check %s: unknown action %q", endpoint, verdict.Action)
		return "", errExternalModerationUnavailable
	}
}

// SubscribeAsync 注册「发布后异步审核」消费者（由 container.Runtime 在启动时调用）。
func (s *moderationService) SubscribeAsync(bus *eventbus.EventBus) {
	if bus == nil {
		return
	}
	bus.Subscribe(eventbus.EventPostCreated, s.handlePostCreated)
}

// handlePostCreated 是异步审核的唯一入口：只处理发帖时显式标了 moderation_async
// 的帖子，且帖子此刻仍是公开状态——已被人工或 TL0 门置为待审的不重复建单。
func (s *moderationService) handlePostCreated(event eventbus.Event) {
	if event.Payload["moderation_async"] != "true" {
		return
	}
	postID, _ := event.Payload["post_id"].(string)
	if postID == "" || s.postRepo == nil {
		return
	}
	ctx := event.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	mode := s.Mode(ctx)
	if mode == model.ModerationModeOff || mode == model.ModerationModeManual {
		return
	}
	post, err := s.postRepo.FindByID(ctx, postID)
	if err != nil || post == nil {
		logrus.Warnf("moderation: async check post %s: %v", postID, err)
		return
	}
	if post.Status != model.PostStatusApproved {
		return
	}
	body := ""
	if s.contentRepo != nil && post.ContentID != "" {
		if c, err := s.contentRepo.FindByID(ctx, post.ContentID); err == nil && c != nil {
			body = c.Body
		}
	}
	role, _ := event.Payload["author_role"].(string)
	action, err := s.CheckContent(ctx, "post", post.ID, body, model.ModerationScenePost, role)
	if err != nil || action == model.ModerationActionAllow {
		return
	}
	// 命中即转待审交人工判断：异步审核不直接删帖/拒稿，只负责把内容挪进队列。
	post.Status = model.PostStatusPending
	post.UpdatedAt = time.Now()
	if err := s.postRepo.Update(ctx, post); err != nil {
		logrus.Warnf("moderation: async check post %s: update status: %v", postID, err)
		return
	}
	if err := s.RecordPendingFrom(ctx, "post", post.ID, model.ModerationScenePost, action, body, "async"); err != nil {
		logrus.Warnf("moderation: async check post %s: record: %v", postID, err)
	}
	s.recomputeAuthor(ctx, "post", post.ID)
}

func (s *moderationService) CreateRule(ctx context.Context, rule *model.ModerationRule) error {
	if msg := validateModerationRule(rule); msg != "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, msg)
	}
	// Ensure server-generated ID; ignore client-supplied value
	rule.ID = ""
	return s.repo.CreateRule(ctx, rule)
}

func (s *moderationService) UpdateRule(ctx context.Context, rule *model.ModerationRule) error {
	if msg := validateModerationRuleForUpdate(rule); msg != "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, msg)
	}
	return s.repo.UpdateRule(ctx, rule)
}

var validModerationScenes = map[model.ModerationScene]bool{
	model.ModerationScenePost:    true,
	model.ModerationSceneComment: true,
	model.ModerationSceneEdit:    true,
}

var validModerationActions = map[model.ModerationAction]bool{
	model.ModerationActionAllow:   true,
	model.ModerationActionBlock:   true,
	model.ModerationActionReplace: true,
	model.ModerationActionFlag:    true,
}

func validateModerationRule(rule *model.ModerationRule) string {
	if rule.Name == "" {
		return "规则名称不能为空"
	}
	if !validModerationScenes[rule.Scene] {
		return "无效的审核场景"
	}
	if rule.Type == "" {
		return "规则类型不能为空"
	}
	if rule.Pattern == "" {
		return "规则模式不能为空"
	}
	if !validModerationActions[rule.Action] {
		return "无效的审核动作"
	}
	return ""
}

// validateModerationRuleForUpdate validates only non-zero fields for partial updates.
// Empty fields are allowed (they won't overwrite existing DB values via GORM zero-check).
func validateModerationRuleForUpdate(rule *model.ModerationRule) string {
	if rule.Name != "" && len(rule.Name) > 100 {
		return "规则名称过长"
	}
	if rule.Scene != "" && !validModerationScenes[rule.Scene] {
		return "无效的审核场景"
	}
	if rule.Type != "" && len(rule.Type) > 50 {
		return "规则类型过长"
	}
	if rule.Pattern != "" && len(rule.Pattern) > 500 {
		return "规则模式过长"
	}
	if rule.Action != "" && !validModerationActions[rule.Action] {
		return "无效的审核动作"
	}
	return ""
}

func (s *moderationService) DeleteRule(ctx context.Context, id string) error {
	return s.repo.DeleteRule(ctx, id)
}

func (s *moderationService) ListRules(ctx context.Context, page, pageSize int) ([]model.ModerationRule, int64, error) {
	offset := (page - 1) * pageSize
	return s.repo.ListRules(ctx, offset, pageSize)
}

func (s *moderationService) GetConfigs(ctx context.Context) (map[string]string, error) {
	configs, err := s.repo.ListConfigs(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(configs))
	for _, c := range configs {
		result[c.Key] = c.Value
	}
	return result, nil
}

func (s *moderationService) SetConfig(ctx context.Context, key, value string) error {
	return s.repo.SetConfig(ctx, key, value)
}

// CheckContent runs moderation rules against content.
// Executes enabled keyword/regex rules for the scene; fails open on infra errors.
func (s *moderationService) CheckContent(ctx context.Context, targetType, targetID, body string, scene model.ModerationScene, authorRole string) (model.ModerationAction, error) {
	configs, err := s.GetConfigs(ctx)
	if err != nil {
		return model.ModerationActionAllow, nil
	}

	if configs[model.ModerationConfigEnabled] != "true" {
		return model.ModerationActionAllow, nil
	}

	// Scene-specific toggle
	sceneKey := map[model.ModerationScene]string{
		model.ModerationScenePost:    model.ModerationConfigPostEnabled,
		model.ModerationSceneComment: model.ModerationConfigCommentEnabled,
		model.ModerationSceneEdit:    model.ModerationConfigEditEnabled,
	}[scene]
	if sceneKey != "" && configs[sceneKey] != "true" {
		return model.ModerationActionAllow, nil
	}

	// Whitelist role auto-pass
	if whitelistRaw := configs[model.ModerationConfigWhitelistRoles]; whitelistRaw != "" {
		var whitelist []string
		if err := json.Unmarshal([]byte(whitelistRaw), &whitelist); err == nil {
			for _, r := range whitelist {
				if r == authorRole {
					return model.ModerationActionAllow, nil
				}
			}
		}
	}

	rules, err := s.repo.ListEnabledRules(ctx, scene)
	if err != nil {
		return model.ModerationActionAllow, nil
	}

	lowerBody := strings.ToLower(body)
	for _, rule := range rules {
		matched := false
		switch strings.ToLower(rule.Type) {
		case "keyword":
			if rule.Pattern != "" && strings.Contains(lowerBody, strings.ToLower(rule.Pattern)) {
				matched = true
			}
		case "regex":
			re, err := regexp.Compile(rule.Pattern)
			if err == nil && re.MatchString(body) {
				matched = true
			}
		default:
			// unknown rule type: skip
		}
		if !matched {
			continue
		}
		switch rule.Action {
		case model.ModerationActionAllow:
			return model.ModerationActionAllow, nil
		case model.ModerationActionReplace:
			// Phase3: simple '*' replacement of keyword matches; regex replace uses full match stars.
			return model.ModerationActionReplace, nil
		case model.ModerationActionBlock:
			return model.ModerationActionBlock, nil
		case model.ModerationActionFlag:
			return model.ModerationActionFlag, nil
		default:
			return model.ModerationActionFlag, nil
		}
	}

	// No rule hit: allow (do not force pending by default once rules engine is live).
	return model.ModerationActionAllow, nil
}
