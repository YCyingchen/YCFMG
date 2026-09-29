#!/usr/bin/env bash
# 通过 SSH 部署到飞牛 NAS / 任意 Linux 服务器
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
[ -f "$ROOT/scripts/deploy.env" ] && . "$ROOT/scripts/deploy.env"

VERSION="${VERSION:-$(cat "$ROOT/VERSION" | tr -d "[:space:]")}"
HOST="${FNOS_HOST:-192.168.1.8}"
USER="${FNOS_USER:-root}"
PASS="${FNOS_PASS:-}"
DEST="${FNOS_DEST:-/opt/ycfmg}"
PORT="${FNOS_PORT:-8686}"

ssh_run() {
  if [ -n "$PASS" ]; then
    sshpass -p "$PASS" ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "$USER@$HOST" "$@"
  else
    ssh -o StrictHostKeyChecking=no "$USER@$HOST" "$@"
  fi
}
ssh_put() {
  if [ -n "$PASS" ]; then
    sshpass -p "$PASS" scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "$1" "$USER@$HOST:$2"
  else
    scp -o StrictHostKeyChecking=no "$1" "$USER@$HOST:$2"
  fi
}

BIN="$ROOT/dist/ycfmg-linux-amd64"
if [ ! -f "$BIN" ]; then
  echo "-- 未找到二进制，先构建"
  "$ROOT/scripts/build.sh"
fi

echo "== 部署到 $USER@$HOST:$DEST =="
ssh_run "mkdir -p $DEST/bin $DEST/etc $DEST/data && systemctl stop ycfmg 2>/dev/null || true"
ssh_put "$BIN" "$DEST/bin/ycfmg"
ssh_put "$ROOT/configs/config.example.yaml" "$DEST/etc/config.example.yaml"
ssh_run "chmod +x $DEST/bin/ycfmg && [ -f $DEST/etc/config.yaml ] || cp $DEST/etc/config.example.yaml $DEST/etc/config.yaml"
ssh_run "cat > /etc/systemd/system/ycfmg.service <<UNIT
[Unit]
Description=YCFMG File Manager and Gallery
After=network.target

[Service]
Type=simple
ExecStart=$DEST/bin/ycfmg --config $DEST/etc/config.yaml --data $DEST/data
Restart=always
RestartSec=5
WorkingDirectory=$DEST
Environment=TZ=Asia/Shanghai

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload && systemctl enable --now ycfmg && sleep 2 && systemctl is-active ycfmg"

echo "== 部署完成，访问 http://$HOST:$PORT =="