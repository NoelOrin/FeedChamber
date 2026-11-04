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
	"sort"
)

// ProtocolVersion1 是当前唯一支持的插件协议版本。
// manifest 的 protocol_version 不在此集合内的一律拒绝加载。
const ProtocolVersion1 = "1"

// 钩子名常量。插件只能实现这里列出的钩子；未知名在解析时被丢弃。
const (
	HookPostCreated       = "post.created"
	HookPostUpdated       = "post.updated"
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
	HookPostUpdated:       {},
	HookPostClosed:        {},
	HookCommentCreated:    {},
	HookUserRegistered:    {},
	HookUserFollowed:      {},
	HookReportCreated:     {},
	HookModerationDecided: {},
	HookCronDaily:         {},
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

	filtered := make([]string, 0, len(m.Hooks))
	for _, h := range m.Hooks {
		if _, ok := knownHooks[h]; ok {
			filtered = append(filtered, h)
		}
	}
	m.Hooks = filtered
	return &m, nil
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
