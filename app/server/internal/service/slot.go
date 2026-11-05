package service

import (
	"context"
	"encoding/json"
	"errors"

	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/repository"
)

// SlotEntryInput 是一次插槽条目的写入参数。
type SlotEntryInput struct {
	Slot       string
	Kind       string
	Props      map[string]any
	Sort       int
	Visibility string
	MinLevel   int
	// Enabled 为 nil 表示「未指定」，按启用处理。
	// 用指针而不是 bool：bool 的零值 false 与「显式关闭」无法区分，
	// 而插槽条目的默认语义是启用。
	Enabled *bool
}

// SlotService 管理壳层插槽条目。
//
// 渲染路径只读库，MUST NOT 同步调用插件——插件内容由插件在异步钩子里
// 经宿主 API 写入（见 shell-slots 规范）。
type SlotService interface {
	Create(ctx context.Context, in SlotEntryInput, actorID string) (*model.SlotEntry, error)
	Update(ctx context.Context, id string, in SlotEntryInput, actorID string) (*model.SlotEntry, error)
	Delete(ctx context.Context, id string, actorID string) error
	List(ctx context.Context, slot string) ([]model.SlotEntry, error)
	// ListForViewer 返回某访问者可见的启用条目，已按 slot + sort 升序。
	ListForViewer(ctx context.Context, isLogin bool, level int) ([]model.SlotEntry, error)
	// SetForPlugin 供插件经宿主 API 写入。同一插件重写同一插槽时是覆盖，不是追加。
	SetForPlugin(ctx context.Context, pluginID, slot, kind string, props map[string]any, sort int) error
	DeleteForPlugin(ctx context.Context, pluginID, slot string) error
	// DeleteByOwner 在插件卸载时清理它写入的全部条目。
	DeleteByOwner(ctx context.Context, pluginID string) error
}

type slotService struct {
	repo  repository.SlotEntryRepository
	audit AuditLogService
}

func NewSlotService(repo repository.SlotEntryRepository, audit AuditLogService) SlotService {
	return &slotService{repo: repo, audit: audit}
}

func (s *slotService) Create(ctx context.Context, in SlotEntryInput, actorID string) (*model.SlotEntry, error) {
	if err := validateSlotInput(&in); err != nil {
		return nil, err
	}
	e := &model.SlotEntry{
		ID:         pkg.NewID(),
		Slot:       in.Slot,
		Kind:       in.Kind,
		Props:      marshalProps(in.Props),
		Sort:       in.Sort,
		Visibility: in.Visibility,
		MinLevel:   in.MinLevel,
		Enabled:    enabledOr(in.Enabled),
	}
	if err := s.repo.Create(ctx, e); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.recordAudit(ctx, actorID, "slot.create", e.ID, e.Slot+"/"+e.Kind)
	return e, nil
}

func (s *slotService) Update(ctx context.Context, id string, in SlotEntryInput, actorID string) (*model.SlotEntry, error) {
	if err := validateSlotInput(&in); err != nil {
		return nil, err
	}
	e, err := s.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "插槽条目不存在")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	e.Slot = in.Slot
	e.Kind = in.Kind
	e.Props = marshalProps(in.Props)
	e.Sort = in.Sort
	e.Visibility = in.Visibility
	e.MinLevel = in.MinLevel
	e.Enabled = enabledOr(in.Enabled)
	if err := s.repo.Update(ctx, e); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.recordAudit(ctx, actorID, "slot.update", e.ID, e.Slot+"/"+e.Kind)
	return e, nil
}

func (s *slotService) Delete(ctx context.Context, id, actorID string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.recordAudit(ctx, actorID, "slot.delete", id, "")
	return nil
}

func (s *slotService) List(ctx context.Context, slot string) ([]model.SlotEntry, error) {
	if slot != "" && !model.ValidSlot(slot) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "未知插槽: "+slot)
	}
	items, err := s.repo.List(ctx, slot)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if items == nil {
		items = []model.SlotEntry{}
	}
	return items, nil
}

func (s *slotService) ListForViewer(ctx context.Context, isLogin bool, level int) ([]model.SlotEntry, error) {
	items, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	out := make([]model.SlotEntry, 0, len(items))
	for _, e := range items {
		// 未知插槽名不下发：前端只认枚举，多传只会让客户端困惑。
		if !model.ValidSlot(e.Slot) {
			continue
		}
		if !e.VisibleTo(isLogin, level) {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *slotService) SetForPlugin(ctx context.Context, pluginID, slot, kind string, props map[string]any, sort int) error {
	yes := true
	in := SlotEntryInput{Slot: slot, Kind: kind, Props: props, Sort: sort,
		Visibility: model.NavVisibilityPublic, Enabled: &yes}
	if err := validateSlotInput(&in); err != nil {
		return err
	}
	// 覆盖语义：同一插件对同一插槽只保留最新一次写入的内容。
	// 追加会让 cron.daily 每跑一次就多一条，几天后插槽被同一插件刷满。
	if err := s.repo.DeleteBySlotAndOwner(ctx, slot, pluginID); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	e := &model.SlotEntry{
		ID:            pkg.NewID(),
		Slot:          slot,
		Kind:          kind,
		Props:         marshalProps(props),
		Sort:          sort,
		Visibility:    model.NavVisibilityPublic,
		Enabled:       true,
		OwnerPluginID: pluginID,
	}
	if err := s.repo.Create(ctx, e); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.recordAudit(ctx, pluginID, "slot.set", e.ID, slot+"/"+kind)
	return nil
}

func (s *slotService) DeleteForPlugin(ctx context.Context, pluginID, slot string) error {
	if err := s.repo.DeleteBySlotAndOwner(ctx, slot, pluginID); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	s.recordAudit(ctx, pluginID, "slot.delete", slot, "")
	return nil
}

func (s *slotService) DeleteByOwner(ctx context.Context, pluginID string) error {
	// 逐个插槽清：repo 的接口按 (slot, owner) 删，这里遍历枚举。
	// 插槽只有 11 个，比加一个 DeleteByOwner 的 SQL 方法更省——且不会漏掉新枚举。
	for _, slot := range model.AllSlots {
		if err := s.repo.DeleteBySlotAndOwner(ctx, slot, pluginID); err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}
	return nil
}

func (s *slotService) recordAudit(ctx context.Context, actorID, action, target, detail string) {
	if s.audit == nil {
		return
	}
	_ = s.audit.Log(ctx, actorID, action, target, detail)
}

// validateSlotInput 校验插槽写入。裸 HTML 在这一层被拒——
// 它是 shell-slots 规范里唯一「不可协商」的类型限制。
func validateSlotInput(in *SlotEntryInput) error {
	if !model.ValidSlot(in.Slot) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "未知插槽: "+in.Slot)
	}
	if !model.ValidSlotKind(in.Kind) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "不支持的条目类型: "+in.Kind)
	}
	if in.Visibility == "" {
		in.Visibility = model.NavVisibilityPublic
	}
	if !model.ValidNavVisibility(in.Visibility) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "不支持的可见性: "+in.Visibility)
	}
	if in.Visibility == model.NavVisibilityLevel && in.MinLevel <= 0 {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "等级可见性必须指定 min_level")
	}
	// 文本类条目必须有内容，否则渲染出一个空节点。
	switch in.Kind {
	case model.SlotKindText:
		if v, _ := in.Props["text"].(string); v == "" {
			return pkg.NewAppError(pkg.INVALID_PARAMS, "text 条目必须提供 props.text")
		}
	case model.SlotKindLink:
		if v, _ := in.Props["href"].(string); v == "" {
			return pkg.NewAppError(pkg.INVALID_PARAMS, "link 条目必须提供 props.href")
		}
	}
	return nil
}

// enabledOr 把「未指定」当作启用。插槽条目的默认语义是可见，
// 与 nav_items 的 default:true 一致。
func enabledOr(p *bool) bool {
	if p == nil {
		return true
	}
	return *p
}

func marshalProps(p map[string]any) string {
	if p == nil {
		return "{}"
	}
	raw, err := json.Marshal(p)
	if err != nil {
		// props 来自已通过 handler 的 JSON 解码，理论上不会失败。
		return "{}"
	}
	return string(raw)
}
