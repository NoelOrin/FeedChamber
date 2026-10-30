package service

import (
	"sort"
	"strings"
)

// RolePermissionTopUp 计算「升级时该给哪些角色补哪些权限码」。
//
// 背景：R-04 要求首次 seed 之后不再覆盖管理员对内置角色的自定义，因此
// 默认映射只在首启全量同步。但版本升级会新增权限码（例如 customize:*），
// 已有部署的管理员角色拿不到它们，表现为「功能不出现且无任何报错」。
//
// 朴素的补齐（每次启动把默认映射里缺的都加回去）会破坏 R-04 的另一半：
// 管理员主动撤销的默认权限会在下次重启静默复活。所以补齐必须**只补新码**：
// applied 是「历史上已经应用过的权限码」集合，只有不在其中的码才补，
// 补完后由调用方把新码并入 applied 持久化。
//
// 纯函数：不碰 IO，便于用表驱动测试钉住「撤销不复活」这条语义。
func RolePermissionTopUp(
	defaults map[string][]string,
	applied map[string]struct{},
) (perRole map[string][]string, newlyApplied []string) {
	perRole = map[string][]string{}
	seen := map[string]struct{}{}
	for roleName, codes := range defaults {
		var fresh []string
		for _, code := range codes {
			if _, ok := applied[code]; ok {
				continue
			}
			// 同一个码可能出现在多个角色上，去重后统一记入 newlyApplied。
			if _, ok := seen[code]; ok {
				fresh = append(fresh, code)
				continue
			}
			seen[code] = struct{}{}
			fresh = append(fresh, code)
		}
		if len(fresh) > 0 {
			perRole[roleName] = fresh
		}
	}
	for code := range seen {
		newlyApplied = append(newlyApplied, code)
	}
	sort.Strings(newlyApplied)
	return perRole, newlyApplied
}

// MergeAppliedCodes 把新应用的权限码并入既有集合并序列化为稳定的逗号串。
func MergeAppliedCodes(applied map[string]struct{}, newlyApplied []string) string {
	out := make([]string, 0, len(applied)+len(newlyApplied))
	for code := range applied {
		out = append(out, code)
	}
	out = append(out, newlyApplied...)
	sort.Strings(out)
	// 去重：新码可能已存在于 applied（调用方传入了重复项时）。
	dedup := out[:0]
	for i, code := range out {
		if i > 0 && out[i-1] == code {
			continue
		}
		dedup = append(dedup, code)
	}
	return strings.Join(dedup, ",")
}

// ParseAppliedCodes 解析 meta 里存的逗号串。
func ParseAppliedCodes(raw string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		if code := strings.TrimSpace(part); code != "" {
			out[code] = struct{}{}
		}
	}
	return out
}

// AllPermissionCodes 返回默认权限定义的全部权限码（首启标记用）。
func AllPermissionCodes(codes []string) []string {
	out := append([]string(nil), codes...)
	sort.Strings(out)
	return out
}
