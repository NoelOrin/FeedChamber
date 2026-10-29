package pkg

import "context"

// 跨包接缝（seam）接口：描述「网关层 / 中间件层需要业务层提供什么」，
// 由 service 层的具体实现满足。
//
// 为什么放在 pkg：此前 internal/gateway 与 internal/middleware 各自声明了一整套
// 同形接口——SettingReader 逐字相同、PermissionChecker 仅参数名不同、
// BanChecker 与 BannedChecker 一字之差且方法集不同（前者只有 IsBanned）。
// 两个 Deps 由 server/iris.go 用同一批具体服务分别装配，接口之间没有任何值传递点，
// 于是重复声明的代价是「读代码要靠后缀猜」，且接错线只在运行期暴露。
//
// 这些接口只描述能力、不引用 service 包，因此不会让 pkg 反向依赖业务层
// （pkg 是本仓库唯一的叶子工具包，被所有层引用）。
//
// 与之配套的先例是 ctxkey.go：跨包契约的常量也放在这里。
type SettingReader interface {
	Get(ctx context.Context, key string) (string, error)
}

type PermissionChecker interface {
	HasPermission(role, code string) bool
	HasPermissionInSpace(role, spaceID, code string) bool
}

// BannedChecker 是禁言判定的完整能力：全局与版面两级。
// 只做全局判定的调用方（网关 banCheck）用同一个接口即可，不需要第二份窄接口。
type BannedChecker interface {
	IsBanned(ctx context.Context, userID string) bool
	IsBannedInSpace(ctx context.Context, userID, spaceID string) bool
}
