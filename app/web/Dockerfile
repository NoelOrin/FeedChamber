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

# ── Stage 2: Production ───────────────────────────────────────────────────
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
# and packages are copied to their original absolute paths. `vinext` also lives
# in devDependencies, so a `pnpm install --prod` prune would remove the server
# binary itself.
#
# 拆成多条 COPY 而不是一次 `COPY /build /build`：node_modules（~1GB）几乎不变，
# 单独成层后永久命中缓存；最后才拷 dist（几 MB），改一行前端代码只重写这一层。
# 原先单层每次都要重新导出 1GB+，实测导出 20-29s，现在 3s 量级。
COPY --from=builder /build/package.json /build/package.json
COPY --from=builder /build/node_modules /build/node_modules
COPY --from=builder /build/packages /build/packages
COPY --from=builder /build/app/web/package.json /build/app/web/package.json
COPY --from=builder /build/app/web/node_modules /build/app/web/node_modules
COPY --from=builder /build/app/web/dist /build/app/web/dist

WORKDIR /build/app/web

ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME=0.0.0.0
# Server-side API origin; the browser still calls same-origin /api/v1 via nginx.
ENV API_BASE_URL=http://nginx:80

EXPOSE 3000

CMD ["bun", "node_modules/vinext/dist/cli.js", "start"]
