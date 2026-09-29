#!/usr/bin/env bash
# 发布下载页与安装包到目标目录（默认 /vol5/1000/空间4/YCFMG）
set -euo pipefail
umask 022
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
[ -f "$ROOT/scripts/deploy.env" ] && . "$ROOT/scripts/deploy.env"

VERSION="${VERSION:-$(cat "$ROOT/VERSION" | tr -d "[:space:]")}"
HOST="${PUB_HOST:-${FNOS_HOST:-192.168.1.8}}"
USER="${PUB_USER:-${FNOS_USER:-root}}"
PASS="${PUB_PASS:-${FNOS_PASS:-}}"
DEST="${PUB_DEST:-/vol5/1000/空间4/YCFMG}"
STAGE="$ROOT/.tools/download-stage"
FILES="$STAGE/files"

echo "== 发布下载页 $VERSION -> $USER@$HOST:$DEST =="

# 1) 准备发布目录
rm -rf "$STAGE"
mkdir -p "$FILES"
cp "$ROOT/download-page/index.html" "$STAGE/index.html"
cp "$ROOT/download-page/logo.svg" "$STAGE/logo.svg"
cp "$ROOT/download-page/README.txt" "$STAGE/README.txt" 2>/dev/null || true
[ -f "$ROOT/dist/YCFMG_$VERSION.fpk" ] && cp "$ROOT/dist/YCFMG_$VERSION.fpk" "$FILES/"
for f in "$ROOT/dist"/*.tar.gz "$ROOT/dist"/ycfmg-linux-amd64 "$ROOT/dist"/ycfmg-linux-arm64 \
         "$ROOT/dist"/ycfmg-windows-amd64.exe "$ROOT/dist"/ycfmg-darwin-amd64 "$ROOT/dist"/ycfmg-darwin-arm64; do
  [ -f "$f" ] && cp "$f" "$FILES/"
done

# 2) 注入文件大小与构建时间
python3 - "$STAGE/index.html" "$FILES" <<'PYEOF'
import os, sys, datetime
html_path, files_dir = sys.argv[1], sys.argv[2]
def human(n):
    units = ["B", "KB", "MB", "GB"]
    v = float(n)
    i = 0
    while v >= 1024 and i < len(units) - 1:
        v /= 1024.0
        i += 1
    return ("%.0f %s" if v >= 100 else "%.1f %s") % (v, units[i])
map_ = {
    "{{SIZE_FPK}}":             "YCFMG_fmg2609.001.fpk",
    "{{SIZE_LINUX_AMD64_TGZ}}": "ycfmg-fmg2609.001-linux-amd64.tar.gz",
    "{{SIZE_LINUX_ARM64_TGZ}}": "ycfmg-fmg2609.001-linux-arm64.tar.gz",
    "{{SIZE_LINUX_AMD64}}":     "ycfmg-linux-amd64",
    "{{SIZE_LINUX_ARM64}}":     "ycfmg-linux-arm64",
    "{{SIZE_WIN}}":             "ycfmg-windows-amd64.exe",
    "{{SIZE_DARWIN_AMD64}}":    "ycfmg-darwin-amd64",
    "{{SIZE_DARWIN_ARM64}}":    "ycfmg-darwin-arm64",
}
s = open(html_path, encoding="utf-8").read()
for token, fname in map_.items():
    p = os.path.join(files_dir, fname)
    s = s.replace(token, human(os.path.getsize(p)) if os.path.exists(p) else "—")
s = s.replace("{{BUILD_TIME}}", datetime.datetime.now().strftime("%Y-%m-%d %H:%M"))
open(html_path, "w", encoding="utf-8").write(s)
print("已注入文件大小")
PYEOF

# 3) 上传
SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=15"
printf "%s" "$PASS" > "$ROOT/.tools/.askpass.pw"
cat > "$ROOT/.tools/askpass.sh" <<'ASKEOF'
#!/bin/sh
cat "$(dirname "$0")/.askpass.pw"
ASKEOF
chmod 755 "$ROOT/.tools/askpass.sh"
chmod 600 "$ROOT/.tools/.askpass.pw"
export SSH_ASKPASS="$ROOT/.tools/askpass.sh" SSH_ASKPASS_REQUIRE=force

ssh_run() { setsid -w ssh $SSH_OPTS "$USER@$HOST" "$@"; }
scp_run() { setsid -w scp $SSH_OPTS "$@"; }

ssh_run "mkdir -p "$DEST/files""
scp_run "$STAGE/index.html" "$STAGE/logo.svg" "$STAGE/README.txt" "$USER@$HOST:$DEST/"
scp_run "$FILES"/* "$USER@$HOST:$DEST/files/"
# 规范化远端权限，保证后续可覆盖更新且可被 Web 服务读取
ssh_run "chmod -R u+rwX "$DEST" && find "$DEST" -type d -exec chmod 755 {} \; && find "$DEST" -type f -exec chmod 644 {} \;"

echo "== 发布完成 =="
echo "目标目录: $DEST"
echo "下载页:   http://$HOST:8686 之外，请在文件管理器中打开 $DEST/index.html"