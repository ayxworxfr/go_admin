#!/usr/bin/env bash
# 创建 / 推送语义化 git tag，供 GitHub Actions 发 Release。
#
# 用法：
#   ./scripts/release-tag.sh tag 1.2.0
#   ./scripts/release-tag.sh release 1.2.0 [remote]
#   ./scripts/release-tag.sh release 1.2.0-rc.1
#
# 也可：make tag VERSION=1.2.0 / make release VERSION=1.2.0
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
usage: scripts/release-tag.sh tag <version>
       scripts/release-tag.sh release <version> [remote]

version: MAJOR.MINOR.PATCH 或带预发布后缀（如 1.2.0-rc.1）；可带或不带 v 前缀
EOF
  exit 1
}

die() {
  echo "error: $*" >&2
  exit 1
}

cmd="${1:-}"
raw="${2:-}"
remote="${3:-origin}"

[[ -n "$cmd" && -n "$raw" ]] || usage
[[ "$cmd" == "tag" || "$cmd" == "release" ]] || usage

ver="${raw#v}"
[[ "$ver" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]] \
  || die "invalid version '$raw' (expect MAJOR.MINOR.PATCH[-suffix])"

tag="v${ver}"

git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "not a git repository"

if [[ -n "$(git status --porcelain)" ]]; then
  git status --short >&2
  die "working tree dirty; commit or stash before tagging"
fi

if git rev-parse -q --verify "refs/tags/${tag}" >/dev/null; then
  die "tag ${tag} already exists"
fi

branch="$(git rev-parse --abbrev-ref HEAD)"
case "$branch" in
  main|master) ;;
  *) echo "warning: tagging from branch '${branch}' (通常应在 main/master)" >&2 ;;
esac

git tag -a "$tag" -m "$tag"
echo "created ${tag} -> $(git rev-parse --short HEAD)"

if [[ "$cmd" == "release" ]]; then
  echo "pushing ${tag} to ${remote}..."
  git push "$remote" "$tag"
  echo "pushed ${tag}; watch Actions for GitHub Release"
fi
