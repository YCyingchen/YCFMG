#!/usr/bin/env bash
# 构建并推送多架构 Docker 镜像
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
[ -f "$ROOT/scripts/deploy.env" ] && . "$ROOT/scripts/deploy.env"

VERSION="${VERSION:-$(cat "$ROOT/VERSION" | tr -d "[:space:]")}"
USER="${DOCKER_USER:-ycyingchen}"
IMAGE="${DOCKER_IMAGE:-ycfmg}"
PLATFORMS="${DOCKER_PLATFORMS:-linux/amd64,linux/arm64}"

if [ -n "${DOCKER_PASS:-}" ]; then
  echo "$DOCKER_PASS" | docker login -u "$USER" --password-stdin
fi

cd "$ROOT"
docker buildx inspect ycfmg-builder >/dev/null 2>&1 || docker buildx create --name ycfmg-builder --use
docker buildx use ycfmg-builder

echo "== 构建并推送 $USER/$IMAGE:$VERSION =="
docker buildx build \
  --platform "$PLATFORMS" \
  -f docker/Dockerfile \
  --build-arg "VERSION=$VERSION" \
  --build-arg "COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo dev)" \
  -t "$USER/$IMAGE:$VERSION" \
  -t "$USER/$IMAGE:latest" \
  --push .

echo "== 完成，镜像: $USER/$IMAGE:$VERSION =="