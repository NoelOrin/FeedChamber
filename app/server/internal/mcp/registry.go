package mcp

import (
	"sort"
	"strings"
	"sync"
)

// Registry 是工具注册表，含工具级启停状态。
//
// 写类工具默认停用：新增一个能改数据的工具时，默认值必须是「关」——
// 让站长显式开启，而不是部署后立刻多出一个可被 AI 调用的写入口。
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*Tool
	// disabled 是被显式停用的工具名集合。默认停用集来自 defaultDisabled。
	disabled map[string]struct{}
}

// defaultDisabledTools 是写类工具的默认停用集。
//
// 用「默认停用集」而不是「默认启用集」的原因：新增工具时忘记登记会落到
// 「没被停用 = 已启用」，对读工具是想要的，对写工具是危险的。因此写工具
// 必须在注册时显式声明 Write=true，并在 Register 里自动进停用集。
func NewRegistry() *Registry {
	return &Registry{
		tools:    make(map[string]*Tool),
		disabled: make(map[string]struct{}),
	}
}

// Register 登记一个工具。写类工具自动进入停用集。
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := t
	r.tools[t.Name] = &cp
	if t.Write {
		r.disabled[t.Name] = struct{}{}
	}
}

// Get 取工具定义。
func (r *Registry) Get(name string) (*Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// Enabled 判定工具当前是否启用。
func (r *Registry) Enabled(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, off := r.disabled[name]
	_, exists := r.tools[name]
	return exists && !off
}

// SetEnabled 启停一个工具。
func (r *Registry) SetEnabled(name string, enabled bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tools[name]; !ok {
		return false
	}
	if enabled {
		delete(r.disabled, name)
	} else {
		r.disabled[name] = struct{}{}
	}
	return true
}

// List 返回通过过滤的、当前启用的工具。
func (r *Registry) List(filter func(Tool) bool) []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.tools))
	for name, t := range r.tools {
		if _, off := r.disabled[name]; off {
			continue
		}
		if filter != nil && !filter(*t) {
			continue
		}
		out = append(out, *t)
	}
	return out
}

// All 返回全部工具（含停用的），供后台管理页展示。
func (r *Registry) All() []ToolState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ToolState, 0, len(r.tools))
	for name, t := range r.tools {
		_, off := r.disabled[name]
		out = append(out, ToolState{
			Name:        t.Name,
			Description: t.Description,
			Perm:        t.Perm,
			Write:       t.Write,
			Enabled:     !off,
		})
	}
	return out
}

// ToolState 是后台展示用的工具状态。
type ToolState struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Perm        string `json:"perm,omitempty"`
	Write       bool   `json:"write"`
	Enabled     bool   `json:"enabled"`
}

// ApplyDisabledList 用逗号分隔的停用名单同步内存状态。
//
// 持久化语义（设置 key mcp_disabled_tools）：存的是「相对默认的覆盖」，
// 而不是完整启停表——
//   - 名单里出现读工具名  → 停用该读工具；
//   - 名单里出现写工具名  → 该写工具保持/变为停用；
//   - 名单以 "-" 前缀出现（如 "-create_post"）→ 启用该写工具。
//
// 为什么用负号而不是「完整启用名单」：写工具的安全默认值是「关」，
// 完整名单一旦被误清空（例如后台保存了空表单），所有写工具会一起打开。
// 负号表达的是「显式开启某一项」，误清空只会回到全关。
func (r *Registry) ApplyDisabledList(raw string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// 先恢复默认：写工具停用，读工具启用。
	r.disabled = make(map[string]struct{})
	for name, t := range r.tools {
		if t.Write {
			r.disabled[name] = struct{}{}
		}
	}
	for _, item := range splitScopes(raw) {
		enable := false
		name := item
		if strings.HasPrefix(item, "-") {
			enable = true
			name = strings.TrimPrefix(item, "-")
		}
		t, ok := r.tools[name]
		if !ok {
			continue
		}
		if enable {
			delete(r.disabled, name)
			continue
		}
		_ = t
		r.disabled[name] = struct{}{}
	}
}

// DisabledList 把当前停用状态序列化为持久化字符串。
//
// 写工具若被显式启用，输出 "-<name>"；被停用的读工具输出 "<name>"；
// 写工具的默认停用不输出（省得名单越滚越长，也避免与「显式停用」混淆）。
func (r *Registry) DisabledList() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	parts := make([]string, 0, len(r.disabled))
	for name := range r.disabled {
		// 写工具的停用是默认值，不必写进名单。
		if t, ok := r.tools[name]; ok && t.Write {
			continue
		}
		parts = append(parts, name)
	}
	// 已启用的写工具需要负号登记。
	for name, t := range r.tools {
		if !t.Write {
			continue
		}
		if _, off := r.disabled[name]; !off {
			parts = append(parts, "-"+name)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}
