package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/kvstore"
	"feedchamber/internal/pkg/logger"
	"feedchamber/internal/pkg/ratelimit"
	"feedchamber/internal/repository"
)

const gateSyncChannel = "gate:sync"

// gateConfigTTL 是订阅失效之外的兜底：即使 pub/sub 消息丢失，配置也会在
// 该窗口内被重新读取。
const gateConfigTTL = 30 * time.Second

type gateConfig struct {
	rules    []model.GateRule
	defaults map[string]string
	loadedAt time.Time
}

type gateService struct {
	repo      repository.GateRepository
	users     repository.UserRepository
	trustRepo repository.TrustRepository
	kv        kvstore.Store
	limiter   *ratelimit.Limiter

	mu  sync.RWMutex
	cfg *gateConfig
}

func NewGateService(repo repository.GateRepository, users repository.UserRepository, trustRepo repository.TrustRepository, kv kvstore.Store) GateService {
	s := &gateService{repo: repo, users: users, trustRepo: trustRepo, kv: kv}
	if kv != nil {
		s.limiter = ratelimit.NewLimiter(kv)
	}
	return s
}

func (s *gateService) config(ctx context.Context) (*gateConfig, error) {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	if cfg != nil && time.Since(cfg.loadedAt) < gateConfigTTL {
		return cfg, nil
	}
	rules, err := s.repo.ListRules(ctx)
	if err != nil {
		return nil, err
	}
	defs, err := s.repo.ListDomainDefaults(ctx)
	if err != nil {
		return nil, err
	}
	cfg = &gateConfig{rules: rules, defaults: make(map[string]string, len(defs)), loadedAt: time.Now()}
	for _, d := range defs {
		cfg.defaults[d.Domain] = d.DefaultAction
	}
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	return cfg, nil
}

func (s *gateService) invalidate(ctx context.Context) {
	s.mu.Lock()
	s.cfg = nil
	s.mu.Unlock()
	if s.kv != nil {
		_ = s.kv.Publish(ctx, gateSyncChannel, []byte("reload"))
	}
}

// Evaluate 求值语义：优先级降序首个命中；无命中走域默认动作。
func (s *gateService) Evaluate(ctx context.Context, domain string, in GateInput) (GateDecision, error) {
	if !isKnownGateDomain(domain) {
		return GateDecision{}, pkg.NewAppError(pkg.INVALID_PARAMS, "未知能力域")
	}
	cfg, err := s.config(ctx)
	if err != nil {
		return GateDecision{}, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	rules := make([]model.GateRule, 0, len(cfg.rules))
	for _, r := range cfg.rules {
		if r.Enabled && r.Domain == domain {
			rules = append(rules, r)
		}
	}
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].Priority == rules[j].Priority {
			return rules[i].ID < rules[j].ID
		}
		return rules[i].Priority > rules[j].Priority
	})
	for _, rule := range rules {
		ok, err := s.matches(ctx, rule, in)
		if err != nil {
			return GateDecision{}, err
		}
		if ok {
			decision := GateDecision{
				Domain: domain, Action: rule.Action, RuleID: rule.ID,
				Priority: rule.Priority, Note: rule.Note, Params: rule.ParsedActionParams(),
			}
			logGateDecision(ctx, domain, decision)
			return decision, nil
		}
	}
	action, ok := cfg.defaults[domain]
	if !ok || action == "" {
		action = model.GateActionAllow
	}
	decision := GateDecision{Domain: domain, Action: action, IsDefault: true}
	logGateDecision(ctx, domain, decision)
	return decision, nil
}

// logGateDecision 把「命中哪条规则 / 走了域默认」写进带 trace 的结构化日志。
func logGateDecision(ctx context.Context, domain string, d GateDecision) {
	fields := []logger.Field{
		logger.F("gate_domain", domain),
		logger.F("gate_action", d.Action),
		logger.F("gate_is_default", d.IsDefault),
	}
	if !d.IsDefault {
		fields = append(fields, logger.F("gate_rule", d.RuleID), logger.F("gate_priority", d.Priority))
	}
	logger.FromContext(ctx).WithContext(ctx).WithComponent("gate").Info("gate decision", fields...)
}

// matches 要求规则的所有条件同时成立（AND）。
func (s *gateService) matches(ctx context.Context, rule model.GateRule, in GateInput) (bool, error) {
	conds := rule.ParsedConditions()
	if len(conds) == 0 {
		return false, nil
	}
	for _, c := range conds {
		ok, err := s.matchCondition(ctx, c, in)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func (s *gateService) matchCondition(ctx context.Context, c model.GateCondition, in GateInput) (bool, error) {
	switch c.Field {
	case model.GateFieldLevel:
		return compareNumeric(in.Level, c.Op, c.Value), nil
	case model.GateFieldXP:
		return compareNumeric(in.XP, c.Op, c.Value), nil
	case model.GateFieldAuthorLevel:
		return compareNumeric(in.AuthorLevel, c.Op, c.Value), nil
	case model.GateFieldSpaceID:
		return compareString(in.SpaceID, c.Op, c.Value), nil
	case model.GateFieldTag:
		return compareAnyString(in.Tags, c.Op, c.Value), nil
	case model.GateFieldIsAuthor:
		return compareBool(in.IsAuthor, c.Value), nil
	case model.GateFieldIsAnonymous:
		return compareBool(in.IsAnonymous, c.Value), nil
	case model.GateFieldEmailVerified:
		return compareBool(in.EmailVerified, c.Value), nil
	case model.GateFieldBanned:
		return compareBool(in.Banned, c.Value), nil
	case model.GateFieldExiled:
		return compareBool(in.Exiled, c.Value), nil
	case model.GateFieldContainsExternalLink:
		return compareBool(in.ContainsExternalLink, c.Value), nil
	case model.GateFieldWindowMetric:
		if s.trustRepo == nil {
			return false, nil
		}
		metric, threshold, ok := parseWindowValue(c.Value)
		if !ok {
			return false, nil
		}
		days := c.WindowDays
		if days <= 0 {
			days = 100
		}
		value, err := s.windowMetric(ctx, in.UserID, metric, days)
		if err != nil {
			return false, err
		}
		return compareNumeric(value, c.Op, threshold), nil
	default:
		return false, nil
	}
}

func (s *gateService) windowMetric(ctx context.Context, userID, metric string, days int) (int, error) {
	since := time.Now().AddDate(0, 0, -days)
	switch metric {
	case "active_days":
		return s.trustRepo.CountActiveDays(ctx, userID, since)
	case "post_count":
		return s.trustRepo.CountMetric(ctx, userID, model.MetricPostApproved)
	case "likes_received":
		return s.trustRepo.CountMetric(ctx, userID, model.MetricLikesReceived)
	case "report_upheld":
		return s.trustRepo.CountUpheldReports(ctx, userID, since)
	default:
		return 0, nil
	}
}

// window_metric 的 Value 约定为「指标:阈值」，如 "active_days:30"。
func parseWindowValue(v any) (string, float64, bool) {
	s, ok := v.(string)
	if !ok {
		return "", 0, false
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return "", 0, false
	}
	n, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return "", 0, false
	}
	return parts[0], n, true
}

func compareNumeric(actual int, op string, value any) bool {
	want, ok := toFloat(value)
	if !ok {
		return false
	}
	a := float64(actual)
	switch op {
	case "eq":
		return a == want
	case "ne":
		return a != want
	case "gt":
		return a > want
	case "gte":
		return a >= want
	case "lt":
		return a < want
	case "lte":
		return a <= want
	}
	return false
}

func compareString(actual, op string, value any) bool {
	switch v := value.(type) {
	case string:
		switch op {
		case "eq":
			return actual == v
		case "ne":
			return actual != v
		case "in":
			return false
		}
	case []any:
		if op != "in" {
			return false
		}
		for _, item := range v {
			if s, ok := item.(string); ok && s == actual {
				return true
			}
		}
	}
	return false
}

func compareAnyString(actual []string, op string, value any) bool {
	switch v := value.(type) {
	case string:
		for _, a := range actual {
			if a == v {
				return true
			}
		}
	case []any:
		for _, item := range v {
			want, ok := item.(string)
			if !ok {
				continue
			}
			for _, a := range actual {
				if a == want {
					return true
				}
			}
		}
	}
	return false
}

func compareBool(actual bool, value any) bool {
	want, ok := value.(bool)
	return ok && actual == want
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

// ── 限流动作 ────────────────────────────────────────────────────────────────

// EnforceRateLimit 按 rate_limit 动作执行滑动窗口限流。
// 键形状：登录用户 rl:gate:<domain>:u:<id>；游客按等级分桶 rl:gate:<domain>:lvl:<n>。
// 新增键形状等于新窗口，不会影响既有端点预算（rl:<ip>:<policy>）。
func (s *gateService) EnforceRateLimit(ctx context.Context, decision GateDecision, in GateInput) error {
	if decision.Action != model.GateActionRateLimit || s.limiter == nil {
		return nil
	}
	limit := intFromParams(decision.Params, "limit", 30)
	window := time.Duration(intFromParams(decision.Params, "window_seconds", 60)) * time.Second
	if limit <= 0 || window <= 0 {
		return nil
	}
	key := fmt.Sprintf("gate:%s:lvl:%d", decision.Domain, in.Level)
	if in.UserID != "" {
		key = fmt.Sprintf("gate:%s:u:%s", decision.Domain, in.UserID)
	}
	res := s.limiter.Check(ctx, "gate:"+decision.Domain, key, limit, window)
	if res == ratelimit.Denied {
		return pkg.NewAppError(pkg.RATE_LIMITED, "操作过于频繁，请稍后再试")
	}
	return nil
}

func intFromParams(params map[string]any, key string, fallback int) int {
	v, ok := params[key]
	if !ok {
		return fallback
	}
	f, ok := toFloat(v)
	if !ok {
		return fallback
	}
	return int(f)
}

// ── 配置读写与校验 ───────────────────────────────────────────────────────────

func (s *gateService) ListRules(ctx context.Context) ([]model.GateRule, error) {
	rules, err := s.repo.ListRules(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return rules, nil
}

func (s *gateService) SaveRule(ctx context.Context, rule *model.GateRule, actorID string) error {
	if err := s.validateRule(ctx, rule); err != nil {
		return err
	}
	rule.UpdatedBy = actorID
	rule.UpdatedAt = time.Now()
	if rule.ID == "" {
		rule.ID = model.NewGateRuleID()
		if err := s.repo.CreateRule(ctx, rule); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	} else if err := s.repo.UpdateRule(ctx, rule); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.invalidate(ctx)
	return nil
}

func (s *gateService) DeleteRule(ctx context.Context, id string) error {
	if err := s.repo.DeleteRule(ctx, id); err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "规则不存在")
	}
	s.invalidate(ctx)
	return nil
}

func (s *gateService) ListDomainDefaults(ctx context.Context) ([]model.GateDomainDefault, error) {
	defs, err := s.repo.ListDomainDefaults(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return defs, nil
}

func (s *gateService) SaveDomainDefault(ctx context.Context, domain, action string) error {
	if !isKnownGateDomain(domain) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "未知能力域")
	}
	if !isKnownGateAction(action) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "未知动作")
	}
	if err := s.repo.UpsertDomainDefault(ctx, &model.GateDomainDefault{Domain: domain, DefaultAction: action, UpdatedAt: time.Now()}); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.invalidate(ctx)
	return nil
}

// validateRule 拒绝非法谓词/动作、空条件、未知字段、死规则与优先级冲突。
func (s *gateService) validateRule(ctx context.Context, rule *model.GateRule) error {
	if rule == nil {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "规则不能为空")
	}
	if !isKnownGateDomain(rule.Domain) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "未知能力域: "+rule.Domain)
	}
	if !isKnownGateAction(rule.Action) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "未知动作: "+rule.Action)
	}
	if rule.Action == model.GateActionRequireReview && rule.Domain != model.GateDomainPostCreate && rule.Domain != model.GateDomainCommentCreate {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "require_review 只能用于发帖 / 评论能力域")
	}
	conds := rule.ParsedConditions()
	if len(conds) == 0 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "规则至少需要一个条件")
	}
	for _, c := range conds {
		if _, ok := model.GateFieldLabels[c.Field]; !ok {
			return pkg.NewAppError(pkg.INVALID_PARAMS, "未知谓词字段: "+c.Field)
		}
		ops, ok := model.GateFieldOps[c.Field]
		if !ok || !containsString(ops, c.Op) {
			return pkg.NewAppError(pkg.INVALID_PARAMS, fmt.Sprintf("字段 %s 不支持操作符 %s", c.Field, c.Op))
		}
		if c.Field == model.GateFieldWindowMetric {
			metric, _, ok := parseWindowValue(c.Value)
			if !ok {
				return pkg.NewAppError(pkg.INVALID_PARAMS, "window_metric 的值必须形如 \"active_days:30\"")
			}
			if _, known := model.GateWindowMetrics[metric]; !known {
				return pkg.NewAppError(pkg.INVALID_PARAMS, "未知窗口指标: "+metric)
			}
		}
		if isDeadCondition(c) {
			return pkg.NewAppError(pkg.INVALID_PARAMS, "条件永不命中: "+c.Field)
		}
	}
	if rule.Priority < 0 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "优先级不能为负数")
	}
	existing, err := s.repo.ListRules(ctx)
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	for _, other := range existing {
		if other.ID == rule.ID || !other.Enabled || other.Domain != rule.Domain {
			continue
		}
		if other.Priority == rule.Priority {
			return pkg.NewAppError(pkg.INVALID_PARAMS, fmt.Sprintf("优先级 %d 在能力域 %s 已被规则 %s 占用", rule.Priority, rule.Domain, other.ID))
		}
	}
	return nil
}

func isDeadCondition(c model.GateCondition) bool {
	switch v := c.Value.(type) {
	case []any:
		if len(v) == 0 {
			return true
		}
	case float64:
		if v < 0 && (c.Field == model.GateFieldLevel || c.Field == model.GateFieldXP || c.Field == model.GateFieldAuthorLevel) {
			return true
		}
	}
	if c.Field == model.GateFieldWindowMetric {
		_, threshold, ok := parseWindowValue(c.Value)
		return !ok || threshold < 0
	}
	return false
}

func isKnownGateDomain(domain string) bool {
	_, ok := model.GateDomainLabels[domain]
	return ok
}

func isKnownGateAction(action string) bool {
	return containsString(model.GateActions, action)
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

// ── 试算 ────────────────────────────────────────────────────────────────────

// Simulate 以指定用户试算某能力域，结果必须与线上请求一致（同一 Evaluate）。
func (s *gateService) Simulate(ctx context.Context, in GateSimulateInput) (GateDecision, error) {
	input, err := s.buildInput(ctx, in.UserID)
	if err != nil {
		return GateDecision{}, err
	}
	input.IsAuthor = in.IsAuthor
	if in.AuthorLevel > 0 {
		input.AuthorLevel = in.AuthorLevel
	}
	input.SpaceID = in.SpaceID
	input.Tags = in.Tags
	if in.ContainsExternalLink != nil {
		input.ContainsExternalLink = *in.ContainsExternalLink
	} else {
		input.ContainsExternalLink = ContainsExternalLink(in.Content)
	}
	return s.Evaluate(ctx, in.Domain, input)
}

// buildInput 从用户行拼出求值输入（游客 = TL0）。
func (s *gateService) buildInput(ctx context.Context, userID string) (GateInput, error) {
	in := GateInput{UserID: userID, IsAnonymous: userID == "", Level: 0}
	if userID == "" || s.users == nil {
		return in, nil
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return in, pkg.NewAppError(pkg.USER_NOT_FOUND)
	}
	in.Role = user.Role
	in.Level = user.TrustLevel
	in.XP = user.TotalXP
	in.EmailVerified = user.EmailVerified
	in.Banned = user.Role == "ban"
	in.Exiled = user.ExiledUntil != nil
	return in, nil
}

// ── 多实例同步 ──────────────────────────────────────────────────────────────

func (s *gateService) StartSyncSubscriber(ctx context.Context) error {
	if s.kv == nil {
		return nil
	}
	ch, err := s.kv.Subscribe(ctx, gateSyncChannel)
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-ch:
				if !ok {
					return
				}
				s.mu.Lock()
				s.cfg = nil
				s.mu.Unlock()
			}
		}
	}()
	return nil
}

// ContainsExternalLink 与旧 TrustService 的实现保持一致（http/https/www）。
func ContainsExternalLink(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "http://") ||
		strings.Contains(lower, "https://") ||
		strings.Contains(lower, "www.")
}

// gateInputForUser 把用户行 + 内容上下文拼成求值输入。
func gateInputForUser(u *model.User, spaceID, content string) GateInput {
	return GateInput{
		UserID:               u.ID,
		Role:                 u.Role,
		Level:                u.TrustLevel,
		XP:                   u.TotalXP,
		IsAnonymous:          false,
		EmailVerified:        u.EmailVerified,
		Banned:               u.Role == "ban",
		Exiled:               u.ExiledUntil != nil && u.ExiledUntil.After(time.Now()),
		SpaceID:              spaceID,
		ContainsExternalLink: ContainsExternalLink(content),
	}
}

// gateDenyMessage 把命中规则的信息拼进错误文案，便于用户与运营排查。
func gateDenyMessage(d GateDecision, fallback string) string {
	if d.IsDefault {
		return fallback + "（域默认动作）"
	}
	note := d.Note
	if note == "" {
		note = d.RuleID
	}
	return fallback + "：" + note
}
