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
FROM alpine:3.21

RUN apk add --no-cache sqlite-libs ca-certificates tzdata wget

WORKDIR /app/server

COPY --from=builder /build/feedchamber ./feedchamber

# 这两个目录会被 compose 挂成 named volume；提前建好并归 root，
# 避免首次挂载时由 app 以非预期属主创建。
RUN mkdir -p /app/server/db /app/server/uploads

ENV GIN_MODE=release
ENV PORT=8080

EXPOSE 8080

# WORKDIR 固定在 /app/server：main.go 的 chdir(自身源码目录) 在镜像里指向构建
# 期的 /build，那里不存在，只会打印一条 warning 并留在当前目录。显式设 WORKDIR
# 让「相对路径的 db / uploads」始终落在 /app/server 下，与 volume 挂载点一致。
ENTRYPOINT ["./feedchamber"]
CMD ["-e", "prod"]
