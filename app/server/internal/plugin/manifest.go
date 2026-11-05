// Package plugin 托管以 Node 子进程运行的第三方插件。
//
// 设计约束（见 openspec/changes/2026-11-04-plugin-hooks）：
//   - 插件进程由本包拉起，通过 stdio JSON-RPC 通信；
//   - 插件不加载进宿主进程，崩溃不影响宿主；
//   - 本包不引用 internal/repository，不持有 *gorm.DB。
package plugin

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// ProtocolVersion1 是当前唯一支持的插件协议版本。
// manifest 的 protocol_version 不在此集合内的一律拒绝加载。
const ProtocolVersion1 = "1"

// 钩子名常量。插件只能实现这里列出的钩子；未知名在解析时被丢弃。
//
// 钩子名**不保证**与 eventbus 的事件名相同——见 hookEvents。post.updated
// 已删除：它没有任何事件源，留着只会让插件声明一个永远不触发的钩子。
const (
	HookPostCreated       = "post.created"
	HookPostClosed        = "post.closed"
	HookCommentCreated    = "comment.created"
	HookUserRegistered    = "user.registered"
	HookUserFollowed      = "user.followed"
	HookReportCreated     = "report.created"
	HookModerationDecided = "moderation.decided"
	HookCronDaily         = "cron.daily"
)

// knownHooks 是核心钩子白名单。
var knownHooks = map[string]struct{}{
	HookPostCreated:       {},
	HookPostClosed:        {},
	HookCommentCreated:    {},
	HookUserRegistered:    {},
	HookUserFollowed:      {},
	HookReportCreated:     {},
	HookModerationDecided: {},
	HookCronDaily:         {},
}

// hookEvents 是钩子 → eventbus 事件类型常量的绑定表。
//
// 刻意不写成同名恒等映射：钩子名是给插件作者看的「语义名」，事件名是站点
// 内部的传输标识，两者会分叉。分叉处在这张表里显式记录，Task 5 必须查表，
// 不得按字符串拼接推导。
//
// 不含 cron.daily：它由 ticker 驱动，不经过事件总线（用 HookIsEventDriven 判定）。
//
// user.registered 绑定的是**别名事件** `user.registered` 本身（而非
// `user.created`）：Auth 在注册成功时同时发布 user.created 与 user.registered
// 两条事件（见 internal/service/auth.go）。若这里绑成 user.created，Task 5
// 按事件反查钩子时会同时命中两条发布，同一插件的同一次注册会被调用两次。
// 绑定别名事件，两条发布中恰好只有一条命中本钩子，天然不重复。
var hookEvents = map[string]string{
	HookPostCreated:       "post.created",
	HookPostClosed:        "post.closed",
	HookCommentCreated:    "content.created",
	HookUserRegistered:    "user.registered",
	HookUserFollowed:      "user.followed",
	HookReportCreated:     "report.created",
	HookModerationDecided: "moderation.decided",
}

// KnownHooks 返回全部核心钩子名（排序后），供后台与文档使用。
func KnownHooks() []string {
	out := make([]string, 0, len(knownHooks))
	for h := range knownHooks {
		out = append(out, h)
	}
	sort.Strings(out)
	return out
}

// HookEvents 返回「事件驱动」钩子到 eventbus 事件类型的映射（副本，调用方可随意修改）。
// 不含 cron.daily —— 它由 ticker 触发，不走事件总线。
func HookEvents() map[string]string {
	out := make(map[string]string, len(hookEvents))
	for h, e := range hookEvents {
		out[h] = e
	}
	return out
}

// HookIsEventDriven 报告某个钩子是否由事件总线驱动。
// false 涵盖两类：cron.daily（ticker 驱动）与任何未知钩子名。
func HookIsEventDriven(hook string) bool {
	_, ok := hookEvents[hook]
	return ok
}

// Manifest 是 plugin.json 的解析结果。
type Manifest struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	ProtocolVersion string `json:"protocol_version"`
	Description     string `json:"description,omitempty"`
	Author          string `json:"author,omitempty"`
	// Entry 是入口文件名，默认 index.js。
	Entry        string   `json:"entry,omitempty"`
	Hooks        []string `json:"hooks"`
	Capabilities []string `json:"capabilities"`
}

// ParseManifest 解析并校验 plugin.json。
//
// 未知钩子被丢弃而不是报错：核心新增钩子是常事，旧插件不该因此被拒。
// 反过来，未知 capability 不丢弃——capability 决定权限，写错就该报错。
func ParseManifest(raw []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("plugin manifest 不是合法 JSON: %w", err)
	}
	if m.Name == "" {
		return nil, fmt.Errorf("plugin manifest 缺少 name")
	}
	if m.Version == "" {
		return nil, fmt.Errorf("plugin manifest 缺少 version")
	}
	if m.ProtocolVersion != ProtocolVersion1 {
		return nil, fmt.Errorf("plugin protocol_version 不支持: %q（需要 %q）", m.ProtocolVersion, ProtocolVersion1)
	}
	if m.Entry == "" {
		m.Entry = "index.js"
	}
	if err := validateEntry(m.Entry); err != nil {
		return nil, err
	}

	filtered := make([]string, 0, len(m.Hooks))
	for _, h := range m.Hooks {
		if _, ok := knownHooks[h]; ok {
			filtered = append(filtered, h)
		}
	}
	m.Hooks = filtered
	return &m, nil
}

// validateEntry 校验 entry 是插件目录内的相对路径。
//
// entry 在 supervisor 里会拼成 <插件目录>/<entry> 交给 node 执行，
// 因此它一旦能逃出插件目录，manifest 就成了任意文件执行入口。
// 拒绝三类：绝对路径、含 ".." 路径段、trim 后为空。
func validateEntry(entry string) error {
	if strings.TrimSpace(entry) == "" {
		return fmt.Errorf("plugin manifest 的 entry 不能为空")
	}
	if filepath.IsAbs(entry) {
		return fmt.Errorf("plugin manifest 的 entry 不能是绝对路径: %q", entry)
	}
	// 按 / 与 \ 双分隔符切分：插件包可能在 Windows 上被打包，
	// 单看 / 会让 "..\..\x" 逃逸。
	for _, seg := range strings.FieldsFunc(entry, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return fmt.Errorf("plugin manifest 的 entry 不能包含 .. 路径段: %q", entry)
		}
	}
	return nil
}

// Validate 校验 capability 是否合法。单独一个方法是因为 capability
// 需要与站点已知的能力集比对，而能力集由 api.go 提供（避免本文件反向依赖）。
//
// knownCaps 为空（含 nil）时任何 capability 都无法匹配，因此一律报错——
// capability 决定权限，宁可拒绝加载也不能放行一个无从校验的声明。
func (m *Manifest) Validate(knownCaps map[string]struct{}) error {
	for _, c := range m.Capabilities {
		if _, ok := knownCaps[c]; !ok {
			return fmt.Errorf("未知 capability: %q", c)
		}
	}
	if len(m.Hooks) == 0 {
		return fmt.Errorf("插件至少需要实现一个钩子")
	}
	return nil
}
