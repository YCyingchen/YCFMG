#!/usr/bin/env bash
# YCFMG 多平台构建：产出静态二进制与发行包
set -euo pipefail
umask 022
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-$(cat "$ROOT/VERSION" | tr -d "[:space:]")}"
COMMIT="${COMMIT:-$(cd "$ROOT" && git rev-parse --short HEAD 2>/dev/null || echo dev)}"
BUILDTIME="$(date +%Y-%m-%dT%H:%M:%S%z)"
DIST="$ROOT/dist"

export GOROOT="${GOROOT:-$ROOT/.tools/go}"
export GOPATH="${GOPATH:-$ROOT/.tools/gopath}"
export GOMODCACHE="${GOMODCACHE:-$GOPATH/pkg/mod}"
export GOCACHE="${GOCACHE:-$ROOT/.tools/gocache}"
export TMPDIR="${TMPDIR:-$ROOT/.tools/tmp}"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export GOSUMDB=off
export GOTOOLCHAIN=local
export CGO_ENABLED=0
export PATH="$GOROOT/bin:$GOPATH/bin:$PATH"
mkdir -p "$DIST" "$TMPDIR"

LDFLAGS="-s -w -X github.com/ycyingchen/ycfmg/internal/version.Version=$VERSION -X github.com/ycyingchen/ycfmg/internal/version.Commit=$COMMIT -X github.com/ycyingchen/ycfmg/internal/version.BuildTime=$BUILDTIME"

TARGETS="${TARGETS:-linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64}"

echo "== YCFMG 构建 $VERSION ($COMMIT) =="
for t in $TARGETS; do
  OS="${t%%/*}"
  ARCH="${t##*/}"
  EXT=""
  [ "$OS" = "windows" ] && EXT=".exe"
  OUT="$DIST/ycfmg-$OS-$ARCH$EXT"
  echo "-- $OS/$ARCH"
  GOOS="$OS" GOARCH="$ARCH" go build -trimpath -buildvcs=false -ldflags "$LDFLAGS" -o "$OUT" "$ROOT/cmd/ycfmg"
  if [ "$OS" = "linux" ]; then
    PKG="$DIST/ycfmg-$VERSION-$OS-$ARCH"
    rm -rf "$PKG" && mkdir -p "$PKG/bin" "$PKG/etc"
    cp "$OUT" "$PKG/bin/ycfmg"
    cp "$ROOT/configs/config.example.yaml" "$PKG/etc/config.example.yaml" 2>/dev/null || true
    cp -r "$ROOT/fnos/cmd" "$PKG/cmd"
    cp "$ROOT/README.md" "$PKG/README.md" 2>/dev/null || true
    cp "$ROOT/LICENSE" "$PKG/LICENSE" 2>/dev/null || true
    chmod +x "$PKG/bin/ycfmg" "$PKG"/cmd/* 2>/dev/null || true
    tar -czf "$PKG.tar.gz" -C "$DIST" "$(basename "$PKG")"
    rm -rf "$PKG"
    echo "   打包 $PKG.tar.gz"
  fi
done

echo "== 完成，产物位于 $DIST =="
ls -lh "$DIST" | tail -n +2