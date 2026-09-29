#!/usr/bin/env bash
# 推送源码到 GitHub
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
[ -f "$ROOT/scripts/deploy.env" ] && . "$ROOT/scripts/deploy.env"

TOKEN="${GITHUB_TOKEN:-}"
REPO="${GITHUB_REPO:-ycyingchen/YCFMG}"
BRANCH="${GITHUB_BRANCH:-main}"

if [ -z "$TOKEN" ]; then
  echo "缺少 GITHUB_TOKEN，请在 scripts/deploy.env 中配置"; exit 1
fi

cd "$ROOT"
if [ ! -d .git ]; then
  git init -q
  git config user.email "ycyingchen@users.noreply.github.com"
  git config user.name "ycyingchen"
fi
git add -A
if git diff --cached --quiet; then
  echo "没有需要提交的变更"
else
  git commit -q -m "release: YCFMG $(cat VERSION)"
fi
git branch -M "$BRANCH"

REMOTE="https://x-access-token:$TOKEN@github.com/$REPO.git"
if git remote get-url origin >/dev/null 2>&1; then
  git remote set-url origin "$REMOTE"
else
  git remote add origin "$REMOTE"
fi
echo "== 推送到 $REPO ($BRANCH) =="
git push -u origin "$BRANCH" --force-with-lease || git push -u origin "$BRANCH" --force

# 打标签
if ! git rev-parse "v$(cat VERSION)" >/dev/null 2>&1; then
  git tag -a "v$(cat VERSION)" -m "YCFMG $(cat VERSION)"
  git push origin "v$(cat VERSION)"
fi
echo "== 推送完成 =="