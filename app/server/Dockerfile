# syntax=docker/dockerfile:1.7
# ── Stage 1: Build ─────────────────────────────────────────────────────
# 构建上下文是**仓库根**（compose 里 `context: ..`），不是 app/server。
# 这样前后端两个镜像共用一份上下文与根 .dockerignore，路径也只有一种写法。
FROM golang:1.25-alpine AS builder

# 依赖源默认官方；国内网络可用 compose 的环境变量或
# `--build-arg GOPROXY=...` 覆盖，无需改文件。
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY} \
    GOFLAGS=-buildvcs=false

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build
COPY app/server/go.mod app/server/go.sum ./
# 模块缓存与编译缓存都挂 cache 挂载：改一行业务代码时不再整包重编译
# （实测 50s → 10s 量级），go.mod 不变时也不再重复下载模块。
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY app/server/ ./
# CGO 必须开：sqlite 驱动是 mattn/go-sqlite3。
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /build/feedchamber .

# ── Stage 2: Production ────────────────────────────────────────────────
#
# 运行时镜像要同时承载「Go 二进制」与「Node 插件子进程」，因此比纯 Go 服务多
# 三个包：nodejs / npm / git。
#
# 为什么用 alpine + apk 而不是 node:*-alpine 做基础层（实测，arm64）：
#   node:22-alpine 的 node 是**完整 ICU** 构建（120MB 二进制），alpine apk 的
#   nodejs 是 small-icu（48MB）。整镜像 `docker images` 因此 335MB vs 212MB，
#   而纯运行时阶段构建只快约 10s（8-10s vs 15-25s）。体积优先时选本方案。
#
# 为什么不能换成 node:22（Debian/glibc，自带 git 2.39）：
#   本镜像是 CGO_ENABLED=1 编出来的 **musl** 二进制（ldd 显示 libc.musl-*）。
#   把同一个二进制放进 glibc 镜像执行会直接 "not found"（实测）。自带 git 的
#   那几个 node 镜像全是 glibc，因此 git 只能自己装。
#
# 三个包各自不可省：
#   - nodejs：拉起插件子进程（Host/supervisor 用 `node <entry>`）；
#             同时提供 `node --watch`，PLUGIN_WATCH=true 时用它做开发期热重载。
#   - npm：   execPluginFetcher.InstallDeps 的 `npm install --omit=dev`。
#             纯 JS 插件（无 package.json）不需要它，带依赖的插件必须要。
#   - git：   execPluginFetcher.Fetch 的 `git clone --depth 1`。没有它任何
#             插件都装不上（除非手工把代码放进 /app/server/plugins 卷）。
FROM alpine:3.21

RUN apk add --no-cache sqlite-libs ca-certificates tzdata wget nodejs npm git

WORKDIR /app/server

COPY --from=builder /build/feedchamber ./feedchamber

# 这三个目录会被 compose 挂成 named volume；提前建好并归 root，
# 避免首次挂载时由 app 以非预期属主创建。
#
# plugins 是插件代码根目录（container 里 HostOptions.PluginRoot = "./plugins"，
# 相对 WORKDIR 即 /app/server/plugins）。它**必须**是 volume：插件代码由
# InstallAs 在运行时 git clone 进来，不挂卷则容器重建即全部丢失，而库里仍留着
# running 状态，重启后 resumePlugins 会去拉起一个空目录。
RUN mkdir -p /app/server/db /app/server/uploads /app/server/plugins

ENV GIN_MODE=release
ENV PORT=8080

EXPOSE 8080

# WORKDIR 固定在 /app/server：main.go 的 chdir(自身源码目录) 在镜像里指向构建
# 期的 /build，那里不存在，只会打印一条 warning 并留在当前目录。显式设 WORKDIR
# 让「相对路径的 db / uploads」始终落在 /app/server 下，与 volume 挂载点一致。
ENTRYPOINT ["./feedchamber"]
CMD ["-e", "prod"]
