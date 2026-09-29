#!/usr/bin/env bash
# 发布下载页与 fpk 安装包到目标目录
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

echo "== 发布 $VERSION -> $USER@$HOST:$DEST =="

rm -rf "$STAGE"
mkdir -p "$FILES" "$STAGE/preview"
cp "$ROOT/download-page/index.html" "$STAGE/index.html"
cp "$ROOT/download-page/logo.svg" "$STAGE/logo.svg"
cp "$ROOT/download-page/README.txt" "$STAGE/README.txt" 2>/dev/null || true
cp "$ROOT/download-page/version.json" "$STAGE/version.json" 2>/dev/null || true
cp "$ROOT/download-page/preview/"*.png "$STAGE/preview/" 2>/dev/null || true
[ -f "$ROOT/dist/YCFMG_$VERSION.fpk" ] && cp "$ROOT/dist/YCFMG_$VERSION.fpk" "$FILES/"

python3 - "$STAGE/index.html" "$FILES" <<'PYEOF'
import os, sys, datetime
html_path, files_dir = sys.argv[1], sys.argv[2]
def human(n):
    units = ["B", "KB", "MB", "GB"]
    v = float(n); i = 0
    while v >= 1024 and i < len(units) - 1:
        v /= 1024.0; i += 1
    return ("%.0f %s" if v >= 100 else "%.1f %s") % (v, units[i])
s = open(html_path, encoding="utf-8").read()
fpk = os.path.join(files_dir, "YCFMG_%s.fpk" % os.environ.get("YCFMG_VER", ""))
cands = [f for f in os.listdir(files_dir) if f.endswith(".fpk")] if os.path.isdir(files_dir) else []
if cands:
    p = os.path.join(files_dir, cands[0])
    s = s.replace("{{SIZE_FPK}}", human(os.path.getsize(p)))
else:
    s = s.replace("{{SIZE_FPK}}", "—")
s = s.replace("{{BUILD_TIME}}", datetime.datetime.now().strftime("%Y-%m-%d %H:%M"))
open(html_path, "w", encoding="utf-8").write(s)
print("已注入文件大小")
PYEOF

SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=15"
printf "%s" "$PASS" > "$ROOT/.tools/.askpass.pw"
cat > "$ROOT/.tools/askpass.sh" <<'ASKEOF'
#!/bin/sh
cat "$(dirname "$0")/.askpass.pw"
ASKEOF
chmod 755 "$ROOT/.tools/askpass.sh"; chmod 600 "$ROOT/.tools/.askpass.pw"
export SSH_ASKPASS="$ROOT/.tools/askpass.sh" SSH_ASKPASS_REQUIRE=force
ssh_run() { setsid -w ssh $SSH_OPTS "$USER@$HOST" "$@"; }
scp_run() { setsid -w scp -r $SSH_OPTS "$@"; }

ssh_run "mkdir -p \"$DEST/files\" \"$DEST/preview\" && rm -f \"$DEST/files\"/*.fpk \"$DEST/files\"/*.tar.gz \"$DEST/files\"/*.exe"
scp_run "$STAGE/index.html" "$STAGE/logo.svg" "$STAGE/README.txt" "$STAGE/version.json" "$USER@$HOST:$DEST/"
scp_run "$STAGE/preview/." "$USER@$HOST:$DEST/preview/"
[ -n "$(ls -A "$FILES" 2>/dev/null)" ] && scp_run "$FILES/." "$USER@$HOST:$DEST/files/"
ssh_run "chmod -R u+rwX \"$DEST\" && find \"$DEST\" -type d -exec chmod 755 {} \; && find \"$DEST\" -type f -exec chmod 644 {} \;"

echo "== 完成: $DEST =="