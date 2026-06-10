#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/release.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

assert_eq() {
  local got="$1"
  local want="$2"
  local label="$3"
  if [ "$got" != "$want" ]; then
    fail "$label: got $got want $want"
  fi
}

assert_fail() {
  local label="$1"
  shift
  if ("$@") >/tmp/opentunnel-release-test.out 2>/tmp/opentunnel-release-test.err; then
    fail "$label: command succeeded unexpectedly"
  fi
}

assert_contains() {
  local got="$1"
  local want="$2"
  local label="$3"
  if [[ "$got" != *"$want"* ]]; then
    fail "$label: missing $want in $got"
  fi
}

assert_not_contains() {
  local got="$1"
  local want="$2"
  local label="$3"
  if [[ "$got" == *"$want"* ]]; then
    fail "$label: found unexpected $want in $got"
  fi
}

assert_file_eq() {
  local file="$1"
  local want="$2"
  local label="$3"
  local got
  got="$(tr -d '[:space:]' < "$file")"
  assert_eq "$got" "$want" "$label"
}

write_fake_command() {
  local path="$1"
  shift
  printf '%s\n' "$@" > "$path"
  chmod +x "$path"
}

setup_fake_repo() {
  local dir="$1"
  mkdir -p "$dir/bin" "$dir/state"
  printf 'dev\n' > "$dir/VERSION"
  printf '0.1.0\n' > "$dir/state/tags"
  printf 'feat: add release command\n' > "$dir/state/log"
  printf 'base-sha\n' > "$dir/state/head"
  : > "$dir/state/dirty"
  : > "$dir/state/events"

  write_fake_command "$dir/bin/git" \
    '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    'state="$FAKE_RELEASE_STATE"' \
    'events="$state/events"' \
    'case "$1" in' \
    '  branch)' \
    '    [ "${2:-}" = "--show-current" ] && { printf "main\n"; exit 0; }' \
    '    ;;' \
    '  fetch)' \
    '    printf "git fetch %s\n" "$*" >> "$events"' \
    '    exit 0' \
    '    ;;' \
    '  rev-parse)' \
    '    case "$2" in' \
    '      HEAD) cat "$state/head" ;;' \
    '      origin/main) printf "base-sha\n" ;;' \
    '      *) exit 1 ;;' \
    '    esac' \
    '    exit 0' \
    '    ;;' \
    '  status)' \
    '    cat "$state/dirty"' \
    '    exit 0' \
    '    ;;' \
  '  tag)' \
  '    [ "${2:-}" = "--list" ] && { cat "$state/tags"; exit 0; }' \
  '    if [ "${2:-}" = "-d" ]; then' \
  '      printf "git tag -d %s\n" "$3" >> "$events"' \
  '      exit 0' \
  '    fi' \
  '    printf "git tag %s %s\n" "$2" "$3" >> "$events"' \
  '    exit 0' \
  '    ;;' \
    '  log)' \
    '    cat "$state/log"' \
    '    exit 0' \
    '    ;;' \
    '  add)' \
    '    printf "git add %s version=%s\n" "$2" "$(tr -d "[:space:]" < VERSION)" >> "$events"' \
    '    exit 0' \
    '    ;;' \
    '  commit)' \
    '    msg=""' \
    '    if [ "${2:-}" = "-m" ]; then msg="$3"; fi' \
    '    version="$(tr -d "[:space:]" < VERSION)"' \
    '    case "$msg" in' \
    '      "chore: release "*) printf "release-sha\n" > "$state/head" ;;' \
    '      "chore: reopen development") printf "reopen-sha\n" > "$state/head" ;;' \
    '      *) exit 1 ;;' \
    '    esac' \
    '    printf "git commit %s version=%s head=%s\n" "$msg" "$version" "$(tr -d "[:space:]" < "$state/head")" >> "$events"' \
    '    exit 0' \
    '    ;;' \
  '  push)' \
  '    if [ "${2:-}" = "--delete" ]; then' \
  '      printf "git push --delete %s %s\n" "$3" "$4" >> "$events"' \
  '      exit 0' \
  '    fi' \
  '    printf "git push %s %s head=%s version=%s\n" "$2" "$3" "$(tr -d "[:space:]" < "$state/head")" "$(tr -d "[:space:]" < VERSION)" >> "$events"' \
  '    if [ "${FAKE_TAG_PUSH_FAIL:-}" = "1" ] && [ "$3" != "main" ]; then exit 1; fi' \
  '    exit 0' \
    '    ;;' \
    'esac' \
    'printf "unexpected git %s\n" "$*" >&2' \
    'exit 1'

  write_fake_command "$dir/bin/gh" \
    '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    'events="$FAKE_RELEASE_STATE/events"' \
    'if [ "$1" = "auth" ] && [ "$2" = "status" ]; then' \
    '  printf "gh auth status\n" >> "$events"' \
    '  exit 0' \
    'fi' \
  'if [ "$1" = "release" ] && [ "$2" = "create" ]; then' \
  '  shift 2' \
  '  printf "gh release create %s\n" "$*" >> "$events"' \
  '  if [ "${FAKE_GH_RELEASE_FAIL:-}" = "1" ]; then exit 1; fi' \
  '  exit 0' \
    'fi' \
    'printf "unexpected gh %s\n" "$*" >&2' \
    'exit 1'

  write_fake_command "$dir/bin/go" \
    '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    'printf "go %s\n" "$*" >> "$FAKE_RELEASE_STATE/events"' \
    'if [ "${FAKE_GO_FAIL:-}" = "1" ]; then exit 1; fi' \
    'exit 0'
}

run_fake_release() {
  local dir="$1"
  shift
  (
    cd "$dir"
    PATH="$dir/bin:$PATH" \
      FAKE_RELEASE_STATE="$dir/state" \
      FAKE_GO_FAIL="${FAKE_GO_FAIL:-}" \
      FAKE_TAG_PUSH_FAIL="${FAKE_TAG_PUSH_FAIL:-}" \
      FAKE_GH_RELEASE_FAIL="${FAKE_GH_RELEASE_FAIL:-}" \
      "$SCRIPT_DIR/release.sh" "$@"
  )
}

run_fake_release_go_fail() {
  FAKE_GO_FAIL=1 run_fake_release "$@"
}

run_fake_release_tag_push_fail() {
  FAKE_TAG_PUSH_FAIL=1 run_fake_release "$@"
}

run_fake_release_gh_fail() {
  FAKE_GH_RELEASE_FAIL=1 run_fake_release "$@"
}

assert_eq "$(bump_version 0.1.0 patch)" "0.1.1" "patch bump"
assert_eq "$(bump_version 0.1.0 minor)" "0.2.0" "minor bump"
assert_eq "$(bump_version 0.1.0 major)" "1.0.0" "major bump"

assert_eq "$(detect_bump $'docs: update readme')" "patch" "fallback bump"
assert_eq "$(detect_bump $'fix: repair relay')" "patch" "fix bump"
assert_eq "$(detect_bump $'feat: add relay image')" "minor" "feat bump"
assert_eq "$(detect_bump $'fix!: change invite format')" "major" "bang bump"
assert_eq "$(detect_bump $'chore: release\n\nBREAKING CHANGE: changed protocol')" "major" "breaking body bump"

assert_eq "$(validate_bump '')" "auto" "empty bump defaults to auto"
assert_eq "$(validate_bump patch)" "patch" "patch override"
assert_eq "$(validate_bump minor)" "minor" "minor override"
assert_eq "$(validate_bump major)" "major" "major override"
assert_fail "invalid bump" validate_bump banana

assert_eq "$(latest_semver_tag_from_list $'0.1.0\n0.2.0\n1.0.0')" "1.0.0" "latest semver tag"
assert_eq "$(latest_semver_tag_from_list $'v1.0.0\n0.3.0\nnot-a-version')" "0.3.0" "ignore v-prefixed and invalid tags"
assert_eq "$(latest_semver_tag_from_list '')" "0.0.0" "no tags initializes from zero"

dirty_dir="$(mktemp -d)"
setup_fake_repo "$dirty_dir"
printf ' M VERSION\n' > "$dirty_dir/state/dirty"
assert_fail "dirty tree preflight" run_fake_release "$dirty_dir" minor
assert_file_eq "$dirty_dir/VERSION" "dev" "dirty preflight leaves VERSION unchanged"
dirty_events="$(cat "$dirty_dir/state/events")"
assert_not_contains "$dirty_events" "git commit" "dirty preflight does not commit"
assert_not_contains "$dirty_events" "git push" "dirty preflight does not push"
assert_not_contains "$dirty_events" "git tag" "dirty preflight does not tag"
assert_not_contains "$dirty_events" "gh release create" "dirty preflight does not create release"

success_dir="$(mktemp -d)"
setup_fake_repo "$success_dir"
run_fake_release "$success_dir" minor >/tmp/opentunnel-release-test.out 2>/tmp/opentunnel-release-test.err
assert_file_eq "$success_dir/VERSION" "dev" "successful release reopens VERSION"
success_events="$(cat "$success_dir/state/events")"
assert_contains "$success_events" "git commit chore: release 0.2.0 version=0.2.0 head=release-sha" "successful release commit"
assert_contains "$success_events" "git tag 0.2.0 release-sha" "successful release creates local tag"
assert_contains "$success_events" "git push origin 0.2.0 head=release-sha version=0.2.0" "successful release pushes tag before main"
assert_contains "$success_events" "gh release create 0.2.0 --verify-tag --generate-notes --latest" "successful release uses pre-pushed tag"
assert_contains "$success_events" "git commit chore: reopen development version=dev head=reopen-sha" "successful reopen commit"
assert_contains "$success_events" "git push origin main head=reopen-sha version=dev" "successful release pushes once after reopen"
assert_not_contains "$success_events" "git push origin main head=release-sha version=0.2.0" "successful release does not push before reopen"
assert_not_contains "$success_events" "--target" "successful release does not ask gh to create the tag"

verify_fail_dir="$(mktemp -d)"
setup_fake_repo "$verify_fail_dir"
assert_fail "verification failure" run_fake_release_go_fail "$verify_fail_dir" minor
assert_file_eq "$verify_fail_dir/VERSION" "dev" "verification failure restores VERSION"
verify_fail_events="$(cat "$verify_fail_dir/state/events")"
assert_not_contains "$verify_fail_events" "git commit" "verification failure does not commit"
assert_not_contains "$verify_fail_events" "git push" "verification failure does not push"
assert_not_contains "$verify_fail_events" "git tag" "verification failure does not tag"
assert_not_contains "$verify_fail_events" "gh release create" "verification failure does not create release"

tag_push_fail_dir="$(mktemp -d)"
setup_fake_repo "$tag_push_fail_dir"
assert_fail "tag push failure" run_fake_release_tag_push_fail "$tag_push_fail_dir" minor
tag_push_fail_events="$(cat "$tag_push_fail_dir/state/events")"
assert_contains "$tag_push_fail_events" "git tag 0.2.0 release-sha" "tag push failure creates local tag before push"
assert_contains "$tag_push_fail_events" "git push origin 0.2.0 head=release-sha version=0.2.0" "tag push failure attempts tag push"
assert_not_contains "$tag_push_fail_events" "gh release create" "tag push failure does not create release"
assert_not_contains "$tag_push_fail_events" "chore: reopen development" "tag push failure does not reopen development"
assert_not_contains "$tag_push_fail_events" "git push origin main" "tag push failure does not push main"

gh_fail_dir="$(mktemp -d)"
setup_fake_repo "$gh_fail_dir"
assert_fail "github release failure" run_fake_release_gh_fail "$gh_fail_dir" minor
gh_fail_events="$(cat "$gh_fail_dir/state/events")"
assert_contains "$gh_fail_events" "git push origin 0.2.0 head=release-sha version=0.2.0" "github release failure pushed tag"
assert_contains "$gh_fail_events" "gh release create 0.2.0 --verify-tag --generate-notes --latest" "github release failure attempted release"
assert_contains "$gh_fail_events" "git push --delete origin 0.2.0" "github release failure deletes remote tag"
assert_contains "$gh_fail_events" "git tag -d 0.2.0" "github release failure deletes local tag"
assert_not_contains "$gh_fail_events" "chore: reopen development" "github release failure does not reopen development"
assert_not_contains "$gh_fail_events" "git push origin main" "github release failure does not push main"

rm -f /tmp/opentunnel-release-test.out /tmp/opentunnel-release-test.err
printf 'release script tests passed\n'
