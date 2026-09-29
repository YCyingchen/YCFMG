#!/usr/bin/env bash
# YCFMG fpk 打包：生成可被飞牛 NAS(fnOS) 应用中心安装的安装包
set -euo pipefail
umask 022
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-$(cat "$ROOT/VERSION" | tr -d "[:space:]")}"
DIST="$ROOT/dist"
STAGE="$DIST/fpk-stage"
APP="$STAGE/app"

echo "== 打包 fpk $VERSION =="

if [ ! -f "$DIST/ycfmg-linux-amd64" ] || [ ! -f "$DIST/ycfmg-linux-arm64" ]; then
  echo "-- 缺少二进制，先执行构建"
  VERSION="$VERSION" TARGETS="linux/amd64 linux/arm64" "$ROOT/scripts/build.sh"
fi

rm -rf "$STAGE"
mkdir -p "$APP/bin" "$APP/etc" "$APP/data" "$STAGE/cmd"
cp "$DIST/ycfmg-linux-amd64" "$APP/bin/ycfmg"
cp "$ROOT/fnos/manifest" "$STAGE/manifest"
cp "$ROOT/fnos/cmd/"* "$STAGE/cmd/"
cp "$ROOT/configs/config.example.yaml" "$APP/etc/config.example.yaml" 2>/dev/null || true
if [ -f "$ROOT/assets/ICON.PNG" ]; then cp "$ROOT/assets/ICON.PNG" "$STAGE/ICON.PNG"; fi
if [ -f "$ROOT/assets/ICON_256.PNG" ]; then cp "$ROOT/assets/ICON_256.PNG" "$STAGE/ICON_256.PNG"; fi
cp "$ROOT/README.md" "$APP/README.md" 2>/dev/null || true
cp "$ROOT/LICENSE" "$APP/LICENSE" 2>/dev/null || true
chmod +x "$APP/bin/ycfmg" "$STAGE"/cmd/*

# 记录架构信息，便于安装器选择二进制
printf "amd64\narm64\n" > "$APP/etc/arch.list"

tar -czf "$STAGE/app.tgz" -C "$APP" .
rm -rf "$APP"

FPK="$DIST/YCFMG_$VERSION.fpk"
( cd "$STAGE" && tar -czf "$FPK" manifest cmd ICON.PNG ICON_256.PNG app.tgz 2>/dev/null || tar -czf "$FPK" manifest cmd app.tgz )
rm -rf "$STAGE"
echo "== 完成: $FPK =="
ls -lh "$FPK"