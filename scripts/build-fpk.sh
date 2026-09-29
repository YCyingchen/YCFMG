#!/usr/bin/env bash
# YCFMG fpk 打包：优先使用飞牛官方 fnpack，缺失时回退到手工 tar 打包
set -euo pipefail
umask 022
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-$(cat "$ROOT/VERSION" | tr -d "[:space:]")}"
DIST="$ROOT/dist"
FNPACK="${FNPACK:-$ROOT/.tools/fnpack}"

export GOROOT="${GOROOT:-$ROOT/.tools/go}"
export GOPATH="${GOPATH:-$ROOT/.tools/gopath}"
export GOMODCACHE="${GOMODCACHE:-$GOPATH/pkg/mod}"
export GOCACHE="${GOCACHE:-$ROOT/.tools/gocache}"
export TMPDIR="${TMPDIR:-$ROOT/.tools/tmp}"
export PATH="$GOROOT/bin:$GOPATH/bin:$PATH"
mkdir -p "$DIST" "$TMPDIR"

echo "== 打包 fpk $VERSION =="

# 1) 确保有 linux/amd64 二进制（manifest.platform = x86）
if [ ! -f "$DIST/ycfmg-linux-amd64" ]; then
  echo "-- 缺少二进制，先构建"
  VERSION="$VERSION" TARGETS="linux/amd64" "$ROOT/scripts/build.sh"
fi

# 2) 填充 fnpack 项目的 app 目录
mkdir -p "$ROOT/fnos/app/bin" "$ROOT/fnos/app/etc"
cp "$DIST/ycfmg-linux-amd64" "$ROOT/fnos/app/bin/ycfmg"
chmod 755 "$ROOT/fnos/app/bin/ycfmg"
cp "$ROOT/configs/config.example.yaml" "$ROOT/fnos/app/etc/config.example.yaml"
cp "$ROOT/README.md" "$ROOT/fnos/app/README.md" 2>/dev/null || true
cp "$ROOT/LICENSE" "$ROOT/fnos/app/LICENSE" 2>/dev/null || true
if [ -f "$ROOT/assets/ICON.PNG" ]; then cp "$ROOT/assets/ICON.PNG" "$ROOT/fnos/ICON.PNG"; fi
if [ -f "$ROOT/assets/ICON_256.PNG" ]; then cp "$ROOT/assets/ICON_256.PNG" "$ROOT/fnos/ICON_256.PNG"; fi

# 3) 同步 manifest 版本号
if [ -f "$ROOT/fnos/manifest" ]; then
  sed -i "s|^version .*|version          = $VERSION|" "$ROOT/fnos/manifest"
fi

FPK="$DIST/YCFMG_$VERSION.fpk"

# 统一权限，避免因权限异常导致打包校验失败
find "$ROOT/fnos" -type d -exec chmod 755 {} \;
find "$ROOT/fnos" -type f -exec chmod 644 {} \;
chmod 755 "$ROOT/fnos/app/bin/ycfmg"
for f in "$ROOT/fnos"/cmd/*; do [ -f "$f" ] && chmod 755 "$f"; done

if [ -x "$FNPACK" ]; then
  echo "-- 使用官方 fnpack 打包"
  rm -f "$ROOT/fnos/$([[ -f "$ROOT/fnos"/*.fpk ]] && echo x)fnos.fpk" 2>/dev/null || true
  ( cd "$ROOT/fnos" && "$FNPACK" build )
  if [ -f "$ROOT/fnos/ycfmg.fpk" ]; then
    mv "$ROOT/fnos/ycfmg.fpk" "$FPK"
  elif [ -f "$ROOT/fnos/ycfmg_$VERSION.fpk" ]; then
    mv "$ROOT/fnos/ycfmg_$VERSION.fpk" "$FPK"
  else
    found=$(ls -1t "$ROOT/fnos"/*.fpk 2>/dev/null | head -1 || true)
    [ -n "$found" ] && mv "$found" "$FPK"
  fi
else
  echo "-- 未找到 fnpack（$FNPACK），回退到手工 tar 打包"
  STAGE="$DIST/fpk-stage"
  rm -rf "$STAGE" && mkdir -p "$STAGE"
  cp -r "$ROOT/fnos/cmd" "$ROOT/fnos/config" "$ROOT/fnos/wizard" "$ROOT/fnos/manifest" "$STAGE/"
  cp "$ROOT/fnos/ICON.PNG" "$ROOT/fnos/ICON_256.PNG" "$STAGE/"
  tar -czf "$STAGE/app.tgz" -C "$ROOT/fnos/app" .
  ( cd "$STAGE" && tar -czf "$FPK" manifest cmd config wizard ICON.PNG ICON_256.PNG app.tgz )
  rm -rf "$STAGE"
fi

if [ -f "$FPK" ]; then
  chmod 644 "$FPK"
  echo "== 完成: $FPK =="
  ls -lh "$FPK"
else
  echo "== 打包失败：未生成 fpk =="
  exit 1
fi