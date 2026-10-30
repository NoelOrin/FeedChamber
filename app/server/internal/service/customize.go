package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/repository"
)

type customizeService struct {
	snippets repository.SnippetRepository
	tokens   repository.ThemeTokenRepository
	nav      repository.NavItemRepository
	modules  repository.HomeModuleRepository
	pages    repository.CustomPageRepository
	audit    AuditLogService
}

func NewCustomizeService(
	snippets repository.SnippetRepository,
	tokens repository.ThemeTokenRepository,
	nav repository.NavItemRepository,
	modules repository.HomeModuleRepository,
	pages repository.CustomPageRepository,
	audit AuditLogService,
) CustomizeService {
	return &customizeService{
		snippets: snippets,
		tokens:   tokens,
		nav:      nav,
		modules:  modules,
		pages:    pages,
		audit:    audit,
	}
}

// ── 分级生效的核心判定 ────────────────────────────────────────

// decideHighRisk 决定一次「高风险自定义内容」写入是否可以直接生效。
//
// 规则（见 site-customization 规范「AI 写入自定义配置必须分级生效」）：
//   - 人工在后台操作：按请求的 enabled / status 写入；
//   - 机器凭证：强制草稿，除非额外持有 customize:publish。
//
// 返回 (是否强制草稿, 给调用方看的原因文案)。
func (s *customizeService) decideHighRisk(actor WriteActor) (forceDraft bool, reason string) {
	if actor.Source != WriteSourceMachine {
		return false, ""
	}
	if actor.CanPublish {
		return false, ""
	}
	return true, "机器凭证未持 customize:publish 权限，已保存为草稿，需在后台启用"
}

// auditWrite 记录一次自定义写入。审计失败不阻断业务，但要留警告日志：
// 审计的价值在于「事后可查」，不能因为审计表写失败就让站长的修改白做。
func (s *customizeService) auditWrite(ctx context.Context, actor WriteActor, action, target, detail string) {
	if s.audit == nil {
		return
	}
	actorID := actor.UserID
	if actorID == "" {
		actorID = "system"
	}
	_ = s.audit.Record(ctx, actorID, action, target, detail)
}

// diffSummary 生成变更摘要：只记录长度与首尾片段，避免把整段 JS 塞进审计表。
//
// 完整内容可能上万字符，逐字入库会让 audit_logs 迅速膨胀；而「改前改后是什么」
// 这个诉求，实际排查时看的是「哪一段被换掉了」，用长度 + 首尾足够定位。
func diffSummary(before, after string) string {
	if before == after {
		return "内容未变化"
	}
	return fmt.Sprintf("长度 %d → %d；改后片段: %s", len(before), len(after), truncate(after, 120))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ── 片段 ──────────────────────────────────────────────────────

func (s *customizeService) ListSnippets(ctx context.Context) ([]model.Snippet, error) {
	items, err := s.snippets.List(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if items == nil {
		items = []model.Snippet{}
	}
	return items, nil
}

func validateSnippetInput(in *SnippetInput) error {
	switch in.Kind {
	case model.SnippetKindCSS, model.SnippetKindJS, model.SnippetKindHTML:
	default:
		return pkg.NewAppError(pkg.INVALID_PARAMS, "kind 必须是 css / js / html")
	}
	if in.Position == "" {
		if in.Kind == model.SnippetKindJS {
			in.Position = model.SnippetPositionHead
		} else {
			// html 与 css 的默认落点是 body_end：head 里的裸 HTML 会在 <body>
			// 之前被解析（行为不可预期），而 css 通常只想微调，放在 body_end
			// 也能覆盖样式。显式传 head 时仍按 EffectivePosition 纠正。
			in.Position = model.SnippetPositionBodyEnd
		}
	}
	switch in.Position {
	case model.SnippetPositionHead, model.SnippetPositionBodyEnd:
	default:
		return pkg.NewAppError(pkg.INVALID_PARAMS, "position 必须是 head / body_end")
	}
	// 归一化：模型里 EffectivePosition 是权威位置，写库也必须是它。
	// 否则后台表单（默认 position=head）提交的 html 片段会落 head，
	// 而读取侧按有效位置分组，内容永远发不出去。
	in.Position = model.Snippet{Kind: in.Kind, Position: in.Position}.EffectivePosition()

	if strings.TrimSpace(in.Content) == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "content 不能为空")
	}
	return validateSnippetContent(in.Kind, in.Content)
}

// validateSnippetContent 保证片段内容能安全地放进它将被内联的容器。
//
// 片段最终会被原样写进 SSR 出来的 HTML 文档：css → <style>、js → <script>、
// html → 容器 <div>。前两者的内容是「RAWTEXT」，出现对应的结束标签就会闭合
// 元素，剩余内容按正常文档解析——`</style><script>…</script>` 会被真正执行。
// 因此这两类必须拒绝包含结束标签的内容；html 片段本来就是任意标记，不设限。
//
// 这是策略边界：写侧拒绝（可读的错误信息）+ 渲染侧按 kind 选容器，两侧都不
// 依赖对方「刚好也做了转义」。
func validateSnippetContent(kind, content string) error {
	switch kind {
	case model.SnippetKindCSS:
		if strings.Contains(strings.ToLower(content), "</style") {
			return pkg.NewAppError(pkg.INVALID_PARAMS, "css 片段不能包含 </style（会截断样式块并注入任意标记）")
		}
	case model.SnippetKindJS:
		if strings.Contains(strings.ToLower(content), "</script") {
			return pkg.NewAppError(pkg.INVALID_PARAMS, "js 片段不能包含 </script（会截断脚本块并注入任意标记）")
		}
	}
	return nil
}

func (s *customizeService) CreateSnippet(ctx context.Context, in SnippetInput, actor WriteActor) (*model.Snippet, WriteResult, error) {
	if err := validateSnippetInput(&in); err != nil {
		return nil, WriteResult{}, err
	}
	enabled := in.Enabled
	res := WriteResult{Applied: enabled}
	if enabled && isHighRiskKind(in.Kind) {
		if force, reason := s.decideHighRisk(actor); force {
			enabled = false
			res = WriteResult{Applied: false, Reason: reason}
		}
	}
	now := time.Now()
	snippet := &model.Snippet{
		ID:        pkg.NewID(),
		Kind:      in.Kind,
		Position:  in.Position,
		Content:   in.Content,
		Enabled:   enabled,
		Sort:      in.Sort,
		UpdatedBy: actor.UserID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.snippets.Create(ctx, snippet); err != nil {
		return nil, WriteResult{}, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.auditWrite(ctx, actor, "customize.snippet.create", snippet.ID,
		fmt.Sprintf("kind=%s position=%s enabled=%v %s", snippet.Kind, snippet.Position, snippet.Enabled, diffSummary("", snippet.Content)))
	if res.Reason == "" && !res.Applied {
		res.Reason = "已保存为草稿（请求中 enabled=false）"
	}
	return snippet, res, nil
}

func (s *customizeService) UpdateSnippet(ctx context.Context, id string, in SnippetInput, actor WriteActor) (*model.Snippet, WriteResult, error) {
	if err := validateSnippetInput(&in); err != nil {
		return nil, WriteResult{}, err
	}
	existing, err := s.snippets.Get(ctx, id)
	if err != nil {
		return nil, WriteResult{}, mapRepoErr(err, "片段不存在")
	}
	before := existing.Content
	enabled := in.Enabled
	res := WriteResult{Applied: enabled}
	if enabled && isHighRiskKind(in.Kind) {
		if force, reason := s.decideHighRisk(actor); force {
			enabled = false
			res = WriteResult{Applied: false, Reason: reason}
		}
	}
	existing.Kind = in.Kind
	existing.Position = in.Position
	existing.Content = in.Content
	existing.Enabled = enabled
	existing.Sort = in.Sort
	existing.UpdatedBy = actor.UserID
	existing.UpdatedAt = time.Now()
	if err := s.snippets.Update(ctx, existing); err != nil {
		return nil, WriteResult{}, mapRepoErr(err, "片段不存在")
	}
	s.auditWrite(ctx, actor, "customize.snippet.update", id, diffSummary(before, in.Content))
	if res.Reason == "" && !res.Applied {
		res.Reason = "已保存为草稿（请求中 enabled=false）"
	}
	return existing, res, nil
}

func (s *customizeService) DeleteSnippet(ctx context.Context, id string, actor WriteActor) error {
	snippet, err := s.snippets.Get(ctx, id)
	if err != nil {
		return mapRepoErr(err, "片段不存在")
	}
	if err := s.snippets.Delete(ctx, id); err != nil {
		return mapRepoErr(err, "片段不存在")
	}
	s.auditWrite(ctx, actor, "customize.snippet.delete", id,
		fmt.Sprintf("kind=%s %s", snippet.Kind, diffSummary(snippet.Content, "")))
	return nil
}

func (s *customizeService) SetSnippetEnabled(ctx context.Context, id string, enabled bool, actor WriteActor) error {
	snippet, err := s.snippets.Get(ctx, id)
	if err != nil {
		return mapRepoErr(err, "片段不存在")
	}
	// 机器凭证不得启用高风险片段：启用动作本身就是「让 JS 上线」，
	// 与写入同一风险级别，必须走 decideHighRisk 的同一判据。
	if enabled && snippet.IsHighRisk() {
		if force, reason := s.decideHighRisk(actor); force {
			return pkg.NewAppError(pkg.FORBIDDEN, reason)
		}
	}
	if err := s.snippets.SetEnabled(ctx, id, enabled, actor.UserID); err != nil {
		return mapRepoErr(err, "片段不存在")
	}
	s.auditWrite(ctx, actor, "customize.snippet.toggle", id, fmt.Sprintf("enabled=%v", enabled))
	return nil
}

func isHighRiskKind(kind string) bool {
	return kind == model.SnippetKindJS || kind == model.SnippetKindHTML
}

func (s *customizeService) ActiveSnippets(ctx context.Context, position string) ([]model.Snippet, error) {
	items, err := s.snippets.ListEnabled(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	// 按有效位置分组，而不是列表里的 position 列：存量数据可能把 html 片段
	// 存成 head，按列过滤会让它们永远发不出去（且不报错）。
	out := make([]model.Snippet, 0, len(items))
	for _, it := range items {
		if it.EffectivePosition() == position {
			out = append(out, it)
		}
	}
	return out, nil
}

// validateThemeToken 校验一条 token 覆盖的形状。
//
// 主题覆盖只负责「改色改圆角」，不承担任意 CSS 的职责（那是 css 片段的活）。
// 渲染侧会把它拼进 `<style>{:root{--x:v}}`，所以名字必须是 CSS 自定义属性、
// 值不能闭合声明块或元素——否则这就不再是「最坏站点变丑」的低风险写入了。
func validateThemeToken(token, value string) error {
	if !themeTokenNameRe.MatchString(token) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "token 必须是 CSS 自定义属性名，如 --primary")
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "token 值不能为空")
	}
	if len(trimmed) > 512 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "token 值过长（上限 512 字符）")
	}
	if strings.ContainsAny(trimmed, ";{}<>") || strings.ContainsAny(trimmed, "\n\r") {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "token 值不能包含 ; { } < > 或换行")
	}
	return nil
}

var themeTokenNameRe = regexp.MustCompile(`^--[a-zA-Z0-9-_]+$`)

// ── 主题 token ────────────────────────────────────────────────

func (s *customizeService) ListThemeTokens(ctx context.Context) ([]model.ThemeToken, error) {
	items, err := s.tokens.List(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if items == nil {
		items = []model.ThemeToken{}
	}
	return items, nil
}

// SetThemeTokens 批量写入覆盖。CSS/token 属于低风险——最坏是站点变丑，
// 可撤销，因此机器凭证也可以直接生效（不需要草稿）。
func (s *customizeService) SetThemeTokens(ctx context.Context, items []ThemeTokenInput, actor WriteActor) (WriteResult, error) {
	if len(items) == 0 {
		return WriteResult{}, pkg.NewAppError(pkg.INVALID_PARAMS, "items 不能为空")
	}
	for i := range items {
		it := items[i]
		if strings.TrimSpace(it.Token) == "" {
			return WriteResult{}, pkg.NewAppError(pkg.INVALID_PARAMS, "token 不能为空")
		}
		if err := validateThemeToken(it.Token, it.Value); err != nil {
			return WriteResult{}, err
		}
		if it.Mode == "" {
			it.Mode = model.ThemeTokenModeAll
		}
		if !model.ValidThemeTokenMode(it.Mode) {
			return WriteResult{}, pkg.NewAppError(pkg.INVALID_PARAMS, "mode 必须是 all / light / dark")
		}
		now := time.Now()
		rec := &model.ThemeToken{
			ID:        pkg.NewID(),
			Token:     strings.TrimSpace(it.Token),
			Value:     it.Value,
			Mode:      it.Mode,
			UpdatedBy: actor.UserID,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.tokens.Upsert(ctx, rec); err != nil {
			return WriteResult{}, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.Token)
	}
	s.auditWrite(ctx, actor, "customize.tokens.set", "theme_tokens", strings.Join(names, ","))
	return WriteResult{Applied: true}, nil
}

func (s *customizeService) DeleteThemeToken(ctx context.Context, token string, actor WriteActor) error {
	if err := s.tokens.DeleteByToken(ctx, token); err != nil {
		return mapRepoErr(err, "token 覆盖不存在")
	}
	s.auditWrite(ctx, actor, "customize.tokens.delete", token, "reverted to default")
	return nil
}

func (s *customizeService) ClearThemeTokens(ctx context.Context, actor WriteActor) error {
	if err := s.tokens.DeleteAll(ctx); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.auditWrite(ctx, actor, "customize.tokens.clear", "theme_tokens", "all overrides removed")
	return nil
}

func (s *customizeService) ThemeOverrides(ctx context.Context) ([]model.ThemeToken, error) {
	items, err := s.tokens.List(ctx)
	if err != nil {
		// 主题覆盖是纯装饰：读失败时返回空集让页面按默认渲染，
		// 不能因为一个 token 表抖动让整站 500。
		return []model.ThemeToken{}, nil
	}
	return items, nil
}

// ── 导航 ──────────────────────────────────────────────────────

func validateNavInput(in *NavItemInput) error {
	if strings.TrimSpace(in.Label) == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "label 不能为空")
	}
	if strings.TrimSpace(in.Href) == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "href 不能为空")
	}
	if in.Visibility == "" {
		in.Visibility = model.NavVisibilityPublic
	}
	if !model.ValidNavVisibility(in.Visibility) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "visibility 必须是 public / login / level")
	}
	if in.Visibility == model.NavVisibilityLevel && in.MinLevel <= 0 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "visibility=level 时 min_level 必须大于 0")
	}
	return nil
}

func (s *customizeService) ListNavItems(ctx context.Context) ([]model.NavItem, error) {
	items, err := s.nav.List(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if items == nil {
		items = []model.NavItem{}
	}
	return items, nil
}

func (s *customizeService) CreateNavItem(ctx context.Context, in NavItemInput, actor WriteActor) (*model.NavItem, WriteResult, error) {
	if err := validateNavInput(&in); err != nil {
		return nil, WriteResult{}, err
	}
	enabled := in.Enabled
	res := WriteResult{Applied: enabled}
	if enabled {
		if force, reason := s.decideHighRisk(actor); force {
			enabled = false
			res = WriteResult{Applied: false, Reason: reason}
		}
	}
	now := time.Now()
	item := &model.NavItem{
		ID:         pkg.NewID(),
		Label:      in.Label,
		Href:       in.Href,
		Icon:       in.Icon,
		Sort:       in.Sort,
		Visibility: in.Visibility,
		MinLevel:   in.MinLevel,
		Enabled:    enabled,
		UpdatedBy:  actor.UserID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.nav.Create(ctx, item); err != nil {
		return nil, WriteResult{}, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.auditWrite(ctx, actor, "customize.nav.create", item.ID, fmt.Sprintf("label=%s href=%s enabled=%v", item.Label, item.Href, item.Enabled))
	if res.Reason == "" && !res.Applied {
		res.Reason = "已保存为草稿（请求中 enabled=false）"
	}
	return item, res, nil
}

func (s *customizeService) UpdateNavItem(ctx context.Context, id string, in NavItemInput, actor WriteActor) (*model.NavItem, WriteResult, error) {
	if err := validateNavInput(&in); err != nil {
		return nil, WriteResult{}, err
	}
	existing, err := s.nav.Get(ctx, id)
	if err != nil {
		return nil, WriteResult{}, mapRepoErr(err, "导航项不存在")
	}
	before := existing.Label + " " + existing.Href
	enabled := in.Enabled
	res := WriteResult{Applied: enabled}
	if enabled {
		if force, reason := s.decideHighRisk(actor); force {
			enabled = false
			res = WriteResult{Applied: false, Reason: reason}
		}
	}
	existing.Label = in.Label
	existing.Href = in.Href
	existing.Icon = in.Icon
	existing.Sort = in.Sort
	existing.Visibility = in.Visibility
	existing.MinLevel = in.MinLevel
	existing.Enabled = enabled
	existing.UpdatedBy = actor.UserID
	existing.UpdatedAt = time.Now()
	if err := s.nav.Update(ctx, existing); err != nil {
		return nil, WriteResult{}, mapRepoErr(err, "导航项不存在")
	}
	s.auditWrite(ctx, actor, "customize.nav.update", id,
		fmt.Sprintf("%s → %s %s", before, existing.Label, existing.Href))
	if res.Reason == "" && !res.Applied {
		res.Reason = "已保存为草稿（请求中 enabled=false）"
	}
	return existing, res, nil
}

func (s *customizeService) DeleteNavItem(ctx context.Context, id string, actor WriteActor) error {
	item, err := s.nav.Get(ctx, id)
	if err != nil {
		return mapRepoErr(err, "导航项不存在")
	}
	if err := s.nav.Delete(ctx, id); err != nil {
		return mapRepoErr(err, "导航项不存在")
	}
	s.auditWrite(ctx, actor, "customize.nav.delete", id, item.Label)
	return nil
}

func (s *customizeService) PublicNav(ctx context.Context, isLogin bool, level int) ([]model.NavItem, error) {
	items, err := s.nav.ListEnabled(ctx)
	if err != nil {
		// 导航读失败时返回空，由前端回退到硬编码默认导航（design D7）。
		return []model.NavItem{}, nil
	}
	out := make([]model.NavItem, 0, len(items))
	for _, it := range items {
		if it.VisibleTo(isLogin, level) {
			out = append(out, it)
		}
	}
	return out, nil
}

// ── 首页模块 ──────────────────────────────────────────────────

func validateHomeModuleInput(in *HomeModuleInput) error {
	if in.Kind == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "kind 不能为空")
	}
	if !model.ValidHomeModuleKind(in.Kind) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "kind 必须是 latest / hot / spaces / html")
	}
	if in.Visibility == "" {
		in.Visibility = model.NavVisibilityPublic
	}
	if !model.ValidNavVisibility(in.Visibility) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "visibility 必须是 public / login / level")
	}
	if in.Visibility == model.NavVisibilityLevel && in.MinLevel <= 0 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "visibility=level 时 min_level 必须大于 0")
	}
	if in.Config != "" && !json.Valid([]byte(in.Config)) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "config 必须是合法 JSON")
	}
	if in.Kind == model.HomeModuleHTML && strings.TrimSpace(in.Config) == "" {
		// html 模块的内容就放在 config 里，空内容没有任何意义。
		return pkg.NewAppError(pkg.INVALID_PARAMS, "html 模块必须在 config 中提供内容")
	}
	return nil
}

func (s *customizeService) ListHomeModules(ctx context.Context) ([]model.HomeModule, error) {
	items, err := s.modules.List(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if items == nil {
		items = []model.HomeModule{}
	}
	return items, nil
}

func (s *customizeService) CreateHomeModule(ctx context.Context, in HomeModuleInput, actor WriteActor) (*model.HomeModule, WriteResult, error) {
	if err := validateHomeModuleInput(&in); err != nil {
		return nil, WriteResult{}, err
	}
	enabled := in.Enabled
	res := WriteResult{Applied: enabled}
	if enabled && in.Kind == model.HomeModuleHTML {
		if force, reason := s.decideHighRisk(actor); force {
			enabled = false
			res = WriteResult{Applied: false, Reason: reason}
		}
	}
	now := time.Now()
	m := &model.HomeModule{
		ID:         pkg.NewID(),
		Kind:       in.Kind,
		Title:      in.Title,
		Sort:       in.Sort,
		Visibility: in.Visibility,
		MinLevel:   in.MinLevel,
		Config:     in.Config,
		Enabled:    enabled,
		UpdatedBy:  actor.UserID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.modules.Create(ctx, m); err != nil {
		return nil, WriteResult{}, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.auditWrite(ctx, actor, "customize.module.create", m.ID, fmt.Sprintf("kind=%s enabled=%v", m.Kind, m.Enabled))
	if res.Reason == "" && !res.Applied {
		res.Reason = "已保存为草稿（请求中 enabled=false）"
	}
	return m, res, nil
}

func (s *customizeService) UpdateHomeModule(ctx context.Context, id string, in HomeModuleInput, actor WriteActor) (*model.HomeModule, WriteResult, error) {
	if err := validateHomeModuleInput(&in); err != nil {
		return nil, WriteResult{}, err
	}
	existing, err := s.modules.Get(ctx, id)
	if err != nil {
		return nil, WriteResult{}, mapRepoErr(err, "首页模块不存在")
	}
	enabled := in.Enabled
	res := WriteResult{Applied: enabled}
	if enabled && in.Kind == model.HomeModuleHTML {
		if force, reason := s.decideHighRisk(actor); force {
			enabled = false
			res = WriteResult{Applied: false, Reason: reason}
		}
	}
	existing.Kind = in.Kind
	existing.Title = in.Title
	existing.Sort = in.Sort
	existing.Visibility = in.Visibility
	existing.MinLevel = in.MinLevel
	existing.Config = in.Config
	existing.Enabled = enabled
	existing.UpdatedBy = actor.UserID
	existing.UpdatedAt = time.Now()
	if err := s.modules.Update(ctx, existing); err != nil {
		return nil, WriteResult{}, mapRepoErr(err, "首页模块不存在")
	}
	s.auditWrite(ctx, actor, "customize.module.update", id, fmt.Sprintf("kind=%s enabled=%v", existing.Kind, existing.Enabled))
	if res.Reason == "" && !res.Applied {
		res.Reason = "已保存为草稿（请求中 enabled=false）"
	}
	return existing, res, nil
}

func (s *customizeService) DeleteHomeModule(ctx context.Context, id string, actor WriteActor) error {
	if _, err := s.modules.Get(ctx, id); err != nil {
		return mapRepoErr(err, "首页模块不存在")
	}
	if err := s.modules.Delete(ctx, id); err != nil {
		return mapRepoErr(err, "首页模块不存在")
	}
	s.auditWrite(ctx, actor, "customize.module.delete", id, "")
	return nil
}

func (s *customizeService) PublicHomeModules(ctx context.Context, isLogin bool, level int) ([]model.HomeModule, error) {
	items, err := s.modules.ListEnabled(ctx)
	if err != nil {
		return []model.HomeModule{}, nil
	}
	out := make([]model.HomeModule, 0, len(items))
	for _, m := range items {
		if m.VisibleTo(isLogin, level) {
			out = append(out, m)
		}
	}
	return out, nil
}

// ── 自定义页 ──────────────────────────────────────────────────

func validateCustomPageInput(in *CustomPageInput) error {
	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "slug 不能为空")
	}
	if strings.ContainsAny(slug, "/?#& ") {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "slug 不能包含 / ? # & 或空格")
	}
	if model.IsReservedSlug(slug) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, fmt.Sprintf("slug %q 与站点既有路由冲突", slug))
	}
	if strings.TrimSpace(in.Title) == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "title 不能为空")
	}
	if in.Status == "" {
		in.Status = model.CustomPageDraft
	}
	if !model.ValidCustomPageStatus(in.Status) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "status 必须是 draft / published")
	}
	return nil
}

func (s *customizeService) ListCustomPages(ctx context.Context, page, pageSize int) ([]model.CustomPage, int64, error) {
	page, pageSize = pkg.NormalizePagination(page, pageSize)
	offset := (page - 1) * pageSize
	items, err := s.pages.List(ctx, offset, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	total, err := s.pages.Count(ctx)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if items == nil {
		items = []model.CustomPage{}
	}
	return items, total, nil
}

func (s *customizeService) CreateCustomPage(ctx context.Context, in CustomPageInput, actor WriteActor) (*model.CustomPage, WriteResult, error) {
	if err := validateCustomPageInput(&in); err != nil {
		return nil, WriteResult{}, err
	}
	// 已存在的 slug 直接拒绝，而不是静默覆盖别人的页面。
	if _, err := s.pages.GetBySlug(ctx, in.Slug); err == nil {
		return nil, WriteResult{}, pkg.NewAppError(pkg.INVALID_PARAMS, "slug 已被占用")
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, WriteResult{}, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	status := in.Status
	res := WriteResult{Applied: status == model.CustomPagePublished}
	if status == model.CustomPagePublished {
		if force, reason := s.decideHighRisk(actor); force {
			status = model.CustomPageDraft
			res = WriteResult{Applied: false, Reason: reason}
		}
	}
	now := time.Now()
	p := &model.CustomPage{
		ID:        pkg.NewID(),
		Slug:      in.Slug,
		Title:     in.Title,
		Content:   in.Content,
		Status:    status,
		UpdatedBy: actor.UserID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.pages.Create(ctx, p); err != nil {
		return nil, WriteResult{}, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.auditWrite(ctx, actor, "customize.page.create", p.ID, fmt.Sprintf("slug=%s status=%s", p.Slug, p.Status))
	if res.Reason == "" && !res.Applied {
		res.Reason = "已保存为草稿（请求中 status=draft）"
	}
	return p, res, nil
}

func (s *customizeService) UpdateCustomPage(ctx context.Context, id string, in CustomPageInput, actor WriteActor) (*model.CustomPage, WriteResult, error) {
	if err := validateCustomPageInput(&in); err != nil {
		return nil, WriteResult{}, err
	}
	existing, err := s.pages.Get(ctx, id)
	if err != nil {
		return nil, WriteResult{}, mapRepoErr(err, "自定义页不存在")
	}
	// slug 改成别人已占用的值要拒绝。
	if in.Slug != existing.Slug {
		if other, err := s.pages.GetBySlug(ctx, in.Slug); err == nil && other.ID != id {
			return nil, WriteResult{}, pkg.NewAppError(pkg.INVALID_PARAMS, "slug 已被占用")
		} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return nil, WriteResult{}, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	before := existing.Content
	status := in.Status
	res := WriteResult{Applied: status == model.CustomPagePublished}
	if status == model.CustomPagePublished {
		if force, reason := s.decideHighRisk(actor); force {
			status = model.CustomPageDraft
			res = WriteResult{Applied: false, Reason: reason}
		}
	}
	existing.Slug = in.Slug
	existing.Title = in.Title
	existing.Content = in.Content
	existing.Status = status
	existing.UpdatedBy = actor.UserID
	existing.UpdatedAt = time.Now()
	if err := s.pages.Update(ctx, existing); err != nil {
		return nil, WriteResult{}, mapRepoErr(err, "自定义页不存在")
	}
	s.auditWrite(ctx, actor, "customize.page.update", id, fmt.Sprintf("slug=%s status=%s %s", existing.Slug, existing.Status, diffSummary(before, in.Content)))
	if res.Reason == "" && !res.Applied {
		res.Reason = "已保存为草稿（请求中 status=draft）"
	}
	return existing, res, nil
}

func (s *customizeService) DeleteCustomPage(ctx context.Context, id string, actor WriteActor) error {
	p, err := s.pages.Get(ctx, id)
	if err != nil {
		return mapRepoErr(err, "自定义页不存在")
	}
	if err := s.pages.Delete(ctx, id); err != nil {
		return mapRepoErr(err, "自定义页不存在")
	}
	s.auditWrite(ctx, actor, "customize.page.delete", id, p.Slug)
	return nil
}

func (s *customizeService) PublishCustomPage(ctx context.Context, id string, publish bool, actor WriteActor) (*model.CustomPage, WriteResult, error) {
	p, err := s.pages.Get(ctx, id)
	if err != nil {
		return nil, WriteResult{}, mapRepoErr(err, "自定义页不存在")
	}
	status := model.CustomPageDraft
	if publish {
		status = model.CustomPagePublished
	}
	res := WriteResult{Applied: publish}
	// 发布是「让自定义内容对访客可见」，与写入同一风险级别。
	if publish {
		if force, reason := s.decideHighRisk(actor); force {
			return nil, WriteResult{}, pkg.NewAppError(pkg.FORBIDDEN, reason)
		}
	}
	if err := s.pages.SetStatus(ctx, id, status, actor.UserID); err != nil {
		return nil, WriteResult{}, mapRepoErr(err, "自定义页不存在")
	}
	p.Status = status
	p.UpdatedBy = actor.UserID
	s.auditWrite(ctx, actor, "customize.page.publish", id, fmt.Sprintf("slug=%s status=%s", p.Slug, status))
	return p, res, nil
}

func (s *customizeService) GetPublishedPage(ctx context.Context, slug string) (*model.CustomPage, error) {
	p, err := s.pages.GetBySlug(ctx, slug)
	if err != nil {
		return nil, mapRepoErr(err, "页面不存在")
	}
	// 草稿对访客表现为「不存在」，而不是 403：否则等于泄露「这里有个未发布页」。
	if !p.IsPublished() {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "页面不存在")
	}
	return p, nil
}

// ── 共用错误映射 ──────────────────────────────────────────────

// mapRepoErr 把仓储哨兵错误翻译成带上下文的 AppError。
//
// 不能直接透传 err.Error()：ErrNotFound 的原文是 "record not found"，
// 对调用方（尤其是 MCP 的 AI 客户端）毫无信息量。
func mapRepoErr(err error, notFoundMsg string) error {
	if errors.Is(err, repository.ErrNotFound) {
		return pkg.NewAppError(pkg.NOT_FOUND, notFoundMsg)
	}
	return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
}
