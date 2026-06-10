#!/usr/bin/env bash
set -euo pipefail

die() {
  printf 'opentunnel release: %s\n' "$1" >&2
  exit 1
}

validate_bump() {
  local bump="${1:-}"
  case "$bump" in
    "") printf 'auto\n' ;;
    patch|minor|major) printf '%s\n' "$bump" ;;
    *) die "bump must be patch, minor, major, or empty" ;;
  esac
}

bump_version() {
  local version="$1"
  local bump="$2"
  local major minor patch
  IFS=. read -r major minor patch <<< "$version"
  case "$bump" in
    patch) patch=$((patch + 1)) ;;
    minor) minor=$((minor + 1)); patch=0 ;;
    major) major=$((major + 1)); minor=0; patch=0 ;;
    *) die "unsupported bump $bump" ;;
  esac
  printf '%s.%s.%s\n' "$major" "$minor" "$patch"
}

detect_bump() {
  local log_text="$1"
  if printf '%s\n' "$log_text" | grep -Eq '(^|\n)BREAKING CHANGE:'; then
    printf 'major\n'
    return
  fi
  if printf '%s\n' "$log_text" | grep -Eq '^[a-zA-Z]+(\([^)]+\))?!:'; then
    printf 'major\n'
    return
  fi
  if printf '%s\n' "$log_text" | grep -Eq '^feat(\([^)]+\))?:'; then
    printf 'minor\n'
    return
  fi
  if printf '%s\n' "$log_text" | grep -Eq '^fix(\([^)]+\))?:'; then
    printf 'patch\n'
    return
  fi
  printf 'patch\n'
}

latest_semver_tag_from_list() {
  local tags="$1"
  local latest
  latest="$(printf '%s\n' "$tags" | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' | sort -t. -k1,1n -k2,2n -k3,3n | tail -n 1 || true)"
  if [ -z "$latest" ]; then
    printf '0.0.0\n'
    return
  fi
  printf '%s\n' "$latest"
}

latest_semver_tag() {
  latest_semver_tag_from_list "$(git tag --list)"
}

commit_log_since() {
  local latest_tag="$1"
  if [ "$latest_tag" = "0.0.0" ]; then
    git log --format='%s%n%b'
    return
  fi
  git log "${latest_tag}..HEAD" --format='%s%n%b'
}

ensure_clean_main() {
  local branch
  branch="$(git branch --show-current)"
  [ "$branch" = "main" ] || die "must run from main branch"
  git fetch origin main --tags
  [ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] || die "local main must match origin/main"
  [ -z "$(git status --porcelain)" ] || die "working tree must be clean, including untracked files"
}

ensure_release_tools() {
  command -v gh >/dev/null 2>&1 || die "gh is required"
  gh auth status >/dev/null 2>&1 || die "gh auth status failed"
}

ensure_dev_version() {
  local version
  version="$(tr -d '[:space:]' < VERSION)"
  [ "$version" = "dev" ] || die "VERSION must be dev before preparing a release"
}

run_verification() {
  go test ./... -count=1 &&
    go vet ./... &&
    go mod tidy -diff &&
    go test -race ./... -count=1 &&
    go build ./cmd/opentunnel &&
    rm -f ./opentunnel
}

main() {
  local requested_bump bump latest_tag log_text next_version release_commit
  requested_bump="$(validate_bump "${1:-}")"
  ensure_clean_main
  ensure_release_tools
  ensure_dev_version

  latest_tag="$(latest_semver_tag)"
  if [ "$requested_bump" = "auto" ]; then
    log_text="$(commit_log_since "$latest_tag")"
    bump="$(detect_bump "$log_text")"
  else
    bump="$requested_bump"
  fi
  next_version="$(bump_version "$latest_tag" "$bump")"

  printf '%s\n' "$next_version" > VERSION
  if ! run_verification; then
    printf 'dev\n' > VERSION
    die "verification failed"
  fi
  git add VERSION
  git commit -m "chore: release $next_version"
  release_commit="$(git rev-parse HEAD)"
  git tag "$next_version" "$release_commit"
  git push origin "$next_version"
  if ! gh release create "$next_version" --verify-tag --generate-notes --latest; then
    git push --delete origin "$next_version" >/dev/null 2>&1 || true
    git tag -d "$next_version" >/dev/null 2>&1 || true
    die "GitHub Release creation failed"
  fi
  printf 'dev\n' > VERSION
  git add VERSION
  git commit -m "chore: reopen development"
  git push origin main
  printf 'released %s\n' "$next_version"
}

if [[ "${BASH_SOURCE[0]}" = "$0" ]]; then
  main "${1:-}"
fi
