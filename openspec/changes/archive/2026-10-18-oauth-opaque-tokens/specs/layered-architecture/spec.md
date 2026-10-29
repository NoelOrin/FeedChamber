# layered-architecture Specification (delta)

## MODIFIED Requirements

### Requirement: 包位置按消费者边界选择

后端 Go 包 MUST 位于 `app/server/internal/` 下；`app/server/pkg/` 只允许存放确有模块外 import 方的包。判据是实测的 import 关系，不是"将来可能被复用"的意图——同模块内 `internal/` 与 `pkg/` 的可达范围完全相同，把只被内部消费的包放在 `pkg/` 不带来任何收益，只让 `internal/pkg`（通用工具）与 `pkg`（外部候选）两个名字互相混淆。

网关层据此于 2026-10-14 由 `pkg/gateway` 迁入 `internal/gateway`（迁移时实测消费者只有 `server/iris.go` 与 `internal/router/router.go`）。该迁移只改变"谁能 import"的声明位置，不改变包名、符号、文件划分或运行结构。

本条此前的唯一例外 `app/server/pkg/jwt` 已不复存在：该包于 2026-10-18 随 OAuth2/OIDC access token 的 opaque 化整包删除，其中唯一被规范强制保留的 JWT 用途——OIDC `id_token` 的 RS256 签发与签名密钥管理——已内联进 `internal/gateway/sso.go`。因此 `app/server/pkg/` 目录下 MUST NOT 再存在任何包；将来若确实出现模块外 import 方，再按本条重新申请位置，不以"曾经有过"为据。

#### Scenario: 新包默认落在 internal

- WHEN 新增一个只被本模块消费的包
- THEN MUST 建在 `app/server/internal/` 下，MUST NOT 放入 `app/server/pkg/`

#### Scenario: 放进 pkg/ 需给出消费者证据

- WHEN 某包申请位于 `app/server/pkg/` 下
- THEN MUST 能指出至少一个 `feedchamber` 模块之外的 import 方；指不出时评审 MUST 要求移入 `internal/`

#### Scenario: 现存例外已清空

- WHEN 审查 `app/server/pkg/` 的目录内容
- THEN MUST 为空（或不存在）；任何重新出现在此处的包 MUST 附模块外消费者证据，否则评审 MUST 要求移入 `internal/`
