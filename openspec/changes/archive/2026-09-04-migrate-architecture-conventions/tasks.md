## 1. 初始化 OpenSpec

- [x] 已在仓库根执行 `openspec init --tools codex,agents`，生成 `openspec/config.yaml`（schema: spec-driven）与 `.agents/skills/` 下 4 个 openspec 技能。

## 2. 编写迁移提案与各 capability spec

- [x] 创建 change `migrate-architecture-conventions` 的 `proposal.md`，声明 6 个新 capability。
- [x] 编写 `specs/api-style/spec.md` — RPC 风格 API 约定。
- [x] 编写 `specs/layered-architecture/spec.md` — 单向分层依赖链。
- [x] 编写 `specs/backend-service-design/spec.md` — 关键服务设计约束。
- [x] 编写 `specs/module-conventions/spec.md` — 模块文件组织与新增模块清单。
- [x] 编写 `specs/frontend-packages/spec.md` — 前端共享包契约。
- [x] 编写 `specs/operations/spec.md` — 运行、配置与测试流程。

## 3. 校验与收敛

- [x] 运行 `openspec validate migrate-architecture-conventions` 确认 change 合规（无缺 artifact、spec 格式正确）。
- [ ] 运行 `openspec archive migrate-architecture-conventions` 将 specs 合并进主 `openspec/specs/`，使规范正式生效。
- [ ] 更新根 `AGENTS.md` 与 `docs/ARCHITECTURE.md`，增加指向 `openspec/specs/` 的导航说明（规范冲突以 specs 为准）。
- [ ] 提交并推送到 `master`。
