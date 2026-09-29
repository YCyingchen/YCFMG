#!/bin/sh
set -e
mkdir -p /config /data
if [ ! -f /config/config.yaml ]; then
  echo "[YCFMG] 生成默认配置 /config/config.yaml"
  cat > /config/config.yaml <<'CFG'
server:
  host: 0.0.0.0
  port: 8686
  base_path: ""
  trust_proxy: true
  public_urls: []
data_dir: /config/ycfmg
libraries:
  - name: 数据
    path: /data
    readonly: false
    index: true
auth:
  username: admin
  password: ""
  session_ttl_hours: 1728
index:
  enabled: true
  scan_interval_minutes: 360
  thumb_sizes: [256, 768]
  auto_tags: true
  hash_tolerance: 6
share:
  default_expire_days: 7
  allow_download: true
  brand_name: YCFMG
  brand_subtitle: 文件管理 · 智能图库
semantic:
  enabled: false
  endpoint: ""
  api_key: ""
log_level: info
CFG
  if [ -n "$YCFMG_PUBLIC_URLS" ]; then
    printf 'server:
' > /tmp/override.yaml
  fi
fi
exec ycfmg --config /config/config.yaml
