---
title: 升级与迁移
description: 升级前的备份、镜像与重建、AutoMigrate 与手写 SQL 的分工、滚动重启、回滚和升级后的对账
---

升级的难点不是换镜像，而是**数据库变更只往前不往后**。这页给一条可回滚的路径：先备份、
再升级、跑完对账，最后才删备份。

## 升级前

### 1. 备份

三条命令覆盖三份数据，缺一不可（数据库、上传文件、配置）：

```bash
# 1) 数据库（PostgreSQL 部署）
docker compose -f deploy/docker-compose.yml exec postgres \
  pg_dump -U feedchamber feedchamber > backup-$(date +%F).sql

# 2) 上传文件
docker run --rm -v feedchamber_uploads:/data -v "$PWD":/out alpine \
  tar czf /out/uploads-$(date +%F).tar.gz -C /data .

# 3) 配置与当前版本
cp deploy/docker-compose.yml compose-$(date +%F).yml
git rev-parse --short HEAD
```

SQLite 部署用 CLI 备份，后台「系统 → 备份」的按钮走同一套逻辑：

```bash
cd app/server && go run main.go -e prod backup --out /backup/2026-10-30
```

它产出 `app.db`（仅 SQLite）、`uploads-manifest.json`、`meta.json`。
:::caution[备份 ≠ 可恢复]
`backup` 不打包上传文件本身，也不为非 SQLite 生成 dump。恢复前先在副本上演练一次，
并确认备份与要升到的目标版本同代。
:::

### 2. 确认现状

`docker compose -f deploy/docker-compose.yml ps` 全部 healthy，且
`curl -s localhost:8080/api/v1/health` 三项都是 `ok`。健康检查本来就是红的就别升级——
先排障，否则分不清是升级引入的还是原有的。

### 3. 读破坏性变更

没有「升级检查」命令，也没有版本端点可查。判断依据是仓库本轮改动里有没有：删列/改类型、
改名设置键、改名事件类型、改 KV key 形状、改权限码、改响应字段语义。任意一条命中，
就按最后一节的清单走。

## 拉新镜像与重建

```bash
git pull
docker compose -f deploy/docker-compose.yml up -d --build
docker compose -f deploy/docker-compose.yml ps      # 等 healthy
```

`deploy/docker-compose.yml` 用 `build` 从当前源码构建，不是拉预构建 tag；想用固定镜像
就把 `image:` 改成你的 tag 并去掉 `build:`。`depends_on` 带 `condition: service_healthy`，
所以 backend 会等 Postgres 与 NATS 就绪——启动顺序不用手动管。

## 迁移怎么发生

数据库变更有三条不同路径，别混：

| 变更类型 | 谁执行 | 说明 |
|----------|--------|------|
| 新增表 / 列 / 索引 | `AutoMigrate`，每次启动 | 自动补齐，对存量库无破坏性 |
| 存量数据升级 | 启动时的一次性函数 | 如 `trust_level` 字符串转数字、`total_points` 回填到 `total_xp` |
| 删列 / 改类型 / 数据搬迁 | `db/migrations/*.sql`，**手动** | 仓库里没有执行器，脚本自带背景与幂等说明 |

关键差异：**PostgreSQL / MySQL 的类型变更必须手动跑脚本**；SQLite 由启动流程自己完成。
现成例子是 `db/migrations/2026-10-29_trust_level_xp.sql`，开头写明「非 SQLite 部署必须执行」
且脚本可重复执行。

推荐顺序：备份 → 停旧实例 → 起新实例（让 `AutoMigrate` 补列）→ 执行需要的手写脚本 →
重启一次 → 对账。脚本放在新版启动之后跑，因为它们依赖 `AutoMigrate` 建出的新列。

## 滚动重启与 NATS/KV 兼容

- **会话不会因重启掉线**。凭证的权威是 `user_sessions` 行，KV 只是 read-through 缓存；
  重启后第一次请求回源数据库即可。清空 NATS 卷不会强制所有人重新登录。
- **NATS bucket 名不能改**（默认 `feedchamber_kv`，另有配套的 `<bucket>_ttl`）。改名等于
  换一个空存储：会话缓存、限流窗口、OAuth state、禁言缓存一起失效。
- **KV key 形状是契约**。`rl:<ip>:<policy>` 与 `rl:ratelimit:<ip>` 这两条形状变了，正在
  计数的窗口会清零，限流在那一刻短暂变松。
- **多实例混跑两个版本**要求事件 payload 只增不减：新字段会被忽略，删字段或改语义会让
  另一个实例的通知/积分算错。做不到就一次性停旧起新。
- **JetStream 存储要保留**。`feedchamber_nats` 卷丢了不致命（可重建），但授权中途的
  OAuth 流程会失败一次。

最小安全做法：一次只动一个实例，等 `/api/v1/health` 回到 ok 再动下一个。

## 降级与回滚

```bash
git checkout <上一个 commit>            # 回到上一个 tag，或用旧源码重建
docker compose -f deploy/docker-compose.yml up -d --build
```

代码可以回滚，**数据不一定**：

- 新增列、新增表：旧代码不认识但会忽略它们，通常可回滚。
- 改类型（如 `trust_level` 字符串 → 整数）、删列、数据搬迁：旧代码会读失败或读出错值，必须从备份恢复。
- 回滚窗口等于备份保留窗口。确认新版稳定前，别删升级前的备份。

## 升级后必做

```bash
# XP 与等级对账（改了规则或等级定义后尤其必要）
cd app/server && go run main.go -e prod recompute-xp
# recomputed 1234 users
```

`recompute-xp` 幂等：XP 由行为聚合派生，重跑不会重复计分，服务运行中执行也安全。每日
全量重算在启动 2 分钟后跑第一次，但升级后不要等它——手工跑一次才知道有没有异常。

## Swagger 与 openapi.json 同步

`openapi.json` 是 `make swag` 的产物，不是手写文件：

```bash
cd app/server
make swag            # 重新生成 docs/ 并复制到仓库根 openapi.json
make swag-check      # CI 用：有漂移就非零退出
```

提交 `openapi.json` 之后再更新文档站的数据：

```bash
cd app/docs && node scripts/gen-reference.mjs   # 刷新端点/设置/指标参考页的数据
```

文档站构建前会把 `openapi.json` 同步到 `public/`（`prebuild` 的 `sync-openapi.mjs`）。

## 破坏性变更清单

升级前逐条打勾，任意一条命中就先写迁移方案：

- [ ] 删除了列或表，或改了列类型
- [ ] 改了设置键的名字或语义（旧键不再被读取）
- [ ] 改了事件类型名，或删了事件 payload 字段
- [ ] 改了 KV key 形状（`rl:`、`sess:`、`setting:`、`ban:`）
- [ ] 改了 `NATS_BUCKET`，或换了 JetStream 存储
- [ ] 新增 / 改名权限码（旧角色映射会缺权限）
- [ ] 改了错误码或响应字段语义（客户端按码分支会挂）
- [ ] 改了存储 provider 或上传目录（历史文件路径失效）

## 验证

```bash
docker compose -f deploy/docker-compose.yml ps          # 全部 healthy
curl -s localhost:8080/api/v1/health                    # 三项 ok
curl -s localhost:9091/metrics | grep -c feedchamber_   # 指标族在
```

再看三件事：老账号能登录、新帖能发能看、通知能收到。这三条过了，剩下都是细节。
