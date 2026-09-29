#!/usr/bin/env bash
# YCFMG 统一 Go 构建环境（缓存全部落在仓库内，避免依赖 HOME 可写）
export ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export GOROOT="$ROOT/.tools/go"
export GOPATH="$ROOT/.tools/gopath"
export GOMODCACHE="$GOPATH/pkg/mod"
export GOCACHE="$ROOT/.tools/gocache"
export GOFLAGS="-mod=mod"
export GOPROXY="https://goproxy.cn,direct"
export GOSUMDB=off
export GOTOOLCHAIN=local
export PATH="$GOROOT/bin:$GOPATH/bin:$PATH"
export CGO_ENABLED=0
