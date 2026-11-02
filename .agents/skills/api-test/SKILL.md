---
name: api-test
description: Use when asked to test Feed Chamber APIs, run a regression / smoke test, or verify backend behaviour end to end. Runs the repo's feedchamber_test.sh against a locally started server and writes a Markdown report to agent_test_logs/.
---

# API Test — Feed Chamber

## 描述

使用项目根目录的 `feedchamber_test.sh` 脚本进行 API 接口测试，并将测试结果以 Markdown 格式保存到 `agent_test_logs/` 文件夹（已纳入 `.gitignore`）。

## 触发条件

- 用户要求测试 API
- 用户要求测试功能
- 用户要求验证系统行为
- 用户要求执行测试用例
- 用户要求全量测试 / 接口回归测试 / smoke test

## 测试脚本

项目根目录的 `feedchamber_test.sh` 是唯一的 API 测试入口，支持模块化测试。默认 `BASE_URL=http://localhost:8080/api/v1`。

### 启动服务器

```bash
cd app/server
DB_TYPE=sqlite DB_DSN=/tmp/of_test.db go run main.go -e dev &
sleep 4
```

建议使用干净的 SQLite 数据库文件以获得稳定的测试结果。脚本用 `admin` / `admin123` 登录；若 `ADMIN_PASSWORD` 设过别的值，请用全新库或先确认密码。

### 运行全量测试

```bash
bash feedchamber_test.sh -all      # 等价于 -a，或不带参数
```

### 按模块测试

flag 与 `feedchamber_test.sh` 的 `case "$arg" in` 分发一一对应（别名一并列出）：

| 参数 | 测试内容 |
|------|---------|
| `-public` | 公共端点：健康检查、公共设置、标签列表、帖子列表/搜索 |
| `-auth` | 认证 + 认证流程（注册、登录、刷新、登出、改密） |
| `-post` | 帖子：公开列表/详情 + CRUD、评论、点赞、收藏 |
| `-user` | 用户：个人资料、搜索、列表、角色修改、删除 |
| `-role` | 角色 & 权限：列表、查询、同步 |
| `-tag` / `-tags` | 标签 CRUD |
| `-follow` | 关注/粉丝 |
| `-fav` / `-favorite` | 收藏列表 |
| `-notif` / `-notification` | 通知列表/计数/已读、SSE 通知流 |
| `-upload` | 上传文件列表、预签名、文件查询/删除 |
| `-report` / `-reports` | 举报创建/列表/状态更新/删除 |
| `-admin` | 统计信息、系统设置 CRUD、用户活动、全局评论列表 |
| `-space` / `-spaces` | 版面列表/详情 |
| `-spaces-crud` | 版面 CRUD（创建/更新/激活/删除） |
| `-role-crud` | 角色 CRUD（创建/重复创建检测/删除） |
| `-ban` | 禁言系统（全局/空间禁言、状态、解除、管理列表） |
| `-announce` | 公告（创建/发布/获取/更新/取消发布/删除） |
| `-checkin` | 签到/状态/历史 |
| `-admin-extras` | Dashboard、管理搜索、批量操作用户/帖子/评论 |
| `-oauth` | OAuth 账号管理 |

> 分类（Category）模块已下线，`-cat` 不存在；版面的能力由 `-space` / `-spaces-crud` 覆盖。

### 组合测试

```bash
bash feedchamber_test.sh -public -auth -post
```

### 使用自定义 BASE_URL

```bash
BASE_URL=http://staging:8080/api/v1 bash feedchamber_test.sh -all
```

### 新增接口时的处理

1. 打开项目根目录 `feedchamber_test.sh`
2. 找到对应模块函数或在文件末尾新增函数
3. 使用 `check` 函数添加测试项（格式：`check "描述" METHOD "/path" '{"body":""}' "$TOKEN" EXPECTED`）
4. 在 `run_tests()` 的 `case "$mod" in` 中添加模块分发
5. 在文件末尾的命令行参数解析部分添加新的 flag 映射，并同步更新该处的 `Usage:` 文案

## 执行流程

### 1. 准备阶段

1. 确认测试目标和范围
2. 启动服务器（如未运行）
3. 准备测试数据（脚本自动处理）

### 2. 执行测试

```bash
cd /path/to/Feed Chamber
bash feedchamber_test.sh -all          # 全量
bash feedchamber_test.sh -auth -post   # 模块组合
```

### 3. 生成报告

将测试结果保存到 `agent_test_logs/` 目录。

#### 命名规范

文件名格式：`{测试内容}-{时间}.md`

- `api-auth-test-2026-05-29.md`
- `full-api-test-2026-05-29.md`
- `post-crud-test-2026-05-29-14-30.md`

规则：

1. 使用小写字母和连字符（kebab-case）
2. 时间格式：`YYYY-MM-DD` 或 `YYYY-MM-DD-HH-MM`
3. 必须以 `.md` 结尾

#### 报告模板

```markdown
# {测试标题}

**测试时间**: YYYY-MM-DD HH:MM:SS
**测试模型**: (当前使用的模型)

## 测试概要

简要描述本次测试的目的和范围。

## 测试结果

| 测试项 | 状态 | 说明 |
|--------|------|------|
| 测试项1 | ✅ 通过 | 成功说明 |
| 测试项2 | ❌ 失败 | 失败原因 |

**总计**: N 通过, M 失败

## 问题与建议

列出测试过程中发现的问题和改进建议。

## 总结

整体测试结论和下一步计划。
```

## 注意事项

- 所有测试通过 `feedchamber_test.sh` 进行，无需手动拼 curl
- `-all` 模式建议在干净的数据库上运行
- 认证模块使用时间戳后缀创建唯一用户，避免重复运行冲突
- 测试脚本不修改生产数据库，但会创建测试数据（帖子、评论、版面等）
- 修改密码测试会重置回原密码
- 测试脚本自动创建和清理临时文件
- 失败的测试项必须在报告中记录失败原因和错误信息
- 敏感信息（如 token）在报告中应适当截断
