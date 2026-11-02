## 改了什么

<!-- 一段话说清变更本身，不要复述 diff。 -->

## 为什么

<!-- 关联 issue：Closes #123。没有 issue 时说明触发这次改动的场景。 -->

## 怎么验证

<!-- 贴命令与输出，或说明手工路径。 -->

```bash
cd app/server && go test ./...
pnpm check
```

## 检查清单

- [ ] 分支与提交符合 [CONTRIBUTING.md](../CONTRIBUTING.md)
- [ ] `gofmt` / `go vet` / `go test` 通过
- [ ] `pnpm check` 通过
- [ ] API 变更已 `make swag` 并提交 `openapi.json`
- [ ] 更新了受影响的 `AGENTS.md` 与 `app/docs` 页面
- [ ] 涉及破坏性变更时，在描述里写明迁移与回滚方式

## 截图 / 录屏

<!-- UI 变更请附前后对比；后端变更可删掉本节。 -->
