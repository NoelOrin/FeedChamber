package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/plugin"
	"feedchamber/internal/repository"
)

// defaultPluginRoot 是插件代码的默认根目录（相对 app/server 工作目录）。
//
// 与 container 里 HostOptions.PluginRoot 的默认值必须一致：service 用它算
// install.Dir，host 用它算 Supervisor.PluginDir。两处不一致的症状是
// 「装完就起不来」，而且不改任何一行代码只改启动目录就会复现。
const defaultPluginRoot = "./plugins"

// 安装审计的 action 名。写成常量而不是散在调用点，因为后台审计筛选按
// action 精确匹配，拼错一个字就查不到。
const (
	auditActionSourceCreate  = "plugin.source.create"
	auditActionSourceApprove = "plugin.source.approve"
	auditActionSourceReject  = "plugin.source.reject"
	auditActionInstall       = "plugin.install"
	auditActionUninstall     = "plugin.uninstall"
)

// pluginIDPattern 限定插件 ID 的形态。
//
// 这不是美观问题：dir 由 <root>/<pluginID> 拼出，一个含 `..` 或 `/` 的 ID
// 会让插件装到根目录之外（例如 ID "../../etc"）。manifest 里的 entry 由
// validateEntry 守住，这里守的是安装记录落盘位置。
var pluginIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type pluginService struct {
	repo     repository.PluginRepository
	sources  repository.PluginSourceRepository
	installs repository.PluginInstallRepository
	audit    AuditLogService
	root     string
	runtime  PluginRuntime
}

func NewPluginService(
	repo repository.PluginRepository,
	sources repository.PluginSourceRepository,
	installs repository.PluginInstallRepository,
	audit AuditLogService,
) PluginService {
	return &pluginService{
		repo:     repo,
		sources:  sources,
		installs: installs,
		audit:    audit,
		root:     defaultPluginRoot,
	}
}

// SetRuntime 注入子进程宿主。container 在构造完 Host 后调用；未注入时
// 管理动作仍是纯元数据登记（测试可用），但不会虚报 running。
func (s *pluginService) SetRuntime(rt PluginRuntime) {
	s.runtime = rt
}

func (s *pluginService) List(ctx context.Context) ([]model.Plugin, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.Plugin{}
	}
	return items, nil
}

func (s *pluginService) Update(ctx context.Context, input PluginInput) error {
	if input.ID == "" || input.Name == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "插件 ID 和名称不能为空")
	}
	if err := validatePluginSettings(input.Settings); err != nil {
		return err
	}
	pluginRow, err := s.repo.Get(ctx, input.ID)
	if err != nil {
		return err
	}
	pluginRow.Name = input.Name
	pluginRow.Version = input.Version
	pluginRow.Author = input.Author
	pluginRow.Description = input.Description
	pluginRow.Homepage = input.Homepage
	pluginRow.Settings = input.Settings
	pluginRow.Enabled = input.Enabled
	pluginRow.UpdatedAt = time.Now()
	return s.repo.Update(ctx, pluginRow)
}

func (s *pluginService) SetEnabled(ctx context.Context, id string, enabled bool) error {
	pluginRow, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}

	// 运行时先动、数据库后动：进程起停成功才把 enabled 落库，否则一次
	// 失败的 Start 会留下「标记启用但根本没在跑」的假状态。
	if s.runtime != nil {
		if enabled {
			if !s.runtime.Running(id) {
				m, caps, rerr := s.manifestForRuntime(ctx, id)
				if rerr != nil {
					return rerr
				}
				if err := s.runtime.StartPlugin(id, m, caps); err != nil {
					return pkg.NewAppError(pkg.INTERNAL_ERROR, "启用插件失败: "+err.Error())
				}
				if err := s.installs.UpdateStatus(ctx, id, model.PluginStatusRunning, ""); err != nil {
					return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
				}
			}
		} else if s.runtime.Running(id) {
			if err := s.runtime.StopPlugin(id); err != nil {
				return pkg.NewAppError(pkg.INTERNAL_ERROR, "停用插件失败: "+err.Error())
			}
			if err := s.installs.UpdateStatus(ctx, id, model.PluginStatusStopped, ""); err != nil {
				return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
			}
		}
	}

	pluginRow.Enabled = enabled
	pluginRow.UpdatedAt = time.Now()
	return s.repo.Update(ctx, pluginRow)
}

// manifestForRuntime 从落库的安装快照复原启动参数。
//
// 刻意用 plugins.Manifest 原文重新 ParseManifest + plugin_installs.Caps 快照，
// 而不是信任任何请求参数：启停时插件的信任边界必须与安装时一致。
func (s *pluginService) manifestForRuntime(ctx context.Context, id string) (*plugin.Manifest, plugin.CapabilitySet, error) {
	row, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if row.Manifest == "" {
		return nil, nil, pkg.NewAppError(pkg.INVALID_PARAMS, "插件缺少 manifest，无法启动")
	}
	m, err := plugin.ParseManifest([]byte(row.Manifest))
	if err != nil {
		return nil, nil, pkg.NewAppError(pkg.INVALID_PARAMS, err.Error())
	}
	inst, err := s.installs.GetByPluginID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	var caps []string
	if inst.Caps != "" {
		if err := json.Unmarshal([]byte(inst.Caps), &caps); err != nil {
			return nil, nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "能力快照非法: "+err.Error())
		}
	}
	return m, plugin.CapabilitySetFromNames(caps), nil
}

// Delete 是后台「卸载」按钮的入口：必须与 Uninstall 等价——停进程、清双表、
// 落审计。此前它只删 plugins 一行，留下 plugin_installs 孤儿与运行中的子进程。
func (s *pluginService) Delete(ctx context.Context, id string) error {
	return s.Uninstall(ctx, id)
}

// ListInstalls 返回全部安装记录（含运行状态、重启次数、最近错误），供后台
// 展示「装了什么、跑没跑起来」。列表接口不分页：插件数量是管理员手工控制
// 的小集合，且前端要按 plugin_id 与 plugins 列表合并。
func (s *pluginService) ListInstalls(ctx context.Context) ([]model.PluginInstall, error) {
	items, err := s.installs.List(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if items == nil {
		items = []model.PluginInstall{}
	}
	return items, nil
}

// ── 来源与审批 ────────────────────────────────────────────────

func (s *pluginService) CreateSource(ctx context.Context, name, kind, url, actorID string) (*model.PluginSource, error) {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if name == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "来源名称不能为空")
	}
	if url == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "来源 URL 不能为空")
	}
	if kind == "" {
		kind = model.PluginSourceGit
	}
	// v1 只支持从受信任代码仓库拉取。未知 kind 直接拒绝而不是落库——
	// 安装流程按 kind 决定「怎么把代码弄到本地」，未知值在安装时才炸更晚。
	if kind != model.PluginSourceGit {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "不支持的插件来源类型: "+kind)
	}

	now := time.Now()
	src := &model.PluginSource{
		ID:        pkg.NewID(),
		Name:      name,
		Kind:      kind,
		URL:       url,
		Status:    model.PluginSourcePending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.sources.Create(ctx, src); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	// 新增来源必须留痕：这是信任模型的入口，事后要能查出「谁在什么时候
	// 把哪个地址加进了白名单」。
	s.recordAudit(ctx, actorID, auditActionSourceCreate, src.ID,
		fmt.Sprintf("name=%s kind=%s url=%s", src.Name, src.Kind, src.URL))
	return src, nil
}

func (s *pluginService) ListSources(ctx context.Context) ([]model.PluginSource, error) {
	items, err := s.sources.List(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if items == nil {
		items = []model.PluginSource{}
	}
	return items, nil
}

func (s *pluginService) ApproveSource(ctx context.Context, id, actorID string) error {
	return s.setSourceStatus(ctx, id, actorID, model.PluginSourceApproved, auditActionSourceApprove)
}

func (s *pluginService) RejectSource(ctx context.Context, id, actorID string) error {
	return s.setSourceStatus(ctx, id, actorID, model.PluginSourceRejected, auditActionSourceReject)
}

func (s *pluginService) setSourceStatus(ctx context.Context, id, actorID, status, action string) error {
	if id == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "来源 ID 不能为空")
	}
	src, err := s.sources.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "插件来源不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	src.Status = status
	// ApprovedBy 记录最近一次状态变更的执行人。批准与拒绝都写它：
	// 拒绝也需要「谁拒的」，而列名沿用计划里的名字，语义是「决策人」。
	src.ApprovedBy = actorID
	src.UpdatedAt = time.Now()
	if err := s.sources.Update(ctx, src); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return pkg.NewAppError(pkg.NOT_FOUND, "插件来源不存在")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.recordAudit(ctx, actorID, action, src.ID, fmt.Sprintf("url=%s", src.URL))
	return nil
}

// ── 安装与运行时 ──────────────────────────────────────────────

// Install 登记并安装一个插件。
//
// 校验顺序是信任模型的核心，不可调换：
//
//  1. ParseManifest —— 结构、协议版本、entry 逃逸（validateEntry）；
//  2. Validate(全量已知能力集) —— 未知 capability **报错**。
//     注意必须传 CapabilitySetFromNames(KnownCapabilities())，而不是
//     CapabilitySetFromNames(m.Capabilities)：后者只把 manifest 自己的
//     声明转换一遍，未知名字被静默丢弃，任何拼错的能力都能装进来；
//  3. 来源存在且已审批 —— 非 approved 一律 FORBIDDEN。
//
// 先校验 manifest 再看来源，是为了让「manifest 本身有问题」这种更好修的
// 错误优先暴露，而不是被「来源未审批」挡住后发现改了来源还是装不上。
func (s *pluginService) Install(ctx context.Context, in PluginInstallInput) (*model.PluginInstall, error) {
	if !pluginIDPattern.MatchString(in.PluginID) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "插件 ID 只能包含字母、数字、点、下划线与连字符，且不超过 64 字符")
	}

	m, err := plugin.ParseManifest([]byte(in.Manifest))
	if err != nil {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, err.Error())
	}
	// 全量已知能力集，不是 manifest 自己的声明集合。见方法注释第 2 条。
	if err := m.Validate(plugin.CapabilitySetFromNames(plugin.KnownCapabilities())); err != nil {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, err.Error())
	}

	if in.SourceID == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "来源 ID 不能为空")
	}
	src, err := s.sources.Get(ctx, in.SourceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "来源不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	// 未审批来源一律拒绝——这是信任模型第一层。
	// 判定用「不等于 approved」而不是枚举 pending/rejected：新增状态时
	// 默认落在拒绝侧，而不是默认放行。
	if src.Status != model.PluginSourceApproved {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "插件来源尚未审批，无法安装")
	}

	dir, err := s.resolveDir(in.Dir, in.PluginID)
	if err != nil {
		return nil, err
	}
	capsJSON, err := json.Marshal(m.Capabilities)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	hooksJSON, err := json.Marshal(m.Hooks)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	now := time.Now()
	record := &model.Plugin{
		ID:              in.PluginID,
		Name:            m.Name,
		Version:         m.Version,
		Author:          m.Author,
		Description:     m.Description,
		Manifest:        in.Manifest,
		ProtocolVersion: m.ProtocolVersion,
		Hooks:           string(hooksJSON),
		Capabilities:    string(capsJSON),
		SourceID:        src.ID,
		Enabled:         true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// 只有 runtime 注入且真的拉起成功时才写 running：没有宿主就虚报
	// running，会让重启恢复去启动一个从未装好代码的插件。
	installStatus := model.PluginStatusInstalled
	if s.runtime != nil {
		installStatus = model.PluginStatusRunning
	}
	install := &model.PluginInstall{
		ID:        pkg.NewID(),
		PluginID:  in.PluginID,
		SourceID:  src.ID,
		Dir:       dir,
		Entry:     m.Entry,
		Status:    installStatus,
		Caps:      string(capsJSON),
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 重装（同 ID）走覆盖而不是报错：插件升级是常态，要求先卸载再装会把
	// 「升级」变成两步非原子操作，中途失败会留下「已卸载未安装」。
	// 重装前先停旧进程，避免升级后旧代码还在收钩子。
	if s.runtime != nil && s.runtime.Running(in.PluginID) {
		if err := s.runtime.StopPlugin(in.PluginID); err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "停止旧插件进程失败: "+err.Error())
		}
	}

	created, err := s.upsertPluginRecord(ctx, record)
	if err != nil {
		return nil, err
	}
	if err := s.upsertInstallRecord(ctx, install); err != nil {
		// 补偿：Plugin 行是本方法写进去的，安装失败就不该留着一条
		// 「装了但没装成」的登记。spec 要求「MUST NOT 留下半安装状态」。
		if created {
			_ = s.repo.Delete(ctx, record.ID)
		}
		return nil, err
	}

	// 先落库再拉起：拉起失败要把刚写的两行一起回滚，绝不能留下
	// 「库里 running、进程根本没起来」的假状态。
	if s.runtime != nil {
		if err := s.runtime.StartPlugin(in.PluginID, m,
			plugin.CapabilitySetFromNames(m.Capabilities)); err != nil {
			_ = s.installs.Delete(ctx, in.PluginID)
			if created {
				_ = s.repo.Delete(ctx, record.ID)
			}
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "启动插件失败: "+err.Error())
		}
	}

	s.recordAudit(ctx, "system", auditActionInstall, in.PluginID,
		fmt.Sprintf("source=%s dir=%s caps=%s", src.ID, dir, string(capsJSON)))
	return install, nil
}

// upsertPluginRecord 写入或更新 plugins 行，返回是否新建。
func (s *pluginService) upsertPluginRecord(ctx context.Context, record *model.Plugin) (bool, error) {
	existing, err := s.repo.Get(ctx, record.ID)
	switch {
	case err == nil && existing != nil:
		record.CreatedAt = existing.CreatedAt
		if err := s.repo.Update(ctx, record); err != nil {
			return false, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		return false, nil
	case errors.Is(err, repository.ErrNotFound):
		if err := s.repo.Create(ctx, record); err != nil {
			return false, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		return true, nil
	default:
		return false, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
}

// upsertInstallRecord 写入或更新 plugin_installs 行。
func (s *pluginService) upsertInstallRecord(ctx context.Context, install *model.PluginInstall) error {
	existing, err := s.installs.GetByPluginID(ctx, install.PluginID)
	switch {
	case err == nil && existing != nil:
		// 安装记录的身份是 plugin_id，ID 与 CreatedAt 沿用既有行。
		install.ID = existing.ID
		install.CreatedAt = existing.CreatedAt
		if err := s.installs.Update(ctx, install); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		return nil
	case errors.Is(err, repository.ErrNotFound):
		if err := s.installs.Create(ctx, install); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		return nil
	default:
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
}

// resolveDir 决定插件代码的落盘目录。
//
// 显式传入的 dir 必须是绝对路径且不含 `..` 段：Dir 会被宿主拼成
// <dir>/<entry> 交给 node 执行，它和 manifest 的 entry 是同一类逃逸面。
func (s *pluginService) resolveDir(explicit, pluginID string) (string, error) {
	if explicit != "" {
		if !filepath.IsAbs(explicit) {
			return "", pkg.NewAppError(pkg.INVALID_PARAMS, "插件目录必须是绝对路径")
		}
		for _, seg := range strings.FieldsFunc(explicit, func(r rune) bool { return r == '/' || r == '\\' }) {
			if seg == ".." {
				return "", pkg.NewAppError(pkg.INVALID_PARAMS, "插件目录不能包含 .. 路径段")
			}
		}
		return filepath.Clean(explicit), nil
	}
	abs, err := filepath.Abs(filepath.Join(s.root, pluginID))
	if err != nil {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, "无法解析插件目录: "+err.Error())
	}
	return abs, nil
}

// Uninstall 卸载插件：先停进程，再删两张表，最后落审计。
//
// 顺序不可调换：先删库再停进程会留下「库里已消失、进程还在收钩子」的窗口；
// 反过来先停进程再删库，中途失败最多是「停了但没删干净」，可重试修复。
func (s *pluginService) Uninstall(ctx context.Context, pluginID string) error {
	if pluginID == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "插件 ID 不能为空")
	}
	if s.runtime != nil && s.runtime.Running(pluginID) {
		if err := s.runtime.StopPlugin(pluginID); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, "停止插件进程失败: "+err.Error())
		}
	}
	if err := s.installs.Delete(ctx, pluginID); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.repo.Delete(ctx, pluginID); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.recordAudit(ctx, "system", auditActionUninstall, pluginID, "")
	return nil
}

// SetRuntimeStatus 回写插件运行状态，由 supervisor 的状态回调驱动。
//
// status 非法时返回 INVALID_PARAMS 而不是静默忽略：状态列被写进一个
// 未知值时，重启恢复逻辑（只认 running）会静默不恢复，症状是「插件明明
// 在跑，重启后不见了」，而这行报错是唯一线索。
func (s *pluginService) SetRuntimeStatus(ctx context.Context, pluginID, status, lastError string) error {
	if pluginID == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "插件 ID 不能为空")
	}
	if !model.ValidPluginStatus(status) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "非法的插件状态: "+status)
	}
	if err := s.installs.UpdateStatus(ctx, pluginID, status, lastError); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// 状态回调可能来自一个已被卸载的插件，这不是调用方的错误：
			// 卸载与崩溃回调的竞态天然存在，静默忽略即可。
			return nil
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// ListResumable 返回宿主重启时需要恢复的插件。
//
// 只挑 running：crashed / stopped / installed 都不自动拉起（见
// model.PluginStatusShouldResume 的理由）。Capabilities 从安装记录的
// Caps 快照解出，**不是**重新解析 manifest——升级 manifest 不应在重启时
// 静默扩权。
func (s *pluginService) ListResumable(ctx context.Context) ([]ResumablePlugin, error) {
	rows, err := s.installs.ListByStatus(ctx, model.PluginStatusRunning)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	out := make([]ResumablePlugin, 0, len(rows))
	for _, row := range rows {
		manifest, err := s.manifestFor(ctx, row.PluginID)
		if err != nil {
			return nil, err
		}
		if manifest == "" {
			// 安装记录还在但插件登记已被删——数据不一致，跳过而不是
			// 让整个恢复流程失败。重启时少起一个插件比一个都起不来好。
			continue
		}
		var caps []string
		if row.Caps != "" {
			if err := json.Unmarshal([]byte(row.Caps), &caps); err != nil {
				return nil, pkg.NewAppError(pkg.INTERNAL_ERROR,
					fmt.Sprintf("插件 %s 的能力快照不是合法 JSON: %v", row.PluginID, err))
			}
		}
		if caps == nil {
			caps = []string{}
		}
		out = append(out, ResumablePlugin{
			PluginID:     row.PluginID,
			Manifest:     manifest,
			Capabilities: caps,
			Dir:          row.Dir,
		})
	}
	return out, nil
}

// manifestFor 取插件登记的 manifest 原文。
func (s *pluginService) manifestFor(ctx context.Context, pluginID string) (string, error) {
	record, err := s.repo.Get(ctx, pluginID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", nil
		}
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return record.Manifest, nil
}

// recordAudit 落一条审计。审计失败不阻断业务，只写警告——
// 与 service/customize.go 的 auditWrite 同一取舍：审计的价值在于事后可查，
// 不能因为审计表写失败就让已经生效的安装/审批白做。
func (s *pluginService) recordAudit(ctx context.Context, actorID, action, target, detail string) {
	if s.audit == nil {
		return
	}
	if actorID == "" {
		actorID = "system"
	}
	_ = s.audit.Record(ctx, actorID, action, target, detail)
}

func validatePluginSettings(raw string) error {
	if raw == "" {
		return nil
	}
	var target any
	if err := json.Unmarshal([]byte(raw), &target); err != nil {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "插件 settings 必须是合法 JSON")
	}
	return nil
}
