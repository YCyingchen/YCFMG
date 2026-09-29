#!/usr/bin/env bash
# 版本号递增：fmg<YY><MM>.<NNN>，每修改一次递增 NNN，并同步到所有出现位置
set -euo pipefail
umask 022
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OLD="$(cat VERSION | tr -d "[:space:]")"
# 解析 fmg2609.001
YYMM="$(printf "%s" "$OLD" | sed -E "s/^fmg([0-9]{4})\..*/\1/")"
SEQ="$(printf "%s" "$OLD" | sed -E "s/^fmg[0-9]{4}\.([0-9]+)$/\1/")"
if [ -z "$YYMM" ] || [ -z "$SEQ" ]; then
  echo "VERSION 格式应为 fmg<YYMM>.<NNN>，当前: $OLD" >&2
  exit 1
fi
NEWSEQ="$(printf "%03d" $((10#$SEQ + 1)))"
NEW="fmg${YYMM}.${NEWSEQ}"

echo "版本: $OLD -> $NEW"

# 1) VERSION
printf "%s\n" "$NEW" > VERSION

# 2) fpk manifest
[ -f fnos/manifest ] && sed -i -E "s/^version[[:space:]]*=.*/version          = $NEW/" fnos/manifest

# 3) docker compose 镜像 tag
[ -f docker/docker-compose.yml ] && sed -i "s|ycyingchen/ycfmg:[A-Za-z0-9._-]*|ycyingchen/ycfmg:$NEW|g" docker/docker-compose.yml

# 4) 文档与下载页中的版本号
for f in README.md docs/*.md download-page/index.html download-page/version.json download-page/README.txt .tools/compose_cn.yml .tools/docker_block.html; do
  [ -f "$f" ] && sed -i "s|$OLD|$NEW|g" "$f"
done

# 5) version.json 的发布日期
if [ -f download-page/version.json ]; then
  TODAY="$(date +%Y-%m-%d)"
  sed -i "s|\"published_at\": \"[0-9-]*\"|\"published_at\": \"$TODAY\"|" download-page/version.json
fi

echo "已同步到: VERSION / fnos/manifest / docker-compose.yml / 文档 / 下载页"
grep -rn "$NEW" VERSION fnos/manifest docker/docker-compose.yml 2>/dev/null | head -5
echo ""
echo "提示：之后请执行 bash scripts/build.sh && bash scripts/build-fpk.sh && bash scripts/deploy-download.sh"