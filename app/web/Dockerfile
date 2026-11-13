# syntax=docker/dockerfile:1.7
# ── Stage 1: Build ────────────────────────────────────────────────────────
FROM node:22-alpine AS builder

# 依赖源默认官方；国内网络可用 `--build-arg NPM_REGISTRY=...` 覆盖。
ARG NPM_REGISTRY=https://registry.npmjs.org
ENV npm_config_registry=${NPM_REGISTRY}

# 与根 package.json 的 `packageManager` 对齐。此前用 @latest，每次构建都要
# 重新解析并下载当前最新版，既多一次网络请求又让构建不可复现。
RUN corepack enable && corepack prepare pnpm@10.26.2 --activate

WORKDIR /build
# Workspace manifests first: @feedchamber/web resolves @feedchamber/* through `workspace:*`.
#
# 只拷各包的 package.json，不拷源码：install 层只依赖 manifest + lockfile，
# 早先整目录 COPY packages/ 会让「改一行 shared-utils 源码」就击穿安装层，
# 每次重装 node_modules（store 命中也仍要重建几百个硬链接）。
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml turbo.json ./
COPY packages/api-client/package.json packages/api-client/
COPY packages/shared-utils/package.json packages/shared-utils/
COPY packages/sse-client/package.json packages/sse-client/
COPY packages/tokens/package.json packages/tokens/
COPY app/web/package.json app/web/package.json

# `@feedchamber/web...` selects web *and its workspace dependencies*, but pnpm's
# `--filter` only installs the devDependencies of the named packages. The four
# shared packages are built below, so they must be named too — otherwise
# `@feedchamber/tokens` has no `tsx` and its build dies with `sh: tsx: not found`.
#
# store 单独放 cache 挂载：manifest 层命中时这次安装本来就是 0s；manifest 变化
# 时也不会把几百个包重新下载一遍，只从本地 store 硬链接。
RUN --mount=type=cache,id=pnpm-store,target=/pnpm/store \
    pnpm install --frozen-lockfile --prefer-offline --store-dir=/pnpm/store \
      --filter @feedchamber/web... \
      --filter @feedchamber/tokens \
      --filter @feedchamber/api-client \
      --filter @feedchamber/shared-utils \
      --filter @feedchamber/sse-client

# 源码层放在 install 之后：只有源码真正变化时才重跑，且不会波及上面那层安装结果。
COPY packages/ packages/
COPY app/web/ app/web/
# The app imports built CSS/JS from the shared packages, so build those first.
# `cd app/web` matters: a bare `pnpm build` at /build runs the root turbo script,
# which filters on @feedchamber/server — a package not present in this image.
RUN pnpm --filter @feedchamber/tokens --filter @feedchamber/api-client --filter @feedchamber/shared-utils --filter @feedchamber/sse-client build \
 && cd app/web && pnpm build

# ── Stage 2: 运行时依赖（只有生产依赖）──────────────────────────────────────
#
# 存在的唯一理由是体积：Stage 1 的 node_modules 是**全量**的（vite / rolldown /
# esbuild / webpack / typescript / biome / turbo / playwright / jsdom / antd …），
# 早先直接把 `COPY --from=builder /build/node_modules` 搬进运行镜像，实测解压后
# 685MB、压缩后 154MB，占整个 web 镜像（解压 1.07GB）的 64%——而真正的产物
# `dist/` 只有 12MB。
#
# 这里用同一份 lockfile 再装一遍 **`--prod`**：解压后 ~287MB，路由实测全通
# （/timeline /login /spaces /admin /leaderboard /timeline/post?id=1 …）。
# 构建阶段仍保留全量依赖，两者互不影响，也互不击穿缓存。
#
# manifest 清单必须与 Stage 1 完全一致：lockfile 里 workspace importer 的路径
# 与 package.json 一一对应，少拷一个 manifest 会让 `--frozen-lockfile` 直接失败。
FROM node:22-alpine AS prod-deps

ARG NPM_REGISTRY=https://registry.npmjs.org
ENV npm_config_registry=${NPM_REGISTRY}

RUN corepack enable && corepack prepare pnpm@10.26.2 --activate

WORKDIR /build
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml turbo.json ./
COPY packages/api-client/package.json packages/api-client/
COPY packages/shared-utils/package.json packages/shared-utils/
COPY packages/sse-client/package.json packages/sse-client/
COPY packages/tokens/package.json packages/tokens/
COPY app/web/package.json app/web/package.json

# 与 Stage 1 共用同一个 store cache（id 相同），命中时这次安装只做硬链接。
# 不需要源码：--prod 不跑任何 workspace 的 build 脚本。
RUN --mount=type=cache,id=pnpm-store,target=/pnpm/store \
    pnpm install --prod --frozen-lockfile --prefer-offline --store-dir=/pnpm/store \
      --filter @feedchamber/web... \
      --filter @feedchamber/tokens \
      --filter @feedchamber/api-client \
      --filter @feedchamber/shared-utils \
      --filter @feedchamber/sse-client

# ── Stage 3: Production ───────────────────────────────────────────────────
#
# 运行时用 Bun，**构建阶段仍是 node:22-alpine + pnpm**（见 Stage 1）：vinext 的
# 产物是纯 JS，Bun 直接吃 pnpm 的相对 symlink 布局；而构建链路上挂着 pnpm
# workspace / pnpm-lock / turbo / onlyBuiltDependencies，把安装换成 bun 等于换锁
# 文件，收益为零、风险全落在构建侧。
#
# 只支持 Bun 1.4.x：1.3.x 的 `node:http` 对 HTTP/1.0 请求返回 200 后**永不关连接**
# （`ab` 直接挂死、`curl --http1.0` 卡满超时），1.4.4 已修复。所以这里 pin 版本而
# 不用 `alpine` 浮标签——与上面固定 pnpm 版本同一个理由：构建可复现。
#
# 实测（同一份 dist，bun 1.4.4 vs node 26）：`/login` 纯 SSR 839 vs 261 req/s，
# 空闲 RSS 147 vs 215MB；依赖后端的页面两者打平。已知偏差：`/login` 会丢
# `Link: rel=preload` 头（Bun 静默丢弃「写出时机偏晚」的响应头），功能无影响。
FROM oven/bun:1.4.4-alpine

# vinext emits dist/{client,server}; `vinext start` serves it with SSR,
# compression and middleware.
#
# pnpm links packages through relative symlinks (app/web/node_modules/vinext ->
# ../../node_modules/.pnpm/...), so the layout must stay identical: node_modules
# and packages are copied to their original absolute paths.
#
# node_modules 一律来自 Stage 2（生产依赖），**不要**改成 `--from=builder`：
# 那是全量依赖树，会把 685MB 的构建工具链塞进运行镜像。
#
# 拆成多条 COPY 而不是一次 `COPY /build /build`：node_modules 单独成层后能长期
# 命中缓存；最后才拷 dist（几 MB），改一行前端代码只重写这一层。
COPY --from=prod-deps /build/node_modules /build/node_modules
COPY --from=prod-deps /build/packages /build/packages
COPY --from=prod-deps /build/app/web/node_modules /build/app/web/node_modules

# 产物：根 package.json（vinext 用它判断应用根）、web 的 manifest 与 dist，
# 以及四个共享包的 dist（@feedchamber/* 的 symlink 指向 packages/，缺了 dist
# 会让「某个依赖没被 bundle 进去」的场景在运行时才炸）。
COPY --from=builder /build/package.json /build/package.json
COPY --from=builder /build/app/web/package.json /build/app/web/package.json
COPY --from=builder /build/app/web/dist /build/app/web/dist
COPY --from=builder /build/packages/api-client/dist /build/packages/api-client/dist
COPY --from=builder /build/packages/shared-utils/dist /build/packages/shared-utils/dist
COPY --from=builder /build/packages/sse-client/dist /build/packages/sse-client/dist
COPY --from=builder /build/packages/tokens/dist /build/packages/tokens/dist

WORKDIR /build/app/web

ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME=0.0.0.0
# Server-side API origin; the browser still calls same-origin /api/v1 via nginx.
ENV API_BASE_URL=http://nginx:80

EXPOSE 3000

CMD ["bun", "node_modules/vinext/dist/cli.js", "start"]
