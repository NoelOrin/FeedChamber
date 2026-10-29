# 文档目录（`docs/`）

架构、设计、计划、评审与 API 参考。**规范真相源在 `openspec/specs/`**，这里是说明性与历史性材料。

> 上级：[根 AGENTS.md](../AGENTS.md) · 入口：[ARCHITECTURE.md](ARCHITECTURE.md) · 规范：[openspec/AGENTS.md](../openspec/AGENTS.md)

## 入口与结构

| 路径 | 说明 |
|------|------|
| `ARCHITECTURE.md` | **架构入口**：分层心智模型、层表（含各层 `AGENTS.md` 链接）、关键设计决策速查 |
| `system-map.md` | 跨栈现状地图：后端/Web/共享包/部署 + 分层与数据流图 + §2.3 目录与职责表 |
| `backend-overview.md` | 后端实测梳理：规模、模块清单、网关层、现状偏差（发现项编号 G*/R*/H*） |
| `api-reference.md` | 端点清单（按模块分组、含权限码）。权威来源是根 `openapi.json` 与运行时 `/swagger/` |
| `branching.md` | 分支管理规范（`main` + `develop`，短命分支命名，保护规则） |
| `adr/` | 架构决策记录（`chat-mvp.md`、`no-otel-sdk-yet.md`） |
| `design/` | 设计文档（`session-auth.md`、`auth-gateway-plan.md`、`nats-deploy.md`、`import-discourse.md` 等） |
| `plans/` | 计划与基线（`discourse-baseline.md` 等） |
| `reviews/` | 评审与复盘（`code-review-*`、`security-audit-*`、`observability-findings-*`、`phase4-discourse-alignment-retro.md`） |
| `02-设计/` | 中文设计稿（权限体系、版面级禁言、shadowban 开发计划） |
| `superpowers/` | **不进版本控制**（`.gitignore` 覆盖），仅本地工作区 |

## 写作约定

- **规范 vs 说明**：能成为强制约束的写进 `openspec/specs/`；`docs/` 只写「为什么这样设计」「当前实现长什么样」「历史怎么走的」。
- **不复制源码清单**：层文档（各 `AGENTS.md`）里已说明「方法签名以源码为准」。`docs/` 同样不维护逐方法清单——过期的清单比没有更误导。
- **数字要实测**：写「N 个文件 / M 行 / K 条路由」前先数一遍。历史上权限码数量在根 AGENTS.md（53）、ARCHITECTURE.md（54/56）、`model/permission.go` 之间长期不一致。
- **死链是缺陷**：改文件名或删文件时全仓 grep 引用。2026-10-28 清理过一次（`docs/design/*`、`docs/plans/*` 的一批历史文档被删除，引用同步移除）。
- **中文文档排版**：中英文之间加空格，代码/路径/命令用反引号，链接用相对路径（便于 GitHub 与编辑器跳转）。

## 改这里要注意

- **`ARCHITECTURE.md` 与 `system-map.md` 的层表是文档枢纽**：新增一层或一个模块文档时，这两处的表要同步加链接。`internal/gateway` 此前就是「见本文 §3.2」的断链状态，现已补 `gateway/AGENTS.md`。
- **`api-reference.md` 不是权威**：改端点后重跑 `make swag`，`openapi.json` 才是事实来源。
- **`backend-overview.md` 的发现项编号（G/R/H）被代码注释与 change 提案引用**，不要重排编号。
- 删文档前先确认没有引用（`grep -rn "<文件名>" --include="*.md"`），并同步 `docs/AGENTS.md` 与本文件。

## 本文件何时更新

- 新增/删除一个子目录或入口文档
- 写作约定变化
- 文档的权威性边界变化（例如某类规范迁进 openspec）
