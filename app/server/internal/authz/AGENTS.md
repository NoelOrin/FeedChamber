# Authz 权限判定（`internal/authz/`）

「一个凭证 + 一个权限码」的唯一判定点。它不持有状态、不读配置、不依赖 service 层。

## 为什么存在

同一份判定此前散在三处，且三处语义各不相同：

| 位置 | 旧判据 | 后果 |
|------|--------|------|
| `gateway/permission.go` | api_key = scope **且** 角色 | 正确基线 |
| `middleware.RequirePermission` | 只查角色权限表 | API Key 的 scope 上限被忽略 |
| `internal/mcp/enforcer.go` | api_key = scope **或** 角色 | 比网关宽一格：read-only Key 能调写工具 |

## 判据（`Allowed`）

| 凭证来源 | 判据 |
|----------|------|
| 会话（`Method == ""`） | 只看角色权限表 |
| `pkg.AuthMethodAPIKey` | scope **且** 角色——scope 是凭证上限 |
| `pkg.AuthMethodOAuthClient` | 只认 scope（客户端不是用户，没有角色） |
| 空权限码 | 登录即可，恒放行 |
| 未知来源 | 拒绝（新增来源必须显式实现） |

## Scope 注册表

`scopeRegistry` 把粗粒度 scope（`read` / `write` / `admin`）展开成精确权限码；精确权限码也可以直接当 scope 用。

**`customize:*` 刻意不进任何粗粒度 scope**：并进去等于让所有存量持该 scope 的 Key 在部署瞬间获得向全站注入 JS 的能力。`internal/mcp/scope_test.go` 用源码扫描钉住这条约束（测试路径已从 `gateway/apikey.go` 改为本包）。

`customize:publish` 更进一步：它只由 `ScopeHas` 判定，永不走角色继承——admin 角色默认持有它，继承会让「admin 创建的任意 Key」都具备让 JS 直接生效的能力。

## 文件

| 文件 | 职责 |
|------|------|
| `authz.go` | `Allowed` / `ScopeHas` / `Expand` / `Normalize(List)` / `ValidScopeString` / `KnownScopes` / `CredentialFrom` |
| `authz_test.go` | 六种凭证来源的判据 + 粗粒度展开 + 未知来源拒绝 |

`CredentialFrom(values)` 只依赖 `ValueGetter`（`GetString(key) string`，`ctx.Values()` 即满足），因此本包不 import web 框架。

## 改这里要注意

- 新增凭证来源：必须在 `Allowed` 里显式加分支，否则默认拒绝（这是有意的 fail-closed）。
- 新增粗粒度 scope：同步 `internal/mcp/scope_test.go` 对 `customize:*` 的断言与后台 scope 说明。
- 改判据等于同时改网关、route 级 `RequirePermission`、MCP 三处的行为——先看 `authz_test.go` 的语义表。
