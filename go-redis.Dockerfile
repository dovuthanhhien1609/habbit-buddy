# go-redis runtime wrapper
# Builds from the submodule source but uses alpine so Docker healthchecks work.
# (The submodule's own Dockerfile uses scratch which has no shell or nc.)

# ── Stage 1: builder (mirrors services/go-redis/Dockerfile) ─────────────────
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app
COPY services/go-redis/go.mod services/go-redis/go.sum* ./
RUN go mod download

COPY services/go-redis/ .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /app/go-redis \
    ./cmd/server

# ── Stage 2: runtime (alpine so nc/wget are available for healthchecks) ──────
FROM alpine:3.20 AS runtime

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/go-redis /go-redis

EXPOSE 6379

ENTRYPOINT ["/go-redis"]
