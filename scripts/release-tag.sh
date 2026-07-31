#!/usr/bin/env bash
# 语义化 git tag / 发版工具。推送 v* tag 会触发 GitHub Actions Release。
#
# 用法：
#   ./scripts/release-tag.sh tag <version>
#   ./scripts/release-tag.sh release <version> [remote]
#   ./scripts/release-tag.sh bump [patch|minor|major] [remote]
#
# Make：
#   make tag VERSION=1.2.0
#   make release VERSION=1.2.0
#   make bump                     # 最新正式版 patch+1 并 push
#   make bump PART=minor
#   make release VERSION=1.2.0 FORCE=1
set -euo pipefail

# ---------------------------------------------------------------------------
# 日志
# ---------------------------------------------------------------------------

usage() {
  cat >&2 <<'EOF'
usage:
  scripts/release-tag.sh tag <version>
  scripts/release-tag.sh release <version> [remote]
  scripts/release-tag.sh bump [patch|minor|major] [remote]

version: MAJOR.MINOR.PATCH 或带预发布后缀（如 1.2.0-rc.1）；可带或不带 v 前缀
bump:    基于最新正式 tag（vX.Y.Z，忽略预发布）递增；默认 patch；结果走 release 流程

行为：
  - 本地 tag 已在 HEAD 且远程缺失 → 直接 push，不重打
  - 本地/远程 tag 指向其他 commit → 报错，提示 FORCE=1
  - FORCE=1 → 在 HEAD 重建 tag；release/bump 时必要时 --force 推送
EOF
  exit 1
}

die() {
  echo "error: $*" >&2
  exit 1
}

info() {
  echo "$*"
}

warn() {
  echo "warning: $*" >&2
}

# ---------------------------------------------------------------------------
# 版本
# ---------------------------------------------------------------------------

# 去掉可选 v 前缀，校验 semver（允许预发布后缀）
normalize_version() {
  local raw="$1" ver
  ver="${raw#v}"
  [[ "$ver" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]] \
    || die "invalid version '$raw' (expect MAJOR.MINOR.PATCH[-suffix])"
  printf '%s\n' "$ver"
}

version_to_tag() {
  printf 'v%s\n' "$1"
}

# 最新正式版 X.Y.Z（无预发布）。仓库无正式 tag 时回落 0.0.0
latest_release_version() {
  local t ver
  while IFS= read -r t; do
    [[ -z "$t" ]] && continue
    ver="${t#v}"
    if [[ "$ver" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      printf '%s\n' "$ver"
      return 0
    fi
  done < <(git tag -l 'v*.*.*' --sort=-v:refname)
  printf '0.0.0\n'
}

bump_semver() {
  local ver="$1" part="$2"
  local major minor patch
  IFS=. read -r major minor patch <<<"$ver"
  case "$part" in
    major) printf '%s\n' "$((major + 1)).0.0" ;;
    minor) printf '%s\n' "${major}.$((minor + 1)).0" ;;
    patch) printf '%s\n' "${major}.${minor}.$((patch + 1))" ;;
    *) die "invalid bump part '$part' (expect patch|minor|major)" ;;
  esac
}

# ---------------------------------------------------------------------------
# Git 状态
# ---------------------------------------------------------------------------

assert_git_repo() {
  git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "not a git repository"
}

# 仅检查已跟踪文件；未跟踪文件不阻断发版
assert_clean_worktree() {
  if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then
    git status --short --untracked-files=no >&2
    die "working tree dirty; commit or stash before tagging"
  fi
}

warn_if_not_main() {
  local branch
  branch="$(git rev-parse --abbrev-ref HEAD)"
  case "$branch" in
    main|master) ;;
    *) warn "tagging from branch '${branch}' (通常应在 main/master)" ;;
  esac
}

head_commit() {
  git rev-parse HEAD
}

short_sha() {
  git rev-parse --short "$1"
}

# 本地 tag 指向的 commit；不存在则空
local_tag_commit() {
  local tag="$1"
  if git rev-parse -q --verify "refs/tags/${tag}" >/dev/null; then
    git rev-parse "refs/tags/${tag}^{}"
  fi
}

# 远程 tag 指向的 commit（annotated 取 peeled）；不存在则空
remote_tag_commit() {
  local remote="$1" tag="$2" out peeled
  # 必须同时查 tag 与 ^{}，否则 annotated tag 只会返回 tag 对象 SHA
  out="$(git ls-remote "$remote" "refs/tags/${tag}" "refs/tags/${tag}^{}" 2>/dev/null || true)"
  [[ -z "$out" ]] && return 0
  peeled="$(printf '%s\n' "$out" | awk '$2 ~ /\^\{\}$/ { print $1; exit }')"
  if [[ -n "$peeled" ]]; then
    printf '%s\n' "$peeled"
  else
    printf '%s\n' "$out" | awk 'NF { print $1; exit }'
  fi
}

hint_force() {
  local tag="$1" remote="$2"
  cat >&2 <<EOF
hint: 确认要用当前 HEAD 覆盖该 tag 时：
  git tag -d ${tag}
  make release VERSION=${tag#v} FORCE=1 REMOTE=${remote}
EOF
}

die_tag_conflict() {
  local where="$1" tag="$2" existing="$3" head="$4" remote="$5"
  echo "error: ${where} tag ${tag} -> $(short_sha "$existing") (HEAD is $(short_sha "$head"))" >&2
  hint_force "$tag" "$remote"
  exit 1
}

# ---------------------------------------------------------------------------
# 执行原语
# ---------------------------------------------------------------------------

create_annotated_tag() {
  local tag="$1"
  git tag -a "$tag" -m "$tag"
  info "created ${tag} -> $(short_sha HEAD)"
}

delete_local_tag() {
  local tag="$1"
  git tag -d "$tag" >/dev/null
}

push_tag() {
  local remote="$1" tag="$2"
  info "pushing ${tag} to ${remote}..."
  git push "$remote" "$tag"
  info "pushed ${tag}; watch Actions for GitHub Release"
}

force_push_tag() {
  local remote="$1" tag="$2" was="$3"
  info "FORCE=1: force-pushing ${tag} to ${remote} (was $(short_sha "$was"))..."
  git push --force "$remote" "refs/tags/${tag}"
  info "pushed ${tag}; watch Actions for GitHub Release"
}

fetch_tag() {
  local remote="$1" tag="$2"
  info "${tag} already on ${remote} at $(short_sha HEAD); fetching local ref"
  git fetch "$remote" "refs/tags/${tag}:refs/tags/${tag}"
}

# ---------------------------------------------------------------------------
# 核心：确保 tag 落在 HEAD，可选推送
# ---------------------------------------------------------------------------

# do_push: 0|1  force: 0|1
ensure_tag() {
  local tag="$1" do_push="$2" force="$3" remote="$4"
  local head local_c remote_c

  head="$(head_commit)"
  local_c="$(local_tag_commit "$tag")"
  remote_c=""
  if [[ "$do_push" == "1" ]]; then
    remote_c="$(remote_tag_commit "$remote" "$tag")"
  fi

  if [[ "$force" == "1" ]]; then
    if [[ -n "$local_c" ]]; then
      info "FORCE=1: deleting local ${tag} ($(short_sha "$local_c"))"
      delete_local_tag "$tag"
    fi
    create_annotated_tag "$tag"
    if [[ "$do_push" == "1" ]]; then
      if [[ -n "$remote_c" && "$remote_c" != "$head" ]]; then
        force_push_tag "$remote" "$tag" "$remote_c"
      else
        push_tag "$remote" "$tag"
      fi
    fi
    return 0
  fi

  if [[ -n "$local_c" && "$local_c" != "$head" ]]; then
    die_tag_conflict "local" "$tag" "$local_c" "$head" "$remote"
  fi

  if [[ "$do_push" == "1" && -n "$remote_c" && "$remote_c" != "$head" ]]; then
    die_tag_conflict "remote ${remote}" "$tag" "$remote_c" "$head" "$remote"
  fi

  # 本地已在 HEAD：复用
  if [[ -n "$local_c" ]]; then
    info "reusing local ${tag} -> $(short_sha "$head")"
    if [[ "$do_push" == "1" ]]; then
      if [[ -n "$remote_c" ]]; then
        info "${tag} already on ${remote} at $(short_sha "$head"); nothing to push"
      else
        push_tag "$remote" "$tag"
      fi
    fi
    return 0
  fi

  # 本地无 tag，远程已在 HEAD：拉回本地
  if [[ "$do_push" == "1" && -n "$remote_c" && "$remote_c" == "$head" ]]; then
    fetch_tag "$remote" "$tag"
    return 0
  fi

  create_annotated_tag "$tag"
  if [[ "$do_push" == "1" ]]; then
    push_tag "$remote" "$tag"
  fi
}

prepare_repo() {
  assert_git_repo
  assert_clean_worktree
  warn_if_not_main
}

# ---------------------------------------------------------------------------
# 子命令
# ---------------------------------------------------------------------------

cmd_tag() {
  local ver tag
  ver="$(normalize_version "$1")"
  tag="$(version_to_tag "$ver")"
  prepare_repo
  ensure_tag "$tag" 0 "${FORCE:-0}" origin
}

cmd_release() {
  local ver tag remote
  ver="$(normalize_version "$1")"
  tag="$(version_to_tag "$ver")"
  remote="${2:-origin}"
  prepare_repo
  ensure_tag "$tag" 1 "${FORCE:-0}" "$remote"
}

cmd_bump() {
  local part remote base next ver tag
  part="${1:-patch}"
  remote="${2:-origin}"
  case "$part" in
    patch|minor|major) ;;
    *) die "invalid bump part '$part' (expect patch|minor|major)" ;;
  esac

  prepare_repo
  base="$(latest_release_version)"
  next="$(bump_semver "$base" "$part")"
  ver="$(normalize_version "$next")"
  tag="$(version_to_tag "$ver")"
  info "bump ${part}: v${base} -> ${tag}"
  ensure_tag "$tag" 1 "${FORCE:-0}" "$remote"
}

# ---------------------------------------------------------------------------
# 入口
# ---------------------------------------------------------------------------

main() {
  local cmd="${1:-}"
  shift || true

  case "$cmd" in
    tag)
      [[ $# -ge 1 ]] || usage
      cmd_tag "$@"
      ;;
    release)
      [[ $# -ge 1 ]] || usage
      cmd_release "$@"
      ;;
    bump)
      cmd_bump "$@"
      ;;
    -h|--help|"")
      usage
      ;;
    *)
      die "unknown command '$cmd'"
      ;;
  esac
}

main "$@"
