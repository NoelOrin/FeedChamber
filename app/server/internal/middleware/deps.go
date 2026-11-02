package middleware

import (
	"feedchamber/internal/pkg"
	"feedchamber/internal/pkg/ratelimit"
)

// Deps holds all dependencies needed by middleware factories.
// Construct once at startup and pass to router.Register().
// The Ban and Perms values are also handed to internal/gateway when it is built
// (see router.Register), so a single service instance serves both layers.
//
// 三个能力接口（SettingReader / PermissionChecker / BannedChecker）统一定义在 internal/pkg/seams.go，
// 与 internal/gateway 共用同一份声明——两个包此前各写一套同形接口，
// SettingReader 逐字相同、PermissionChecker 仅参数名不同、
// BanChecker 与 BannedChecker 一字之差且方法集不同，读代码要靠后缀猜。
type Deps struct {
	Settings pkg.SettingReader
	Perms    pkg.PermissionChecker
	Ban      pkg.BannedChecker
	Limiter  *ratelimit.Limiter
}
