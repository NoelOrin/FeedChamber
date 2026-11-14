# Docker构建流程重构设计

> 日期：2026-06-22
> 范围：Dockerfile构建流程全面重构

## 1. 架构设计

```
                    ┌─────────────────┐
                    │  Nginx Gateway  │
                    │   (端口 80/443) │
                    └────────┬────────┘
                             │
          ┌──────────────────┼──────────────────┐
          │                  │                  │
          ▼                  ▼                  ▼
    ┌──────────┐      ┌──────────┐      ┌──────────┐
    │ /api/**  │      │/api_doc  │      │   /*     │
    └──────────┘      └──────────┘      └──────────┘
         │                                   │
         │                                   │
         ▼                                   ▼
   ┌──────────────┐                   ┌─────────────┐
   │ Go Backend   │                   │ Node/SSR    │
   │  :8080       │                   │  :3000      │
   │ orinforum    │                   │ nitro       │
   └──────────────┘                   └─────────────┘
```

## 2. 镜像拆分方案

### 2.1 后端镜像 (`app/server/Dockerfile`)

| 属性 | 值 |
|------|------|
| 基础镜像 | `golang:1.24-alpine` (builder) → `alpine:3.21` (runtime) |
| 构建依赖 | `gcc`, `musl-dev`, `sqlite-dev` (CGO) |
| 运行时依赖 | `sqlite-libs`, `ca-certificates`, `tzdata` |
| 构建优化 | `go mod download` 层缓存，`go build -ldflags"-s -w"` |
| 端口 | 8080 |
| 用户 | 非 root (可选) |

### 2.2 前端镜像 (`app/web/Dockerfile`)

| 属性 | 值 |
|------|------|
| 基础镜像 | `node:22-alpine` (builder) → `node:22-alpine` (runtime) |
| 构建工具 | `corepack` + `pnpm@latest` |
| 构建顺序 | 1. 构建 workspace 依赖包 → 2. 构建 web |
| 构建优化 | `pnpm install --frozen-lockfile` 层缓存 |
| 端口 | 3000 |
| 命令 | `node .output/server/index.mjs` |

### 2.3 网关镜像 (`app/server/deploy/nginx/`

| 属性 | 值 |
|------|------|
| 基础镜像 | `nginx:alpine` |
| 配置文件 | `nginx.conf` 模板 (80 和 443) |
| SSL | 支持 (宿主机挂载证书) |
| 端口 | 80, 443 |
| 目录 | `/etc/nginx/conf.d/` |

## 3. Docker Compose 层级

现有 5 个变体统一适配新架构：

| Compose 文件 | 服务 | 升级链路 |
|-------------|------|---------|
| docker-compose.lite.yml | SQLite only | 单后端 + nginx |
| docker-compose.basic.yml | PostgreSQL | + psql + nginx |
| docker-compose.yml | PostgreSQL + Redis | + psql + redis + nginx |
| docker-compose.full.yml | + NATS | + psql + redis + nats + nginx |
| docker-compose.cluster.yml | + 监控 | + all + prometheus + grafana + nginx |

每个 compose 新增 `nginx` 服务，作为统一入口。

## 4. CI/CD 构建工作流

### 触发条件
- PR 提交时
- `master` 分支更新时
- 手动触发

### 矩阵构建
- 平台：linux/amd64, linux/arm64
- 镜像：orinforum/server, orinforum/web

### 输出
- 本地镜像 `orinforum/server:latest`, `orinforum/web:latest`
- 可选：保存为 tar 文件

### 环境变量文件
- 所有敏感配置通过 `.env` 文件和 Docker Secrets 管理
- 支持 `JWT_SECRET` 等通过环境变量注入

## 5. 配置管理

### 前端 API 代理
nitro.config.ts 在生产环境下保持代理逻辑，但指向 nginx 网关：

```ts
const apiBase = process.env.API_BASE_URL ?? 'http://nginx:80';
```

### 后端 CORS
Go 后端在 nginx 代理下可以关闭 CORS，或者允许 nginx IP。

### Nginx 路由规则
- `/api/v1/**` → `backend:8080`
- `/swagger/**` → `backend:8080`
- `/api_doc` → `backend:8080`
- `/*` → `frontend:3000`

## 6. .dockerignore 优化

```
# node_modules, .git 已有
# 新增：
.env*
!.env.example
.pnpm-store
*.md
dist/
.output/
.turbo/
packages/*/dist/
```

## 7. 构建缓存策略

| 阶段 | 缓存策略 |
|------|---------|
| Go deps | `go mod download` 单独缓存 mod 层 |
| Node deps | `pnpm install` 缓存 `/pnpm-store` |
| Workspace 包 | `packages/*/dist` 优先 COPY 已构建产物 |

## 8. 总结

- 前后端完全拆分为独立镜像
- 统一 nginx 网关作为入口
- 保留现有 5 个 compose 变体，统一适配
- CI/CD 仅构建到本地，不推送 Registry
- 配置通过 `.env` 文件管理