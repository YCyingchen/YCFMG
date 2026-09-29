FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS builder
# 说明：builder 阶段固定跑在构建机原生架构，用 GOARCH=$TARGETARCH 交叉编译，
# 因此多架构构建无需 qemu 模拟，速度快且产物一致。
ARG TARGETARCH
ARG TARGETOS=linux
ARG VERSION=fmg2609.001
ARG COMMIT=dev
ENV CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download
COPY . .
RUN go build -trimpath -buildvcs=false -ldflags "-s -w -X github.com/ycyingchen/ycfmg/internal/version.Version=$VERSION -X github.com/ycyingchen/ycfmg/internal/version.Commit=$COMMIT" -o /out/ycfmg ./cmd/ycfmg

FROM alpine:3.20
RUN apk add --no-cache tzdata ca-certificates ffmpeg su-exec && rm -rf /var/cache/apk/*
ENV TZ=Asia/Shanghai LANG=C.UTF-8
WORKDIR /app
COPY --from=builder /out/ycfmg /usr/local/bin/ycfmg
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh && mkdir -p /config /data
VOLUME ["/config", "/data"]
EXPOSE 8686
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s CMD wget -qO- http://127.0.0.1:8686/api/health || exit 1
ENTRYPOINT ["/entrypoint.sh"]
