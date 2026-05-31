# ── Stage 1: Build Go backend ─────────────────────────────────────────────
FROM golang:1.24-alpine AS go-builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build
COPY packages/server/go.mod packages/server/go.sum ./
RUN go mod download

COPY packages/server/ ./
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /orinforum .

# ── Stage 2: Build frontend (SSR via TanStack Start / Nitro) ─────────────
FROM node:22-alpine AS web-builder

RUN corepack enable && corepack prepare pnpm@latest --activate

WORKDIR /build
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY packages/web/package.json packages/web/package.json
RUN pnpm install --frozen-lockfile --filter @orinforum/web

COPY packages/web/ packages/web/
RUN cd packages/web && pnpm build

# ── Stage 3: Production ──────────────────────────────────────────────────
FROM node:22-alpine

RUN apk add --no-cache sqlite-libs ca-certificates tzdata

WORKDIR /app

# Go backend
COPY --from=go-builder /orinforum /app/orinforum
COPY --from=go-builder /build/.env* /app/packages/server/
RUN mkdir -p /app/packages/server/uploads /app/packages/server/db

# Frontend SSR
COPY --from=web-builder /build/packages/web/.output /app/packages/web/.output

ENV GIN_MODE=release
ENV PORT=8080
ENV WEB_PORT=3000

EXPOSE 8080 3000

COPY <<'EOF' /app/entrypoint.sh
#!/bin/sh
set -e
cd /app/packages/server && /app/orinforum -e prod &
cd /app/packages/web && node .output/server/index.mjs &
wait -n
exit $?
EOF
RUN chmod +x /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"]
