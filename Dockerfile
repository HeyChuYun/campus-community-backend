FROM golang:1.26.2-bookworm AS builder

ARG SERVICE_PATH

WORKDIR /src

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    test -n "$SERVICE_PATH" && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/service ./service/${SERVICE_PATH}

FROM alpine:3.20

ARG SERVICE_PATH

WORKDIR /app

COPY --from=builder /out/service /app/service
# 统一携带仓库级配置目录，兼容本地文件配置与容器内排障场景。
# 生产环境默认通过 etcd 读取配置，不再依赖各服务私有 etc 目录。
COPY --from=builder /src/config /app/config
COPY --from=builder /src/deploy/config /app/deploy/config

ENV TZ=Asia/Shanghai

ENTRYPOINT ["/app/service"]
