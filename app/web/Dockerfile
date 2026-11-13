# syntax=docker/dockerfile:1.7
# ── Stage 1: Build ────────────────────────────────────────────────────────
FROM node:22-alpine AS builder

# 依赖源默认官方；国内网络可用 --build-arg NPM_REGISTRY=... 覆盖。
ARG NPM_REGISTRY=https://registry.npmjs.org
ENV npm_config_registry=${NPM_REGISTRY}

# 与根 package.json 的 packageManager 对齐。此前用 @latest，每次构建都要
# 重新解析并下载当前最新版，既多一次网络请求又让构建不可复现。
RUN corepack enable && corepack prepare pnpm@10.26.2 --activate

WORKDIR /build
# Workspace manifests first: @feedchamber/web resolves @feedchamber/* through workspace:*.
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

# @feedchamber/web... selects web *and its workspace dependencies*, but pnpm's
# --filter only installs the devDependencies of the named packages. The four
# shared packages are built below, so they must be named too — otherwise
# @feedchamber/tokens has no tsx and its build dies with sh: tsx: not found.
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
# cd app/web matters: a bare pnpm build at /build runs the root turbo script,
# which filters on @feedchamber/server — a package not present in this image.
RUN pnpm --filter @feedchamber/tokens --filter @feedchamber/api-client --filter @feedchamber/shared-utils --filter @feedchamber/sse-client build \
 && cd app/web && pnpm build

# 生成运行时的最小 manifest：只含产物真正外置的包（dist/server/vinext-externals.json）。
#
# 版本从刚装好的 node_modules 里读，而不是手抄 —— 手抄会和 package.json 漂移。
# 这个文件随后被 Stage 3 当成 package.json 用来装运行时依赖。
RUN node -e "const fs=require('fs');const v=n=>JSON.parse(fs.readFileSync('/build/app/web/node_modules/'+n+'/package.json','utf8')).version;const ext=JSON.parse(fs.readFileSync('/build/app/web/dist/server/vinext-externals.json','utf8'));const deps={};for(const n of ext){deps[n]=v(n);}fs.writeFileSync('/build/runtime-package.json',JSON.stringify({name:'feedchamber-web-runtime',private:true,type:'module',dependencies:deps},null,2));console.log('runtime deps:',JSON.stringify(deps));"

# ── Stage 2: 把启动器打成单文件 ─────────────────────────────────────────────
#
# 为什么需要这一步：vinext start 在启动时会 import() vinext 整个 dist 树
# （服务器 + 构建器 + 配置加载器），所以镜像必须带 vinext 及其依赖 —— 等于把
# 框架和构建工具链一起运到生产。而真正跑请求的只有 vinext/server/prod-server，
# 它依赖闭合、不碰构建期模块。
#
# app/web/server.mjs 只调 startProdServer，这里用 bun build 把它连同 vinext
# 运行时打成 ~350KB 单文件。打包后运行时不再需要 vinext 包本身。
#
# 已验证：打包产物与不打包的 prod-server 渲染结果逐字节相同（除时间戳），
# 路由 / 静态资源 / 404 行为一致。本 app 不用 ISR（无 revalidate）也不用
# next/image，所以没碰到 bun build 可能外置的 sharp / og 那部分。
#
# 用 bun 镜像而不是往 node:22-alpine 里装 bun：省一次下载，产物本来也只需 bun 跑。
FROM oven/bun:1.4.4-alpine AS bundler

WORKDIR /build
# 两层 node_modules 都要：根层是 .pnpm 实体，app/web 层是 vinext 等包的 symlink
# （app/web/node_modules/vinext -> ../../node_modules/.pnpm/...）。少拷任何一层，
# bun 都解析不到 vinext/server/prod-server。
COPY --from=builder /build/node_modules /build/node_modules
COPY --from=builder /build/app/web/node_modules /build/app/web/node_modules
COPY --from=builder /build/app/web/package.json /build/app/web/package.json
COPY --from=builder /build/app/web/server.mjs /build/app/web/server.mjs
WORKDIR /build/app/web
RUN bun build ./server.mjs --target=bun --outfile=./server.bundle.mjs

# ── Stage 3: 运行时依赖（只有 react / react-dom）──────────────────────────
#
# dist/server/vinext-externals.json 声明了这个产物运行时唯一需要外置的包就是
# react 与 react-dom —— 其余（框架、markdown、tiptap、tanstack、zod …）都已被
# Vite 打进 dist/。所以运行镜像的 node_modules 只需要这两个包。
#
# 这比「带全量生产依赖」再小一个数量级（287MB → ~8MB），也是这一版镜像从
# 1.08GB 压到 ~150MB 的最后一步。
FROM node:22-alpine AS prod-deps

ARG NPM_REGISTRY=https://registry.npmjs.org
ENV npm_config_registry=${NPM_REGISTRY}

RUN corepack enable && corepack prepare pnpm@10.26.2 --activate

WORKDIR /runtime
COPY --from=builder /build/runtime-package.json ./package.json

# 没有 lockfile：这份 manifest 是上一步从已安装的 node_modules 生成的精确版本，
# 依赖只有 react / react-dom 两个，解析结果确定。
RUN --mount=type=cache,id=pnpm-store,target=/pnpm/store \
    pnpm install --prod --prefer-offline --store-dir=/pnpm/store

# ── Stage 4: Production ───────────────────────────────────────────────────
#
# 运行时用 Bun，构建阶段仍是 node:22-alpine + pnpm：产物是纯 JS，Bun 直接吃；
# 构建链路上挂着 pnpm workspace / lockfile / turbo，换 bun 等于换锁文件，收益为零。
#
# 只支持 Bun 1.4.x：1.3.x 的 node:http 对 HTTP/1.0 请求返回 200 后永不关连接
# （ab 直接挂死、curl --http1.0 卡满超时），1.4.4 已修复。所以 pin 版本而不用
# alpine 浮标签 —— 与上面固定 pnpm 版本同一个理由：构建可复现。
#
# 实测（同一份 dist，bun 1.4.4 vs node 26）：/login 纯 SSR 839 vs 261 req/s，
# 空闲 RSS 147 vs 215MB；依赖后端的页面两者打平。已知偏差：/login 会丢
# Link: rel=preload 头（Bun 静默丢弃「写出时机偏晚」的响应头），功能无影响。
FROM oven/bun:1.4.4-alpine

WORKDIR /app

# 产物 + 打包后的启动器 + 最小运行时依赖。运行镜像里没有 vinext、没有 vite、
# 没有构建工具链 —— 全部在构建期用掉了。
COPY --from=bundler /build/app/web/server.bundle.mjs ./server.mjs
COPY --from=builder /build/app/web/dist ./dist
COPY --from=prod-deps /runtime/node_modules ./node_modules
COPY --from=builder /build/app/web/package.json ./package.json

ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME=0.0.0.0
# Server-side API origin; the browser still calls same-origin /api/v1 via nginx.
ENV API_BASE_URL=http://nginx:80

EXPOSE 3000

CMD ["bun", "server.mjs"]
